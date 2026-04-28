package server

import (
	"net/http"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/steinfletcher/apitest"
	account_manager "github.com/teranode-group/proto/account-manager"
	"github.com/teranode-group/woc-api/apikeys"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ---- Test Helpers ----

type middlewareConfig struct {
	offlineMode            bool
	apiKeyCheckEnabled     bool
	apiKeyRateLimitEnabled bool
	isMainnet              bool
}

// globalPatches holds the current patches to ensure proper cleanup between tests
var globalPatches *gomonkey.Patches

func setupMiddlewareTest(t *testing.T, cfg middlewareConfig) func() {
	t.Helper()

	// Reset any lingering patches from previous tests
	if globalPatches != nil {
		globalPatches.Reset()
		globalPatches = nil
	}

	oldOfflineMode := offlineMode
	oldApiKeyCheck := apiKeyCheckEnabled
	oldRateLimit := apiKeyRateLimitEnabled
	oldIsMainnet := isMainnet

	offlineMode = cfg.offlineMode
	apiKeyCheckEnabled = cfg.apiKeyCheckEnabled
	apiKeyRateLimitEnabled = cfg.apiKeyRateLimitEnabled
	isMainnet = cfg.isMainnet

	return func() {
		offlineMode = oldOfflineMode
		apiKeyCheckEnabled = oldApiKeyCheck
		apiKeyRateLimitEnabled = oldRateLimit
		isMainnet = oldIsMainnet
	}
}

func createPatches(t *testing.T) *gomonkey.Patches {
	t.Helper()
	globalPatches = gomonkey.NewPatches()
	t.Cleanup(func() {
		if globalPatches != nil {
			globalPatches.Reset()
			globalPatches = nil
		}
	})
	return globalPatches
}

func successHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`"success"`))
	})
}

func validAPIKey(network string) *account_manager.APIKey {
	return &account_manager.APIKey{
		ApikeyId:        1,
		AccountId:       100,
		IsAccountActive: true,
		Network:         network,
		WocAccess:       true,
		WocRateLimit:    1000,
		WocDailyRateLimit: 100000,
	}
}

// ---- Tests ----

func TestMiddleware_OfflineMode_ValidKey(t *testing.T) {
	cleanup := setupMiddlewareTest(t, middlewareConfig{
		offlineMode:            true,
		apiKeyCheckEnabled:     true,
		apiKeyRateLimitEnabled: false,
		isMainnet:              true,
	})
	t.Cleanup(cleanup)

	patches := createPatches(t)
	patches.ApplyFunc(apikeys.GetFromCache, func(key string) *account_manager.APIKey {
		return validAPIKey("mainnet")
	})

	apitest.New().
		Handler(TaalAPIKeyMiddleware(successHandler())).
		Get("/test").
		Header("Authorization", "Bearer mainnet_test-key").
		Expect(t).
		Status(http.StatusOK).
		Body(`"success"`).
		End()
}

func TestMiddleware_OfflineMode_InvalidKey(t *testing.T) {
	cleanup := setupMiddlewareTest(t, middlewareConfig{
		offlineMode:            true,
		apiKeyCheckEnabled:     true,
		apiKeyRateLimitEnabled: false,
		isMainnet:              true,
	})
	t.Cleanup(cleanup)

	patches := createPatches(t)
	patches.ApplyFunc(apikeys.GetFromCache, func(key string) *account_manager.APIKey {
		return nil
	})

	apitest.New().
		Handler(TaalAPIKeyMiddleware(successHandler())).
		Get("/test").
		Header("Authorization", "Bearer unknown-key").
		Expect(t).
		Status(http.StatusUnauthorized).
		Body(`"Invalid APIKey"`).
		End()
}

func TestMiddleware_NormalMode_ValidKey(t *testing.T) {
	cleanup := setupMiddlewareTest(t, middlewareConfig{
		offlineMode:            false,
		apiKeyCheckEnabled:     true,
		apiKeyRateLimitEnabled: false,
		isMainnet:              true,
	})
	t.Cleanup(cleanup)

	patches := createPatches(t)
	patches.ApplyFunc(apikeys.Get, func(key string) *account_manager.APIKey {
		return validAPIKey("mainnet")
	})

	apitest.New().
		Handler(TaalAPIKeyMiddleware(successHandler())).
		Get("/test").
		Header("Authorization", "Bearer mainnet_test-key").
		Expect(t).
		Status(http.StatusOK).
		Body(`"success"`).
		End()
}

func TestMiddleware_AccountNotActive(t *testing.T) {
	cleanup := setupMiddlewareTest(t, middlewareConfig{
		offlineMode:            true,
		apiKeyCheckEnabled:     true,
		apiKeyRateLimitEnabled: false,
		isMainnet:              true,
	})
	t.Cleanup(cleanup)

	patches := createPatches(t)
	patches.ApplyFunc(apikeys.GetFromCache, func(key string) *account_manager.APIKey {
		return &account_manager.APIKey{
			ApikeyId:        1,
			AccountId:       100,
			IsAccountActive: false,
			Network:         "mainnet",
		}
	})

	apitest.New().
		Handler(TaalAPIKeyMiddleware(successHandler())).
		Get("/test").
		Header("Authorization", "Bearer mainnet_test-key").
		Expect(t).
		Status(http.StatusUnauthorized).
		Body(`"Account not active"`).
		End()
}

func TestMiddleware_Revoked(t *testing.T) {
	cleanup := setupMiddlewareTest(t, middlewareConfig{
		offlineMode:            true,
		apiKeyCheckEnabled:     true,
		apiKeyRateLimitEnabled: false,
		isMainnet:              true,
	})
	t.Cleanup(cleanup)

	revokedTime := timestamppb.Now()
	patches := createPatches(t)
	patches.ApplyFunc(apikeys.GetFromCache, func(key string) *account_manager.APIKey {
		return &account_manager.APIKey{
			ApikeyId:        1,
			AccountId:       100,
			IsAccountActive: true,
			Network:         "mainnet",
			Revoked:         revokedTime,
		}
	})

	apitest.New().
		Handler(TaalAPIKeyMiddleware(successHandler())).
		Get("/test").
		Header("Authorization", "Bearer mainnet_test-key").
		Expect(t).
		Status(http.StatusUnauthorized).
		Body(`"APIKey not active (revoked)"`).
		End()
}

func TestMiddleware_Expired(t *testing.T) {
	cleanup := setupMiddlewareTest(t, middlewareConfig{
		offlineMode:            false,
		apiKeyCheckEnabled:     true,
		apiKeyRateLimitEnabled: false,
		isMainnet:              true,
	})
	t.Cleanup(cleanup)

	expiredTime := timestamppb.New(time.Now().Add(-24 * time.Hour))
	patches := createPatches(t)
	patches.ApplyFunc(apikeys.Get, func(key string) *account_manager.APIKey {
		return &account_manager.APIKey{
			ApikeyId:        1,
			AccountId:       100,
			IsAccountActive: true,
			Network:         "mainnet",
			Expiry:          expiredTime,
		}
	})

	apitest.New().
		Handler(TaalAPIKeyMiddleware(successHandler())).
		Get("/test").
		Header("Authorization", "Bearer mainnet_test-key").
		Expect(t).
		Status(http.StatusUnauthorized).
		Body(`"Account not active (expired)"`).
		End()
}

// In offlineMode, StartAPIKeysFromCache nils out Expiry only on keys whose expiry
// is still in the future, so the middleware's time-based expiry check short-circuits
// and the request is allowed.
func TestMiddleware_OfflineMode_ExpiryCleared(t *testing.T) {
	cleanup := setupMiddlewareTest(t, middlewareConfig{
		offlineMode:            true,
		apiKeyCheckEnabled:     true,
		apiKeyRateLimitEnabled: false,
		isMainnet:              true,
	})
	t.Cleanup(cleanup)

	patches := createPatches(t)
	patches.ApplyFunc(apikeys.GetFromCache, func(key string) *account_manager.APIKey {
		return &account_manager.APIKey{
			ApikeyId:        1,
			AccountId:       100,
			IsAccountActive: true,
			Network:         "mainnet",
			Expiry:          nil,
		}
	})

	apitest.New().
		Handler(TaalAPIKeyMiddleware(successHandler())).
		Get("/test").
		Header("Authorization", "Bearer mainnet_test-key").
		Expect(t).
		Status(http.StatusOK).
		Body(`"success"`).
		End()
}

// In offlineMode, keys whose expiry was already in the past at load time keep
// their original Expiry timestamp, so the middleware continues to reject them.
func TestMiddleware_OfflineMode_PastExpiryStillRejected(t *testing.T) {
	cleanup := setupMiddlewareTest(t, middlewareConfig{
		offlineMode:            true,
		apiKeyCheckEnabled:     true,
		apiKeyRateLimitEnabled: false,
		isMainnet:              true,
	})
	t.Cleanup(cleanup)

	expiredTime := timestamppb.New(time.Now().Add(-24 * time.Hour))
	patches := createPatches(t)
	patches.ApplyFunc(apikeys.GetFromCache, func(key string) *account_manager.APIKey {
		return &account_manager.APIKey{
			ApikeyId:        1,
			AccountId:       100,
			IsAccountActive: true,
			Network:         "mainnet",
			Expiry:          expiredTime,
		}
	})

	apitest.New().
		Handler(TaalAPIKeyMiddleware(successHandler())).
		Get("/test").
		Header("Authorization", "Bearer mainnet_test-key").
		Expect(t).
		Status(http.StatusUnauthorized).
		Body(`"Account not active (expired)"`).
		End()
}

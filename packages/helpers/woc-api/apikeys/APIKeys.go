package apikeys

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ordishs/gocore"
	"github.com/teranode-group/common"
	account_manager "github.com/teranode-group/proto/account-manager"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	mu         sync.RWMutex
	keys       = make(map[string]*account_manager.APIKey)
	filepath   string
	amConnOnce sync.Once
	amConn     *grpc.ClientConn
	amConnErr  error
	logger     = gocore.Log("woc-api")
)

func init() {
	var ok bool
	filepath, ok = gocore.Config().Get("apiKeyCacheDestination")
	if !ok {
		logger.Panicf("Cannot continue: could not find setting apiKeyCacheDestination\n")
	}
}

func getAccountManagerConn(ctx context.Context) (*grpc.ClientConn, error) {
	amConnOnce.Do(func() {
		m := make(map[string]string)
		m["is_admin"] = "true"

		amConn, amConnErr = common.GetGRPCConnection(
			ctx,
			"accountManager",
			m)
	})
	return amConn, amConnErr
}

func set(keysArray *account_manager.APIKeysArray) {
	mu.Lock()
	defer mu.Unlock()

	// Delete all the keys
	for k := range keys {
		delete(keys, k)
	}

	// Add in the new ones...
	var sb strings.Builder

	for _, k := range keysArray.Keys {
		sb.Reset()
		sb.WriteString(k.Network)
		sb.WriteByte('_')
		sb.WriteString(k.Value)

		keys[sb.String()] = k
	}
}

func isValid() bool {
	mu.RLock()
	defer mu.RUnlock()

	return len(keys) > 0
}

func Get(key string) *account_manager.APIKey {
	return get(key)
}

// func GetWithDoubleCheck(key string) *account_manager.APIKey {
// 	return get(key, true)
// }

func get(key string) *account_manager.APIKey {

	mu.RLock()

	details, found := keys[key]

	mu.RUnlock()

	if found {
		return details
	} else {
		// If the apikey is not found in the local cache, it might exist in the system, but has not yet
		// been streamed to this server.
		// We will quickly ask the account manager if it knows about this key.
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
		defer cancel()

		apiKey, err := getDetails(ctx, key)
		if err != nil {
			return nil
		}

		return apiKey
	}
}

// StartAPIKeysListener will keep looking for new API Keys in the accountManager
// and write the api keys into a local file for in case account manager is down
func StartAPIKeysListener() {
	// The very first thing we will do is read any saved APIKeys
	// from disk. This will be replaced by the first successful APIKeys
	// that we receive from accountManager stream.
	j, e := os.ReadFile(filepath)
	if e != nil {
		logger.Warnf("Did not load '%s', %v\n", filepath, e)
	} else {
		_ = json.Unmarshal(j, &keys)
	}

	logger.Info("Starting Periodic APIKey Cache Updates")

	go startAPIKeysStream()
}

// StartAPIKeysFromCache loads API keys from the cache file only (no gRPC connection).
// Used in offlineMode when account-manager is not available.
func StartAPIKeysFromCache() {
	j, e := os.ReadFile(filepath)
	if e != nil {
		logger.Fatalf("offlineMode: Could not load '%s', %v\n", filepath, e)
	}
	if err := json.Unmarshal(j, &keys); err != nil {
		logger.Fatalf("offlineMode: Could not parse '%s', %v\n", filepath, err)
	}
	if gocore.Config().GetBool("offlineMode", false) {
		// account-manager is being retired. Without refreshes from its stream, keys
		// with a future Expiry would eventually tip into the past and the middleware
		// would start rejecting valid customers. Null Expiry on future-dated keys so
		// they don't age out. Past-dated expiries are left intact so already-lapsed
		// keys (active at the account level but not renewed) stay rejected.
		// TODO: remove this block once account-manager's replacement is in place.
		now := time.Now()
		cleared := 0
		for _, k := range keys {
			if k.Expiry != nil && k.Expiry.AsTime().After(now) {
				k.Expiry = nil
				cleared++
			}
		}
		logger.Infof("offlineMode: Loaded %d API keys from cache (expiry cleared on %d future-dated keys)", len(keys), cleared)
	} else {
		logger.Infof("Loaded %d API keys from cache", len(keys))
	}
}

// GetFromCache returns an API key from the local cache only (no gRPC fallback).
// Used in offlineMode when account-manager is not available.
func GetFromCache(key string) *account_manager.APIKey {
	mu.RLock()
	defer mu.RUnlock()
	return keys[key]
}

func startAPIKeysStream() {

	//resync every 10 sec with account-manager
	ticker := time.NewTicker(10 * time.Second)
	isBusy := false
	skipCounter := 0
	maxSkipCounter := 5

	for ; true; <-ticker.C {

		//skip for 1 min if failed last time.
		if isBusy && skipCounter <= maxSkipCounter {
			logger.Warnf("Skipping APIKeysStream update from account-manager. isBusy:%+v, skipCounter:%+v/%+v\n", isBusy, skipCounter, maxSkipCounter)
			skipCounter++
			continue
		}

		isBusy = true
		skipCounter = 0

		conn, err := getAccountManagerConn(context.Background())
		if err != nil {
			// Cannot connect, panic if we don't have any settings...
			if !isValid() {
				logger.Fatalf("Could not connect to accountManager; no last known APIKeys found: %s", err)
			} else {
				logger.Warnf("Could not connect to accountManager; defaulting to last known APIKeys: %s", err)
				continue
			}
		}

		c := account_manager.NewAccountManagerClient(conn)

		stream, err := c.APIKeysStream(context.Background(), &emptypb.Empty{})
		if err != nil {
			// Cannot connect, panic if we don't have any settings...
			if !isValid() {
				logger.Fatalf("startAPIKeysStream: Could not read APIKeys from AccountManager; no last known APIKeys found: %s", err)
			} else {
				logger.Warnf("startAPIKeysStream: Could not read APIKeys from AccountManager; defaulting to last known APIKeys: %s", err)
			}
			continue
		} else {
			for {
				c, err := stream.Recv()
				if err != nil {
					logger.Warnf("startAPIKeysStream: Could not get stream: %v", err)
					break
				}

				if c != nil && c.Keys != nil && len(c.Keys) > 0 {
					set(c)
				} else {
					logger.Warnf("startAPIKeysStream: Received empty APIKeys from accountManager\n")
					continue
				}

				jsonData, err := json.MarshalIndent(&keys, "", "  ")
				if err != nil {
					logger.Warnf("startAPIKeysStream: Could not unmarshal APIKeys: %v", err)
					break
				}

				if err := os.WriteFile(filepath, jsonData, 0666); err != nil {
					logger.Errorf("startAPIKeysStream: Could not write APIKeys: %v", err)
					break
				}
			}
		}
		isBusy = false
	}
}

func getDetails(ctx context.Context, apiKey string) (*account_manager.APIKey, error) {
	m := make(map[string]string)
	m["is_admin"] = "true"

	conn, err := getAccountManagerConn(ctx)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	client := account_manager.NewAccountManagerClient(conn)

	amReq := &account_manager.GetAPIKeyDetailsRequest{ApiKey: apiKey}

	amRes, err := client.GetAPIKeyDetails(ctx, amReq)
	if err != nil {
		key := apiKey
		if len(apiKey) > 11 {
			key = apiKey[0:11] + "..." + apiKey[len(apiKey)-3:]
		}
		logger.Warnf("getDetails: Error from accountManager for APIkey %s, %v", key, err)
		return nil, err
	}

	return amRes, nil
}

func Close() {
	if amConn != nil {
		if err := amConn.Close(); err != nil {
			logger.Warnf("failed to close account-manager connection: %v", err)
		}
	}
}

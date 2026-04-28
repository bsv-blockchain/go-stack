package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/gorilla/mux"
	"github.com/steinfletcher/apitest"
	"github.com/stretchr/testify/require"

	gobitcoin "github.com/ordishs/go-bitcoin"
	"github.com/teranode-group/woc-api/bitcoin"
	"github.com/teranode-group/woc-api/internal"
)

func TestMerkleProofTsc(t *testing.T) {
	t.Parallel()

	type scenario struct {
		name   string
		setup  func(t *testing.T) *callTracker
		path   string
		expect string
		assert func(t *testing.T, tracker *callTracker)
	}

	scenarios := []scenario{
		{
			name: "service_passthrough",
			setup: func(t *testing.T) *callTracker {
				cleanup := mockMerkleService(t, []map[string]interface{}{{
					"index": 0,
					"nodes": []string{"aaaaaaaaaaaaaaaaaaaaaaaa"},
				}})
				t.Cleanup(cleanup)
				return nil
			},
			path:   "/tx/txid-array/proof/tsc",
			expect: `[{"index":0,"nodes":["aaaaaaaaaaaaaaaaaaaaaaaa"]}]`,
		},
		{
			name: "service_object_response",
			setup: func(t *testing.T) *callTracker {
				cleanup := mockMerkleService(t, map[string]interface{}{
					"index": 1,
					"nodes": []string{"bbbbbbbbbbbbbbbbbbbbbbbb"},
				})
				t.Cleanup(cleanup)
				return nil
			},
			path:   "/tx/txid-object/proof/tsc",
			expect: `[{"index":1,"nodes":["bbbbbbbbbbbbbbbbbbbbbbbb"]}]`,
		},
		{
			name: "node_returns_proof",
			setup: func(t *testing.T) *callTracker {
				cleanup := disableMerkleService()
				t.Cleanup(cleanup)
				return stubNodeSuccess(t, &gobitcoin.MerkleProof{Index: 5})
			},
			path:   "/tx/success/proof/tsc",
			expect: `[{"index":5,"txOrId":"","target":"","nodes":null}]`,
			assert: func(t *testing.T, tracker *callTracker) {
				require.NotNil(t, tracker)
				require.True(t, *tracker.bitcoin, "expected bitcoin GetMerkleProof to be invoked")
			},
		},
		{
			name: "node_missing_tx_fallback",
			setup: func(t *testing.T) *callTracker {
				cleanup := disableMerkleService()
				t.Cleanup(cleanup)
				return stubNodeError(t, "Transaction(s) not found in provided block")
			},
			path:   "/tx/missing/proof/tsc",
			expect: `[{"index":1,"txOrId":"missing","target":"blockhash","nodes":null}]`,
			assert: func(t *testing.T, tracker *callTracker) {
				require.NotNil(t, tracker)
				require.True(t, *tracker.bitcoin, "expected bitcoin GetMerkleProof to be invoked")
			},
		},
		{
			name: "node_generic_error_fallback",
			setup: func(t *testing.T) *callTracker {
				cleanup := disableMerkleService()
				t.Cleanup(cleanup)
				return stubNodeError(t, "some other error")
			},
			path:   "/tx/error/proof/tsc",
			expect: `null`,
			assert: func(t *testing.T, tracker *callTracker) {
				require.NotNil(t, tracker)
				require.True(t, *tracker.bitcoin, "expected bitcoin GetMerkleProof to be invoked")
			},
		},
	}

	for _, sc := range scenarios {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			tracker := (*callTracker)(nil)
			if sc.setup != nil {
				tracker = sc.setup(t)
			}

			apitest.New().
				Handler(newRouterForTest()).
				Get(sc.path).
				Expect(t).
				Status(http.StatusOK).
				Body(sc.expect).
				End()

			if sc.assert != nil {
				sc.assert(t, tracker)
			}
		})
	}
}

// ---- helpers ----

type callTracker struct {
	internal *bool
	bitcoin  *bool
}

func newRouterForTest() *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc("/tx/{txid}/proof/tsc", getMerkleProofWithNodeAsBackup).Methods(http.MethodGet)
	return router
}

func mockMerkleService(t *testing.T, response interface{}) func() {
	t.Helper()

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))

	oldEnabled := merkleProofServiceEnabled
	oldAddr := merkleProofServiceAddress
	merkleProofServiceEnabled = true
	merkleProofServiceAddress = stub.URL

	return func() {
		merkleProofServiceEnabled = oldEnabled
		merkleProofServiceAddress = oldAddr
		stub.Close()
	}
}

func disableMerkleService() func() {
	oldEnabled := merkleProofServiceEnabled
	merkleProofServiceEnabled = false
	return func() { merkleProofServiceEnabled = oldEnabled }
}

func stubNodeSuccess(t *testing.T, proof *gobitcoin.MerkleProof) *callTracker {
	t.Helper()

	tracker := &callTracker{internal: new(bool), bitcoin: new(bool)}

	ensureBitcoinClient(t)

	patches := gomonkey.NewPatches()
	t.Cleanup(patches.Reset)

	patches.ApplyFunc(internal.GetTransaction, func(txid string) (*gobitcoin.RawTransaction, error) {
		*tracker.internal = true
		return &gobitcoin.RawTransaction{BlockHash: "blockhash"}, nil
	})

	patches.ApplyMethodFunc((*bitcoin.Client)(nil), "GetMerkleProof", func(blockhash, txID string) (*gobitcoin.MerkleProof, error) {
		*tracker.bitcoin = true
		return proof, nil
	})

	return tracker
}

func stubNodeError(t *testing.T, errMsg string) *callTracker {
	t.Helper()

	tracker := &callTracker{internal: new(bool), bitcoin: new(bool)}

	ensureBitcoinClient(t)

	patches := gomonkey.NewPatches()
	t.Cleanup(patches.Reset)

	patches.ApplyFunc(internal.GetTransaction, func(txid string) (*gobitcoin.RawTransaction, error) {
		*tracker.internal = true
		return &gobitcoin.RawTransaction{BlockHash: "blockhash"}, nil
	})

	patches.ApplyMethodFunc((*bitcoin.Client)(nil), "GetMerkleProof", func(blockhash, txID string) (*gobitcoin.MerkleProof, error) {
		*tracker.bitcoin = true
		return nil, errors.New(errMsg)
	})

	return tracker
}

func ensureBitcoinClient(t *testing.T) {
	if bitcoinClient != nil {
		return
	}

	bitcoinClient = &bitcoin.Client{}
	t.Cleanup(func() { bitcoinClient = nil })
}

package serverfiber

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testHarness sets up mock I/O functions and restores originals on cleanup.
type testHarness struct {
	txHexByID map[string]string                  // txid → raw hex
	proofs    map[string]*transaction.MerklePath // raw hex → merkle path (nil = unconfirmed)
	fetchLog  []string                           // records order of txid fetches
	nonce     uint64                             // makes each tx unique
}

func newHarness(t *testing.T) *testHarness {
	t.Helper()
	origFetchRawHex := fetchRawHex
	origResolveProof := resolveProof
	t.Cleanup(func() {
		fetchRawHex = origFetchRawHex
		resolveProof = origResolveProof
	})

	h := &testHarness{
		txHexByID: make(map[string]string),
		proofs:    make(map[string]*transaction.MerklePath),
	}

	fetchRawHex = func(txid string) (string, error) {
		h.fetchLog = append(h.fetchLog, txid)
		raw, ok := h.txHexByID[txid]
		if !ok {
			return "", fmt.Errorf("raw tx for %s not found", txid)
		}
		return raw, nil
	}

	resolveProof = func(rawHex string) (*transaction.MerklePath, error) {
		mp := h.proofs[rawHex]
		return mp, nil
	}

	return h
}

// addTx builds a transaction spending the given parent txids (one input each,
// vout 0) with a single P2PKH output, registers it in the harness, and returns its txid.
func (h *testHarness) addTx(t *testing.T, parentTxids []string, confirmed bool) string {
	t.Helper()
	tx := transaction.NewTransaction()
	for _, pid := range parentTxids {
		hash, err := chainhash.NewHashFromHex(pid)
		require.NoError(t, err)
		tx.AddInput(&transaction.TransactionInput{
			SourceTXID:       hash,
			SourceTxOutIndex: 0,
			UnlockingScript:  script.NewFromBytes([]byte{0x00}), // dummy
			SequenceNumber:   0xffffffff,
		})
	}
	if len(parentTxids) == 0 {
		// coinbase-like: use zero hash
		tx.AddInput(&transaction.TransactionInput{
			SourceTXID:       &chainhash.Hash{},
			SourceTxOutIndex: 0xffffffff,
			UnlockingScript:  script.NewFromBytes([]byte{0x04, 0xff, 0xff, 0x00, 0x1d}),
			SequenceNumber:   0xffffffff,
		})
	}
	h.nonce++
	ls, _ := script.NewFromHex("76a91489abcdefabbaabbaabbaabbaabbaabbaabbaabba88ac")
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      50000 + h.nonce, // unique per tx
		LockingScript: ls,
	})

	rawHex := hex.EncodeToString(tx.Bytes())
	txid := tx.TxID().String()

	h.txHexByID[txid] = rawHex
	if confirmed {
		h.proofs[rawHex] = dummyMerklePath(t, txid)
	}
	return txid
}

// addTxWithOpReturn is like addTx but adds an OP_RETURN output alongside the P2PKH output.
func (h *testHarness) addTxWithOpReturn(t *testing.T, parentTxids []string, confirmed bool) string {
	t.Helper()
	tx := transaction.NewTransaction()
	for _, pid := range parentTxids {
		hash, err := chainhash.NewHashFromHex(pid)
		require.NoError(t, err)
		tx.AddInput(&transaction.TransactionInput{
			SourceTXID:       hash,
			SourceTxOutIndex: 0,
			UnlockingScript:  script.NewFromBytes([]byte{0x00}),
			SequenceNumber:   0xffffffff,
		})
	}
	if len(parentTxids) == 0 {
		tx.AddInput(&transaction.TransactionInput{
			SourceTXID:       &chainhash.Hash{},
			SourceTxOutIndex: 0xffffffff,
			UnlockingScript:  script.NewFromBytes([]byte{0x04, 0xff, 0xff, 0x00, 0x1d}),
			SequenceNumber:   0xffffffff,
		})
	}
	h.nonce++
	ls, _ := script.NewFromHex("76a91489abcdefabbaabbaabbaabbaabbaabbaabbaabba88ac")
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      50000 + h.nonce,
		LockingScript: ls,
	})
	// OP_RETURN output: 0x6a followed by some data
	opReturnScript, _ := script.NewFromHex("6a0568656c6c6f")
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      0,
		LockingScript: opReturnScript,
	})

	rawHex := hex.EncodeToString(tx.Bytes())
	txid := tx.TxID().String()

	h.txHexByID[txid] = rawHex
	if confirmed {
		h.proofs[rawHex] = dummyMerklePath(t, txid)
	}
	return txid
}

func dummyMerklePath(t *testing.T, txid string) *transaction.MerklePath {
	t.Helper()
	leafRaw, _ := hex.DecodeString(txid)
	// reverse for little-endian
	for i, j := 0, len(leafRaw)-1; i < j; i, j = i+1, j-1 {
		leafRaw[i], leafRaw[j] = leafRaw[j], leafRaw[i]
	}
	leafHash, _ := chainhash.NewHash(leafRaw)
	isLeaf := true
	return &transaction.MerklePath{
		BlockHeight: 100,
		Path: [][]*transaction.PathElement{
			{
				{Hash: leafHash, Offset: 0, Txid: &isLeaf},
				{Hash: leafHash, Offset: 1}, // dummy sibling
			},
		},
	}
}

// bgCtx returns a context with generous timeout for normal tests.
func bgCtx() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	return ctx
}

// --- Tests ---

func TestBuildSdkBeef_ConfirmedTx(t *testing.T) {
	h := newHarness(t)
	s := &Server{}

	txid := h.addTx(t, nil, true)

	result, err := s.buildSdkBeef(bgCtx(), txid, make(map[string]*transaction.Transaction))

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.MerklePath, "confirmed tx should have merkle path attached")
	assert.Equal(t, 1, len(h.fetchLog))
}

func TestBuildSdkBeef_OneUnconfirmedParent(t *testing.T) {
	h := newHarness(t)
	s := &Server{}

	// confirmed grandparent → unconfirmed parent → unconfirmed child
	grandparent := h.addTx(t, nil, true)
	parent := h.addTx(t, []string{grandparent}, false)
	child := h.addTx(t, []string{parent}, false)

	result, err := s.buildSdkBeef(bgCtx(), child, make(map[string]*transaction.Transaction))

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, len(h.fetchLog), "should fetch child, parent, and grandparent")

	// Verify tree structure: child → parent → grandparent
	assert.NotNil(t, result.Inputs[0].SourceTransaction, "child should have parent attached")
	parentTx := result.Inputs[0].SourceTransaction
	assert.NotNil(t, parentTx.Inputs[0].SourceTransaction, "parent should have grandparent attached")
	gpTx := parentTx.Inputs[0].SourceTransaction
	assert.NotNil(t, gpTx.MerklePath, "grandparent should have merkle path")
}

func TestBuildSdkBeef_DeepLinearChain(t *testing.T) {
	h := newHarness(t)
	s := &Server{}

	// Build a chain 20 deep — old maxDepth=5 would have failed
	prev := h.addTx(t, nil, true) // confirmed root
	for i := 0; i < 19; i++ {
		prev = h.addTx(t, []string{prev}, false)
	}
	tip := prev

	result, err := s.buildSdkBeef(bgCtx(), tip, make(map[string]*transaction.Transaction))

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 20, len(h.fetchLog), "20-deep chain: 19 unconfirmed + 1 confirmed root")
}

func TestBuildSdkBeef_TimeoutExceeded(t *testing.T) {
	h := newHarness(t)
	s := &Server{}

	// Build a long chain; use an already-expired context to trigger timeout immediately
	prev := h.addTx(t, nil, true)
	for i := 0; i < 50; i++ {
		prev = h.addTx(t, []string{prev}, false)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	_, err := s.buildSdkBeef(ctx, prev, make(map[string]*transaction.Transaction))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot build BEEF")
}

func TestBuildSdkBeef_CacheDedup(t *testing.T) {
	h := newHarness(t)
	s := &Server{}

	// Diamond: two parents share the same confirmed grandparent
	//     grandparent (confirmed)
	//      /        \
	//   parent1    parent2
	//      \        /
	//       child
	grandparent := h.addTx(t, nil, true)
	parent1 := h.addTx(t, []string{grandparent}, false)
	parent2 := h.addTx(t, []string{grandparent}, false)
	child := h.addTx(t, []string{parent1, parent2}, false)

	result, err := s.buildSdkBeef(bgCtx(), child, make(map[string]*transaction.Transaction))

	require.NoError(t, err)
	assert.NotNil(t, result)
	// child + parent1 + parent2 + grandparent = 4, NOT 5
	assert.Equal(t, 4, len(h.fetchLog), "shared grandparent should only be fetched once")

	// Verify grandparent was fetched exactly once
	gpCount := 0
	for _, id := range h.fetchLog {
		if id == grandparent {
			gpCount++
		}
	}
	assert.Equal(t, 1, gpCount, "grandparent txid should appear exactly once in fetch log")
}

func TestBuildSdkBeef_FetchError(t *testing.T) {
	h := newHarness(t)
	s := &Server{}

	// Parent exists but grandparent doesn't
	grandparent := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	parent := h.addTx(t, []string{grandparent}, false)

	_, err := s.buildSdkBeef(bgCtx(), parent, make(map[string]*transaction.Transaction))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestBuildSdkBeef_FanIn(t *testing.T) {
	h := newHarness(t)
	s := &Server{}

	// Tx with 15 inputs — old maxInputs=10 would have rejected this
	var parents []string
	for i := 0; i < 15; i++ {
		parents = append(parents, h.addTx(t, nil, true))
	}
	child := h.addTx(t, parents, false)

	result, err := s.buildSdkBeef(bgCtx(), child, make(map[string]*transaction.Transaction))

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 16, len(h.fetchLog), "1 child + 15 confirmed parents")
}

func TestBuildSdkBeef_ConfirmedOpReturn(t *testing.T) {
	h := newHarness(t)
	s := &Server{}

	// A confirmed tx with OP_RETURN should still produce valid BEEF
	txid := h.addTxWithOpReturn(t, nil, true)

	result, err := s.buildSdkBeef(bgCtx(), txid, make(map[string]*transaction.Transaction))

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.MerklePath, "confirmed OP_RETURN tx should have merkle path attached")
	assert.Equal(t, 1, len(h.fetchLog))
}

func TestHasOpReturn(t *testing.T) {
	// Build a tx with OP_RETURN output and verify detection
	tx := transaction.NewTransaction()
	tx.AddInput(&transaction.TransactionInput{
		SourceTXID:       &chainhash.Hash{},
		SourceTxOutIndex: 0xffffffff,
		UnlockingScript:  script.NewFromBytes([]byte{0x04, 0xff, 0xff, 0x00, 0x1d}),
		SequenceNumber:   0xffffffff,
	})
	ls, _ := script.NewFromHex("76a91489abcdefabbaabbaabbaabbaabbaabbaabbaabba88ac")
	tx.AddOutput(&transaction.TransactionOutput{Satoshis: 50000, LockingScript: ls})

	// No OP_RETURN yet
	hasOR := false
	for _, out := range tx.Outputs {
		if out.LockingScript != nil && out.LockingScript.IsData() {
			hasOR = true
		}
	}
	assert.False(t, hasOR, "P2PKH-only tx should not be flagged")

	// Add OP_RETURN
	opReturnScript, _ := script.NewFromHex("6a0568656c6c6f")
	tx.AddOutput(&transaction.TransactionOutput{Satoshis: 0, LockingScript: opReturnScript})

	hasOR = false
	for _, out := range tx.Outputs {
		if out.LockingScript != nil && out.LockingScript.IsData() {
			hasOR = true
		}
	}
	assert.True(t, hasOR, "tx with OP_RETURN output should be flagged")

	// OP_FALSE OP_RETURN variant
	tx2 := transaction.NewTransaction()
	tx2.AddInput(&transaction.TransactionInput{
		SourceTXID:       &chainhash.Hash{},
		SourceTxOutIndex: 0xffffffff,
		UnlockingScript:  script.NewFromBytes([]byte{0x04}),
		SequenceNumber:   0xffffffff,
	})
	opFalseReturn, _ := script.NewFromHex("006a0568656c6c6f")
	tx2.AddOutput(&transaction.TransactionOutput{Satoshis: 0, LockingScript: opFalseReturn})

	hasOR = false
	for _, out := range tx2.Outputs {
		if out.LockingScript != nil && out.LockingScript.IsData() {
			hasOR = true
		}
	}
	assert.True(t, hasOR, "tx with OP_FALSE OP_RETURN should be flagged")
}

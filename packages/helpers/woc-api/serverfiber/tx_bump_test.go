package serverfiber

import (
	"fmt"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBump_ConfirmedTx(t *testing.T) {
	h := newHarness(t)

	txid := h.addTx(t, nil, true)
	rawHex := h.txHexByID[txid]
	mp := h.proofs[rawHex]

	result, err := resolveProof(rawHex)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, mp.BlockHeight, result.BlockHeight)
	assert.NotEmpty(t, result.Hex())

	// verify the hex can be parsed back
	parsed, err := transaction.NewMerklePathFromHex(result.Hex())
	require.NoError(t, err)
	assert.Equal(t, result.BlockHeight, parsed.BlockHeight)
}

func TestBump_UnconfirmedTx(t *testing.T) {
	h := newHarness(t)

	txid := h.addTx(t, nil, false)
	rawHex := h.txHexByID[txid]

	result, err := resolveProof(rawHex)

	require.NoError(t, err)
	assert.Nil(t, result, "unconfirmed tx should return nil merkle path")
}

func TestBump_InvalidTxid(t *testing.T) {
	h := newHarness(t)
	_ = h // harness replaces fetchRawHex with one that returns error for unknown txids

	_, err := fetchRawHex("deadbeef")

	assert.Error(t, err)
}

func TestBump_FetchError(t *testing.T) {
	origFetchRawHex := fetchRawHex
	t.Cleanup(func() { fetchRawHex = origFetchRawHex })

	fetchRawHex = func(txid string) (string, error) {
		return "", fmt.Errorf("connection refused")
	}

	_, err := fetchRawHex("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestBump_HexRoundTrip(t *testing.T) {
	h := newHarness(t)

	txid := h.addTx(t, nil, true)
	rawHex := h.txHexByID[txid]

	mp, err := resolveProof(rawHex)
	require.NoError(t, err)
	require.NotNil(t, mp)

	bumpHex := mp.Hex()

	// parse the hex back and re-encode — should be identical
	parsed, err := transaction.NewMerklePathFromHex(bumpHex)
	require.NoError(t, err)
	assert.Equal(t, bumpHex, parsed.Hex())
	assert.Equal(t, mp.BlockHeight, parsed.BlockHeight)
	assert.Equal(t, len(mp.Path), len(parsed.Path))
}

package subtree

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
)

func TestCoinbasePlaceholderTx(t *testing.T) {
	coinbasePlaceholderTx := generateCoinbasePlaceholderTx()
	coinbasePlaceholderTxHash := coinbasePlaceholderTx.TxID()
	assert.True(t, IsCoinbasePlaceHolderTx(coinbasePlaceholderTx))
	assert.Equal(t, uint32(0xFFFFFFFF), coinbasePlaceholderTx.Version)
	assert.Equal(t, uint32(0xFFFFFFFF), coinbasePlaceholderTx.LockTime)
	assert.Equal(t, coinbasePlaceholderTxHash, coinbasePlaceholderTx.TxID())
	assert.False(t, IsCoinbasePlaceHolderTx(transaction.NewTransaction()))
	assert.Equal(t, "a8502e9c08b3c851201a71d25bf29fd38a664baedb777318b12d19242f0e46ab", coinbasePlaceholderTx.TxID().String())
}

package subtree

import (
	"github.com/bsv-blockchain/go-sdk/chainhash"
)

// hashMap is a simple TxMap implementation backed by a plain Go map.
// This avoids depending on go-tx-map (which pulls in go-bt/v2).
type hashMap struct {
	m map[chainhash.Hash]uint64
}

func newHashMap(length uint32) *hashMap {
	return &hashMap{m: make(map[chainhash.Hash]uint64, length)}
}

func (h *hashMap) Put(hash chainhash.Hash, value uint64) error {
	h.m[hash] = value
	return nil
}

func (h *hashMap) Get(hash chainhash.Hash) (uint64, bool) {
	v, ok := h.m[hash]
	return v, ok
}

func (h *hashMap) Exists(hash chainhash.Hash) bool {
	_, ok := h.m[hash]
	return ok
}

func (h *hashMap) Length() int {
	return len(h.m)
}

func (h *hashMap) Keys() []chainhash.Hash {
	keys := make([]chainhash.Hash, 0, len(h.m))
	for k := range h.m {
		keys = append(keys, k)
	}
	return keys
}

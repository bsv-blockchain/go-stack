package models

import "sync"

// PoolTags comment
type PoolTags struct {
	Mu              sync.RWMutex
	CoinbaseTags    map[string]MinerDetails `json:"coinbase_tags"`
	PayoutAddresses map[string]MinerDetails `json:"payout_addresses"`
}

// Detail comment
type MinerDetails struct {
	Name string `json:"name"`
	Link string `json:"link"`
	Type string `json:"type"`
}

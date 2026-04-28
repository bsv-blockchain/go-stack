package electrum

import (
	"errors"
	"strings"
	"time"

	"github.com/checksum0/go-electrum/electrum"
	"github.com/ordishs/gocore"
	"github.com/teranode-group/woc-api/redis"
)

var logger = gocore.Log("woc-api")

const (
	KEY_HistoryTooLarge = "ScriptHistoryTooLarge_"
	KEY_UnspentTooLarge = "ScriptUnspentListTooLarge_"
)

// Client comment
type Client struct {
	electrum *electrum.Server
}

// New return a new electrum client
func New(url string) (*Client, error) {
	server := electrum.NewServer()
	if err := server.ConnectTCP(url); err != nil {
		logger.Errorf("Can't connect to electrum on %s. %v", url, err)
		return nil, err
	}

	return &Client{server}, nil
}

// AddressBalance comment
type AddressBalance struct {
	Confirmed   float64 `json:"confirmed"`
	Unconfirmed float64 `json:"unconfirmed"`
}

// GetMerkleProofResult represents the content of the result field in the response to GetMerkleProof().
type GetMerkleProofResult struct {
	Merkle   []string `json:"merkle"`
	Height   uint32   `json:"block_height"`
	Position uint32   `json:"pos"`
}

// GetAddressHistory comment
func (c *Client) GetAddressHistory(scriptHash string) ([]*electrum.GetMempoolResult, error) {

	var isHistoryTooLarge bool
	cacheKey := KEY_HistoryTooLarge + scriptHash

	if redis.RedisClient.Enabled {
		err := redis.GetCachedValue(cacheKey, &isHistoryTooLarge, nil)
		if err == nil && isHistoryTooLarge == true {
			// this is odd but changing return error might break client apps.
			// Use GetAddressHistoryOrTooLargeError for new calls.
			return nil, nil
		}
	}

	h, err := c.electrum.GetHistory(scriptHash)
	if err != nil {
		logger.Errorf("Can't get history for scripthash %s. %v", scriptHash, err)

		if strings.Contains(err.Error(), "history too large") {
			isHistoryTooLarge = true
			if redis.RedisClient.Enabled {
				//cache it for 1 day, history is not going to change.
				err := redis.SetCacheValueWithExpire(cacheKey, isHistoryTooLarge, 86400, nil)
				if err != nil {
					logger.Errorf("Unbale to cache redis KEY %s, %v", cacheKey, err)
				}
			}
		}
	}

	//Satoshi first tx for scripthash - electrumX doesn't add this
	if scriptHash == "8b01df4e368ea28f8dc0423bcf7a4923e3a12d307c875e47a0cfbf90b5c39161" {
		firstTx := electrum.GetMempoolResult{Hash: "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b", Height: 0}
		h = append([]*electrum.GetMempoolResult{&firstTx}, h...)
	}
	return h, nil
}

func (c *Client) GetAddressHistoryOrTooLargeError(scriptHash string) ([]*electrum.GetMempoolResult, error) {

	var isHistoryTooLarge bool
	cacheKey := KEY_HistoryTooLarge + scriptHash

	if redis.RedisClient.Enabled {
		err := redis.GetCachedValue(cacheKey, &isHistoryTooLarge, nil)
		if err == nil && isHistoryTooLarge == true {
			return nil, errors.New("history too large")
		}
	}

	h, err := c.electrum.GetHistory(scriptHash)
	if err != nil {
		logger.Errorf("Can't get history for scripthash %s. %v", scriptHash, err)
		if strings.Contains(err.Error(), "history too large") {
			isHistoryTooLarge = true
			if redis.RedisClient.Enabled {
				// cache it for 1 day, history is not going to change.
				err := redis.SetCacheValueWithExpire(cacheKey, isHistoryTooLarge, 86400, nil)
				if err != nil {
					logger.Errorf("Unbale to cache redis KEY %s, %v", cacheKey, err)
				}
			}

			return nil, errors.New("history too large")
		}
	}

	//Satoshi first tx for scripthash - electrumX doesn't add this
	if scriptHash == "8b01df4e368ea28f8dc0423bcf7a4923e3a12d307c875e47a0cfbf90b5c39161" {
		firstTx := electrum.GetMempoolResult{Hash: "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b", Height: 0}
		h = append([]*electrum.GetMempoolResult{&firstTx}, h...)
	}
	return h, nil
}

// ListUnspent comment
func (c *Client) ListUnspent(scriptHash string) ([]*electrum.ListUnspentResult, error) {

	var isUspentListTooLarge bool
	cacheKey := KEY_UnspentTooLarge + scriptHash

	if redis.RedisClient.Enabled {
		err := redis.GetCachedValue(cacheKey, &isUspentListTooLarge, nil)
		if err == nil && isUspentListTooLarge == true {
			return nil, errors.New("request timeout. Possible reason: UnpsentList too large.")
		}
	}

	h, err := c.electrum.ListUnspent(scriptHash)
	if err != nil {

		logger.Errorf("Can't get unspent for scripthash %s. %v", scriptHash, err)

		if strings.Contains(err.Error(), "request timeout") {
			isUspentListTooLarge = true
			if redis.RedisClient.Enabled {
				// cache it for 1 day, history is not going to change.
				err := redis.SetCacheValueWithExpire(cacheKey, isUspentListTooLarge, 120, nil)
				if err != nil {
					logger.Errorf("Unbale to cache redis KEY %s, %v", cacheKey, err)
				}
			}

			return nil, errors.New("request timeout. Possible reason: UnpsentList too large.")
		}
	}
	return h, nil
}

// GetAddressBalance from electrum. We must use scripthash of the address now as explained in ElectrumX docs
func (c *Client) GetAddressBalance(scriptHash string) (*AddressBalance, error) {
	timeNowElectrumRequest := time.Now()
	balance, err := c.electrum.GetBalance(scriptHash)
	if err != nil {
		logger.Errorf(
			"Can't get balance for scripthash %s: duration: %s: %v",
			scriptHash,
			time.Since(timeNowElectrumRequest).String(),
			err,
		)
		return nil, err
	}
	// durationElectrumRequest := time.Since(timeNowElectrumRequest)

	//Satoshi first tx reward scripthash - electrumX doesn't add this
	if scriptHash == "8b01df4e368ea28f8dc0423bcf7a4923e3a12d307c875e47a0cfbf90b5c39161" {
		balance.Confirmed = balance.Confirmed + (50 * 100000000)
	}

	ret := &AddressBalance{
		Confirmed:   balance.Confirmed,
		Unconfirmed: balance.Unconfirmed,
	}

	// if utxostore.IsEnabled() && utxostore.ShouldCompare() {
	// 	go utxostore.CompareBalanceWithElectrumX(
	// 		durationElectrumRequest,
	// 		scriptHash,
	// 		int64(balance.Confirmed),
	// 		int64(balance.Unconfirmed),
	// 	)
	// }

	return ret, nil
}

// If there is a history means its used.
func (c *Client) HasHistory(scriptHash string) (bool, error) {

	var isHistoryTooLarge bool
	cacheKey := KEY_HistoryTooLarge + scriptHash

	if redis.RedisClient.Enabled {
		err := redis.GetCachedValue(cacheKey, &isHistoryTooLarge, nil)
		if err == nil && isHistoryTooLarge == true {
			return true, nil
		}
	}

	h, err := c.electrum.GetHistory(scriptHash)

	if err != nil {

		if strings.Contains(err.Error(), "history too large") {
			isHistoryTooLarge = true
			if redis.RedisClient.Enabled {
				//cache it for 1 day, history is not going to change.
				err := redis.SetCacheValueWithExpire(cacheKey, isHistoryTooLarge, 86400, nil)
				if err != nil {
					logger.Errorf("Unbale to cache redis KEY %s, %v", cacheKey, err)
				}
			}

			return true, nil
		}
		return false, err
	}

	//Satoshi first tx for scripthash - electrumX doesn't add this
	if scriptHash == "8b01df4e368ea28f8dc0423bcf7a4923e3a12d307c875e47a0cfbf90b5c39161" {
		firstTx := electrum.GetMempoolResult{Hash: "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b", Height: 0}
		h = append([]*electrum.GetMempoolResult{&firstTx}, h...)
	}

	if len(h) > 0 {
		return true, nil
	}

	return false, nil
}

// GetMerkleProof from electrum.
func (c *Client) GetMerkleProof(txid string, height uint32) (*GetMerkleProofResult, error) {
	proof, err := c.electrum.GetMerkleProof(txid, height)
	if err != nil {
		logger.Errorf("Can't get proof for txid %s and height %d. %v", txid, height, err)
		return nil, err
	}

	ret := &GetMerkleProofResult{
		Merkle:   proof.Merkle,
		Height:   proof.Height,
		Position: proof.Position,
	}
	return ret, nil
}

// Ping Make sure connection is not closed with timed "server.ping" call
func (c *Client) Ping() {
	if err := c.electrum.Ping(); err != nil {
		logger.Errorf("Can't ping electrum. %v", err)
		return
	}
}

func (c *Client) PingOrError() error {
	if err := c.electrum.Ping(); err != nil {
		logger.Errorf("Unable to ping electrumX. %v", err)
		return err
	}
	return nil
}

func (c *Client) GetBlockHeader(height uint32) (*electrum.GetBlockHeaderResult, error) {
	resp, err := c.electrum.GetBlockHeader(height, 0)
	if err != nil {
		logger.Errorf("Unable to GetBlockHeaderOrError electrumX. %v", err)
		return nil, err
	}
	return resp, nil
}

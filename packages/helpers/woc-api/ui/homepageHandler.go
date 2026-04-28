package ui

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/teranode-group/common/bsdecoder"
	"github.com/teranode-group/woc-api/bstore"
	"github.com/teranode-group/woc-api/configs"
	"github.com/teranode-group/woc-api/internal"
	"github.com/teranode-group/woc-api/pools"
	"github.com/teranode-group/woc-api/price"
	"github.com/teranode-group/woc-api/redis"

	gobitcoin "github.com/ordishs/go-bitcoin"
	"github.com/teranode-group/common/utils"
)

/*
},
"transaction-stats-summary":{
"count":[284,422,234,23434,56756],
"rate::[284,422,234,23434,56756]
}
*/
const (
	KEY_GetHomepage     = "GetHomepage"
	KEY_GetHomepage24hr = "GetHomepage24Hr"
)

type chainSummary struct {
	Blocks        int32   `json:"blocks"`
	BestBlockHash string  `json:"bestblockhash"`
	Difficulty    float64 `json:"difficulty"`
	ChainWork     string  `json:"chainwork,omitempty"`

	NetworkHashPS     float64 `json:"networkhashps"`
	CirculatingSupply float64 `json:"circulatingSupply"`

	MempoolSize  int               `json:"mempoolSize"`
	MempoolBytes int               `json:"mempoolBytes"`
	MempoolUsage int               `json:"mempoolUsage"`
	ExchangeRate map[string]string `json:"exchangeRate"`
}

type feeRecommendation struct {
	// FeeUnit Unit for fee rate fields.
	FeeUnit string `json:"fee_unit"`
	// Fee Current fee for the unit.
	Fee int `json:"fee"`
	// MempoolMinFee Current mempool minimum relay fee (sat/vB).
	MempoolMinFee int    `json:"mempool_min_fee"`
	FeeUSD        string `json:"fee_usd,omitempty"`
}

type homepageData struct {
	// BlockSummary            []*blockSummary           `json:"block-summary`
	ChainSummary            chainSummary                        `json:"chainSummary"`
	TransactionStatsSummary []*gobitcoin.ChainTXStats           `json:"txStatsSummary"`
	LatestBlocks            []*bsdecoder.BlockHeaderAndCoinbase `json:"latestBlocks"`
	FeeRecommendation       feeRecommendation                   `json:"feeRecommendation"`
}

// GetHomepage returns homepage data
// Note: This handler is also used by API endpoint /chain/summary and woc-sockets
func GetHomepage(w http.ResponseWriter, r *http.Request) {

	var err error
	var exchangeRate float64
	ba := []*bsdecoder.BlockHeaderAndCoinbase{}

	h := homepageData{}
	cs := chainSummary{}

	// use goroutines to get info in parallel
	bci, err := bitcoinClient.GetBlockchainInfo()
	if err != nil {
		logger.Errorf("GetHomepage - GetBlockchainInfo %+v\n", err)
		return
	}

	miningInfo, err := bitcoinClient.GetMiningInfo()
	if err != nil {
		logger.Errorf("GetHomepage - error GetMiningInfo %+v\n", err)
		return
	}
	mempoolInfo, err := bitcoinClient.GetMempoolInfo()
	if err != nil {
		logger.Errorf("GetHomepage - error GetMempoolInfo %+v\n", err)
		return
	}

	if isMainnet {
		exchangeRate, err = price.GetUSDPrice()

		if err != nil {
			logger.Errorf("GetHomepage - error Getting exchange rate %+v\n", err)
		}

		if exchangeRate > 0 {
			cs.ExchangeRate = map[string]string{"rate": strconv.FormatFloat(exchangeRate, 'f', -1, 64), "currency": "USD"}
		}
	}

	cs.BestBlockHash = bci.BestBlockHash
	cs.Blocks = bci.Blocks
	cs.ChainWork = bci.ChainWork
	cs.Difficulty = bci.Difficulty

	cs.CirculatingSupply = utils.CirculatingSupply(bci.Blocks)

	cs.NetworkHashPS = miningInfo.NetworkHashPS

	cs.MempoolBytes = mempoolInfo.Bytes
	cs.MempoolSize = mempoolInfo.Size
	cs.MempoolUsage = mempoolInfo.Usage

	h.ChainSummary = cs

	blockHash := bci.BestBlockHash

	for i := 0; i < 10; i++ {
		// get block from bstore
		block, err := bstore.GetBlockDetails(blockHash, 0)
		// code for bstore
		if err != nil {
			logger.Errorf("GetHomepage - error bstore.GetBlockDetails for hash %+v - %+v\n", blockHash, err)
			//try cache and node
			block, err = getFromMongoCacheOrNode(blockHash)

			if err != nil {
				//occasionally bstore is few seconds behind the bitcoin node
				logger.Errorf("GetHomepage - error getFromMongoCacheOrNode for hash %+v - %+v\n", blockHash, err)

				// Get from cache - send last known respose from cache
				if redis.RedisClient.Enabled {
					logger.Warn("GetHomepage - Serving last known response from cache\n")
					err = redis.GetCachedValue(KEY_GetHomepage, &h, nil)
					if err != nil {
						logger.Errorf("GetHomepage - error serving from cache %+v\n", err)
					}
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(h)
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			block.Src = "w" // code for woc node

		}
		//else {
		// 	block.Src = "b" //code for bstore
		// }

		//add miner info
		if block.Coinbase != nil && len(block.Coinbase.Vin) > 0 {
			tx := block.Coinbase
			address := ""
			if len(tx.Vout) > 0 && tx.Vout[0].ScriptPubKey.Addresses != nil && len(tx.Vout[0].ScriptPubKey.Addresses) > 0 {
				address = block.Coinbase.Vout[0].ScriptPubKey.Addresses[0]
			}
			minerDetails, err := pools.GetMinerTag(tx.Vin[0].Coinbase, address)
			block.Coinbase.Vin[0].MinerInfo = &bsdecoder.MinerDetails{}
			if err == nil {
				block.Coinbase.Vin[0].MinerInfo.Name = minerDetails.Name
				block.Coinbase.Vin[0].MinerInfo.Type = minerDetails.Type
				block.Coinbase.Vin[0].MinerInfo.Link = minerDetails.Link
			} else {
				src := []byte(tx.Vin[0].Coinbase)

				dst := make([]byte, hex.DecodedLen(len(src)))
				n, err := hex.Decode(dst, src)
				if err != nil {

					block.Coinbase.Vin[0].MinerInfo.Name = tx.Vin[0].Coinbase
				} else {
					block.Coinbase.Vin[0].MinerInfo.Name = fmt.Sprintf("%s\n", dst[:n])
				}
			}
		}

		ba = append(ba, block)
		blockHash = block.PreviousBlockHash
	}
	h.LatestBlocks = ba

	var feeUSD float64
	if exchangeRate > 0 {
		val := float64(configs.Settings.FeeRate) * 1e-8 * exchangeRate
		feeUSD = val
	}

	h.FeeRecommendation = feeRecommendation{
		FeeUnit:       configs.Settings.FeeUnit,
		Fee:           configs.Settings.FeeRate,
		MempoolMinFee: configs.Settings.MinFee,
		FeeUSD:        fmt.Sprintf("%.8f", feeUSD),
	}

	css, err := bitcoinClient.GetAllChainTxStats()

	if err != nil {
		logger.Errorf("GetHomepage - GetAllChainTxStats: %+v\n", err)
	} else {
		h.TransactionStatsSummary = css
	}

	// Cache this response
	if redis.RedisClient.Enabled {
		err = redis.SetCacheValue(KEY_GetHomepage, h, nil)

		if err != nil {
			logger.Errorf("Unable to cache GetHomepage response %+v\n", err)
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(h)
}

func getFromMongoCacheOrNode(hash string) (block *bsdecoder.BlockHeaderAndCoinbase, err error) {
	blk, err := internal.GetBlock(hash)

	if err != nil {
		return nil, errors.New("Info: getFromMongoCacheOrNode - unable to get block")
	}

	block = &bsdecoder.BlockHeaderAndCoinbase{}

	if blk != nil && len(blk.Hash) == 64 {

		block.Hash = blk.Hash
		block.Confirmations = uint32(blk.Confirmations)
		block.Size = blk.Size
		block.Height = blk.Height
		block.Version = uint32(blk.Version)
		block.VersionHex = blk.VersionHex
		block.MerkleRoot = blk.MerkleRoot
		block.TxCount = blk.TxCount
		block.Time = blk.Time
		block.MedianTime = blk.MedianTime
		block.Nonce = blk.Nonce
		block.Bits = blk.Bits
		block.Difficulty = blk.Difficulty
		block.Chainwork = blk.Chainwork
		block.PreviousBlockHash = blk.PreviousBlockHash
		block.NextBlockHash = blk.NextBlockHash

		if len(blk.Tx[0]) > 0 {
			hex, err := bitcoinClient.GetRawTransactionHex(blk.Tx[0])

			if err == nil {
				txDetails, err := bstore.ParseRawTx(*hex)

				if err != nil {
					return nil, errors.New("Non standard raw multisig")
				}
				block.Coinbase = txDetails
			}
		}

		// Add total fee
		if block.Coinbase != nil && len(block.Coinbase.Vout) > 0 {
			blockReward := utils.CalculateReward(block.Height)
			var totalOutput float64

			for _, vout := range block.Coinbase.Vout {
				if vout.Value > 0 {
					totalOutput += vout.Value
				}
			}
			blk.TotalFees = totalOutput - blockReward
		}

	} else {
		logger.Errorf("getFromMongoCacheOrNode - block %s with incorrect details %+v\n", hash, blk)
		return nil, errors.New("Error: getFromMongoCacheOrNode - block with incorrect details")
	}

	return block, nil
}

// temp endpoint for 1B tx demo
func GetHomepage24Hr(w http.ResponseWriter, r *http.Request) {

	var err error
	ba := []*bsdecoder.BlockHeaderAndCoinbase{}

	h := homepageData{}
	cs := chainSummary{}

	// use goroutines to get info in parallel
	bci, err := bitcoinClient.GetBlockchainInfo()
	if err != nil {
		logger.Errorf("GetHomepage - GetBlockchainInfo %+v\n", err)
		return
	}

	miningInfo, err := bitcoinClient.GetMiningInfo()
	if err != nil {
		logger.Errorf("GetHomepage - error GetMiningInfo %+v\n", err)
		return
	}
	mempoolInfo, err := bitcoinClient.GetMempoolInfo()
	if err != nil {
		logger.Errorf("GetHomepage - error GetMempoolInfo %+v\n", err)
		return
	}

	// exchangeRate, err := price.GetUSDPrice()

	// if err != nil {
	// 	logger.Errorf("GetHomepage - error Getting exchange rate %+v\n", err)
	// }

	// if exchangeRate > 0 {
	// 	cs.ExchangeRate = map[string]string{"rate": strconv.FormatFloat(exchangeRate, 'f', -1, 64), "currency": "USD"}
	// }

	cs.BestBlockHash = bci.BestBlockHash
	cs.Blocks = bci.Blocks
	cs.ChainWork = bci.ChainWork
	cs.Difficulty = bci.Difficulty

	cs.CirculatingSupply = utils.CirculatingSupply(bci.Blocks)

	cs.NetworkHashPS = miningInfo.NetworkHashPS

	cs.MempoolBytes = mempoolInfo.Bytes
	cs.MempoolSize = mempoolInfo.Size
	cs.MempoolUsage = mempoolInfo.Usage

	h.ChainSummary = cs

	blockHash := bci.BestBlockHash

	currentTime := time.Now()
	timelast24Hours := currentTime.Add(-time.Hour * 24)

	for i := 0; i < 180; i++ {
		// get block from bstore
		block, err := bstore.GetBlockHeader(blockHash, 0)
		// code for bstore
		if err != nil {
			logger.Errorf("GetHomepage - error bstore.GetBlockDetails for hash %+v - %+v\n", blockHash, err)
			//try cache and node
			block, err = getFromMongoCacheOrNode(blockHash)

			if err != nil {
				//occasionally bstore is few seconds behind the bitcoin node
				logger.Errorf("GetHomepage - error getFromMongoCacheOrNode for hash %+v - %+v\n", blockHash, err)

				// Get from cache - send last known respose from cache
				if redis.RedisClient.Enabled {
					logger.Warn("GetHomepage - Serving last known response from cache\n")
					err = redis.GetCachedValue(KEY_GetHomepage24hr, &h, nil)
					if err != nil {
						logger.Errorf("GetHomepage - error serving from cache %+v\n", err)
					}
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(h)
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			block.Src = "w" // code for woc node

		}
		block.Coinbase = nil
		logger.Info("block.Time - +v", int64(block.Time))
		logger.Info("timelast24Hours.Unix() - +v", timelast24Hours.Unix())
		logger.Info("i", i)
		if int64(block.Time) < timelast24Hours.Unix() {
			break
		}
		//else {
		// 	block.Src = "b" //code for bstore
		// }

		//add miner info
		// if block.Coinbase != nil && len(block.Coinbase.Vin) > 0 {
		// 	tx := block.Coinbase
		// 	address := ""
		// 	if len(tx.Vout) > 0 && tx.Vout[0].ScriptPubKey.Addresses != nil && len(tx.Vout[0].ScriptPubKey.Addresses) > 0 {
		// 		address = block.Coinbase.Vout[0].ScriptPubKey.Addresses[0]
		// 	}
		// 	minerDetails, err := pools.GetMinerTag(tx.Vin[0].Coinbase, address)
		// 	block.Coinbase.Vin[0].MinerInfo = &bsdecoder.MinerDetails{}
		// 	if err == nil {
		// 		block.Coinbase.Vin[0].MinerInfo.Name = minerDetails.Name
		// 		block.Coinbase.Vin[0].MinerInfo.Type = minerDetails.Type
		// 		block.Coinbase.Vin[0].MinerInfo.Link = minerDetails.Link
		// 	} else {
		// 		src := []byte(tx.Vin[0].Coinbase)

		// 		dst := make([]byte, hex.DecodedLen(len(src)))
		// 		n, err := hex.Decode(dst, src)
		// 		if err != nil {

		// 			block.Coinbase.Vin[0].MinerInfo.Name = tx.Vin[0].Coinbase
		// 		} else {
		// 			block.Coinbase.Vin[0].MinerInfo.Name = fmt.Sprintf("%s\n", dst[:n])
		// 		}
		// 	}
		// }

		ba = append(ba, block)
		blockHash = block.PreviousBlockHash
	}
	h.LatestBlocks = ba

	// css, err := bitcoinClient.GetAllChainTxStats()

	// if err != nil {
	// 	logger.Errorf("GetHomepage - GetAllChainTxStats: %+v\n", err)
	// } else {
	// 	h.TransactionStatsSummary = css
	// }

	// Cache this response
	if redis.RedisClient.Enabled {
		err = redis.SetCacheValue(KEY_GetHomepage24hr, h, nil)

		if err != nil {
			logger.Errorf("Unable to cache GetHomepage response %+v\n", err)
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(h)
}

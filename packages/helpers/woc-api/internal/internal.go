package internal

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	gobitcoin "github.com/ordishs/go-bitcoin"
	"github.com/ordishs/gocore"
	common_logger "github.com/teranode-group/common/logger"
	"github.com/teranode-group/common/parser"
	"github.com/teranode-group/common/utils"
	"github.com/teranode-group/woc-api/mongocache"
)

// GetBlock - mongo cache first and then node
func GetBlock(hash string) (block *gobitcoin.Block, err error) {
	// try to get the block from the cache
	ok := false
	if useMongoCache {
		block, ok = mongocache.GetBlockFromCache(hash)
	}
	if ok {

		blockHeader, e := bitcoinClient.GetBlockHeader(hash)
		if e != nil {
			return nil, e
		}

		block.NextBlockHash = blockHeader.NextBlockHash

		confirmations := blockHeader.Confirmations

		if confirmations > -1 {
			block.Confirmations = confirmations
			if block.CoinbaseTx != nil {
				block.CoinbaseTx.Confirmations = uint32(confirmations)
			}
		}

		addMinerInfo(block)
	} else {
		block, err = GetBlockByHash(hash)
		if err != nil {
			return
		}
		if ShouldCacheBlock(block) {
			// add the block to the cache
			mongocache.AddBlockToCache(*block)
			//Saved and paginated, therefore send only maxTxids tx
			if len(block.Tx) >= mongocache.MaxTxids {
				block.Tx = block.Tx[:mongocache.MaxTxids]
			}
		}
	}

	//Add pages if required
	if block.TxCount > uint64(len(block.Tx)) {
		countOfIdsNotIncluded := (block.TxCount - uint64(mongocache.MaxTxids))

		totalPages := countOfIdsNotIncluded / uint64(mongocache.BlockTxidCollectionMaxTx)
		if countOfIdsNotIncluded%uint64(mongocache.BlockTxidCollectionMaxTx) > 0 {
			totalPages++
		}

		pages := make([]string, totalPages)
		for i := 0; i < int(totalPages); i++ {
			pages[i] = "/block/hash/" + hash + "/page/" + strconv.Itoa(i+1)
		}

		block.Pagination = &gobitcoin.BlockPage{URI: pages, Size: uint64(mongocache.BlockTxidCollectionMaxTx)}

	}

	return
}

// GetBlockHeader - mongo cache first and then node
func GetBlockHeader(hash string) (blockHeader *gobitcoin.BlockHeader, err error) {
	// try to get the block from the cache
	ok := false

	//TODO: should we even check cache here????
	//Cache saves block (header + tx), removing tx if reading from cache
	block := &gobitcoin.Block{}
	if useMongoCache {
		block, ok = mongocache.GetBlockFromCache(hash)
	}
	if ok {

		blockHeader, err = bitcoinClient.GetBlockHeader(hash)
		if err != nil {
			return
		}

		block.NextBlockHash = blockHeader.NextBlockHash
		confirmations := blockHeader.Confirmations

		if confirmations > -1 {
			block.Confirmations = confirmations
		}

		blockJSON, err := json.Marshal(block)
		if err == nil {
			_ = json.Unmarshal([]byte(blockJSON), &blockHeader)
		}

	} else {
		// not in cache get from node
		blockHeader, err = getBlockHeaderByHash(hash)
		if err != nil {
			return
		}
	}

	return
}

// GetTxConfirmations - from node
func GetTxConfirmations(txID string) int64 {
	tx, err := bitcoinClient.GetRawTransaction(txID, true, true)
	if err != nil {
		return -1
	}
	return int64(tx.Confirmations)
}

// GetBlockByHash - from node
func GetBlockByHash(hash string) (block *gobitcoin.Block, err error) {
	block, err = bitcoinClient.GetBlock(hash)
	if err != nil {
		logger.Errorf("getBlockByHash - error getblock for hash %+v - %+v\n", hash, err)
		return nil, err
	}

	txCount := len(block.Tx)
	block.TxCount = uint64(txCount)
	if txCount > 0 {
		addMinerInfo(block)
	}
	return
}

// GetTransaction - mongo cache first and then node
func GetTransaction(txid string) (*gobitcoin.RawTransaction, error) {
	var err error
	var tx *gobitcoin.RawTransaction
	ok := false
	// try to get the transaction from the cache
	if useMongoCache {
		tx, ok = mongocache.GetTransactionFromCache(txid)
	}
	if !ok || (tx != nil && len(tx.BlockHash) == 64 && tx.Blocktime == 0) {
		tx, err = bitcoinClient.GetRawTransaction(txid, true, true)
		if err != nil {

			return nil, err
		}

		// todo: extract to common method
		if len(tx.Vout) > 0 {
			for i, vout := range tx.Vout {

				if strings.HasPrefix(vout.ScriptPubKey.ASM, "0 ") {
					vout.ScriptPubKey.ASM = strings.TrimPrefix(vout.ScriptPubKey.ASM, "0 ")
					vout.ScriptPubKey.Hex = vout.ScriptPubKey.Hex[2:]
				}

				if strings.HasPrefix(vout.ScriptPubKey.ASM, "OP_RETURN") {
					buf, err := hex.DecodeString(vout.ScriptPubKey.Hex)
					if err != nil {
						logger.Errorf("error parsing op_return script %+v", err)
						continue
					}
					tag, subtag, parts, err := parser.ParseOpReturn(buf)

					if err == nil {
						ps := &gobitcoin.OpReturn{}
						ps.Type = tag
						ps.Action = subtag

						if parts != nil && *parts != nil && len(*parts) > 0 && !BadTxids.Has(txid) {
							ps.Text = (*parts)[0].URI

							var up []string
							for _, p := range *parts {
								up = append(up, p.UTF8)
							}
							ps.Parts = up
						}
						tx.Vout[i].ScriptPubKey.OpReturn = ps
					}
					if BadTxids.Has(txid) {
						tx.Vout[i].ScriptPubKey.ASM = "removed"
						tx.Vout[i].ScriptPubKey.Hex = "removed"
					}

				} else if vout.ScriptPubKey.Type == "nonstandard" {

					buf, err := hex.DecodeString(vout.ScriptPubKey.Hex)
					if err != nil {
						logger.Errorf("error parsing op_return script %+v", err)
						continue
					}

					tag, subtag, err := parser.ParseNonStandard(buf)
					if err == nil {
						t := &gobitcoin.Tag{}
						t.Type = tag
						t.Action = subtag
						tx.Vout[i].ScriptPubKey.Tag = t
					}
				}

			}
		}

		if ShouldCacheTx(tx) {
			// add the transaction to the cache
			cacheWriteTransaction <- tx
		}
	} else {
		//tx from cache, its cheap to get confirmation from blockheader
		blockHeader, err := bitcoinClient.GetBlockHeader(tx.BlockHash)
		if err != nil {
			return nil, err
		}
		confirmations := blockHeader.Confirmations

		if confirmations > -1 {
			tx.Confirmations = uint32(confirmations)
		}

	}
	return tx, nil
}

func ShouldCacheBlock(block *gobitcoin.Block) bool {
	if !useMongoCache {
		return false
	}
	if block != nil && (block.TxCount > blockCacheTxCountMin || block.Size > blockCacheSizeMin) {
		return true
	}
	return false
}

func ShouldCacheTx(tx *gobitcoin.RawTransaction) bool {
	if !useMongoCache {
		return false
	}
	if tx == nil {
		return false
	}
	//Cache tx if vin or vout count greater than txCacheVoutVinCountMin or tx size then 0.3 MB && is mined
	if (len(tx.Vin) > txCacheVoutVinCountMin ||
		len(tx.Vout) > txCacheVoutVinCountMin ||
		tx.Size > txCacheSizeMin) &&
		len(tx.BlockHash) > 2 {
		return true
	}
	return false
}

func CacheTx(tx *gobitcoin.RawTransaction) {
	if ShouldCacheTx(tx) {
		cacheWriteTransaction <- tx
	}
}

func getBlockHeaderByHash(hash string) (block *gobitcoin.BlockHeader, err error) {
	block, err = bitcoinClient.GetBlockHeader(hash)
	if err != nil {
		logger.Errorf("getBlockHeaderByHash - error getblock for hash %+v - %+v\n", hash, err)
		return nil, err
	}
	return
}

func addMinerInfo(block *gobitcoin.Block) {
	txCount := len(block.Tx)
	if txCount > 0 {
		// add coinbasetx to block
		tx, err := bitcoinClient.GetRawTransaction(block.Tx[0], true, true)
		if err != nil {
			logger.Errorf("addMinerInfo: error gettransaction for txid %+v - %+v\n", block.Tx[0], err)
		} else {
			block.CoinbaseTx = tx
			tf, err := getBlockTotalFeesFromCoinbaseTxAndBlockHeight(tx, block.Height)
			if err == nil {
				block.TotalFees = tf
			}
		}
		miner, err := getMinerFromCoinbaseTx(block.Hash, tx)
		if err != nil {
			logger.Errorf("Error getting miner from coinbase tx: %+v\n", err)
		}
		block.Miner = miner
	}
}

func getBlockTotalFeesFromCoinbaseTxAndBlockHeight(coinbaseTx *gobitcoin.RawTransaction, height uint64) (fees float64, err error) {
	if coinbaseTx == nil {
		return 0, errors.New("no coinbase supplied to GetBlockTotalFeesFromCoinbaseTxAndBlockHeight")
	}

	blockReward := utils.CalculateReward(height)
	var totalOutput float64
	for _, vout := range coinbaseTx.Vout {
		if vout.Value > 0 {
			totalOutput += vout.Value
		}
	}

	return totalOutput - blockReward, nil
}

func getMinerFromCoinbaseTx(blockHash string, coinbaseTx *gobitcoin.RawTransaction) (miner string, err error) {
	if coinbaseTx == nil || len(coinbaseTx.Vin) == 0 {
		genesisBlock, ok := gocore.Config().Get("genesisBlock")
		if ok && genesisBlock == blockHash {
			return
		}
		return "", errors.New("no valid coinbase supplied to GetMinerFromCoinbaseTx")
	}
	miner = "unknown"
	poolTags.Mu.RLock()
	defer poolTags.Mu.RUnlock()
	if poolTags != nil {
		// check the payout addresses
		if len(coinbaseTx.Vout) > 0 && len(coinbaseTx.Vout[0].ScriptPubKey.Addresses) > 0 {
			minerInfo, ok := poolTags.PayoutAddresses[coinbaseTx.Vout[0].ScriptPubKey.Addresses[0]]
			if ok {
				miner = minerInfo.Name
				return
				// minerInfo.identifiedBy = "payout address " + payoutAddress;
			}
		}
		// check the coinbase tag
		if len(coinbaseTx.Vin) > 0 {
			hexCoinbase := coinbaseTx.Vin[0].Coinbase
			bCoinbase, err := hex.DecodeString(hexCoinbase)
			if err != nil {
				logger.Errorf("Can't decode coinbase. %+v\n", err)
			}
			//Add raw text as default
			miner = string(bCoinbase)

			for key := range poolTags.CoinbaseTags {
				if strings.Contains(string(bCoinbase), key) {
					minerInfo, ok := poolTags.CoinbaseTags[key]
					if ok {
						miner = minerInfo.Name
						// minerInfo.identifiedBy = "coinbase tag '" + coinbaseTag + "'";
					}
				}
			}
		}
	}
	return
}

func SaveBlockHeaders() error {

	blockHeadersPath, ok := gocore.Config().Get("block_headers_path")
	if !ok {
		return fmt.Errorf("block_headers_path not found in settings")
	}
	blockDiff, ok := gocore.Config().GetInt("block_headers_block_diff")
	if !ok {
		return fmt.Errorf("block_headers_block_diff not found in settings")
	}
	totalBlocks, ok := gocore.Config().GetInt("block_headers_total_blocks_to_write")
	if !ok {
		return fmt.Errorf("block_headers_total_blocks_to_write not found in settings")
	}

	// make sure the directory exists
	if err := os.MkdirAll(blockHeadersPath, 0o755); err != nil {
		return err
	}

	lastHeightFile := fmt.Sprintf("%s/last_height", blockHeadersPath)

	// Following folder structure needs to exist at root level of server:
	// 'data/block_headers/last_height'
	// Initial value needs to be set in last_height
	lastHeightData, err := ReadFromFile(lastHeightFile)
	var lastHeight int64
	if errors.Is(err, os.ErrNotExist) {
		lastHeight = 0
		if err := WriteToFile(lastHeightFile, []byte("0")); err != nil {
			return fmt.Errorf("init last_height: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("error reading latest height file: %w", err)
	} else {
		lastHeight, err = strconv.ParseInt(strings.TrimSpace(string(lastHeightData)), 10, 32)
		if err != nil {
			return fmt.Errorf("error parsing lastHeight: %w", err)
		}
	}

	chainInfo, err := bitcoinClient.GetBlockchainInfo()
	if err != nil {
		return fmt.Errorf("unable to GetBlockchainInfo: %w", err)
	}

	latestHeight := int64(chainInfo.Blocks)

	if (latestHeight - lastHeight) < int64(blockDiff) {
		logger.Info("skip blocker header writing")
		return nil
	}

	var startHeight int64

	startHeight = lastHeight + 1
	endHeight := lastHeight + int64(totalBlocks)

	if lastHeight == 0 {
		startHeight = 0
	}

	blockFile := fmt.Sprintf("%s/%d_%d_headers.bin", blockHeadersPath, startHeight, endHeight)

	var fileData []byte

	//very first file - append block 0
	if lastHeight == 0 {
		hash, err := bitcoinClient.GetBlockHash(0)
		if err != nil {
			return fmt.Errorf("unable to GetBlockHash from node for block height %d : %w", 0, err)
		}

		HeaderBytes, err := bitcoinClient.GetBlockHeaderHex(hash)
		if err != nil {
			return fmt.Errorf("unable to GetBlockHash from node for hash %s: %w", hash, err)
		}

		data, err := hex.DecodeString(*HeaderBytes)
		if err != nil {
			return fmt.Errorf("unable to decode header for block height %d: %w", 0, err)
		}

		fileData = append(fileData, data...)
	}

	for index := 1; index <= totalBlocks; index++ {
		if index%500 == 0 {
			time.Sleep(2 * time.Second)
		}

		height := int(lastHeight) + index
		hash, err := bitcoinClient.GetBlockHash(height)
		if err != nil {
			return fmt.Errorf("unable to GetBlockHash from node for block height %d : %w", height, err)
		}

		HeaderBytes, err := bitcoinClient.GetBlockHeaderHex(hash)
		if err != nil {
			return fmt.Errorf("unable to GetBlockHash from node for hash %s: %w", hash, err)
		}

		data, err := hex.DecodeString(*HeaderBytes)
		if err != nil {
			return fmt.Errorf("unable to decode header for block height %d: %w", height, err)
		}

		fileData = append(fileData, data...)
	}

	err = WriteToFile(blockFile, fileData)
	if err != nil {
		return fmt.Errorf("error writing %s: %w", blockFile, err)
	}

	common_logger.Log.Info("written to file", zap.String("file", blockFile))

	byteData := []byte(fmt.Sprint(endHeight))

	err = WriteToFile(lastHeightFile, byteData)
	if err != nil {
		return fmt.Errorf("error writing %s: %w", lastHeightFile, err)
	}

	return nil
}

func WriteToFile(filename string, data []byte) error {
	var f *os.File

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func(file *os.File) error {
		err := file.Close()
		if err != nil {
			return fmt.Errorf("can't close file %s: %w", filename, err)
		}
		return nil
	}(f)
	_, err = f.Write(data)
	return err
}

func ReadFromFile(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	defer func(file *os.File) error {
		err := file.Close()
		if err != nil {
			return fmt.Errorf("can't close file %s: %w", filename, err)
		}
		return nil
	}(f)

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return data, err
}

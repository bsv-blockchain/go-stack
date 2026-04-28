package bitcoin

import (
	"time"
	"unicode/utf8"

	gobitcoin "github.com/ordishs/go-bitcoin"
	"github.com/ordishs/gocore"
	"github.com/teranode-group/woc-api/redis"
)

// Client comment
type Client struct {
	bitcoind            *gobitcoin.Bitcoind
	bitcoindWithTimeout *gobitcoin.Bitcoind
	maxTxHexLength      int
}

const (
	KEY_NetworkInfo        = "GetNetworkInfo"
	KEY_BlockchainInfo     = "GetBlockchainInfo"
	KEY_NetTotals          = "GetNetTotals"
	KEY_MempoolInfo        = "GetMempoolInfo"
	KEY_MiningInfo         = "GetMiningInfo"
	KEY_Uptime             = "Uptime"
	KEY_PeerInfo           = "GetPeerInfo"
	KEY_AllChainTxStats    = "AllChainTXStats"
	KEY_ChainTips          = "GetChainTips"
	KEY_LatestBlockHeaders = "LatestBlockHeaders"
)

// New return a new bitcoin client
func New() (*Client, error) {

	bitcoinHost, ok := gocore.Config().Get("BSV_host")
	if !ok {
		logger.Fatal("Must have a bitcoind host setting")
	}
	bitcoinPort, ok := gocore.Config().GetInt("BSV_port")
	if !ok {
		logger.Fatal("Must have a bitcoind port setting")
	}
	bitcoinTimeout, ok := gocore.Config().GetInt("BSV_timeout")
	if !ok {
		logger.Fatal("Must have a bitcoind timout setting")
	}
	bitcoinUsername, ok := gocore.Config().Get("BSV_username")
	if !ok {
		logger.Fatal("Must have a bitcoind username setting")
	}
	bitcoinPassword, ok := gocore.Config().Get("BSV_password")
	if !ok {
		logger.Fatal("Must have a bitcoind password setting")
	}
	maxTxHexLength, ok := gocore.Config().GetInt("maxTxHexLength")
	if !ok {
		logger.Fatal("Must have a max tx hex length setting")
	}

	// two bitcoin clients, 1 using node timeout and other with custome timeout
	var err error
	bitcoind, err := gobitcoin.New(bitcoinHost, bitcoinPort, bitcoinUsername, bitcoinPassword, false)
	if err != nil {
		logger.Errorf("Unable to create bitocin client:", err)
	}

	bitcoindWithTimeout, err := gobitcoin.New(bitcoinHost, bitcoinPort, bitcoinUsername, bitcoinPassword, false, gobitcoin.WithTimeoutDuration(time.Duration(bitcoinTimeout)*time.Millisecond))
	if err != nil {
		logger.Errorf("Unable to create bitocin client:", err)
	}

	return &Client{bitcoind, bitcoindWithTimeout, maxTxHexLength}, nil
}

// GetBlockchainInfo comment
func (c *Client) GetBlockchainInfo() (info gobitcoin.BlockchainInfo, err error) {

	var cached gobitcoin.BlockchainInfo
	if redis.RedisClient.Enabled {
		err = redis.GetCachedValue(KEY_BlockchainInfo, &cached, nil)
	}

	if !redis.RedisClient.Enabled || err != nil {
		return c.bitcoindWithTimeout.GetBlockchainInfo()
	}

	return cached, nil
}

func (c *Client) GetBlockchainInfoNoCache() (info gobitcoin.BlockchainInfo, err error) {
	return c.bitcoindWithTimeout.GetBlockchainInfo()
}

// GetNetworkInfo comment
func (c *Client) GetNetworkInfo() (info gobitcoin.NetworkInfo, err error) {
	var cached gobitcoin.NetworkInfo
	if redis.RedisClient.Enabled {
		err = redis.GetCachedValue(KEY_NetworkInfo, &cached, nil)
	}
	if !redis.RedisClient.Enabled || err != nil {
		return c.bitcoindWithTimeout.GetNetworkInfo()
	}

	return cached, nil
}

// GetNetTotals comment
func (c *Client) GetNetTotals() (info gobitcoin.NetTotals, err error) {
	var cached gobitcoin.NetTotals
	if redis.RedisClient.Enabled {
		err = redis.GetCachedValue(KEY_NetTotals, &cached, nil)
	}

	if !redis.RedisClient.Enabled || err != nil {
		return c.bitcoindWithTimeout.GetNetTotals()
	}

	return cached, nil
}

// GetMempoolInfo comment
func (c *Client) GetMempoolInfo() (info gobitcoin.MempoolInfo, err error) {
	var cached gobitcoin.MempoolInfo
	if redis.RedisClient.Enabled {
		err = redis.GetCachedValue(KEY_MempoolInfo, &cached, nil)
	}
	if !redis.RedisClient.Enabled || err != nil {
		return c.bitcoindWithTimeout.GetMempoolInfo()
	}
	return cached, nil
}

// GetMiningInfo comment
func (c *Client) GetMiningInfo() (info gobitcoin.MiningInfo, err error) {
	var cached gobitcoin.MiningInfo
	if redis.RedisClient.Enabled {
		err = redis.GetCachedValue(KEY_MiningInfo, &cached, nil)
	}

	if !redis.RedisClient.Enabled || err != nil {
		return c.bitcoindWithTimeout.GetMiningInfo()
	}
	return cached, nil
}

// Uptime comment
func (c *Client) Uptime() (uptime uint64, err error) {
	var cached uint64
	if redis.RedisClient.Enabled {
		err = redis.GetCachedValue(KEY_Uptime, &cached, nil)
	}

	if !redis.RedisClient.Enabled || err != nil {
		return c.bitcoindWithTimeout.Uptime()
	}
	return cached, nil
}

// GetPeerInfo comment
func (c *Client) GetPeerInfo() (info gobitcoin.PeerInfo, err error) {
	var cached gobitcoin.PeerInfo
	if redis.RedisClient.Enabled {
		err = redis.GetCachedValue(KEY_PeerInfo, &cached, nil)
	}

	if !redis.RedisClient.Enabled || err != nil {
		return c.bitcoindWithTimeout.GetPeerInfo()
	}

	return cached, nil
}

// GetRawMempool comment
func (c *Client) GetRawMempool(details bool) (raw []byte, err error) {
	return c.bitcoind.GetRawMempool(details)
}

// GetMempoolAncestors comment
func (c *Client) GetMempoolAncestors(txid string, details bool) (raw []byte, err error) {
	return c.bitcoind.GetMempoolAncestors(txid, details)
}

// GetMempoolDescendants comment
func (c *Client) GetMempoolDescendants(txid string, details bool) (raw []byte, err error) {
	return c.bitcoind.GetMempoolDescendants(txid, details)
}

// GetRawNonFinalMempool
func (c *Client) GetRawNonFinalMempool() (raw []string, err error) {
	return c.bitcoind.GetRawNonFinalMempool()
}

// GetChainTips comment
func (c *Client) GetChainTips() (tips gobitcoin.ChainTips, err error) {

	var cached gobitcoin.ChainTips
	if redis.RedisClient.Enabled {
		err = redis.GetCachedValue(KEY_ChainTips, &cached, nil)
	}

	if !redis.RedisClient.Enabled || err != nil {
		return c.bitcoindWithTimeout.GetChainTips()
	}

	return cached, nil
}

// GetChainTxStats comment
func (c *Client) GetChainTxStats(blockcount int) (stats gobitcoin.ChainTXStats, err error) {
	return c.bitcoind.GetChainTxStats(blockcount)
}

func (c *Client) GetCachedChainTxStats() (stats []*gobitcoin.ChainTXStats, err error) {
	var cached []*gobitcoin.ChainTXStats
	if redis.RedisClient.Enabled {
		err = redis.GetCachedValue(KEY_AllChainTxStats, &cached, nil)
	}
	if err != nil {
		return nil, err
	}
	return cached, nil
}

func (c *Client) GetCachedLatestHeaders() (stats []*gobitcoin.BlockHeader, err error) {
	var cached []*gobitcoin.BlockHeader
	if redis.RedisClient.Enabled {
		err = redis.GetCachedValue(KEY_LatestBlockHeaders, &cached, nil)
	}
	if err != nil {
		return nil, err
	}
	return cached, nil
}

// ValidateAddress comment
func (c *Client) ValidateAddress(address string) (addr gobitcoin.Address, err error) {
	return c.bitcoindWithTimeout.ValidateAddress(address)
}

// GetHelp comment
func (c *Client) GetHelp() (j []byte, err error) {
	return c.bitcoindWithTimeout.GetHelp()
}

// GetTxOut comment
func (c *Client) GetTxOut(txHex string, vout int, includeMempool bool) (*gobitcoin.TXOut, error) {
	return c.bitcoindWithTimeout.GetTxOut(txHex, vout, includeMempool)
}

// GetBlock comment
func (c *Client) GetBlock(blockHash string) (block *gobitcoin.Block, err error) {
	return c.bitcoind.GetBlock(blockHash)
}

// GetBlockHeader comment
func (c *Client) GetBlockHeader(blockHash string) (block *gobitcoin.BlockHeader, err error) {
	return c.bitcoindWithTimeout.GetBlockHeader(blockHash)
}

// GetBlockHeaderAndCoinbase comment
func (c *Client) GetBlockHeaderAndCoinbase(blockHash string) (block *gobitcoin.BlockHeaderAndCoinbase, err error) {
	return c.bitcoindWithTimeout.GetBlockHeaderAndCoinbase(blockHash)
}

// GetRawTransaction comment
func (c *Client) GetRawTransaction(txID string, truncateVouts bool, removeHex bool) (rawTx *gobitcoin.RawTransaction, err error) {

	rt, err := c.bitcoind.GetRawTransaction(txID)
	if err != nil {
		return nil, err
	}
	if removeHex {
		rt.Hex = ""
	}

	if truncateVouts == true {
		for i, o := range rt.Vout {
			if len(o.ScriptPubKey.Hex) > c.maxTxHexLength {
				rt.Vout[i].ScriptPubKey.Hex = truncateStrings(o.ScriptPubKey.Hex, c.maxTxHexLength)
				rt.Vout[i].ScriptPubKey.ASM = truncateStrings(o.ScriptPubKey.ASM, c.maxTxHexLength)
				rt.Vout[i].ScriptPubKey.IsTruncated = true
			}
		}
	}

	return rt, nil
}

// GetRawTransactionHex comment
func (c *Client) GetRawTransactionHex(txID string) (rawTx *string, err error) {
	rt, err := c.bitcoind.GetRawTransactionHex(txID)
	if err != nil {
		return nil, err
	}

	return rt, nil
}

// SendRawTransaction comment
func (c *Client) SendRawTransaction(hex string) (txid string, err error) {
	return c.bitcoind.SendRawTransaction(hex)
}

// SendRawTransactionWithoutFeeCheck comment
func (c *Client) SendRawTransactionWithoutFeeCheck(hex string) (txid string, err error) {
	return c.bitcoind.SendRawTransactionWithoutFeeCheck(hex)
}

// DecodeRawTransaction comment
func (c *Client) DecodeRawTransaction(txHex string) (string, error) {
	return c.bitcoindWithTimeout.DecodeRawTransaction(txHex)
}

// GetBlockHash comment
func (c *Client) GetBlockHash(blockHeight int) (blockHash string, err error) {
	return c.bitcoindWithTimeout.GetBlockHash(blockHeight)
}

// GetBlockOverview comment
func (c *Client) GetBlockOverview(blockHash string) (block *gobitcoin.BlockOverview, err error) {
	return c.bitcoindWithTimeout.GetBlockOverview(blockHash)
}

// GetBestBlockHash comment
func (c *Client) GetBestBlockHash() (hash string, err error) {
	return c.bitcoindWithTimeout.GetBestBlockHash()
}

// GetMerkleProof
func (c *Client) GetMerkleProof(blockhash string, txID string) (proof *gobitcoin.MerkleProof, err error) {
	return c.bitcoindWithTimeout.GetMerkleProof(blockhash, txID)
}

func truncateStrings(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for !utf8.ValidString(s[:n]) {
		n--
	}
	return s[:n]
}

func (c *Client) GetAllChainTxStats() (stats []*gobitcoin.ChainTXStats, err error) {

	var cached []*gobitcoin.ChainTXStats
	if redis.RedisClient.Enabled {
		err = redis.GetCachedValue(KEY_AllChainTxStats, &cached, nil)
	}

	if !redis.RedisClient.Enabled || err != nil {

		chainInfo, err := c.bitcoind.GetBlockchainInfo()
		if err != nil {
			logger.Errorf("Unable to GetBlockchainInfo %+v\n", err)
			return nil, err
		}

		chainTxStatsIntervals := []int{144, 144 * 7, 144 * 30, 144 * 265, int(chainInfo.Blocks - 1)}

		for _, interval := range chainTxStatsIntervals {
			// Dont check for blockheights that exceed the current block height
			if interval >= int(chainInfo.Blocks) {
				continue
			}
			tss, err := c.bitcoind.GetChainTxStats(interval)
			if err != nil {
				logger.Errorf("getChainTxStats %+v\n", err)
				return nil, err
			}

			if tss.WindowBlockCount == int(chainInfo.Blocks-1) {
				tss.WindowTXCount = tss.TXCount
				tss.TXRate = float64(tss.TXCount) / float64(tss.WindowInterval)
			}

			cached = append(cached, &tss)
		}

	}

	return cached, nil

}

func StartNodeStatsCache() {

	//Create new client
	var err error
	bitcoinClient, err := New()
	if err != nil {
		logger.Errorf("Unable to connect to bitcoin", err)
	}

	if !redis.RedisClient.Enabled {
		logger.Info("Skipping Periodic Node Stats Cache because Redis Cache is not enabled")
		return
	}
	// defer bitcoinClient.Close()

	logger.Info("Starting Periodic Node Stats Caching")

	//instant first run and then every 10 seconds
	lastKnowBlockHash := ""
	ticker := time.NewTicker(10 * time.Second)
	isBusy := false
	skipCounter := 0
	maxSkipCounter := 5
	for ; true; <-ticker.C {
		//skip for 1 min if failed last time.
		if isBusy && skipCounter <= maxSkipCounter {
			logger.Warnf("Skipping NodeStatsCache update. isBusy:%+v, skipCounter:%+v/%+v\n", isBusy, skipCounter, maxSkipCounter)
			skipCounter++
			continue
		}
		isBusy = true
		skipCounter = 0
		conn := redis.RedisClient.ConnPool.Get()
		//GetBlockchainInfo c.bitcoind.GetBlockchainInfo()
		chainInfo, err := bitcoinClient.bitcoindWithTimeout.GetBlockchainInfo()
		if err != nil {
			logger.Errorf("Unable to CacheBlockchainInfo %+v\n", err)
			conn.Close()
			continue
		}
		err = redis.SetCacheValue(KEY_BlockchainInfo, chainInfo, conn)
		if err != nil {
			logger.Errorf("Unable to cache CacheBlockchainInfo %+v\n", err)
		}

		//GetNetworkInfo
		netInfo, err := bitcoinClient.bitcoindWithTimeout.GetNetworkInfo()
		if err != nil {
			logger.Errorf("Unable to CacheBlockchainInfo %+v\n", err)
			conn.Close()
			continue
		}
		err = redis.SetCacheValue(KEY_NetworkInfo, netInfo, conn)
		if err != nil {
			logger.Errorf("Unable to cache CacheBlockchainInfo %+v\n", err)
		}

		//GetNetTotals
		netTotals, err := bitcoinClient.bitcoindWithTimeout.GetNetTotals()
		if err != nil {
			logger.Errorf("Unable to GetNetTotals %+v\n", err)
			conn.Close()
			continue
		}
		err = redis.SetCacheValue(KEY_NetTotals, netTotals, conn)
		if err != nil {
			logger.Errorf("Unable to cache GetNetTotals %+v\n", err)
		}

		//GetMempoolInfo
		memInfo, err := bitcoinClient.bitcoindWithTimeout.GetMempoolInfo()
		if err != nil {
			logger.Errorf("Unable to GetMempoolInfo %+v\n", err)
			conn.Close()
			continue
		}
		err = redis.SetCacheValue(KEY_MempoolInfo, memInfo, conn)
		if err != nil {
			logger.Errorf("Unable to cache GetMempoolInfo %+v\n", err)
		}

		//GetMiningInfo
		miningInfo, err := bitcoinClient.bitcoindWithTimeout.GetMiningInfo()
		if err != nil {
			logger.Errorf("Unable to GetMiningInfo %+v\n", err)
			conn.Close()
			continue
		}
		err = redis.SetCacheValue(KEY_MiningInfo, miningInfo, conn)
		if err != nil {
			logger.Errorf("Unable to cache GetMiningInfo %+v\n", err)
		}

		//Uptime
		uptime, err := bitcoinClient.bitcoindWithTimeout.Uptime()
		if err != nil {
			logger.Errorf("Unable to Uptime %+v\n", err)
			conn.Close()
			continue
		}
		err = redis.SetCacheValue(KEY_Uptime, uptime, conn)
		if err != nil {
			logger.Errorf("Unable to cache Uptime %+v\n", err)
		}

		//GetPeerInfo
		peerInfo, err := bitcoinClient.bitcoindWithTimeout.GetPeerInfo()
		if err != nil {
			logger.Errorf("Unable to GetPeerInfo %+v\n", err)
			conn.Close()
			continue
		}
		err = redis.SetCacheValue(KEY_PeerInfo, peerInfo, conn)
		if err != nil {
			logger.Errorf("Unable to cache GetPeerInfo %+v\n", err)
		}

		// Only do these when we have a new block
		if lastKnowBlockHash != chainInfo.BestBlockHash {

			chainTxStatsIntervals := []int{144, 144 * 7, 144 * 30, 144 * 365, int(chainInfo.Blocks - 1)}
			css := []*gobitcoin.ChainTXStats{}
			cacheChainStats := true
			for _, interval := range chainTxStatsIntervals {
				// Dont check for blockheights that exceed the current block height
				if interval >= int(chainInfo.Blocks) {
					continue
				}
				tss, err := bitcoinClient.bitcoind.GetChainTxStats(interval)
				if err != nil {
					logger.Errorf("Unable to GetChainTxStats %+v\n", err)
					cacheChainStats = false
					break
				}

				if tss.WindowBlockCount == int(chainInfo.Blocks-1) {
					tss.WindowTXCount = tss.TXCount
					tss.TXRate = float64(tss.TXCount) / float64(tss.WindowInterval)
				}
				css = append(css, &tss)
			}

			if cacheChainStats {
				err = redis.SetCacheValue(KEY_AllChainTxStats, css, conn)
				if err != nil {
					logger.Errorf("Unable to cache value for GetChainTxStatsAllInterval %+v\n", err)
				}
			}

			//GetChainTips
			tips, err := bitcoinClient.bitcoindWithTimeout.GetChainTips()
			if err != nil {
				logger.Errorf("Unable to GetChainTips %+v\n", err)
				conn.Close()
				continue
			}
			err = redis.SetCacheValue(KEY_ChainTips, tips, conn)
			if err != nil {
				logger.Errorf("Unable to cache value for GetChainTips %+v\n", err)
			}

			//Get latest 10 headers
			bestHash, err := bitcoinClient.bitcoindWithTimeout.GetBestBlockHash()
			if err != nil {
				logger.Errorf("getBestBlockHash: %v", err)
				return
			}
			var headers []*gobitcoin.BlockHeader
			hash := bestHash
			for len(headers) < 10 && hash != "" {
				hdr, err := bitcoinClient.bitcoindWithTimeout.GetBlockHeader(hash)
				if err != nil {
					logger.Errorf("GetBlockHeader(%s): %v", hash, err)
					break
				}
				headers = append(headers, hdr)
				hash = hdr.PreviousBlockHash
			}

			if len(headers) == 10 {
				if err := redis.SetCacheValue(KEY_LatestBlockHeaders, headers, conn); err != nil {
					logger.Errorf("cache latest block headers: %v", err)
				}
			}

		}

		lastKnowBlockHash = chainInfo.BestBlockHash
		conn.Flush()
		conn.Close()
		isBusy = false
	}
}

func (c *Client) GetBlockHeaderHex(hash string) (header *string, err error) {
	return c.bitcoindWithTimeout.GetBlockHeaderHex(hash)
}

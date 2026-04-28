package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/ordishs/gocore"
	"github.com/teranode-group/woc-api/bstore"
	"github.com/teranode-group/woc-api/configs"
	"github.com/teranode-group/woc-api/mongocache"
	"github.com/teranode-group/woc-api/p2pservice"
	"github.com/teranode-group/woc-api/redis"
	"github.com/teranode-group/woc-api/serverfiber"
	"github.com/teranode-group/woc-api/sockets"
	"github.com/teranode-group/woc-api/tokens"
	"github.com/teranode-group/woc-api/utxosmempool"
	"github.com/teranode-group/woc-api/utxostore"
)

var isNodeResponding = true
var isBStoreOutOfSync = false
var isUtxoStoreOutOfSync = false
var outOfSyncBStoreStartTime = time.Now()
var outOfSyncUtxoStoreStartTime = time.Now()
var noRespNodeStartTime = time.Now()

const (
	STATUS_OK       string = "OK"
	STATUS_FAILED   string = "Failed"
	STATUS_UNKNOWN  string = "Unknown"
	STATUS_DISABLED string = "Disabled"
	STATUS_DEGRADED string = "Degraded"
	STATUS_SKIPPED  string = "Skipped"
	STATUS_TRUE     string = "True"
	STATUS_FALSE    string = "False"
	STATUS_NORMAL   string = "Normal"
	STATUS_HIGH     string = "High"
	REASON_TXCOUNT  string = "Transactions Count exceeding Threshold: " // don't change as uptimeRobot using these words for alerting
	REASON_SIZE     string = "Size in Bytes exceeding Threshold: "      // don't change as uptimeRobot using these words for alerting
)

type HealthStatusResp struct {
	Overall       string                           `json:"overall"`
	Src           string                           `json:"src"`
	Failover      []string                         `json:"failover,omitempty"`
	BitcoinNode   string                           `json:"bNode"`
	MongoDB       string                           `json:"mDB"`
	ElectrumX     string                           `json:"eleX"`
	BStore        *BStoreStatus                    `json:"bStore"`
	UtxoStore     *UtxoStatus                      `json:"utxoStore"`
	Mempool       *MempoolStatus                   `json:"mempool"`
	ExchangeRate  *ExchangeRateStatus              `json:"exchangeRate"`
	WocStats      *WocStatsStatus                  `json:"wocStats"`
	RedisStats    *RedisPoolStats                  `json:"redisPoolStats"`
	P2pService    *p2pservice.Status               `json:"p2pService"`
	WocChainStats *WocChainStatsStatus             `json:"wocChainStats"`
	UtxosMempool  *utxosmempool.UtxosMempoolStatus `json:"utxosMempool"`
	Sockets       *sockets.Status                  `json:"sockets"`
	TokenMempool  *tokens.TokenMempoolStatus       `json:"tokenMempool"`
	TokenService  *tokens.TokenServiceStatus       `json:"tokenService"`
}

type MempoolStatus struct {
	UsageStatus     string `json:"usageStatus"`
	HighUsageReason string `json:"highUsageReason,omitempty"`
	TxCount         uint64 `json:"mempoolTxCount"`
	SizeInBytes     uint64 `json:"sizeInBytes"`
}

type BStoreStatus struct {
	NodeHeight       uint64 `json:"nodeHeight"`
	BStoreHeight     uint64 `json:"bstoreHeight"`
	SyncStatus       string `json:"syncStatus"`
	ReadStatus       string `json:"readStatus"`
	CatchupMode      string `json:"inCatchupMode"`
	ProcessingBlock  string `json:"isProcessingBlock"`
	EndpointOk       string `json:"endpointOk"`
	EndpointDuration string `json:"endpointDuration"`
}

type UtxoStatus struct {
	NodeHeight      uint64 `json:"nodeHeight"`
	UtxoStoreHeight uint64 `json:"utxoStoreHeight"`
	SyncStatus      string `json:"syncStatus"`
	ReadStatus      string `json:"readStatus"`
	CatchupMode     string `json:"inCatchupMode"`
	ProcessingBlock string `json:"isProcessingBlock"`
	UpTime          string `json:"UpTime"`
}

type ExchangeRateStatus struct {
	Status                     string `json:"status,omitempty"`
	IsLatestDailyExchangeRate  bool   `json:"isLatestDailyExchangeRate,omitempty"`
	IsLatestExchangeRate       bool   `json:"isLatestExchangeRate,omitempty"`
	LastDailyExchangeRateDate  string `json:"lastDailyExchangeRateDate,omitempty"`
	LastLatestExchangeRateDate string `json:"lastLatestExchangeRateDate,omitempty"`
}

type WocChainStatsStatus struct {
	Status string `json:"status,omitempty"`
	Hash   string `json:"hash"`
	Height uint64 `json:"height"`
}

type WocStatsStatus struct {
	Status            string `json:"status"`
	LastBlockHeight   uint64 `json:"lastBlockHeight"`
	LastBlockHash     string `json:"lastBlockHash"`
	InCatchupMode     bool   `json:"inCatchupMode"`
	IsProcessingBlock bool   `json:"isProcessingBlock"`
}

type RedisPoolStats struct {
	// ActiveCount is the number of connections in the pool. The count includes
	// idle connections and connections in use.
	ActiveCount int
	// IdleCount is the number of idle connections in the pool.
	IdleCount int

	// WaitCount is the total number of connections waited for.
	// This value is currently not guaranteed to be 100% accurate.
	WaitCount int64

	// WaitDuration is the total time blocked waiting for a new connection.
	// This value is currently not guaranteed to be 100% accurate.
	WaitDuration time.Duration
}

func healthCheck(w http.ResponseWriter, r *http.Request) {

	checkBStoreReadable := gocore.Config().GetBool("check_bStoreReadable", true)
	checkBStoreSynced := gocore.Config().GetBool("check_bStoreSynced", true)
	checkElectrumX := gocore.Config().GetBool("check_electrumX", true)
	checkBitcoinNode := gocore.Config().GetBool("check_bitcoinNode", true)
	checkMongoDB := gocore.Config().GetBool("check_mongoDB", true)
	checkWocExchangeRate := gocore.Config().GetBool("check_wocExchangeRate", true)
	checkWocStats := gocore.Config().GetBool("check_wocStats", true)
	checkP2pService := gocore.Config().GetBool("check_p2pService", true)
	checkSockets := gocore.Config().GetBool("check_sockets", true)
	checkSocketsUrl, _ := gocore.Config().Get("check_sockets_url")
	checkTokenMempool := gocore.Config().GetBool("check_tokenMempool", true)
	checkTokenService := gocore.Config().GetBool("check_tokenService", true)
	checkWocChainStats := gocore.Config().GetBool("check_wocChainStats", true)
	checkUtxoStore := gocore.Config().GetBool("check_utxoStore", true)
	checkUtxosMempool := gocore.Config().GetBool("check_utxosMempool", true)
	mempoolHighAlertDataLimit, _ := gocore.Config().GetInt("mempool_high_alert_size_limit", 1000000000)   //1 GB
	mempoolHighAlertTxCountLimit, _ := gocore.Config().GetInt("mempool_high_alert_txcount_limit", 100000) //100k

	loc, _ := time.LoadLocation("UTC")

	//Mongo
	mongoCacheEnabled := gocore.Config().GetBool("mongoCache", false)
	// resp := &HealthStatus{}

	resp := &HealthStatusResp{
		Overall:     STATUS_UNKNOWN,
		Src:         gocore.Config().GetContext(),
		BitcoinNode: STATUS_UNKNOWN,
		MongoDB:     STATUS_UNKNOWN,
		ElectrumX:   STATUS_UNKNOWN,
		BStore: &BStoreStatus{NodeHeight: 0, BStoreHeight: 0, SyncStatus: STATUS_UNKNOWN,
			ReadStatus: STATUS_UNKNOWN, CatchupMode: STATUS_UNKNOWN, ProcessingBlock: STATUS_UNKNOWN, EndpointOk: STATUS_UNKNOWN, EndpointDuration: STATUS_UNKNOWN},
		UtxoStore: &UtxoStatus{NodeHeight: 0, UtxoStoreHeight: 0, SyncStatus: STATUS_UNKNOWN,
			ReadStatus: STATUS_UNKNOWN, CatchupMode: STATUS_UNKNOWN, ProcessingBlock: STATUS_UNKNOWN},
		Mempool:      &MempoolStatus{UsageStatus: STATUS_UNKNOWN},
		UtxosMempool: &utxosmempool.UtxosMempoolStatus{Status: STATUS_UNKNOWN}}

	if mongoCacheEnabled {
		if checkMongoDB {
			status := mongocache.MongoHealthCheck()
			if status {
				resp.MongoDB = STATUS_OK
			} else {
				resp.MongoDB = STATUS_FAILED
			}
		} else {
			resp.MongoDB = STATUS_SKIPPED
		}
	} else {
		resp.MongoDB = STATUS_DISABLED
	}

	//bitcoin node
	if checkBitcoinNode {

		if isNodeResponding {
			info, err := bitcoinClient.GetBlockchainInfoNoCache()

			if err != nil {
				resp.BitcoinNode = STATUS_FAILED
				noRespNodeStartTime = time.Now().In(loc)
				isNodeResponding = true
			} else {
				resp.BitcoinNode = STATUS_OK
				resp.BStore.NodeHeight = uint64(info.Blocks)
				resp.UtxoStore.NodeHeight = uint64(info.Blocks)

				//mempool high usage alert
				mempoolInfo, err := bitcoinClient.GetMempoolInfo()
				if err == nil {

					resp.Mempool.SizeInBytes = uint64(mempoolInfo.Bytes)
					resp.Mempool.TxCount = uint64(mempoolInfo.Size)

					if mempoolInfo.Size > mempoolHighAlertTxCountLimit ||
						mempoolInfo.Bytes > mempoolHighAlertDataLimit {

						resp.Mempool.UsageStatus = STATUS_HIGH

						if mempoolInfo.Bytes > mempoolHighAlertDataLimit {
							resp.Mempool.HighUsageReason = REASON_SIZE + strconv.FormatInt(int64(mempoolHighAlertDataLimit), 10)
						} else {
							resp.Mempool.HighUsageReason = REASON_TXCOUNT + strconv.FormatInt(int64(mempoolHighAlertTxCountLimit), 10)
						}

					} else {
						resp.Mempool.UsageStatus = STATUS_NORMAL
					}
				}
			}

		} else {
			delay, _ := gocore.Config().GetInt("nodeRetryDelayInMins", 2)
			logger.Warnf("Skipping Node status check for total %d mins", delay)
			now := time.Now().In(loc)
			diff := now.Sub(noRespNodeStartTime)

			if int(diff.Minutes()) >= delay {
				isNodeResponding = false
			}
			resp.BitcoinNode = STATUS_FAILED
		}

	} else {
		resp.BitcoinNode = STATUS_SKIPPED
	}

	//bstore
	if bstore.IsEnabled() {

		if checkBStoreReadable {
			status, height, _, processingBlock, catchupMode, txEndpointOk, txEndpointDuration := bstore.BStoreHealthCheck()
			if status {
				resp.BStore.ReadStatus = STATUS_OK
				resp.BStore.BStoreHeight = height

				if txEndpointOk {
					resp.BStore.EndpointOk = STATUS_TRUE
				} else {
					resp.BStore.EndpointOk = STATUS_FALSE
				}

				if txEndpointDuration > 0 {
					resp.BStore.EndpointDuration = txEndpointDuration.String()
				}

				if processingBlock {
					resp.BStore.ProcessingBlock = STATUS_TRUE
				} else {
					resp.BStore.ProcessingBlock = STATUS_FALSE
				}

				if catchupMode {
					resp.BStore.CatchupMode = STATUS_TRUE
				} else {
					resp.BStore.CatchupMode = STATUS_FALSE
				}

				if checkBStoreSynced {
					//check if it synced
					if resp.BitcoinNode == STATUS_OK && resp.BStore.NodeHeight != height {
						// only save time first time we detect out of sync
						if !isBStoreOutOfSync {
							outOfSyncBStoreStartTime = time.Now().In(loc)
						}
						isBStoreOutOfSync = true
					} else {
						isBStoreOutOfSync = false
						resp.BStore.SyncStatus = STATUS_OK
					}

					//Delay in mins before we set bstore SyncStatus to Failed
					delay, _ := gocore.Config().GetInt("bstoreSyncCheckerDelayInMins", 5)
					now := time.Now().In(loc)
					diff := now.Sub(outOfSyncBStoreStartTime)

					if isBStoreOutOfSync && int(diff.Minutes()) >= delay {
						resp.BStore.SyncStatus = STATUS_FAILED
					}
				} else {
					resp.BStore.SyncStatus = STATUS_SKIPPED
				}

			} else {
				resp.BStore.ReadStatus = STATUS_FAILED
			}
		} else {
			resp.BStore.ReadStatus = STATUS_SKIPPED
			resp.BStore.SyncStatus = STATUS_SKIPPED
		}
	} else {
		resp.BStore.ReadStatus = STATUS_DISABLED
	}

	//electrumX
	if wrappedEleC.electrumClient != nil {
		if checkElectrumX {
			err := wrappedEleC.electrumClient.PingOrError()
			if err == nil {
				resp.ElectrumX = STATUS_OK
			} else {
				resp.ElectrumX = STATUS_FAILED
			}
		} else {
			resp.ElectrumX = STATUS_SKIPPED
		}

	} else {
		if checkElectrumX {
			resp.ElectrumX = STATUS_FAILED
		} else {
			resp.ElectrumX = STATUS_SKIPPED
		}
	}

	//utxo-store
	if utxostore.IsEnabled() {

		if checkUtxoStore {
			status, height, _, processingBlock, catchupMode, uptime := utxostore.HealthCheck()
			resp.UtxoStore.UpTime = uptime
			if status {
				resp.UtxoStore.ReadStatus = STATUS_OK
				resp.UtxoStore.UtxoStoreHeight = height

				if processingBlock {
					resp.UtxoStore.ProcessingBlock = STATUS_TRUE
				} else {
					resp.UtxoStore.ProcessingBlock = STATUS_FALSE
				}

				if catchupMode {
					resp.UtxoStore.CatchupMode = STATUS_TRUE
				} else {
					resp.UtxoStore.CatchupMode = STATUS_FALSE
				}

				//check if it synced
				if resp.BitcoinNode == STATUS_OK && resp.UtxoStore.NodeHeight != height {
					// only save time first time we detect out of sync
					if !isUtxoStoreOutOfSync {
						outOfSyncUtxoStoreStartTime = time.Now().In(loc)
					}
					isUtxoStoreOutOfSync = true
				} else {
					isUtxoStoreOutOfSync = false
					resp.UtxoStore.SyncStatus = STATUS_OK
				}

				//Delay in mins before we set utxostore SyncStatus to Failed
				delay, _ := gocore.Config().GetInt("utxoStoreSyncCheckerDelayInMins", 5)
				now := time.Now().In(loc)
				diff := now.Sub(outOfSyncUtxoStoreStartTime)

				if isUtxoStoreOutOfSync && int(diff.Minutes()) >= delay {
					resp.UtxoStore.SyncStatus = STATUS_FAILED
				}

			} else {
				resp.UtxoStore.ReadStatus = STATUS_FAILED
			}
		} else {
			resp.UtxoStore.ReadStatus = STATUS_SKIPPED
			resp.UtxoStore.SyncStatus = STATUS_SKIPPED
		}
	} else {
		resp.UtxoStore.ReadStatus = STATUS_DISABLED
	}

	// utxos-mempool
	if utxosmempool.IsEnabled() {
		if checkUtxosMempool {
			status, err := utxosmempool.HealthCheck()
			if err == nil {
				resp.UtxosMempool = status
				resp.UtxosMempool.Status = STATUS_OK
			}
		}
	} else {
		resp.UtxosMempool.Status = STATUS_DISABLED
	}

	if checkWocExchangeRate {
		ctx, cancelFunc := context.WithTimeout(r.Context(), time.Second)
		defer cancelFunc()
		wocExchangeRateHealthResponse, err := serverfiber.WocExchangeRateHealthCheck(ctx)
		resp.ExchangeRate = &ExchangeRateStatus{}
		if err != nil {
			resp.ExchangeRate.Status = STATUS_FAILED
		} else {
			resp.ExchangeRate.Status = STATUS_OK
			resp.ExchangeRate.IsLatestExchangeRate = wocExchangeRateHealthResponse.IsLatestDailyExchangeRate
			resp.ExchangeRate.IsLatestDailyExchangeRate = wocExchangeRateHealthResponse.IsLatestDailyExchangeRate
			resp.ExchangeRate.LastDailyExchangeRateDate = wocExchangeRateHealthResponse.LastDailyExchangeRateDate
			resp.ExchangeRate.LastLatestExchangeRateDate = wocExchangeRateHealthResponse.LastLatestExchangeRateDate
		}
	}

	if checkP2pService {
		ctx, cancelFunc := context.WithTimeout(r.Context(), time.Second)
		defer cancelFunc()
		p2pServiceResponse, err := p2pservice.HealthCheck(ctx, configs.Settings.P2pServiceAddress)
		resp.P2pService = &p2pservice.Status{}
		if err == nil {
			resp.P2pService = p2pServiceResponse
			if resp.P2pService.Status != STATUS_OK {
				resp.P2pService.Status = STATUS_FAILED
			}
		} else {
			resp.P2pService.Status = STATUS_FAILED
		}
	} else {
		resp.P2pService.Status = STATUS_DISABLED
	}

	// woc-sockets-v2
	resp.Sockets = getSocketsHealthCheck(checkSockets, checkSocketsUrl)
	// token-mempool
	resp.TokenMempool = getTokenMempoolHealthCheck(checkTokenMempool)
	// token-service
	resp.TokenService = getTokenServiceHealthCheck(checkTokenService)

	if checkWocChainStats {
		resp.WocChainStats = &WocChainStatsStatus{}
		data, _ := getLatestBlockFromWocChainStats()
		resp.WocChainStats = &data
	}

	if checkWocStats {
		ctx, cancelFunc := context.WithTimeout(r.Context(), time.Second)
		defer cancelFunc()

		wocStatsHealthResponse, err := serverfiber.WocStatsHealthCheck(ctx)
		resp.WocStats = &WocStatsStatus{}
		if err != nil {
			resp.WocStats.Status = STATUS_FAILED
		} else {
			resp.WocStats.Status = STATUS_OK
			resp.WocStats.LastBlockHeight = wocStatsHealthResponse.LastBlockHeight
			resp.WocStats.LastBlockHash = wocStatsHealthResponse.LastBlockHash
			resp.WocStats.IsProcessingBlock = wocStatsHealthResponse.IsProcessingBlock
			resp.WocStats.InCatchupMode = wocStatsHealthResponse.IsCatchingUp

		}
	}

	if redis.RedisClient.Enabled && redis.RedisClient.ConnPool != nil {
		poolStats := redis.RedisClient.ConnPool.Stats()
		resp.RedisStats = &RedisPoolStats{}
		resp.RedisStats.ActiveCount = poolStats.ActiveCount
		resp.RedisStats.IdleCount = poolStats.IdleCount
		resp.RedisStats.WaitCount = poolStats.WaitCount
		resp.RedisStats.WaitDuration = poolStats.WaitDuration
	}

	// check if we have any failover services
	failover := make([]string, 0)

	if resp.UtxosMempool != nil && resp.UtxosMempool.Status == STATUS_FAILED {
		failover = append(failover, "utxosMempool")
	}
	if resp.UtxoStore != nil && resp.UtxoStore.ReadStatus == STATUS_FAILED {
		failover = append(failover, "utxoStoreRead")
	}
	if resp.UtxoStore != nil && resp.UtxoStore.SyncStatus == STATUS_FAILED {
		failover = append(failover, "utxoStoreSync")
	}
	if resp.ExchangeRate != nil && resp.ExchangeRate.Status == STATUS_FAILED {
		failover = append(failover, "exchangeRate")
	}
	if resp.BStore != nil && resp.BStore.ReadStatus == STATUS_FAILED {
		failover = append(failover, "bstoreRead")
	}
	if resp.BStore != nil && resp.BStore.SyncStatus == STATUS_FAILED {
		failover = append(failover, "bstoreSync")
	}
	if resp.BitcoinNode == STATUS_FAILED {
		failover = append(failover, "bitcoinNode")
	}
	if resp.MongoDB == STATUS_FAILED {
		failover = append(failover, "mongoDB")
	}

	if len(failover) > 0 {
		resp.Failover = failover
		resp.Overall = STATUS_DEGRADED
	} else {
		resp.Overall = STATUS_OK
	}

	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func getLatestBlockFromWocChainStats() (WocChainStatsStatus, error) {
	var data WocChainStatsStatus
	data.Status = STATUS_OK

	wocChainStatsUrl, ok := gocore.Config().Get("woc_chain_stats_url")
	if !ok {
		return data, errors.New("unbale to get woc_chain_stats_url")
	}

	wocChainStatsClient := http.Client{
		Timeout: time.Second * 3,
	}

	req, err := http.NewRequest(http.MethodGet, wocChainStatsUrl+"/latest-block", nil)
	if err != nil {
		return data, err
	}

	res, err := wocChainStatsClient.Do(req)

	if err != nil {
		return data, err
	}

	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)

	if err != nil {
		return data, err
	}

	err = json.Unmarshal(body, &data)

	if err != nil {
		return data, err
	}

	return data, nil

}

func getSocketsHealthCheck(isCheckEnabled bool, healthStatusUrl string) *sockets.Status {
	if !isCheckEnabled || healthStatusUrl == "" {
		return &sockets.Status{Status: STATUS_DISABLED}
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second)
	defer cancelFunc()
	resp, err := sockets.HealthCheck(ctx, healthStatusUrl)
	if err != nil {
		return &sockets.Status{Status: STATUS_FAILED}
	}
	resp.Status = STATUS_OK

	return &resp
}

func getTokenMempoolHealthCheck(isCheckEnabled bool) *tokens.TokenMempoolStatus {
	if !isCheckEnabled {
		return &tokens.TokenMempoolStatus{Status: STATUS_DISABLED}
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second)
	defer cancelFunc()
	resp, err := tokens.TokenMempoolHealthCheck(ctx)
	if err != nil {
		return &tokens.TokenMempoolStatus{Status: STATUS_FAILED}
	}
	resp.Status = STATUS_OK

	return &resp
}

func getTokenServiceHealthCheck(isCheckEnabled bool) *tokens.TokenServiceStatus {
	if !isCheckEnabled {
		return &tokens.TokenServiceStatus{Status: STATUS_DISABLED}
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second)
	defer cancelFunc()
	resp, err := tokens.TokenServiceHealthCheck(ctx)
	if err != nil {
		return &tokens.TokenServiceStatus{Status: STATUS_FAILED}
	}
	resp.Status = STATUS_OK

	return &resp
}

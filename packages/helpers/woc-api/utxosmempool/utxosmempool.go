package utxosmempool

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ordishs/gocore"
	"github.com/teranode-group/common"
	utxos_mempool "github.com/teranode-group/proto/utxos-mempool"
	"github.com/teranode-group/woc-api/redis"
	"google.golang.org/grpc"
)

var logger = gocore.Log("woc-api")

type UtxosMempoolStatus struct {
	Status                 string `json:"status"`
	NodeMemPoolSize        uint64 `json:"nodeMemPoolSize"`
	OurMemPoolSize         uint64 `json:"utxosMemPoolSize"`
	NewTxChannelSize       uint64 `json:"newTxChannelSize"`
	DiscardedTxChannelSize uint64 `json:"discardedTxChannelSize"`
	RemovedTxChannelSize   uint64 `json:"removedTxChannelSize"`
	QueryNodeSkipCounter   uint64 `json:"queryNodeSkipCounter"`
	QueryNodeInProgress    bool   `json:"queryNodeInProgress"`
	BstoreEnabled          bool   `json:"bstoreEnabled"`
	UpTime                 string `json:"UpTime"`
}

const (
	KEY_mempoolStats = "GetMempoolStats"
)

var (
	utxosMempoolConnOnce sync.Once
	utxosMempoolConn     *grpc.ClientConn
	utxosMempoolConnErr  error
)

var utxosMempoolEnabled bool
var mempoolStatsCacheOverideDuration int

func init() {
	utxosMempoolEnabled = gocore.Config().GetBool("utxosMempoolEnabled", true)
	mempoolStatsCacheOverideDuration, _ = gocore.Config().GetInt("mempoolStatsCacheOverideDuration", 5)

	if utxosMempoolEnabled {
		logger.Info("utxosMempool is Enabled")
	} else {
		logger.Info("utxosMempool is Disabled")
	}
}

func getUtxosMempoolConn() (*grpc.ClientConn, error) {
	utxosMempoolConnOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		utxosMempoolConn, utxosMempoolConnErr = common.GetGRPCConnection(ctx, "utxosMempool")
	})
	return utxosMempoolConn, utxosMempoolConnErr
}

func IsEnabled() bool {
	return utxosMempoolEnabled
}

func HealthCheck() (status *UtxosMempoolStatus, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, err := getUtxosMempoolConn()
	if err != nil {
		logger.Errorf("Failed to connect utxosMempool %+v", err)
		return nil, err
	}

	utxosMempoolClient := utxos_mempool.NewHealthClient(conn)

	resp, err := utxosMempoolClient.Check(ctx, &utxos_mempool.HealthCheckRequest{})

	if err != nil {
		return nil, err
	}

	return &UtxosMempoolStatus{NodeMemPoolSize: resp.NodeMempoolSize,
			OurMemPoolSize:         resp.OurMempoolSize,
			NewTxChannelSize:       resp.NewtxChannelSize,
			DiscardedTxChannelSize: resp.DiscardedtxChannelSize,
			RemovedTxChannelSize:   resp.RemovedtxChannelSize,
			QueryNodeSkipCounter:   resp.QuerySkipCounter,
			QueryNodeInProgress:    resp.QueryNodeInProgress,
			BstoreEnabled:          resp.BstoreEnabled != nil && *resp.BstoreEnabled,
			UpTime:                 resp.TimeRunning,
		},
		nil
}

func GetMempoolBalanceByScriptHash(hash string) (blockDetails *utxos_mempool.GetBalanceResponse, err error) {

	wait, err := time.ParseDuration("2s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxosMempoolConn()
	if err != nil {
		logger.Errorf("GetBalanceByScriptHash - Unable to connect utxosMempool")
		return nil, fmt.Errorf("Error: %+v", err)
	}

	utxosMempoolClient := utxos_mempool.NewUTXOSMempoolClient(conn)

	resBalance, err := utxosMempoolClient.GetBalance(ctx, &utxos_mempool.GetBalanceRequest{
		Scripthash: hash,
	})

	if err != nil && !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "unknown") {
		logger.Errorf("GetBalanceByScriptHash - Unable to GetBalance for  %s , %+v", hash, err)
		return nil, fmt.Errorf("error: %+v", err)
	} else if resBalance == nil {
		//patch to fix live issue as utxos-mempool changed response contract
		return &utxos_mempool.GetBalanceResponse{
			ScripthashMempool: &utxos_mempool.ScriptHashDetailsMempool{
				TotalTxsQty: 0,
				Unspent: &utxos_mempool.ScriptHashUnspent{
					Satoshis: 0,
					Qty:      0,
				},
			},
		}, nil
	}

	return resBalance, nil
}

func GetMempoolHistoryByScriptHash(hash string, pageSize int32, token string, afterBlockHeight uint32, order int) (blockDetails *utxos_mempool.ListScriptHashHistoryResponse, err error) {

	wait, err := time.ParseDuration("2s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxosMempoolConn()
	if err != nil {
		logger.Errorf("GetHistoryByScriptHash - Unable to connect utxosMempool")
		return nil, fmt.Errorf("error: %+v", err)
	}

	utxosMempoolClient := utxos_mempool.NewUTXOSMempoolClient(conn)

	resHistory, err := utxosMempoolClient.ListScriptHashHistory(ctx, &utxos_mempool.ListScriptHashHistoryRequest{
		Scripthash: hash,
		PageSize:   pageSize,
		PageToken:  token,
		Sort:       utxos_mempool.Sort(order),
	})

	if err != nil && !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "unknown") {
		logger.Errorf("GetHistoryByScriptHash - Unable to ListScriptHashHistory for  %s", hash)
		return nil, fmt.Errorf("error: %+v", err)
	} else if resHistory == nil {
		//patch to fix live issue as utxos-mempool changed response contract
		return &utxos_mempool.ListScriptHashHistoryResponse{
			MempoolTransactions: []*utxos_mempool.MempoolTransaction{},
		}, nil
	}

	return resHistory, nil

}

func HasHistoryInMempool(hash string) (bool, error) {

	wait, err := time.ParseDuration("1s")
	if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxosMempoolConn()
	if err != nil {
		logger.Errorf("GetHistoryByScriptHash - Unable to connect utxosMempool")
		return false, fmt.Errorf("error: %+v", err)
	}

	utxosMempoolClient := utxos_mempool.NewUTXOSMempoolClient(conn)

	resHistory, err := utxosMempoolClient.ListScriptHashHistory(ctx, &utxos_mempool.ListScriptHashHistoryRequest{
		Scripthash: hash,
		PageSize:   1,
		PageToken:  "",
		Sort:       utxos_mempool.Sort(0), //asc
	})

	if err == nil && len(resHistory.MempoolTransactions) > 0 {
		return true, nil
	}

	return false, nil
}

func GetMempoolUnspentByScriptHash(hash string, pageSize int32, token string, debugMode bool) (blockDetails *utxos_mempool.ListScriptHashUnspentResponse, err error) {

	wait, err := time.ParseDuration("3s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxosMempoolConn()
	if err != nil {
		logger.Errorf("GetConfirmedUnspentByScriptHash - Unable to connect utxosMempool")
		return nil, fmt.Errorf("error: %+v", err)
	}

	utxosMempoolClient := utxos_mempool.NewUTXOSMempoolClient(conn)

	resUnspent, err := utxosMempoolClient.ListScriptHashUnspent(ctx, &utxos_mempool.ListScriptHashUnspentRequest{
		Scripthash: hash,
		PageSize:   pageSize,
		PageToken:  token,
		Debug:      debugMode,
	})

	if err != nil && !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "unknown") {
		logger.Errorf("GetConfirmedUnspentByScriptHash - Unable to ListScriptHashUnspent for  %s", hash)
		return nil, fmt.Errorf("error: %+v", err)
	} else if resUnspent == nil {
		//patch to fix live issue as utxos-mempool changed response contract
		return &utxos_mempool.ListScriptHashUnspentResponse{
			UnspentTransactionsMempool: []*utxos_mempool.UnspentTransaction{},
		}, nil
	}

	return resUnspent, nil

}

func GetMempoolSpentInByTxIdOut(txId string, vout uint32) (blockDetails *utxos_mempool.GetSpentTransactionResponse, err error) {
	wait, err := time.ParseDuration("1s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxosMempoolConn()
	if err != nil {
		logger.Errorf("GetMempoolSpentInByTxIdOut - Unable to connect utxosMempool")
		return nil, fmt.Errorf("Error: %+v", err)
	}

	utxosMempoolClient := utxos_mempool.NewUTXOSMempoolClient(conn)

	resSpentIn, err := utxosMempoolClient.GetSpentTransaction(ctx, &utxos_mempool.GetSpentTransactionRequest{
		TxId: txId,
		Vout: vout,
	})

	if err != nil {

		if !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "unknown") {
			logger.Errorf("GetMempoolSpentInByTxIdOut - failed for txid:%s , vin: %+v, err: %+v", txId, vout, err)
		}
		return nil, fmt.Errorf("error: %+v", err)
	}

	return resSpentIn, nil
}

func GetBatchedMempoolSpentIn(items []*utxos_mempool.GetSpentTransactionRequest) ([]*utxos_mempool.BatchSpentResult, error) {
	wait, err := time.ParseDuration("5s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxosMempoolConn()
	if err != nil {
		logger.Errorf("GetBatchedMempoolSpentIn - Unable to connect utxosMempool")
		return nil, fmt.Errorf("error: %+v", err)
	}

	utxosMempoolClient := utxos_mempool.NewUTXOSMempoolClient(conn)

	res, err := utxosMempoolClient.GetBatchedSpentTransactions(ctx, &utxos_mempool.GetBatchedSpentTransactionsRequest{
		Requests: items,
	})

	if err != nil {
		logger.Errorf("GetBatchedMempoolSpentIn - failed: %+v", err)
		return nil, fmt.Errorf("error: %+v", err)
	}

	return res.Results, nil
}

func GetMempoolScriptsByAddress(address string) (blockDetails *utxos_mempool.GetScripthashesByAddressResponse, err error) {

	wait, err := time.ParseDuration("2s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxosMempoolConn()
	if err != nil {
		logger.Errorf("GetMempoolScriptsByAddress - Unable to connect utxosMempool")
		return nil, fmt.Errorf("error: %+v", err)
	}

	utxoStoreClient := utxos_mempool.NewUTXOSMempoolClient(conn)

	scriptList, err := utxoStoreClient.GetScriptHashesByAddress(ctx, &utxos_mempool.GetScripthashesByAddressRequest{
		Address: address,
	})

	if err != nil {
		if !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "Unknown") {
			logger.Errorf("GetMempoolScriptsByAddress - Unable to GetScriptHashesByAddress for  %s - %+v", address, err)
		}
		return nil, fmt.Errorf("error: %+v", err)
	}

	return scriptList, nil
}

func GetMempoolStats() (*utxos_mempool.GetStatsResponse, error) {

	var mempoolStats *utxos_mempool.GetStatsResponse
	var err error

	if redis.RedisClient.Enabled {
		err = redis.GetCachedValue(KEY_mempoolStats, &mempoolStats, nil)
		if err != nil {
			logger.Errorf("GetMempoolStats - Unable to get cached value ", err)
		} else {
			return mempoolStats, nil
		}
	}

	mempoolStats, err = getMempoolStatsFromUtxoService()

	if err != nil {
		logger.Errorf("GetMempoolStats ", err)
		return nil, fmt.Errorf("error: %+v", err)
	}
	return mempoolStats, nil

}

func getMempoolStatsFromUtxoService() (blockDetails *utxos_mempool.GetStatsResponse, err error) {
	wait, err := time.ParseDuration("1s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxosMempoolConn()
	if err != nil {
		logger.Errorf("getMempoolStatsFromUtxoService - Unable to connect utxosMempool")
		return nil, fmt.Errorf("error: %+v", err)
	}

	utxosMempoolClient := utxos_mempool.NewUTXOSMempoolClient(conn)

	stats, err := utxosMempoolClient.GetStats(ctx, &utxos_mempool.GetStatsRequest{})

	if err != nil {
		logger.Errorf("getMempoolStatsFromUtxoService:", err)
		return nil, fmt.Errorf("error: %+v", err)
	}

	return stats, nil
}

func StartMempoolStatsCache() {

	if !redis.RedisClient.Enabled {
		return
	}

	logger.Info("Starting Perodic MempoolStats Caching")

	ticker := time.NewTicker(time.Duration(mempoolStatsCacheOverideDuration) * time.Second)
	for ; true; <-ticker.C {

		redisConn := redis.RedisClient.ConnPool.Get()

		stats, err := getMempoolStatsFromUtxoService()

		if err != nil {
			logger.Errorf("StartMempoolStatsCache: Unable to get Mempoolstats from service  %+v\n", err)
		} else {
			err = redis.SetCacheValue(KEY_mempoolStats, stats, redisConn)

			if err != nil {
				logger.Errorf("Unable to Cache mempool stats %+v\n", err)
			}
		}

		redisConn.Flush()
		redisConn.Close()

	}
}

func Close() {
	if utxosMempoolConn != nil {
		if err := utxosMempoolConn.Close(); err != nil {
			logger.Warnf("failed to close utxos-mempool connection: %v", err)
		}
	}
}

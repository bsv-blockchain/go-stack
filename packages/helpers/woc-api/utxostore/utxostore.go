package utxostore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ordishs/gocore"
	"github.com/teranode-group/common"
	"github.com/teranode-group/common/utils"
	utxo_store "github.com/teranode-group/proto/utxo-store"
	"google.golang.org/grpc"
)

var logger = gocore.Log("woc-api")

var (
	utxoStoreConnOnce sync.Once
	utxoStoreConn     *grpc.ClientConn
	utxoStoreConnErr  error
)

var utxoStoreEnabled bool
var compareWithElectrumX bool
var compareWithElectrumXMempool bool
var bigTxScripthashes = utils.NewSet()

func getUtxoStoreConn() (*grpc.ClientConn, error) {
	utxoStoreConnOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		utxoStoreConn, utxoStoreConnErr = common.GetGRPCConnection(ctx, "utxoStore")
	})
	return utxoStoreConn, utxoStoreConnErr
}

func addBigTxScripthashes() {
	btxListStr, ok := gocore.Config().Get("bigTxScripthashes")
	if !ok {
		logger.Warn("No big tx addresses found in config file")
	}
	btxList := strings.Split(btxListStr, ",")
	for _, t := range btxList {
		logger.Infof("bigTxAddress: %s", t)
		bigTxScripthashes.Add(t)
	}
}

func init() {
	utxoStoreEnabled = gocore.Config().GetBool("utxoStoreEnabled", false)

	if utxoStoreEnabled {
		logger.Info("utxoStore is Enabled")
	} else {
		logger.Info("utxoStore is Disabled")
	}

	compareWithElectrumX = gocore.Config().GetBool("utxoStoreElectrumXBalanceComparison", false)

	compareWithElectrumXMempool = gocore.Config().GetBool("utxoStoreElectrumXBalanceComparisonMempool", false)

	if compareWithElectrumX {
		logger.Info("Comparison of utxo and electrumX balance is Enabled")
	} else {
		logger.Info("Comparison of utxo and electrumX balance is Disabled")
	}

	addBigTxScripthashes()

}

func IsEnabled() bool {
	return utxoStoreEnabled
}

func ShouldCompare() bool {
	return compareWithElectrumX
}

func isScripthashBlacklisted(scripthash string) bool {
	return bigTxScripthashes.Has(scripthash)
}

// message HealthCheckResponse {
// 	enum ServingStatus {
// 	  UNKNOWN = 0;
// 	  SERVING = 1;
// 	  NOT_SERVING = 2;
// 	}
// 	ServingStatus status = 1;
// 	uint64 last_block_height = 2;
// 	string last_block_hash = 3;
// 	bool is_catching_up = 4;
// 	bool is_processing_block = 5;
//  string uptime = 6; // format: 2 days, 4 hours, 12 minutes
//   }

func HealthCheck() (ok bool, height uint64, hash string, isProcessingBlock bool, isCatchingUp bool, Uptime string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, err := getUtxoStoreConn()
	if err != nil {
		logger.Errorf("Failed to connect utxoStore %+v", err)
		return false, 0, "", false, false, ""
	}

	utxoStoreClient := utxo_store.NewHealthClient(conn)

	resp, err := utxoStoreClient.Check(ctx, &utxo_store.HealthCheckRequest{})
	if err != nil || len(resp.LastBlockHash) != 64 {
		logger.Errorf("Failed to make health check: %v", err)
		return false, 0, "", false, false, ""
	}

	return true, resp.LastBlockHeight, resp.LastBlockHash, resp.IsProcessingBlock, resp.IsCatchingUp, resp.Uptime
}

func GetBalanceByScriptHash(hash string) (resp *utxo_store.GetBalanceResponse, err error) {

	if isScripthashBlacklisted(hash) {
		return nil, errors.New("scripthash is blacklisted")
	}
	wait, err := time.ParseDuration("30s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxoStoreConn()
	if err != nil {
		logger.Errorf("GetBalanceByScriptHash - Unable to connect utxoStore")
		return nil, fmt.Errorf("Error: %+v", err)
	}

	utxoStoreClient := utxo_store.NewUTXOStoreClient(conn)

	resBalance, err := utxoStoreClient.GetBalance(ctx, &utxo_store.GetBalanceRequest{
		Scripthash: hash,
	})

	if err != nil {
		if !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") {
			logger.Errorf("GetBalanceByScriptHash - Unable to GetBalance for  %s , %+v", hash, err)
		}
		return nil, fmt.Errorf("error: %+v", err)
	}

	return resBalance, nil
}

func GetHistoryStatsByScriptHash(hash string) (resp *utxo_store.GetScriptHashHistoryStatsResponse, err error) {

	if isScripthashBlacklisted(hash) {
		return nil, errors.New("scripthash is blacklisted")
	}
	wait, err := time.ParseDuration("30s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxoStoreConn()
	if err != nil {
		logger.Errorf("GetBalanceByScriptHash - Unable to connect utxoStore")
		return nil, fmt.Errorf("Error: %+v", err)
	}

	utxoStoreClient := utxo_store.NewUTXOStoreClient(conn)

	resStats, err := utxoStoreClient.GetScriptHashHistoryStats(ctx, &utxo_store.GetScriptHashHistoryStatsRequest{
		Scripthash: hash,
	})

	if err != nil {
		if !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") {
			logger.Errorf("GetBalanceByScriptHash - Unable to GetBalance for  %s , %+v", hash, err)
		}
		return nil, fmt.Errorf("error: %+v", err)
	}

	return resStats, nil
}

func GetScriptHashHistoryStatsPITB(hash string) (resp *utxo_store.GetScriptHashHistoryStatsResponse, err error) {

	if isScripthashBlacklisted(hash) {
		return nil, errors.New("scripthash is blacklisted")
	}
	wait, err := time.ParseDuration("30s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxoStoreConn()
	if err != nil {
		logger.Errorf("GetBalanceByScriptHash - Unable to connect utxoStore")
		return nil, fmt.Errorf("Error: %+v", err)
	}

	utxoStoreClient := utxo_store.NewUTXOStoreClient(conn)

	resStats, err := utxoStoreClient.GetScriptHashHistoryStatsPITB(ctx, &utxo_store.GetScriptHashHistoryStatsRequest{
		Scripthash: hash,
	})

	if err != nil {
		if !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") {
			logger.Errorf("GetBalanceByScriptHash - Unable to GetBalance for  %s , %+v", hash, err)
		}
		return nil, fmt.Errorf("error: %+v", err)
	}

	return resStats, nil
}

func GetConfirmedHistoryByScriptHash(hash string, pageSize int32, token string, afterBlockHeight uint32, order int) (resp *utxo_store.ListScriptHashHistoryResponse, err error) {

	if isScripthashBlacklisted(hash) {
		return nil, errors.New("scripthash is blacklisted")
	}
	wait, err := time.ParseDuration("20s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxoStoreConn()
	if err != nil {
		logger.Errorf("GetHistoryByScriptHash - Unable to connect utxoStore")
		return nil, fmt.Errorf("error: %+v", err)
	}

	utxoStoreClient := utxo_store.NewUTXOStoreClient(conn)

	resHistory, err := utxoStoreClient.ListScriptHashHistory(ctx, &utxo_store.ListScriptHashHistoryRequest{
		Scripthash:       hash,
		PageSize:         pageSize,
		PageToken:        token,
		AfterBlockHeight: afterBlockHeight,
		Sort:             utxo_store.Sort(order),
	})

	if err != nil {
		if !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") {
			logger.Errorf("GetHistoryByScriptHash - Unable to ListScriptHashHistory for  %s - %+v", hash, err)
		}
		return nil, fmt.Errorf("error: %+v", err)
	}

	return resHistory, nil

}

func GetConfirmedUnspentByScriptHash(hash string, pageSize int32, token string) (resp *utxo_store.ListScriptHashUnspentResponse, err error) {

	if isScripthashBlacklisted(hash) {
		return nil, errors.New("scripthash is blacklisted")
	}
	wait, err := time.ParseDuration("60s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxoStoreConn()
	if err != nil {
		logger.Errorf("GetConfirmedUnspentByScriptHash - Unable to connect utxoStore")
		return nil, fmt.Errorf("error: %+v", err)
	}

	utxoStoreClient := utxo_store.NewUTXOStoreClient(conn)

	resUnspent, err := utxoStoreClient.ListScriptHashUnspent(ctx, &utxo_store.ListScriptHashUnspentRequest{
		Scripthash: hash,
		PageSize:   pageSize,
		PageToken:  token,
	})

	if err != nil {
		if !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") {
			logger.Errorf("GetConfirmedUnspentByScriptHash - Unable to ListScriptHashUnspent for  %s", hash)
		}
		return nil, fmt.Errorf("error: %+v", err)
	}

	return resUnspent, nil

}

func CompareBalanceWithElectrumX(
	durationElectrumRequest time.Duration,
	hash string,
	elecConfirmed int64,
	elecUnconfirmed int64,
) (matchedConfirmed bool, machedUnconfirmed bool, err error) {
	timeNowUtxoRequest := time.Now()
	utxoBalance, err := GetBalanceByScriptHash(hash)
	if err != nil {
		logger.Errorf(
			"Error: CompareBalanceWithElectrumX - Unable to GetBalanceByScriptHash for  %s: utxo duration: %s: %v",
			hash,
			time.Since(timeNowUtxoRequest),
			err,
		)
		return false, false, err
	}
	durationUtxoRequest := time.Since(timeNowUtxoRequest)

	var utxoStoreConfirmed int64 = 0
	var utxoStoreUnconfirmed int64 = 0
	var utxoStoreRealUnconfirmed int64 = 0

	if utxoBalance.Unspent != nil {
		utxoStoreConfirmed = utxoBalance.Unspent.Satoshis
	}

	/*if utxoBalance.ScripthashMempool.Unspent != nil {
		utxoStoreUnconfirmed = utxoBalance.ScripthashMempool.Unspent.Satoshis

		// to compare with the electrumX we need difference.
		if utxoStoreUnconfirmed != 0 {
			utxoStoreRealUnconfirmed = utxoStoreUnconfirmed
			utxoStoreUnconfirmed = utxoStoreUnconfirmed - utxoStoreConfirmed
		}
	}*/

	// elecTime := ""
	// if durationElectrumRequest != nil {
	// 	elecTime = durationElectrumRequest.String()
	// }

	if utxoStoreConfirmed != elecConfirmed || (compareWithElectrumXMempool && utxoStoreUnconfirmed != elecUnconfirmed) {
		logger.Warnf(
			"Warn: Balance mismatch for hash %s - Confirmed: { ElectrumX: %d, UTXO: %d }, Unconfirmed: { ElectrumX: %d, UTXO: %d, Real UTXO: %d }, Duration: { ElectrumX: %s, UTXO: %s }",
			hash,
			elecConfirmed,
			utxoStoreConfirmed,
			elecUnconfirmed,
			utxoStoreUnconfirmed,
			utxoStoreRealUnconfirmed,
			durationElectrumRequest.String(),
			durationUtxoRequest.String(),
		)
	}

	return utxoStoreConfirmed == elecConfirmed, (compareWithElectrumXMempool && utxoStoreUnconfirmed == elecUnconfirmed), nil

}

func GetConfirmedSpentInByTxIdOut(txId string, vout uint32) (resp *utxo_store.GetSpentTransactionResponse, err error) {
	wait, err := time.ParseDuration("1s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxoStoreConn()
	if err != nil {
		logger.Errorf("GetConfirmedSpentInByTxIdOut - Unable to connect utxoStore")
		return nil, fmt.Errorf("Error: %+v", err)
	}

	utxoStoreClient := utxo_store.NewUTXOStoreClient(conn)

	resSpentIn, err := utxoStoreClient.GetSpentTransaction(ctx, &utxo_store.GetSpentTransactionRequest{
		TxId: txId,
		Vout: vout,
	})

	if err != nil {
		//Note: Not found will also end up here!
		return nil, fmt.Errorf("error: %+v", err)
	}

	return resSpentIn, nil
}

func GetConfirmedScriptsByAddress(address string) (resp *utxo_store.GetScripthashesByAddressResponse, err error) {

	wait, err := time.ParseDuration("2s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxoStoreConn()
	if err != nil {
		logger.Errorf("GetConfirmedScriptsByAddress - Unable to connect utxoStore")
		return nil, fmt.Errorf("error: %+v", err)
	}

	utxoStoreClient := utxo_store.NewUTXOStoreClient(conn)

	scriptList, err := utxoStoreClient.GetScripthashesByAddress(ctx, &utxo_store.GetScripthashesByAddressRequest{
		Address: address,
	})

	if err != nil {
		if !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "Unknown") {
			logger.Errorf("GetConfirmedScriptsByAddress - Unable to GetScripthashesByAddress for  %s - %+v", address, err)
		}
		return nil, fmt.Errorf("error: %+v", err)
	}

	return scriptList, nil
}

func GetScripthashBlockBalance(hash string) (balancePerBlock []*utxo_store.ScripthashBlockBalance, err error) {

	if isScripthashBlacklisted(hash) {
		return nil, errors.New("scripthash is blacklisted")
	}
	wait, err := time.ParseDuration("60s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxoStoreConn()
	if err != nil {
		logger.Errorf("GetScripthashBlockBalance - Unable to connect utxoStore")
		return nil, fmt.Errorf("error: %+v", err)
	}

	utxoStoreClient := utxo_store.NewUTXOStoreClient(conn)

	balResp, err := utxoStoreClient.GetBalance(ctx, &utxo_store.GetBalanceRequest{
		Scripthash: hash,
	})

	if err != nil {
		logger.Errorf("GetBalance - Unable to connect utxoStore")
		return nil, fmt.Errorf("error: %+v", err)
	}

	balPITBResp, err := utxoStoreClient.GetBalancePITB(ctx, &utxo_store.GetBalanceRequest{
		Scripthash: hash,
	})

	if err != nil {
		logger.Errorf("GetBalancePITB- Unable to connect utxoStore")
		return nil, fmt.Errorf("error: %+v", err)
	}

	if balResp.Unspent.Satoshis != balPITBResp.Unspent.Satoshis {
		logger.Errorf("GetBalancePITB satoshis not equal to GetBalance satoshis")
		return nil, fmt.Errorf("error: %+v", err)
	}

	var balances []*utxo_store.ScripthashBlockBalance

	nextPageToken := ""

	for {
		resStats, err := utxoStoreClient.GetScripthashBlockBalance(ctx, &utxo_store.GetScripthashBlockBalanceRequest{
			Scripthash: hash,
			PageSize:   8000,
			PageToken:  nextPageToken,
			Sort:       0,
		})

		if err != nil {
			logger.Errorf("GetBalanceByScriptHash - Unable to GetBalance for  %s , %+v", hash, err)
			return nil, fmt.Errorf("error: %+v", err)
		}

		balances = append(balances, resStats.ScripthashBlockBalance...)
		nextPageToken = resStats.NextPageToken

		if nextPageToken == "" {
			break
		}

	}

	return balances, nil
}

func GetBalancePITB(hash string) (balance *utxo_store.GetBalanceResponse, err error) {

	if isScripthashBlacklisted(hash) {
		return nil, errors.New("scripthash is blacklisted")
	}

	wait, err := time.ParseDuration("30s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getUtxoStoreConn()
	if err != nil {
		logger.Errorf("GetScripthashBlockBalance - Unable to connect utxoStore")
		return nil, fmt.Errorf("error: %+v", err)
	}

	utxoStoreClient := utxo_store.NewUTXOStoreClient(conn)

	balPITBResp, err := utxoStoreClient.GetBalancePITB(ctx, &utxo_store.GetBalanceRequest{
		Scripthash: hash,
	})

	if err != nil {
		logger.Errorf("GetBalancePITB- Unable to connect utxoStore")
		return nil, fmt.Errorf("error: %+v", err)
	}

	return balPITBResp, nil
}

func Close() {
	if utxoStoreConn != nil {
		if err := utxoStoreConn.Close(); err != nil {
			logger.Warnf("failed to close utxo-store connection: %v", err)
		}
	}
}

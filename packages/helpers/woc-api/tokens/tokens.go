package tokens

import (
	"context"
	"time"

	"github.com/teranode-group/common"
	token_mempool "github.com/teranode-group/proto/token-mempool"
	token_service "github.com/teranode-group/proto/token-service"
)

const (
	defaultItemsLimit = 30
	tokenMempoolName  = "tokenMempool"
	tokenServiceName  = "tokenService"
)

type TokenMempoolStatus struct {
	Status                   string `json:"status"`
	IsIndexing               bool   `json:"isIndexing"`
	StoredAddresses          uint64 `json:"storedAddresses"`
	Stored1SatOrdinalsTokens uint64 `json:"stored1SatOrdinalsTokens"`
	StoredBsv21              uint64 `json:"storedBsv21"`
	StoredStasTokens         uint64 `json:"storedStasTokens"`
	StoredTxs                uint64 `json:"storedTxs"`
	UpTime                   string `json:"upTime"`
}

type TokenServiceStatus struct {
	Status            string `json:"status"`
	LastBlockHash     string `json:"lastBlockHash"`
	LastBlockHeight   uint64 `json:"lastBlockHeight"`
	InCatchupMode     bool   `json:"inCatchupMode"`
	IsListenerUp      bool   `json:"isListenerUp"`
	IsProcessingBlock bool   `json:"isProcessingBlock"`
	IsProcessingReorg bool   `json:"isProcessingReorg"`
	UpTime            string `json:"upTime"`
}

// HasTokenTxVout :
func HasTokenTxVout(txid string, index int64) (bool, error) {

	wait, _ := time.ParseDuration("10s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, "tokenService")
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		return false, err
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	tsReq := &token_service.GetTokenVoutRequest{
		Txid:  txid,
		Index: index,
	}

	tsRes, err := tokenServiceClient.GetTokenVout(ctx, tsReq)
	if err != nil {
		logger.Errorf("Unable to GetTokenVout for %s, %d: Error: %+v", txid, index, err)
		return false, err
	}
	if tsRes != nil {
		return true, nil
	}
	return false, nil
}

func TokenMempoolHealthCheck(ctx context.Context) (TokenMempoolStatus, error) {
	var resp TokenMempoolStatus
	conn, err := common.GetGRPCConnection(ctx, tokenMempoolName)
	if err != nil {
		logger.Errorf("failed connecting %s %+v", tokenMempoolName, err)
		return resp, err
	}
	defer conn.Close()

	tsReq := &token_mempool.GetServiceCurrentStatusRequest{}
	tokenMempCli := token_mempool.NewTokenMempoolClient(conn)

	respData, err := tokenMempCli.GetServiceCurrentStatus(ctx, tsReq)
	if err != nil {
		return resp, err
	}

	dataMap := respData.Data.AsMap()
	resp.IsIndexing = boolCastingDefault(dataMap["listener-is_running"], false)
	resp.StoredAddresses = uint64(float64Casting(dataMap["memory-addresses"]))
	resp.Stored1SatOrdinalsTokens = uint64(float64Casting(dataMap["memory-1satord-tokens"]))
	resp.StoredBsv21 = uint64(float64Casting(dataMap["memory-bsv21-tokens"]))
	resp.StoredStasTokens = uint64(float64Casting(dataMap["memory-stas-tokens"]))
	resp.StoredTxs = uint64(float64Casting(dataMap["memory-txs"]))
	resp.UpTime = stringCasting(dataMap["listener-uptime"])

	return resp, nil
}

func TokenServiceHealthCheck(ctx context.Context) (TokenServiceStatus, error) {
	var resp TokenServiceStatus
	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("failed connecting %s %+v", tokenServiceName, err)
		return resp, err
	}
	defer conn.Close()

	tsReq := &token_service.GetServiceCurrentStatusRequest{}
	tokenSrvCli := token_service.NewTokenServiceClient(conn)

	respData, err := tokenSrvCli.GetServiceCurrentStatus(ctx, tsReq)
	dataMap := respData.Data.AsMap()

	resp.LastBlockHash = stringCasting(dataMap["indexer-last_block_hash_received"])
	resp.LastBlockHeight = uint64(float64Casting(dataMap["indexer-last_block_header_processed"]))
	resp.InCatchupMode = boolCastingDefault(dataMap["indexer-is_catching_up"], false)
	resp.IsListenerUp = boolCastingDefault(dataMap["indexer-is_listener_up"], false)
	resp.IsProcessingBlock = boolCastingDefault(dataMap["indexer-is_processing_block"], false)
	resp.IsProcessingReorg = boolCastingDefault(dataMap["indexer-is_processing_reorg"], false)
	resp.UpTime = stringCasting(dataMap["indexer-uptime"])

	return resp, nil
}

func boolCastingDefault(x interface{}, fallback bool) bool {
	i, ok := x.(bool)
	if !ok {
		return fallback
	}
	return i
}

func float64Casting(x interface{}) float64 {
	i, ok := x.(float64)
	if !ok {
		return 0
	}
	return i
}

func stringCasting(x interface{}) string {
	s, ok := x.(string)
	if !ok {
		return ""
	}
	return s
}

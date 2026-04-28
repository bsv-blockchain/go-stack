package bstore

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/genproto/protobuf/field_mask"

	"github.com/gorilla/mux"
	"github.com/ordishs/gocore"
	"github.com/patrickmn/go-cache"
	"github.com/teranode-group/common"
	"github.com/teranode-group/common/bsdecoder"
	"github.com/teranode-group/common/parser"
	"github.com/teranode-group/proto/bstore"
	"github.com/teranode-group/woc-api/pools"
	"github.com/teranode-group/woc-api/slack"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var (
	bstoreConnOnce sync.Once
	bstoreConn     *grpc.ClientConn
	bstoreConnErr  error

	// No expiry time
	bStoreHandlerCache = cache.New(0, 0)
)

func getBstoreConn() (*grpc.ClientConn, error) {
	bstoreConnOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		bstoreConn, bstoreConnErr = common.GetGRPCConnection(ctx, "bstore")
	})
	return bstoreConn, bstoreConnErr
}

type TxWithMeta struct {
	IndexInBlock int    `json:"index"`
	Txid         string `json:"txid"`
	Tags         []Tags `json:"tags,omitempty"`
}

type Tags struct {
	Type   string `json:"type"`
	Action string `json:"action,omitempty"`
	Count  int    `json:"count,omitempty"`
}

type TxRequestBody struct {
	Txids []string `json:"txids"`
}

type BulkVoutRequest struct {
	Txs []BulkVoutItem `json:"txids"`
}

type BulkVoutItem struct {
	TxID  string  `json:"txid"`
	Vouts []int32 `json:"vouts"`
}

type VoutHexItem struct {
	TxID  string                        `json:"txid"`
	Vout  map[uint32]*bstore.VoutCustom `json:"vout,,omitempty"`
	Error string                        `json:"error,omitempty"`
}

type TxStatusAndHex struct {
	TxID          string  `json:"txid"`
	Hex           string  `json:"hex,omitempty"`
	BlockHash     string  `json:"blockhash,omitempty"`
	BlockHeight   *uint64 `json:"blockheight,omitempty"`
	Blocktime     int64   `json:"blocktime,omitempty"`
	Confirmations uint32  `json:"confirmations,omitempty"`
	Error         string  `json:"error,omitempty"`
}

type TxOpReturnResponse struct {
	OpReturn []TxOpReturn `json:"opReturn"`
}

type TxOpReturn struct {
	N   int    `json:"n"`
	Hex string `json:"hex"`
}

var bstoreLastBlockHash = ""

func BStoreHealthCheck() (
	ok bool,
	height uint64,
	hash string,
	isProcessingBlock bool,
	isCatchingUp bool,
	txEndpointOk bool,
	txEndpointDuration time.Duration,
) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, err := getBstoreConn()
	if err != nil {
		logger.Errorf("Failed to connect bstore: %+v", err)
		return false, 0, "", false, false, false, 0
	}

	client := bstore.NewHealthClient(conn)
	resp, err := client.Check(ctx, &bstore.HealthCheckRequest{})
	if err != nil || len(resp.GetLastBlockHash()) != 64 {
		logger.Errorf("Failed to GetBlockHeader for Height 0: %v", err)
		return false, 0, "", false, false, false, 0
	}

	txEndpointOk = resp.GetEndpointOk()

	if d := resp.GetEndpointDuration(); d != nil {
		txEndpointDuration = d.AsDuration()
	}

	if NewBlockTxStatusTestEnabled() {
		if bstoreLastBlockHash != resp.GetLastBlockHash() {
			go TestBlockTxStatusCheck(resp.GetLastBlockHash())
			bstoreLastBlockHash = resp.GetLastBlockHash()
		}
	}

	return true,
		resp.GetLastBlockHeight(),
		resp.GetLastBlockHash(),
		resp.GetIsProcessingBlock(),
		resp.GetIsCatchingUp(),
		txEndpointOk,
		txEndpointDuration
}

func TestBlockTxStatusCheck(blockHash string) {
	wait, err := time.ParseDuration("5s")
	if err != nil {
		logger.Errorf("TestBlockTxStatusCheck - invalid page number")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getBstoreConn()
	if err != nil {
		logger.Errorf("TestBlockTxStatusCheck - Unable to connect bstore")
		return
	}

	bstoreClient := bstore.NewBStoreClient(conn)

	//get max 5 txs of the hash
	resTransactions, err := bstoreClient.ListBlockTransactions(ctx, &bstore.ListBlockTransactionsRequest{
		Hash:      blockHash,
		PageToken: "0",
		PageSize:  5,
	})

	if err != nil {
		logger.Errorf("TestBlockTxStatusCheck - Unable to ListBlockTransactions for  %s", blockHash)
		return
	}

	length := len(resTransactions.TxIds)

	if length > 0 {
		//get last tx is the list
		tsReq := &bstore.GetTransactionsStatusRequest{
			TxIds: []string{resTransactions.TxIds[int32(length-1)]},
		}

		resp, err := bstoreClient.GetTransactionsStatus(ctx, tsReq)

		if err != nil {
			logger.Errorf("GetTransactionsStatus:  %v ", err)
			return
		}

		// check if  status is updated
		for _, tx := range resp.TransactionsStatus {
			if tx.BlockHash != blockHash {
				//log and send msg to slack
				logger.Warnf("TestBlockTxStatusCheck -  bstore transactions status not in sync for blockhash: %s", tx.BlockHash)
				slack.SendTextMsg("Warning: bstore transactions status not in sync for blockhash: " + tx.BlockHash)
			}
		}

	}

}

func BulkTxStatusHandler(w http.ResponseWriter, r *http.Request) {

	var txidBody TxRequestBody
	b, _ := ioutil.ReadAll(r.Body)
	err := json.Unmarshal(b, &txidBody)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	txids := txidBody.Txids
	//TODO: remove this is for testing only
	// logger.Infof("INFO: /status request received with body: Tx: %s", txids)

	if len(txids) > 20 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode("Maximum number of transactions per request has been exceeded")
		return
	}

	wait, _ := time.ParseDuration("10s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getBstoreConn()
	if err != nil {
		logger.Errorf("Error: Unable to connect bstore %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	bstoreClient := bstore.NewBStoreClient(conn)

	tsReq := &bstore.GetTransactionsStatusRequest{
		TxIds: txids,
	}

	var trailer metadata.MD
	resp, err := bstoreClient.GetTransactionsStatus(ctx, tsReq, grpc.Trailer(&trailer))

	if err != nil {
		logger.Errorf("Unable to call BStore GetTransactionsStatus for  %s,%+v", txids, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	results := make([]TxStatusAndHex, 0)

	for _, tx := range resp.TransactionsStatus {
		var confirmations uint32
		var blockTime int64
		var blockHeight *uint64
		var blockHash string

		if tx.BlockInfo != nil {
			confirmations = uint32(tx.BlockInfo.CurrentBlockHeight-tx.BlockInfo.BlockHeight) + 1
			blockTime = int64(tx.BlockInfo.BlockTime)
			height := tx.BlockInfo.BlockHeight
			blockHeight = &height
			blockHash = tx.BlockInfo.BlockHash
		}

		results = append(results, TxStatusAndHex{
			TxID:          tx.Txid,
			BlockHash:     blockHash,
			BlockHeight:   blockHeight,
			Blocktime:     blockTime,
			Confirmations: confirmations,
		})
	}

	if valueArray, ok := trailer["notfound"]; ok {
		for _, item := range valueArray {
			var idsArray []string
			err := json.Unmarshal([]byte(item), &idsArray)
			if err != nil {
				logger.Errorf("Metadata Not a json value, %v", err)
			}

			for _, txid := range idsArray {

				if nodeAsBackupEnabled {
					txStatus, err := getTxFromBitcoinNode(txid, true)
					if err == nil {
						results = append(results, txStatus)
						logger.Warnf("txid notfound in bstore, but found in node: %s", txid)
						continue
					}
				}

				results = append(results, TxStatusAndHex{
					TxID:  txid,
					Error: "unknown",
				})
			}

		}
	}

	// Reorder results to match the original request order
	indexMap := make(map[string]int, len(txids))
	for i, txid := range txids {
		indexMap[txid] = i
	}
	orderedResults := make([]TxStatusAndHex, len(txids))
	for _, r := range results {
		if i, ok := indexMap[r.TxID]; ok {
			orderedResults[i] = r
		}
	}

	json.NewEncoder(w).Encode(orderedResults)
}

func BulkTxHexHandler(w http.ResponseWriter, r *http.Request) {

	var txidBody TxRequestBody
	b, _ := ioutil.ReadAll(r.Body)
	err := json.Unmarshal(b, &txidBody)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	txids := txidBody.Txids

	if len(txids) > 20 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode("Maximum number of transactions per request has been exceeded")
		return
	}

	wait, _ := time.ParseDuration("10s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getBstoreConn()
	if err != nil {
		logger.Errorf("Unable to connect bstore %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	bstoreClient := bstore.NewBStoreClient(conn)

	tsReq := &bstore.GetTransactionsRequest{
		TxIds: txids,
	}
	var trailer metadata.MD
	resp, err := bstoreClient.GetTransactions(ctx, tsReq, grpc.Trailer(&trailer))
	if err != nil {
		logger.Errorf("Unable to get BStoreGetTransactions for  %s", txids, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	results := make([]TxStatusAndHex, 0)

	for _, tx := range resp.Transactions {

		var confirmations uint32
		var blockTime int64
		var blockHeight *uint64
		var blockHash string

		if tx.BlockInfo != nil {
			confirmations = uint32(tx.BlockInfo.CurrentBlockHeight-tx.BlockInfo.BlockHeight) + 1
			blockTime = int64(tx.BlockInfo.BlockTime)
			height := tx.BlockInfo.BlockHeight
			blockHeight = &height
			blockHash = tx.BlockInfo.BlockHash
		}

		results = append(results, TxStatusAndHex{
			TxID:          tx.Txid,
			Hex:           hex.EncodeToString(tx.Raw),
			BlockHash:     blockHash,
			BlockHeight:   blockHeight,
			Blocktime:     blockTime,
			Confirmations: confirmations,
		})
	}

	if valueArray, ok := trailer["notfound"]; ok {

		for _, item := range valueArray {
			var idsArray []string
			err := json.Unmarshal([]byte(item), &idsArray)
			if err != nil {
				logger.Errorf("Metadata: Not a json value, %v", err)
			}

			for _, txid := range idsArray {

				//check node
				if nodeAsBackupEnabled {
					txStatus, err := getTxFromBitcoinNode(txid, false)
					if err == nil {
						results = append(results, txStatus)
						logger.Warnf("txid notfound in bstore, but found in node: %s", txid)
						continue
					}
				}
				results = append(results, TxStatusAndHex{
					TxID:  txid,
					Error: "unknown",
				})
			}

		}
	}

	json.NewEncoder(w).Encode(results)

}

func GetBlockHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	heightOrhash := vars["heightOrhash"]
	hash := ""
	var height int64
	if len(heightOrhash) == 64 {
		hash = heightOrhash
	} else {
		var err error
		height, err = strconv.ParseInt(heightOrhash, 10, 64)
		if err != nil {
			logger.Errorf("invalid block height")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	blockDetails, err := getBlock(hash, height)
	if err != nil {
		logger.Errorf("unable to get details of block: %s , %s", heightOrhash, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(blockDetails)

}

func GetBlocksHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	heightStr := vars["height"]
	countStr := vars["count"]
	order := r.URL.Query().Get("order")

	hash := ""
	var height int64
	blocks := []*bsdecoder.BlockHeaderAndCoinbase{}

	var err error
	height, err = strconv.ParseInt(heightStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	count, err := strconv.Atoi(countStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if count < 1 || count > 20 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if order != "" && (order != "asc" && order != "desc") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	currentHeight := height
	endHeight := currentHeight + int64(count) - 1
	if order == "desc" {
		endHeight = currentHeight - int64(count) + 1
	}

	for {
		blockDetails, err := getBlock(hash, currentHeight)
		if err != nil {
			logger.Errorf("unable to get details of block: %d , %s", currentHeight, err)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode("Block not found")
			return
		}

		blocks = append(blocks, blockDetails)
		if currentHeight == endHeight {
			break
		}
		if order == "" || order == "asc" {
			currentHeight++
		} else {
			currentHeight--
		}
	} // for

	json.NewEncoder(w).Encode(blocks)
}

func GetBlockTxHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	heightOrhash := vars["heightOrhash"]
	hash := ""

	// var height int64 //TODO:
	if len(heightOrhash) == 64 {
		hash = heightOrhash
	} else {
		//TODO: bstore by height very slow bug??
		w.WriteHeader(http.StatusBadRequest)
		return
		// var err error
		// height, err = strconv.ParseInt(heightOrhash, 10, 64)
		// if err != nil {
		// 	w.WriteHeader(http.StatusBadRequest)
		// 	return
		// }

	}

	//skip skipTags if requested
	skipTags := false
	skipTagsStr := strings.ToLower(r.URL.Query().Get("skipTags"))

	if skipTagsStr == "true" {
		skipTags = true
	}

	// limit
	limitStr := strings.ToLower(r.URL.Query().Get("limit"))
	if limitStr == "" {
		limitStr = "10" //TODO: const
	}
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit < 1 || limit > 50 {
		limit = 10 //TODO: const
	}

	// offset
	offsetStr := strings.ToLower(r.URL.Query().Get("offset"))
	if offsetStr == "" {
		offsetStr = "0"
	}
	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	page := offset

	wait, err := time.ParseDuration("10s")
	if err != nil {
		//TODO proper error
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getBstoreConn()
	if err != nil {
		logger.Errorf("Unable to connect bstore")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	bStoreClient := bstore.NewBStoreClient(conn)

	resTransactions, err := bStoreClient.ListBlockTransactions(ctx, &bstore.ListBlockTransactionsRequest{
		Hash:      hash,
		PageToken: strconv.FormatInt(page, 10),
		PageSize:  int32(limit),
	})

	if err != nil {
		logger.Errorf("Unable to ListBlockTransactions for  %s", hash)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	keys := make([]int, len(resTransactions.TxIds))
	i := 0
	for k := range resTransactions.TxIds {
		keys[i] = int(k)
		i++
	}

	sort.Ints(keys)

	txids := make([]TxWithMeta, len(keys))

	for i, k := range keys {
		txids[i].IndexInBlock = k
		txids[i].Txid = resTransactions.TxIds[int32(k)]
		var tagsArray []Tags

		if !skipTags {
			//TODO: Optimization - should try elastic search first
			txDetails, err := GetTx(txids[i].Txid)
			if err != nil {
				logger.Errorf("unable to get details of tx: %s , %s", txids[i].Txid, err)
				continue
			}

			for _, v := range txDetails.Vout {

				if v.ScriptPubKey.Type == "nonstandard" {
					tag, err := GetNonStandardTag(v.ScriptPubKey.Hex)
					if err != nil {
						continue
					}

					if len(tag.Type) > 0 {
						index, err := ContainsAtIndex(tagsArray, tag.Type, tag.Action)
						if err == nil && index >= 0 {
							tagsArray[index].Count++
						} else {
							tagsArray = append(tagsArray, Tags{Type: tag.Type, Action: tag.Action, Count: 1})
						}
					}

					continue

				}

				asm := ""
				if len(v.ScriptPubKey.ASM) > 50 {
					asm = v.ScriptPubKey.ASM[0:50]
					v.ScriptPubKey.ASM = ""
				} else {
					asm = v.ScriptPubKey.ASM
				}

				//tag opreturn
				tag, err := GetOpReturnTag(asm, v.ScriptPubKey.Hex)
				if err != nil {
					continue
				}

				if len(tag.Type) > 0 {
					index, err := ContainsAtIndex(tagsArray, tag.Type, tag.Action)
					if err == nil && index >= 0 {
						tagsArray[index].Count++
					} else {
						tagsArray = append(tagsArray, Tags{Type: tag.Type, Action: tag.Action, Count: 1})
					}
				}
			}

			if len(tagsArray) > 0 {
				txids[i].Tags = tagsArray
			}

		}
	}

	json.NewEncoder(w).Encode(txids)

}

func GetTransactionByBlockHeightAndIndex(blockHeight int64, txIndex int, pageSize int) (*bsdecoder.RawTransaction, error) {
	wait, _ := time.ParseDuration("10s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	if pageSize <= 0 || pageSize > 1000 {
		pageSize = 100
	}

	blockDetails, err := GetBlockDetails("", blockHeight)
	if err != nil || blockDetails == nil {
		return nil, fmt.Errorf("unable to get block details for height %d: %w", blockHeight, err)
	}
	blockHash := blockDetails.Hash

	conn, err := getBstoreConn()
	if err != nil {
		return nil, fmt.Errorf("unable to connect to bstore: %w", err)
	}

	client := bstore.NewBStoreClient(conn)

	pageOffset := (txIndex / pageSize) * pageSize

	resp, err := client.ListBlockTransactions(ctx, &bstore.ListBlockTransactionsRequest{
		Hash:      blockHash,
		PageToken: strconv.Itoa(pageOffset), // Convert offset to string
		PageSize:  int32(pageSize),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}

	if txid, ok := resp.TxIds[int32(txIndex)]; ok {
		return GetTx(txid)
	}

	return nil, fmt.Errorf("transaction index %d not found in block %d", txIndex, blockHeight)
}

func GetOpreturnData(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hash := vars["txid"]

	if len(hash) != 64 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var opReturnArray []TxOpReturn
	txDetails, err := GetTx(hash)

	if err != nil && nodeAsBackupEnabled {
		//check node
		txFromNode, err := getTxFromBitcoinNode(hash, false)
		if err == nil {
			txDetails, err = ParseRawTx(txFromNode.Hex)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}

	if txDetails == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	for index, v := range txDetails.Vout {
		if strings.HasPrefix(v.ScriptPubKey.ASM, "OP_FALSE") || strings.HasPrefix(v.ScriptPubKey.ASM, "0 ") || strings.HasPrefix(v.ScriptPubKey.ASM, "OP_RETURN") {
			opReturnArray = append(opReturnArray, TxOpReturn{N: index, Hex: v.ScriptPubKey.Hex})
		}
	}

	json.NewEncoder(w).Encode(opReturnArray)
	return
}

func BulkTxVoutHex(w http.ResponseWriter, r *http.Request) {
	var txidBody BulkVoutRequest
	b, _ := ioutil.ReadAll(r.Body)
	err := json.Unmarshal(b, &txidBody)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	txs := txidBody.Txs

	if len(txs) > 40 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	results, err := GetVoutHex(txs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}

	json.NewEncoder(w).Encode(results)

}

func GetVoutHex(txs []BulkVoutItem) ([]VoutHexItem, error) {
	wait, _ := time.ParseDuration("10s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getBstoreConn()
	if err != nil {
		logger.Errorf("Unable to connect bstore %+v", err)
		return nil, err
	}

	bstoreClient := bstore.NewBStoreClient(conn)

	fieldMask := &field_mask.FieldMask{
		Paths: []string{"transactions.vout.scriptPubKey.hex"},
	}

	var reqItems []*bstore.TransactionsHeaderCustomRequest
	results := make([]VoutHexItem, 0)

	for _, tx := range txs {
		if len(tx.Vouts) > 40 {
			results = append(results, VoutHexItem{
				TxID:  tx.TxID,
				Error: "Bad Request: Max 40 vouts allowed per txid",
			})
			continue
		}
		reqItems = append(reqItems, &bstore.TransactionsHeaderCustomRequest{TxId: tx.TxID, Outputs: tx.Vouts})
	}

	tsReq := &bstore.GetTransactionsHeaderCustomRequest{
		Txs:       reqItems,
		FieldMask: fieldMask,
	}

	var trailer metadata.MD
	resp, err := bstoreClient.GetTransactionsHeaderCustom(ctx, tsReq, grpc.Trailer(&trailer))

	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "not found") {
			for _, tx := range txs {
				results = append(results, VoutHexItem{
					TxID:  tx.TxID,
					Error: "unknown",
				})
			}
			return results, nil
		}

		logger.Errorf("Unable to GetTransactionsHeaderCustom %v", err)
		return nil, err
	}

	for txid, tx := range resp.Transactions {
		results = append(results, VoutHexItem{
			TxID: txid,
			Vout: tx.Vout,
		})
	}

	if valueArray, ok := trailer["notfound"]; ok {
		for _, item := range valueArray {
			var idsArray []string
			err := json.Unmarshal([]byte(item), &idsArray)
			if err != nil {
				logger.Errorf("Metadata Not a json value, %v", err)
			}

			for _, txid := range idsArray {

				results = append(results, VoutHexItem{
					TxID:  txid,
					Error: "unknown",
				})
			}

		}
	}
	return results, nil
}

func GetTxHex(txid string) (rawTx *string, err error) {
	txDetailsFromBstore, err := GetTx(txid)
	if err != nil {
		return nil, fmt.Errorf("error: %+v", err)
	}

	return &txDetailsFromBstore.Hex, nil
}

func GetTxVoutHex(txid string, voutIndex int64) (rawTx *string, err error) {
	txDetailsFromBstore, err := GetTx(txid)
	if err != nil {
		return nil, fmt.Errorf("error: %+v", err)
	}

	if txDetailsFromBstore.VoutCount < voutIndex {
		return nil, fmt.Errorf("error: %+v", err)
	}

	return &txDetailsFromBstore.Vout[voutIndex].ScriptPubKey.Hex, nil
}

func GetDiscardedTxReason(txid string) (discardReason string, err error) {
	wait, _ := time.ParseDuration("10s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getBstoreConn()
	if err != nil {
		logger.Errorf("Unable to connect bstore %+v", err)
		return "", fmt.Errorf("error: %+v", err)
	}

	bStoreClient := bstore.NewBStoreClient(conn)

	req := &bstore.GetDiscardedTransactionRequest{
		TxIds:     []string{txid},
		WithRawTx: false,
	}

	res, err := bStoreClient.GetDiscardedTransaction(ctx, req)

	if err != nil {
		logger.Errorf("Unable to GetDiscardedTransaction for %s: %v", txid, err)
		return "", fmt.Errorf("error: %+v", err)
	}

	for _, tx := range res.DiscardedTransaction {
		if tx.Txid == txid {
			return tx.Reason, nil
		}
	}

	return "", fmt.Errorf("not found")
}

func GetTx(txid string) (txDetails *bsdecoder.RawTransaction, err error) {
	// defer timeTrack(time.Now(), "GetTxDetails")
	wait, _ := time.ParseDuration("20s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getBstoreConn()
	if err != nil {
		logger.Errorf("Unable to connect bstore")
		return nil, fmt.Errorf("error: %+v", err)
	}

	bStoreClient := bstore.NewBStoreClient(conn)

	req := &bstore.GetTransactionRequest{
		Txid: txid,
	}

	res, err := bStoreClient.GetTransaction(ctx, req, grpc.MaxCallRecvMsgSize(bstoreGrpcMaxCallRecvMsgSizeMB*1024*1024))
	if err != nil {
		logger.Errorf("Unable to GetTransaction for %s: %v", txid, err)
		return nil, fmt.Errorf("error: %+v", err)
	}
	tx, err := bsdecoder.DecodeRawTransaction(res, IsMainnet())

	if err != nil {
		logger.Errorf("Unable to generate transaction JSON")
		return nil, fmt.Errorf("error: %+v", err)
	}

	return tx, nil
}

func GetTxLight(txid string) (txDetails *bsdecoder.RawTransaction, err error) {
	return GetTxWithRange(txid, -1, -1, -1, -1)
}

func GetTxWithRange(txid string, vinStart, vinEnd, voutStart, voutEnd int) (txDetails *bsdecoder.RawTransaction, err error) {
	wait, _ := time.ParseDuration("20s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getBstoreConn()
	if err != nil {
		logger.Errorf("Unable to connect bstore")
		return nil, fmt.Errorf("error: %+v", err)
	}

	bStoreClient := bstore.NewBStoreClient(conn)

	req := &bstore.GetTransactionRequest{
		Txid: txid,
	}

	res, err := bStoreClient.GetTransaction(ctx, req, grpc.MaxCallRecvMsgSize(bstoreGrpcMaxCallRecvMsgSizeMB*1024*1024))
	if err != nil {
		logger.Errorf("Unable to GetTransaction for %s: %v", txid, err)
		return nil, fmt.Errorf("error: %+v", err)
	}
	tx, err := bsdecoder.DecodeRawTransactionWithRange(res, IsMainnet(), vinStart, vinEnd, voutStart, voutEnd)

	if err != nil {
		logger.Errorf("Unable to generate transaction JSON")
		return nil, fmt.Errorf("error: %+v", err)
	}

	return tx, nil
}

func GetVoutValue(txid string, vout uint32) (value uint64, err error) {
	wait, _ := time.ParseDuration("10s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getBstoreConn()
	if err != nil {
		logger.Errorf("Unable to connect bstore")
		return 0, fmt.Errorf("error: %+v", err)
	}

	bStoreClient := bstore.NewBStoreClient(conn)

	req := &bstore.GetTransactionHeadersRequest{
		TxIds: []string{txid},
	}

	res, err := bStoreClient.GetTransactionsHeaders(ctx, req)
	if err != nil {
		logger.Errorf("Unable to GetTransactionsHeaders for %s: %v", txid, err)
		return 0, fmt.Errorf("error: %+v", err)
	}

	if len(res.Transactions) > 0 && res.Transactions[txid] != nil {
		return res.Transactions[txid].Vout[vout].Value, nil
	}

	return 0, fmt.Errorf("not found")
}

func GetTxHeaders(txids []string) (headers interface{}, err error) {
	if len(txids) > 50 {
		return nil, fmt.Errorf("error: max 50 tx per request")
	}

	wait, _ := time.ParseDuration("10s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getBstoreConn()
	if err != nil {
		logger.Errorf("Unable to connect bstore")
		return nil, fmt.Errorf("error: %+v", err)
	}

	bStoreClient := bstore.NewBStoreClient(conn)

	req := &bstore.GetTransactionHeadersRequest{
		TxIds: txids,
	}

	res, err := bStoreClient.GetTransactionsHeaders(ctx, req)
	if err != nil {
		logger.Errorf("Unable to GetTransactionsHeaders for %s: %v", txids, err)
		return nil, fmt.Errorf("error: %+v", err)
	}

	if res.Transactions != nil {
		return res.Transactions, nil
	}

	return nil, errors.New("not found")
}

func GetBlockDetails(hash string, height int64) (blockDetails *bsdecoder.BlockHeaderAndCoinbase, err error) {

	wait, err := time.ParseDuration("10s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getBstoreConn()
	if err != nil {
		logger.Errorf("Unable to connect bstore")
		return nil, fmt.Errorf("error: %+v", err)
	}

	bStoreClient := bstore.NewBStoreClient(conn)

	resBlock, err := bStoreClient.GetBlockHeader(ctx, &bstore.GetBlockHeaderRequest{
		Hash:   hash,
		Height: uint64(height),
	})
	if err != nil {
		logger.Errorf("Unable to GetBlockHeader for  %s", hash)
		return nil, fmt.Errorf("error: %+v", err)
	}

	resTransactions, err := bStoreClient.ListBlockTransactions(ctx, &bstore.ListBlockTransactionsRequest{
		Hash:      resBlock.Hash,
		PageToken: "0",
		PageSize:  1,
	})
	if err != nil {
		logger.Errorf("Unable to ListBlockTransactions for  %s", hash)
		return nil, fmt.Errorf("error: %+v", err)
	}

	coinbaseRes, err := bStoreClient.GetTransaction(ctx, &bstore.GetTransactionRequest{
		Txid: resTransactions.TxIds[0],
	},
		grpc.MaxCallRecvMsgSize(bstoreGrpcMaxCallRecvMsgSizeMB*1024*1024),
	)
	if err != nil {
		logger.Errorf("Unable to GetTransaction for coinbase %s", resTransactions.TxIds[0])
		return nil, fmt.Errorf("error: %+v", err)
	}

	coinbaseTx, err := bsdecoder.DecodeRawTransaction(coinbaseRes, IsMainnet())
	if err != nil {
		logger.Errorf("Unable to DecodeRawTransaction for coinbase")
		return nil, fmt.Errorf("error: %+v", err)
	}
	// Set the blockHeight in the coinbase tx to avoid duplication
	coinbaseTx.BlockHeight = nil

	keys := make([]int, len(resTransactions.TxIds))
	i := 0
	for k := range resTransactions.TxIds {
		keys[i] = int(k)
		i++
	}

	sort.Ints(keys)

	txids := make([]string, 0)
	for k := range keys {
		txids = append(txids, resTransactions.TxIds[int32(k)])
	}

	blk, err := bsdecoder.DecodeRawBlock(resBlock, coinbaseTx, txids)
	if err != nil {
		logger.Errorf("Unable to generate block JSON")
		return nil, fmt.Errorf("error: %+v", err)
	}

	return blk, nil
}

func GetBlockHeader(hash string, height int64) (blockDetails *bsdecoder.BlockHeaderAndCoinbase, err error) {

	wait, err := time.ParseDuration("10s")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := getBstoreConn()
	if err != nil {
		logger.Errorf("Unable to connect bstore")
		return nil, fmt.Errorf("error: %+v", err)
	}

	bStoreClient := bstore.NewBStoreClient(conn)

	resBlock, err := bStoreClient.GetBlockHeader(ctx, &bstore.GetBlockHeaderRequest{
		Hash:   hash,
		Height: uint64(height),
	})
	if err != nil {
		logger.Errorf("Unable to GetBlockHeader for  %s", hash)
		return nil, fmt.Errorf("error: %+v", err)
	}

	txids := make([]string, 0)

	blk, err := bsdecoder.DecodeRawBlock(resBlock, nil, txids)
	if err != nil {
		logger.Errorf("Unable to generate block JSON")
		return nil, fmt.Errorf("error: %+v", err)
	}

	return blk, nil
}

// TODO Mo: Do we really needs this?
func GetMaxTxHexLength() (maxTxHexLength int) {

	cacheKey := "maxTxHexLength"

	cachedValueInterface, found := bStoreHandlerCache.Get(cacheKey)

	if found {
		maxTxHexLength := cachedValueInterface.(int)
		return maxTxHexLength
	}

	maxTxHexLength, ok := gocore.Config().GetInt("maxTxHexLength")
	if !ok {
		maxTxHexLength = 100000 // default for now
	}

	bStoreHandlerCache.Set(cacheKey, maxTxHexLength, cache.NoExpiration)

	return maxTxHexLength

}

func GetOpReturnTag(vAsm string, vHex string) (opTag *bsdecoder.OpReturn, err error) {
	opTag = &bsdecoder.OpReturn{}
	if strings.HasPrefix(vAsm, "OP_FALSE") || strings.HasPrefix(vAsm, "0 ") || strings.HasPrefix(vAsm, "OP_RETURN") {
		buf, err := hex.DecodeString(vHex)

		if err != nil {
			logger.Errorf("parsing op_return script %+v", err)
			return nil, errors.New("error: parsing op_return script")
		}
		tag, subtag, parts, err := parser.ParseOpReturn(buf)

		if err == nil {
			opTag.Type = tag
			opTag.Action = subtag
			if parts != nil && *parts != nil && len(*parts) > 0 {
				opTag.Text = (*parts)[0].URI
			}
		}
	}
	return opTag, nil
}

func GetNonStandardTag(vHex string) (nsTag *bsdecoder.Tag, err error) {
	nsTag = &bsdecoder.Tag{}
	buf, err := hex.DecodeString(vHex)
	if err != nil {
		logger.Errorf("decoding script %+v", err)
		return nil, errors.New("error: unabel to DecodeString")
	}

	tag, subtag, err := parser.ParseNonStandard(buf)
	if err != nil {
		logger.Errorf("ParseNonStandard %+v", err)
		return nil, errors.New("error: unabel to ParseNonStandard")
	}
	nsTag.Type = tag
	nsTag.Action = subtag

	return nsTag, nil
}

func ContainsAtIndex(tags []Tags, tagType string, action string) (index int, err error) {

	for i, t := range tags {
		if t.Type == tagType && t.Action == action {
			return i, nil
		}
	}

	return -1, nil
}

func getBlock(hash string, height int64) (*bsdecoder.BlockHeaderAndCoinbase, error) {
	blockDetails, err := GetBlockDetails(hash, height)
	if err != nil {
		return nil, err
	}

	//add miner info
	if blockDetails.Coinbase != nil && len(blockDetails.Coinbase.Vin) > 0 {
		tx := blockDetails.Coinbase
		address := ""
		if len(tx.Vout) > 0 && tx.Vout[0].ScriptPubKey.Addresses != nil && len(tx.Vout[0].ScriptPubKey.Addresses) > 0 {
			address = blockDetails.Coinbase.Vout[0].ScriptPubKey.Addresses[0]
		}

		minerDetails, err := pools.GetMinerTag(tx.Vin[0].Coinbase, address)
		blockDetails.Coinbase.Vin[0].MinerInfo = &bsdecoder.MinerDetails{}
		if err == nil {
			blockDetails.Coinbase.Vin[0].MinerInfo.Name = minerDetails.Name
			blockDetails.Coinbase.Vin[0].MinerInfo.Type = minerDetails.Type
			blockDetails.Coinbase.Vin[0].MinerInfo.Link = minerDetails.Link
		} else {
			src := []byte(tx.Vin[0].Coinbase)
			dst := make([]byte, hex.DecodedLen(len(src)))
			n, err := hex.Decode(dst, src)
			if err != nil {
				blockDetails.Coinbase.Vin[0].MinerInfo.Name = tx.Vin[0].Coinbase
			} else {
				blockDetails.Coinbase.Vin[0].MinerInfo.Name = fmt.Sprintf("%s\n", dst[:n])
			}
		}
	}
	return blockDetails, nil
}

func ParseRawTx(raw string) (txDetails *bsdecoder.RawTransaction, err error) {
	rawHex, _ := hex.DecodeString(raw)

	req := &bstore.GetTransactionResponse{
		Raw: rawHex,
	}

	tx, err := bsdecoder.DecodeRawTransaction(req, IsMainnet())
	if err != nil {
		logger.Errorf("parseRawTx: Unable to generate transaction JSON")
		return nil, fmt.Errorf("error: %+v", err)
	}

	return tx, nil
}

func getTxFromBitcoinNode(txID string, removeHex bool) (txDetails TxStatusAndHex, err error) {
	tx, err := bitcoinClient.GetRawTransaction(txID, false, removeHex)
	if err != nil {
		logger.Errorf("getTxFromBitcoinNode - gettransactionhex for txid %+v - %+v\n", txID, err)
		return TxStatusAndHex{}, err
	}

	if len(tx.BlockHash) != 64 {
		return TxStatusAndHex{
			TxID: tx.TxID,
			Hex:  tx.Hex,
		}, nil
	} else {

		return TxStatusAndHex{
			TxID:          tx.TxID,
			Hex:           tx.Hex,
			BlockHash:     tx.BlockHash,
			BlockHeight:   &tx.BlockHeight,
			Blocktime:     tx.Blocktime,
			Confirmations: tx.Confirmations,
		}, nil
	}
}

func Close() {
	if bstoreConn != nil {
		if err := bstoreConn.Close(); err != nil {
			logger.Warnf("failed to close bstore connection: %v", err)
		}
	}
}

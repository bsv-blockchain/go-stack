package bstore

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/teranode-group/common"
	"github.com/teranode-group/common/bsdecoder"
	"github.com/teranode-group/proto/bstore"
	"github.com/teranode-group/woc-api/pools"
)

type BlocksAtHeight struct {
	BlockHeight int64                               `json:"height"`
	Headers     []*bsdecoder.BlockHeaderAndCoinbase `json:"headers"`
}

func GetBlocksAtHeightIncludeOrphans(w http.ResponseWriter, r *http.Request) {

	//?height=xxxx&order=desc&limit=10

	heightStr := r.URL.Query().Get("height")
	order := strings.ToLower(r.URL.Query().Get("order"))
	limitStr := r.URL.Query().Get("limit")

	height, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Default order desc
	if order == "" {
		order = "desc"
	}

	// Default limit 1
	if limitStr == "" {
		limitStr = "1"
	}

	if order != "desc" && order != "asc" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if limit < 1 || limit > 50 {
		mapD := map[string]string{"error": "limit can't be less than 1 or greater than 50"}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(mapD)
		return
	}

	wait, err := time.ParseDuration("20s")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, "bstore")
	if err != nil {
		logger.Errorf("Unable to connect bstore")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	blocksResponse := []*BlocksAtHeight{}
	bStoreClient := bstore.NewBStoreClient(conn)

	currentHeight := height
	endHeight := currentHeight + int64(limit) - 1
	if order == "desc" {
		endHeight = currentHeight - int64(limit) + 1
	}

	for {

		blocksAtHeight := &BlocksAtHeight{BlockHeight: currentHeight}
		blocks, err := bStoreClient.GetBlockHeadersByHeight(ctx, &bstore.GetBlockHeadersByHeightRequest{
			Height: uint64(currentHeight),
		})
		if err != nil {
			logger.Errorf("Unable to GetBlockHeadersByHeight for  %s", height)
			break
		}

		for _, k := range blocks.BlockHeaders {

			resTransactions, err := bStoreClient.ListBlockTransactions(ctx, &bstore.ListBlockTransactionsRequest{
				Hash:      k.Hash,
				PageToken: "0",
				PageSize:  1,
			})
			if err != nil {
				logger.Errorf("Unable to ListBlockTransactions for  %s", k.Hash)
				continue
			}

			coinbaseRes, err := bStoreClient.GetTransaction(ctx, &bstore.GetTransactionRequest{
				Txid: resTransactions.TxIds[0],
			})
			if err != nil {
				logger.Errorf("Unable to GetTransaction for coinbase %s: %v", resTransactions.TxIds[0], err)
				continue
			}

			coinbaseTx, err := bsdecoder.DecodeRawTransaction(coinbaseRes, IsMainnet())
			if err != nil {
				logger.Errorf("unable to DecodeRawTransaction for coinbase: %v", err)
				continue
			}
			// Set the blockHeight in the coinbase tx to avoid duplication
			coinbaseTx.BlockHeight = nil

			txids := make([]string, 0)
			blockDetails, err := bsdecoder.DecodeRawBlock(k, coinbaseTx, txids)
			if err != nil {
				logger.Errorf("Unable to generate block JSON")
				continue
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

			blocksAtHeight.Headers = append(blocksAtHeight.Headers, blockDetails)
		}

		blocksResponse = append(blocksResponse, blocksAtHeight)
		// blockDetails, err := getBlock(hash, currentHeight)

		if currentHeight == endHeight {
			break
		}
		if order == "" || order == "asc" {
			currentHeight++
		} else {
			currentHeight--
		}
	} // for

	json.NewEncoder(w).Encode(blocksResponse)
}

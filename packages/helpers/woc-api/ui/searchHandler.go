package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/teranode-group/common/utils"
	"github.com/teranode-group/woc-api/bitcoin"
	"github.com/teranode-group/woc-api/bstore"
	"github.com/teranode-group/woc-api/configs"
	"github.com/teranode-group/woc-api/search"
	"github.com/teranode-group/woc-api/utxosmempool"
	"github.com/teranode-group/woc-api/utxostore"
)

type SearchResult struct {
	Count     int           `json:"count"`
	Type      string        `json:"type"`
	Message   string        `json:"message,omitempty"`
	TxID      string        `json:"txid,omitempty"`
	OpReturns search.Result `json:"opReturns,omitempty"`
	Tag       string        `json:"tag,omitempty"`
}

// Search returns search results
// isNumeric returns true if the string can be parsed as an int64.
func isNumeric(s string) bool {
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

var fiberHTTPClient = &http.Client{Timeout: 10 * time.Second}

const fiberTaggedOutputsURL = "http://localhost:8084/ui/searchtagoutput"

func fetchTaggedOutput(ctx context.Context, fullTag string) (string, error) {
	u := fiberTaggedOutputsURL + "?query=" + url.QueryEscape(fullTag)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Accept", "application/json")

	resp, err := fiberHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil
	}

	var out struct {
		FullTag string `json:"full_tag"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}

	return out.FullTag, nil
}
func Search(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	query := strings.TrimSpace(vars["query"])

	// Parse limit (for elastic search, etc.) with a default.
	limitStr := strings.ToLower(r.URL.Query().Get("limit"))
	if limitStr == "" {
		limitStr = "10"
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	searchResult := SearchResult{}

	// --- Support for <blockheight>.<txindex> format ---
	if strings.Contains(query, ".") {
		parts := strings.Split(query, ".")
		if len(parts) == 2 {
			part0 := strings.TrimSpace(parts[0])
			part1 := strings.TrimSpace(parts[1])
			if isNumeric(part0) && isNumeric(part1) {
				// Both parts are numeric, so try to parse them.
				blockHeight, err1 := strconv.ParseInt(part0, 10, 64)
				txIndex, err2 := strconv.Atoi(part1)
				if err1 != nil || err2 != nil || blockHeight < 0 || txIndex < 0 {
					// Bail out quietly so that the query falls through.
				} else {
					// Try Option 1: Use the bstore lookup.
					tx, err := bstore.GetTransactionByBlockHeightAndIndex(blockHeight, txIndex, limit)
					if err == nil && tx != nil {
						searchResult.Count = 1
						searchResult.Type = "tx"
						searchResult.TxID = tx.TxID
						w.WriteHeader(http.StatusOK)
						json.NewEncoder(w).Encode(searchResult)
						return
					}

					// Option 2 (fallback): Use the bitcoin client to get the block and then the tx.
					hash, err := bitcoinClient.GetBlockHash(int(blockHeight))
					if err != nil {
						logger.Errorf("Error getting block hash for height %d: %v", blockHeight, err)
						searchResult.Count = 0
						searchResult.Type = "not found"
						searchResult.Message = "Block not found"
						json.NewEncoder(w).Encode(searchResult)
						return
					}

					resp, err := bitcoinClient.GetBlock(hash)
					if err != nil {
						logger.Errorf("Error getting block for hash %s: %v", hash, err)
						searchResult.Count = 0
						searchResult.Type = "not found"
						searchResult.Message = "Error retrieving block details"
						json.NewEncoder(w).Encode(searchResult)
						return
					}
					if txIndex >= len(resp.Tx) {
						searchResult.Count = 0
						searchResult.Type = "not found"
						searchResult.Message = "Transaction index out of range"
						json.NewEncoder(w).Encode(searchResult)
						return
					}

					txid := resp.Tx[txIndex]
					searchResult.Count = 1
					searchResult.Type = "tx"
					searchResult.TxID = txid
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(searchResult)
					return
				}
			} // end if both parts are numeric
		} // end if len(parts)==2
	}

	// --- 64-character hash search ---
	if len(query) == 64 {
		if tx, _ := bstore.GetTx(query); tx != nil {
			searchResult.Count = 1
			searchResult.Type = "tx"
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(searchResult)
			return
		}

		if block, _ := bstore.GetBlockDetails(query, 0); block != nil {
			searchResult.Count = 1
			searchResult.Type = "blockhash"
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(searchResult)
			return
		}

		if txMapi, _ := bitcoin.GetRawTransactionFromTaalNode(query); txMapi != nil {
			searchResult.Count = 1
			searchResult.Type = "tx"
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(searchResult)
			return
		}

		if nonFinalMempoolSearchEnabled {
			if nonFinalMempool, _ := bitcoinClient.GetRawNonFinalMempool(); len(nonFinalMempool) > 0 {
				if containsTxId, _ := utils.SliceContains(nonFinalMempool, query); containsTxId {
					searchResult.Count = 1
					searchResult.Type = "Info"
					searchResult.Message = `Transaction is in the node's <a href="https://wiki.bitcoinsv.io/index.php/Transaction_Pools#Types_of_transaction_pool" target="_blank">Non-Final mempool</a>. Transaction details should be available after transitioning to the mempool.`
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(searchResult)
					return
				}
			}
		}

		if reason, err := bstore.GetDiscardedTxReason(query); err == nil {
			searchResult.Count = 1
			searchResult.Type = "WarningWithDiscardReason"
			searchResult.Message = reason
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(searchResult)
			return
		}

		if list, err := utxostore.GetConfirmedHistoryByScriptHash(query, 1, "", 0, 1); err == nil && len(list.ConfirmedTransactions) > 0 {
			searchResult.Count = 1
			searchResult.Type = "scriptHash"
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(searchResult)
			return
		}

		if listMempool, err := utxosmempool.GetMempoolHistoryByScriptHash(query, 1, "", 0, 1); err == nil && len(listMempool.MempoolTransactions) > 0 {
			searchResult.Count = 1
			searchResult.Type = "scriptHash"
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(searchResult)
			return
		}

		searchResult.Count = 0
		searchResult.Type = "not found"
		json.NewEncoder(w).Encode(searchResult)
		return
	}

	// --- Block height search ---
	if h, err := strconv.Atoi(query); err == nil {
		if block, _ := bstore.GetBlockDetails("", int64(h)); block != nil {
			searchResult.Count = 1
			searchResult.Type = "blockheight"
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(searchResult)
			return
		}
	}

	// --- Address search ---
	if addr, err := bitcoinClient.ValidateAddress(query); err == nil && addr.Address != "" {
		searchResult.Count = 1
		searchResult.Type = "address"
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(searchResult)
		return
	}

	//Tag search
	if configs.Settings.WocStatsEnabled {
		if tag, err := fetchTaggedOutput(r.Context(), query); err == nil && tag != "" {
			searchResult.Type = "tag"
			searchResult.Tag = tag
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(searchResult)
			return
		}
	}

	searchResult.Count = 0
	searchResult.Type = "not found"
	json.NewEncoder(w).Encode(searchResult)
}

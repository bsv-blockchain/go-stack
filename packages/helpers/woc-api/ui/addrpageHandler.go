package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/teranode-group/common/utils"
	utxo_store "github.com/teranode-group/proto/utxo-store"
	"github.com/teranode-group/woc-api/bstore"
	"github.com/teranode-group/woc-api/electrum"
	"github.com/teranode-group/woc-api/utxosmempool"
	"github.com/teranode-group/woc-api/utxostore"

	electrumTypes "github.com/checksum0/go-electrum/electrum"

	"github.com/gorilla/mux"
)

const (
	SRC_ELECTRUMX           = 0
	SRC_UTXO_STORE          = 1
	PAGINATION_WITH_OFFSET  = 1
	MAX_PAGESIZE_FOR_OFFSET = 20000
	ERR_HISTORY_TOO_LARGE   = "History too large"
)

type AddressPageResponse struct {
	Address              string                   `json:"address,omitempty"`
	Scripthash           string                   `json:"scripthash"`
	ScripthashType       string                   `json:"scripthashType"`
	ScriptPubKey         string                   `json:"scriptPubKey,omitempty"`
	TxCount              uint64                   `json:"txCount"`
	UnconfirmedTxCount   uint32                   `json:"unconfirmedTxCount,omitempty"`
	FirstSeenAt          int64                    `json:"firstSeenAt,omitempty"`
	Balance              *electrum.AddressBalance `json:"balance,omitempty"`
	History              []txWithMeta             `json:"history,omitempty"`
	Src                  int8                     `json:"src,omitempty"`
	PaginationWithOffset int8                     `json:"paginationWithoffset,omitempty"`
	PageToken            string                   `json:"pageToken,omitempty"`
	Order                string                   `json:"order,omitempty"`
	Error                string                   `json:"err,omitempty"`
}

type txWithMeta struct {
	Index    uint64        `json:"index"`
	TxID     string        `json:"txid"`
	Time     int64         `json:"time,omitempty"`
	Height   int32         `json:"height,omitempty"`
	Tags     []bstore.Tags `json:"tags,omitempty"`
	InValue  *float64      `json:"inValue,omitempty"`
	OutValue *float64      `json:"outValue,omitempty"`
}

type ScripthashBlockBalance struct {
	Timestamp        int64  `json:"timestamp"`
	BlockHeight      int32  `json:"block_height,omitempty"`
	SpentSatoshis    uint64 `json:"spent_satoshis"`
	ReceivedSatoshis uint64 `json:"received_satoshis"`
	TxCount          int32  `json:"tx_count"`
	UtxoCount        int32  `json:"utxo_count"`
}

// TODO: This is very first call on the address/script page
// if the response has err = historytoolarge and src = SRC_UTXO_STORE then ui triggers
// unconfirmed and confirmed tabs
func GetAddressOrScripthashPage(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	addressOrHash := vars["addressOrScripthash"]
	isScripthash := false
	// var height int64 //TODO:
	if len(addressOrHash) == 64 {
		isScripthash = true
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

	//offset
	offsetStr := strings.ToLower(r.URL.Query().Get("offset"))
	if offsetStr == "" {
		offsetStr = "0"
	}
	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil || offset < 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//token
	pageTokenStr := strings.ToLower(r.URL.Query().Get("token"))

	//order
	orderStr := strings.ToLower(r.URL.Query().Get("order"))

	// Default order desc
	order := 1
	if orderStr == "asc" {
		order = 0
	} else {
		orderStr = "desc" //used for response
	}

	// Default source is electrumx
	srcStr := strings.ToLower(r.URL.Query().Get("src"))
	if srcStr == "" {
		srcStr = "0"
	}

	src, err := strconv.ParseInt(srcStr, 10, 8)
	if err != nil || (src != SRC_ELECTRUMX && src != SRC_UTXO_STORE) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if utxoStorePriorityForUI {
		src = SRC_UTXO_STORE
	}

	scriptHash := addressOrHash
	scriptPubKey := ""

	if isScripthash == false {
		scriptHash, err = utils.AddressToScriptHash(addressOrHash, network)
		if err != nil {
			logger.Errorf("error: AddressToScriptHash request failure for address %s , %+v", addressOrHash, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		scriptPubKey, err = utils.AddressToScriptPubKey(addressOrHash, isMainnet)
		if err != nil {
			logger.Errorf("error: AddressToScriptPubKey request failure for address %s , %+v", addressOrHash, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	electrumXConfBalance := 0.0
	balance := &electrum.AddressBalance{}

	if src != SRC_UTXO_STORE {
		balance, err = wrappedEleC.electrumClient.GetAddressBalance(scriptHash)
		if err != nil {
			logger.Errorf("Couldn't get balance from electrumX for address %+v", err)
			// w.WriteHeader(http.StatusNotFound)
			// return

			// switch to utxo-store
			src = SRC_UTXO_STORE
		} else {

			electrumXConfBalance = balance.Confirmed

			if balance.Confirmed > 0 {
				balance.Confirmed = float64(balance.Confirmed) / float64(1e8)
			}

			if balance.Unconfirmed > 0 {
				balance.Unconfirmed = float64(balance.Unconfirmed) / float64(1e8)
			}
		}

	}

	//building result
	result := &AddressPageResponse{
		Balance:      balance,
		Scripthash:   scriptHash,
		ScriptPubKey: scriptPubKey,
		Order:        orderStr,
	}

	if !isScripthash {
		result.Address = addressOrHash
	}

	txCount := 0
	offsetPossible := false

	//get history from electrumx
	var history []*electrumTypes.GetMempoolResult
	if src != SRC_UTXO_STORE {
		history, err = wrappedEleC.electrumClient.GetAddressHistoryOrTooLargeError(scriptHash)
	} else {
		err = nil
	}

	if err != nil || src == SRC_UTXO_STORE {

		history = nil
		// electrumx error - try to get history from utxo-store if balance matches
		if utxostore.IsEnabled() {
			var scripts *utxo_store.GetScripthashesByAddressResponse

			if !isScripthash {
				//get associated scripts
				scripts, err = utxostore.GetConfirmedScriptsByAddress(addressOrHash)
				if err != nil {
					logger.Errorf("error: Couldn't get scripts by address %s , %+v", addressOrHash, err)
				} else if scripts != nil && scripts.ScripthashType != nil {
					for _, s := range scripts.ScripthashType {
						if s.Scripthash == scriptHash {
							result.ScripthashType = s.Type
						}
					}
				}
			}

			//confirmed
			utxoScriptHash := scriptHash
			if !isScripthash && scripts != nil && scripts.ScripthashType != nil && len(scripts.ScripthashType) > 0 {
				// default to first script; prefer explicit "pubkey" (P2PK) if present
				utxoScriptHash = scripts.ScripthashType[0].Scripthash
				for _, s := range scripts.ScripthashType {
					if s.Type == "pubkeyhash" {
						utxoScriptHash = s.Scripthash
						break
					}
				}
				// surface the picked type, if available
				for _, s := range scripts.ScripthashType {
					if s.Scripthash == utxoScriptHash {
						result.ScripthashType = s.Type
						break
					}
				}
			}
			// when UI is backed by UTXO-store, reflect the canonical script
			if src == SRC_UTXO_STORE {
				result.Scripthash = utxoScriptHash
			}

			// confirmed
			utxoStoreBalance, errBalance := utxostore.GetBalancePITB(utxoScriptHash)
			if errBalance != nil {
				logger.Errorf("error: Couldn't get utxo store  GetBalancePITB balance for scripthash %s , %+v", scriptHash, err)
			}

			if errBalance == nil && utxoStoreBalance != nil && utxoStoreBalance.Unspent != nil {
				balance.Confirmed = float64(utxoStoreBalance.Unspent.Satoshis) / float64(1e8)
				//TODO: utxo-mempool call for unconfirmed
				result.Balance = balance
			}

			// if its an address add other associated scripts balance
			if !isScripthash && scripts != nil && scripts.ScripthashType != nil {
				for _, s := range scripts.ScripthashType {
					if s.Scripthash != utxoScriptHash && s.Scripthash != "multisig" {
						otherBalance, errBalance := utxostore.GetBalancePITB(s.Scripthash)
						if errBalance != nil {
							logger.Errorf("error: Couldn't get utxo store  GetBalancePITB balance for scripthash %s , %+v", scriptHash, err)
						}

						if errBalance == nil && otherBalance != nil && otherBalance.Unspent != nil {
							balance.Confirmed = balance.Confirmed + float64(otherBalance.Unspent.Satoshis)/float64(1e8)
							//TODO: utxo-mempool call for unconfirmed
							result.Balance = balance
						}
					}
				}
			}

			//unconfirmed
			utxosmempoolBalance, errMempoolBalance := utxosmempool.GetMempoolBalanceByScriptHash(utxoScriptHash)
			if errMempoolBalance != nil {
				logger.Errorf("error: Couldn't get utxo store  GetMempoolBalanceByScriptHash balance for scripthash %s , %+v", scriptHash, err)
			}

			if errMempoolBalance == nil && utxosmempoolBalance != nil && utxosmempoolBalance.ScripthashMempool != nil &&
				utxosmempoolBalance.ScripthashMempool.Unspent != nil {
				balance.Unconfirmed = float64(utxosmempoolBalance.ScripthashMempool.Unspent.Satoshis) / float64(1e8)
				//TODO: utxo-mempool call for unconfirmed
				result.Balance = balance
				if utxosmempoolBalance.ScripthashMempool.Unspent.Qty > 0 {
					result.Src = SRC_UTXO_STORE
					result.UnconfirmedTxCount = uint32(utxosmempoolBalance.ScripthashMempool.Unspent.Qty)
				}
			}

			utxoStoreHistoryStats, errHistoryStats := utxostore.GetScriptHashHistoryStatsPITB(utxoScriptHash)
			if errHistoryStats != nil {
				logger.Errorf("error: Couldn't get utxo history GetScriptHashHistoryStatsPITB stats for scripthash %s , %+v", scriptHash, err)
			}

			if (errBalance == nil && utxoStoreBalance.Unspent != nil) ||
				(errHistoryStats == nil && utxoStoreHistoryStats != nil && utxoStoreHistoryStats.TotalTxsQty > 0) {

				utxoStorePageSize := limit
				if utxoStoreHistoryStats.TotalTxsQty <= MAX_PAGESIZE_FOR_OFFSET {
					offsetPossible = true
					utxoStorePageSize = MAX_PAGESIZE_FOR_OFFSET
				}

				list, err := utxostore.GetConfirmedHistoryByScriptHash(utxoScriptHash, int32(utxoStorePageSize), pageTokenStr, 0, order)
				if err != nil {
					logger.Errorf("error: Couldn't get utxo store GetConfirmedHistoryByScriptHash history for scripthash %s , %+v", scriptHash, err)
				} else {
					for _, tx := range list.ConfirmedTransactions {
						record := electrumTypes.GetMempoolResult{Hash: tx.GetTxId(), Height: int32(tx.BlockHeight)}
						history = append([]*electrumTypes.GetMempoolResult{&record}, history...)
					}
					if len(history) > 0 {
						if errHistoryStats == nil && utxoStoreHistoryStats != nil {
							txCount = int(utxoStoreHistoryStats.TotalTxsQty)
							// When utxoStoreHistoryStats.TotalTxsQty is 0 but history is provided
							// it means history is too large to calcualte the count by DB

						} else {
							txCount = len(history)
							result.Error = ERR_HISTORY_TOO_LARGE
						}
						result.Src = SRC_UTXO_STORE
						result.PageToken = list.ConfirmedNextPageToken

					}
				}

			}

			if utxoStorePriorityForUI != true && utxoStoreBalance != nil && utxoStoreBalance.Unspent != nil && utxoStoreBalance.Unspent.Satoshis != int64(electrumXConfBalance) {
				logger.Warnf(
					"Warn: Balance mismatch for hash %s - Confirmed: { ElectrumX: %d, UTXO: %v }",
					scriptHash,
					electrumXConfBalance,
					utxoStoreBalance,
				)
			}

		} else {

			logger.Errorf("Couldn't get electrumx history for scripthash  %s , %+v", scriptHash, err)

			if err != nil && strings.Contains(err.Error(), ERR_HISTORY_TOO_LARGE) {
				result.Src = SRC_ELECTRUMX
				result.Error = ERR_HISTORY_TOO_LARGE
			}

		}

	} else {
		txCount = len(history)
		result.Src = SRC_ELECTRUMX
	}

	historyWithDetails := make([]txWithMeta, 0)

	if txCount > 0 {
		result.TxCount = uint64(txCount)

		//TODO: electumX docs: "0 if all inputs are confirmed, and -1 otherwise.Fee is only present in unconfirm tx"
		//only  satoshi tx at height 0
		if result.Src != SRC_UTXO_STORE && history[0].Height > 0 || history[0].Hash == "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b" {
			firstTxDetails, err := bstore.GetTxLight(history[0].Hash)
			if err != nil {
				logger.Errorf("unable to get details of tx: %s , %s", history[0].Hash, err)
			} else {

				if len(firstTxDetails.BlockHash) > 0 {
					result.FirstSeenAt = firstTxDetails.Time
				}
			}

		}

		// utxo-store does't have offset params
		if result.Src == SRC_UTXO_STORE && offsetPossible == false {
			offset = 0
		}

		if offset < int64(txCount) {

			if result.Src != SRC_UTXO_STORE || (result.Src == SRC_UTXO_STORE && offsetPossible == true) {
				// sort the history = latest first if src is electrumx or when offsetPossible is set for utxo-store
				for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
					history[i], history[j] = history[j], history[i]
				}

				// only process txs for the page
				history = history[offset:Min(int64(txCount), offset+limit)]
			}

			//add index, inValue, outValue and tags
			for i, tx := range history {

				//default decending
				index := uint64((txCount - 1) - (i + int(offset)))

				//assending only for utxo-store as src
				if result.Src == SRC_UTXO_STORE && orderStr == "asc" {
					index = uint64((i + int(offset)))
				}

				txInfo := txWithMeta{Index: index, TxID: tx.Hash, Height: tx.Height}
				var tagsArray []bstore.Tags

				txDetails, err := bstore.GetTxWithRange(tx.Hash, -1, -1, 0, int(limit))
				if err != nil {
					logger.Errorf("unable to get details of tx: %s , %s", tx.Hash, err)
					historyWithDetails = append(historyWithDetails, txInfo)
					continue
				}

				if txDetails.Time > 0 {
					txInfo.Time = int64(txDetails.Time)
				}

				// vout tags , outValue
				var totalOutValue float64
				for _, v := range txDetails.Vout {
					if v.Scripthash == result.Scripthash {
						totalOutValue += v.Value
					}

					//tag nonstandard
					if v.ScriptPubKey.Type == "nonstandard" {
						tag, err := bstore.GetNonStandardTag(v.ScriptPubKey.Hex)
						if err != nil {
							continue
						}

						if len(tag.Type) > 0 {
							index, err := bstore.ContainsAtIndex(tagsArray, tag.Type, tag.Action)
							if err == nil && index >= 0 {
								tagsArray[index].Count++
							} else {
								tagsArray = append(tagsArray, bstore.Tags{Type: tag.Type, Action: tag.Action, Count: 1})
							}
						}

						continue
					}
					//dont't need full asm string
					asm := ""
					if len(v.ScriptPubKey.ASM) > 50 {
						asm = v.ScriptPubKey.ASM[0:50]
						v.ScriptPubKey.ASM = ""
					} else {
						asm = v.ScriptPubKey.ASM
					}

					//tag opreturn
					tag, err := bstore.GetOpReturnTag(asm, v.ScriptPubKey.Hex)
					if err != nil {
						continue
					}

					if len(tag.Type) > 0 {
						index, err := bstore.ContainsAtIndex(tagsArray, tag.Type, tag.Action)
						if err == nil && index >= 0 {
							tagsArray[index].Count++
						} else {
							tagsArray = append(tagsArray, bstore.Tags{Type: tag.Type, Action: tag.Action, Count: 1})
						}
					}
				}
				if totalOutValue > 0 {
					txInfo.OutValue = &totalOutValue
				}

				if len(tagsArray) > 0 {
					txInfo.Tags = tagsArray
				}

				if txDetails.VinCount <= GetMaxVinCountForProcessing() {
					//workout inValue
					var totalInValue float64
					for _, v := range txDetails.Vin {

						if len(v.Coinbase) == 0 {
							txd, err := bstore.GetTxLight(v.Txid)
							if err != nil {
								//TODO
								logger.Errorf("unable to get details of tx: %s , %s", v.Txid, err)
								break
							}
							var outIndex = int(*v.Vout)
							if txd.Vout[outIndex].Scripthash == result.Scripthash {
								totalInValue += txd.Vout[outIndex].Value
							}
						}
					}

					if totalInValue > 0 {
						txInfo.InValue = &totalInValue
					}
				}

				historyWithDetails = append(historyWithDetails, txInfo)
			}

			result.History = historyWithDetails

			//make sure the data us sorted for the view
			if result.Src == SRC_UTXO_STORE {
				if orderStr == "asc" {
					sort.Slice(result.History[:], func(i, j int) bool {
						return result.History[i].Time < result.History[j].Time
					})
				} else {
					sort.Slice(result.History[:], func(i, j int) bool {
						return result.History[i].Time > result.History[j].Time
					})
				}

			}
		}
	}

	//Patch to make sure we don't display incorrect count if we are unable to fetch the exact count
	if result.Error == ERR_HISTORY_TOO_LARGE && result.Src == SRC_UTXO_STORE {
		result.TxCount = 0
	}

	if result.Src == SRC_UTXO_STORE && offsetPossible {
		result.PaginationWithOffset = PAGINATION_WITH_OFFSET
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)

}

// This only serves data from utxo-store
func GetConfirmedHistoryByScripthash(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	addressOrHash := vars["addressOrScripthash"]
	isScripthash := false
	// var height int64 //TODO:
	if len(addressOrHash) == 64 {
		isScripthash = true
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

	//token
	pageTokenStr := strings.ToLower(r.URL.Query().Get("token"))

	//order
	orderStr := strings.ToLower(r.URL.Query().Get("order"))

	// Default order desc
	order := 1
	if orderStr == "asc" {
		order = 0
	} else {
		orderStr = "desc" //used for response
	}

	scriptHash := addressOrHash
	scriptPubKey := ""

	if isScripthash == false {
		scriptHash, err = utils.AddressToScriptHash(addressOrHash, network)
		if err != nil {
			logger.Errorf("error: AddressToScriptHash request failure for address %s , %+v", addressOrHash, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		scriptPubKey, err = utils.AddressToScriptPubKey(addressOrHash, isMainnet)
		if err != nil {
			logger.Errorf("error: AddressToScriptPubKey request failure for address %s , %+v", addressOrHash, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	//building result
	result := &AddressPageResponse{
		Scripthash:   scriptHash,
		ScriptPubKey: scriptPubKey,
		Order:        orderStr,
		Src:          SRC_UTXO_STORE,
	}

	if !isScripthash {
		result.Address = addressOrHash
	}

	txCount := 0

	var history []*electrumTypes.GetMempoolResult

	// electrumx error - try to get history from utxo-store if balance matches

	utxoStoreBalance, errBalance := utxostore.GetBalancePITB(scriptHash)
	if err != nil {
		logger.Errorf("error: Couldn't get utxo store balance for scripthash %s , %+v", scriptHash, err)
	} else if utxoStoreBalance != nil && utxoStoreBalance.Unspent != nil {
		result.Balance = &electrum.AddressBalance{Confirmed: float64(utxoStoreBalance.Unspent.Satoshis) / float64(1e8)}
	}

	utxoStoreHistoryStats, errHistoryStats := utxostore.GetScriptHashHistoryStatsPITB(scriptHash)
	if err != nil {
		result.Error = ERR_HISTORY_TOO_LARGE
		logger.Errorf("error: Couldn't get utxo history stats for scripthash %s , %+v", scriptHash, err)
	} else if utxoStoreHistoryStats != nil {
		result.TxCount = uint64(utxoStoreHistoryStats.TotalTxsQty)
		txCount = int(utxoStoreHistoryStats.TotalTxsQty)
	}

	if (errBalance == nil && utxoStoreBalance != nil && utxoStoreBalance.Unspent != nil) ||
		(errHistoryStats == nil && utxoStoreHistoryStats != nil && utxoStoreHistoryStats.TotalTxsQty > 0) {

		list, err := utxostore.GetConfirmedHistoryByScriptHash(scriptHash, int32(limit), pageTokenStr, 0, order)

		if err != nil {
			logger.Errorf("error: Couldn't get utxo store history for scripthash %s , %+v", scriptHash, err)
		} else {

			for _, tx := range list.ConfirmedTransactions {
				record := electrumTypes.GetMempoolResult{Hash: tx.GetTxId(), Height: int32(tx.BlockHeight)}
				history = append([]*electrumTypes.GetMempoolResult{&record}, history...)
			}
			result.PageToken = list.ConfirmedNextPageToken
		}

	}

	historyWithDetails := make([]txWithMeta, 0)

	if len(history) > 0 {

		//if were unabel to get txCount from history Stats endpoint
		if txCount == 0 {
			txCount = len(history)
		}

		//add index, inValue, outValue and tags
		for i, tx := range history {

			//default decending
			index := uint64((txCount - 1) - (i))

			//assending only for utxo-store as src
			if orderStr == "asc" {
				index = uint64(i)
			}

			txInfo := txWithMeta{Index: index, TxID: tx.Hash, Height: tx.Height}
			var tagsArray []bstore.Tags

			txDetails, err := bstore.GetTxWithRange(tx.Hash, -1, -1, 0, int(limit))
			if err != nil {
				logger.Errorf("unable to get details of tx: %s , %s", tx.Hash, err)
				historyWithDetails = append(historyWithDetails, txInfo)
				continue
			}

			if txDetails.Time > 0 {
				txInfo.Time = int64(txDetails.Time)
			}

			// vout tags , outValue
			var totalOutValue float64
			for _, v := range txDetails.Vout {
				if v.Scripthash == result.Scripthash {
					totalOutValue += v.Value
				}

				//tag nonstandard
				if v.ScriptPubKey.Type == "nonstandard" {
					tag, err := bstore.GetNonStandardTag(v.ScriptPubKey.Hex)
					if err != nil {
						continue
					}

					if len(tag.Type) > 0 {
						index, err := bstore.ContainsAtIndex(tagsArray, tag.Type, tag.Action)
						if err == nil && index >= 0 {
							tagsArray[index].Count++
						} else {
							tagsArray = append(tagsArray, bstore.Tags{Type: tag.Type, Action: tag.Action, Count: 1})
						}
					}

					continue
				}

				//dont't need full asm string
				asm := ""
				if len(v.ScriptPubKey.ASM) > 50 {
					asm = v.ScriptPubKey.ASM[0:50]
					v.ScriptPubKey.ASM = ""
				} else {
					asm = v.ScriptPubKey.ASM
				}
				//tag opreturn
				tag, err := bstore.GetOpReturnTag(asm, v.ScriptPubKey.Hex)
				if err != nil {
					continue
				}

				if len(tag.Type) > 0 {
					index, err := bstore.ContainsAtIndex(tagsArray, tag.Type, tag.Action)
					if err == nil && index >= 0 {
						tagsArray[index].Count++
					} else {
						tagsArray = append(tagsArray, bstore.Tags{Type: tag.Type, Action: tag.Action, Count: 1})
					}
				}
			}
			if totalOutValue > 0 {
				txInfo.OutValue = &totalOutValue
			}

			if len(tagsArray) > 0 {
				txInfo.Tags = tagsArray
			}

			if txDetails.VinCount <= GetMaxVinCountForProcessing() {
				//workout inValue
				var totalInValue float64
				for _, v := range txDetails.Vin {

					if len(v.Coinbase) == 0 {
						txd, err := bstore.GetTxLight(v.Txid)
						if err != nil {
							//TODO
							logger.Errorf("unable to get details of tx: %s , %s", v.Txid, err)
							break
						}
						var outIndex = int(*v.Vout)
						if txd.Vout[outIndex].Scripthash == result.Scripthash {
							totalInValue += txd.Vout[outIndex].Value
						}
					}
				}

				if totalInValue > 0 {
					txInfo.InValue = &totalInValue
				}
			}

			historyWithDetails = append(historyWithDetails, txInfo)
		}

		result.History = historyWithDetails

		//make sure the data us sorted for the view
		if result.Src == SRC_UTXO_STORE {
			if orderStr == "asc" {
				sort.Slice(result.History[:], func(i, j int) bool {
					return result.History[i].Time < result.History[j].Time
				})
			} else {
				sort.Slice(result.History[:], func(i, j int) bool {
					return result.History[i].Time > result.History[j].Time
				})
			}

		}

	}

	if utxostore.IsEnabled() && !isScripthash {

		scripts, err := utxostore.GetConfirmedScriptsByAddress(addressOrHash)
		if err != nil {
			logger.Errorf("error: Couldn't get scripts by address %s , %+v", addressOrHash, err)
		} else {
			for _, s := range scripts.ScripthashType {
				if s.Scripthash == scriptHash {
					result.ScripthashType = s.Type
				}
			}
		}

	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)

}

// This only serves data from utxos-mempool
func GetMempoolHistoryByScripthash(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	addressOrHash := vars["addressOrScripthash"]
	isScripthash := false
	// var height int64 //TODO:
	if len(addressOrHash) == 64 {
		isScripthash = true
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

	//offset
	offsetStr := strings.ToLower(r.URL.Query().Get("offset"))
	if offsetStr == "" {
		offsetStr = "0"
	}
	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil || offset < 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//offset
	pageTokenStr := strings.ToLower(r.URL.Query().Get("token"))

	//order
	orderStr := strings.ToLower(r.URL.Query().Get("order"))

	// Default order desc
	order := 1
	if orderStr == "asc" {
		order = 0
	} else {
		orderStr = "desc" //used for response
	}

	// Default source is electrumx
	srcStr := strings.ToLower(r.URL.Query().Get("src"))
	if srcStr == "" {
		srcStr = "0"
	}

	src, err := strconv.ParseInt(srcStr, 10, 8)
	if err != nil || (src != SRC_ELECTRUMX && src != SRC_UTXO_STORE) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	scriptHash := addressOrHash

	if isScripthash == false {
		scriptHash, err = utils.AddressToScriptHash(addressOrHash, network)
		if err != nil {
			logger.Errorf("error: AddressToScriptHash request failure for scripthash %s , %+v", addressOrHash, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	//building result
	result := &AddressPageResponse{
		Scripthash: scriptHash,
		Order:      orderStr,
	}

	if !isScripthash {
		result.Address = addressOrHash
	}

	txCount := 0

	//This will give us total tx count
	utxosMempoolBalance, err := utxosmempool.GetMempoolBalanceByScriptHash(scriptHash)

	list, err := utxosmempool.GetMempoolHistoryByScriptHash(scriptHash, int32(limit), pageTokenStr, 0, order)

	var history []*electrumTypes.GetMempoolResult

	if err != nil {
		logger.Errorf("error: Couldn't get utxos mempool history for scripthash %s , %+v", scriptHash, err)
	} else {

		for _, tx := range list.MempoolTransactions {
			record := electrumTypes.GetMempoolResult{Hash: tx.GetTxId()}
			history = append([]*electrumTypes.GetMempoolResult{&record}, history...)
		}
		if len(history) > 0 {
			txCount = int(utxosMempoolBalance.ScripthashMempool.TotalTxsQty)
			result.Src = SRC_UTXO_STORE
			result.PageToken = list.NextPageToken
		}
	}

	historyWithDetails := make([]txWithMeta, 0)

	if txCount > 0 {
		result.TxCount = uint64(txCount)

		if offset < int64(txCount) {

			//add index, inValue, outValue and tags
			for i, tx := range history {

				//default decending
				index := uint64((txCount - 1) - (i + int(offset)))

				//assending only for utxo-store as src
				if result.Src == SRC_UTXO_STORE && orderStr == "asc" {
					index = uint64((i + int(offset)))
				}

				txInfo := txWithMeta{Index: index, TxID: tx.Hash, Height: tx.Height}
				var tagsArray []bstore.Tags

				txDetails, err := bstore.GetTxWithRange(tx.Hash, -1, -1, 0, int(limit))
				if err != nil {
					logger.Errorf("unable to get details of tx: %s , %s", tx.Hash, err)
					historyWithDetails = append(historyWithDetails, txInfo)
					continue
				}

				if txDetails.Time > 0 {
					txInfo.Time = int64(txDetails.Time)
				}

				// vout tags , outValue
				var totalOutValue float64
				for _, v := range txDetails.Vout {
					if v.Scripthash == result.Scripthash {
						totalOutValue += v.Value
					}

					//tag nonstandard
					if v.ScriptPubKey.Type == "nonstandard" {
						tag, err := bstore.GetNonStandardTag(v.ScriptPubKey.Hex)
						if err != nil {
							continue
						}

						if len(tag.Type) > 0 {
							index, err := bstore.ContainsAtIndex(tagsArray, tag.Type, tag.Action)
							if err == nil && index >= 0 {
								tagsArray[index].Count++
							} else {
								tagsArray = append(tagsArray, bstore.Tags{Type: tag.Type, Action: tag.Action, Count: 1})
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

					tag, err := bstore.GetOpReturnTag(asm, v.ScriptPubKey.Hex)
					if err != nil {
						continue
					}

					if len(tag.Type) > 0 {
						index, err := bstore.ContainsAtIndex(tagsArray, tag.Type, tag.Action)
						if err == nil && index >= 0 {
							tagsArray[index].Count++
						} else {
							tagsArray = append(tagsArray, bstore.Tags{Type: tag.Type, Action: tag.Action, Count: 1})
						}
					}
				}
				if totalOutValue > 0 {
					txInfo.OutValue = &totalOutValue
				}

				if len(tagsArray) > 0 {
					txInfo.Tags = tagsArray
				}

				if txDetails.VinCount <= GetMaxVinCountForProcessing() {
					//workout inValue
					var totalInValue float64
					for _, v := range txDetails.Vin {

						if len(v.Coinbase) == 0 {

							txd, err := bstore.GetTxLight(v.Txid)

							if err != nil {
								//TODO
								logger.Errorf("unable to get details of tx: %s , %s", v.Txid, err)
								break
							}
							var outIndex = int(*v.Vout)
							if txd.Vout[outIndex].Scripthash == result.Scripthash {
								totalInValue += txd.Vout[outIndex].Value
							}

						}
					}

					if totalInValue > 0 {
						txInfo.InValue = &totalInValue
					}
				}

				historyWithDetails = append(historyWithDetails, txInfo)
			}

			result.History = historyWithDetails
		}
	}

	if utxostore.IsEnabled() && !isScripthash {

		scripts, err := utxostore.GetConfirmedScriptsByAddress(addressOrHash)
		if err != nil {
			logger.Errorf("error: Couldn't get scripts by address %s , %+v", addressOrHash, err)
		} else {

			for _, s := range scripts.ScripthashType {
				if s.Scripthash == scriptHash {
					result.ScripthashType = s.Type
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
	return

}

func GetScripthashBlockBalance(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	hash := vars["scripthash"]
	var scripthashBlockBalanceList []ScripthashBlockBalance

	scripthashBlockBlance, err := utxostore.GetScripthashBlockBalance(hash)
	if err != nil {
		logger.Errorf("error: Couldn't get utxo store balance for scripthash %s , %+v", hash, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//Switch to daily if number of items greater then or equal to 20000
	if len(scripthashBlockBlance) > 1000 {

		dailyTotals := make(map[int64]ScripthashBlockBalance)

		for _, balance := range scripthashBlockBlance {

			day := time.Unix(balance.Timestamp.Seconds, 0).In(time.UTC).Truncate(24 * time.Hour).Unix()

			if existingTxn, ok := dailyTotals[day]; ok {

				dailyTotals[day] = ScripthashBlockBalance{
					Timestamp:        day,
					SpentSatoshis:    existingTxn.SpentSatoshis + balance.SpentSatoshis,
					ReceivedSatoshis: existingTxn.ReceivedSatoshis + balance.ReceivedSatoshis,
					TxCount:          existingTxn.TxCount + balance.TxCount,
					UtxoCount:        existingTxn.UtxoCount + balance.UtxoCount,
				}
			} else {

				if day == 0 {
					fmt.Printf("%d", day)
				}
				dailyTotals[day] = ScripthashBlockBalance{
					Timestamp:        day,
					SpentSatoshis:    balance.SpentSatoshis,
					ReceivedSatoshis: balance.ReceivedSatoshis,
					TxCount:          existingTxn.TxCount + balance.TxCount,
					UtxoCount:        existingTxn.UtxoCount + balance.UtxoCount,
				}
			}
		}

		for _, v := range dailyTotals {
			scripthashBlockBalanceList = append(scripthashBlockBalanceList, v)
		}

		sort.Slice(scripthashBlockBalanceList, func(i, j int) bool {
			return scripthashBlockBalanceList[i].Timestamp < scripthashBlockBalanceList[j].Timestamp
		})
	} else {
		for _, item := range scripthashBlockBlance {
			newItem := ScripthashBlockBalance{
				Timestamp:        item.Timestamp.Seconds,
				SpentSatoshis:    item.SpentSatoshis,
				ReceivedSatoshis: item.ReceivedSatoshis,
				BlockHeight:      item.BlockHeight,
				TxCount:          item.TxCount,
				UtxoCount:        item.UtxoCount,
			}
			scripthashBlockBalanceList = append(scripthashBlockBalanceList, newItem)
		}

	}

	if len(scripthashBlockBalanceList) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(scripthashBlockBalanceList)

}

func Min(value_0, value_1 int64) int64 {
	if value_0 < value_1 {
		return value_0
	}
	return value_1
}

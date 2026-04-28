package ui

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/libsv/go-bt"
	"github.com/teranode-group/common/bsdecoder"
	"github.com/teranode-group/common/utils"
	"github.com/teranode-group/woc-api/bitcoin"
	"github.com/teranode-group/woc-api/bstore"
	"github.com/teranode-group/woc-api/internal"
	"github.com/teranode-group/woc-api/pools"
	"github.com/teranode-group/woc-api/redis"
	"github.com/teranode-group/woc-api/utxosmempool"
	"github.com/teranode-group/woc-api/utxostore"
)

// TODO: MO: This should come from a pre calculated table. woc-stats?
func GetTxStats(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	txid := vars["txid"]

	if len(txid) != 64 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//check in cache first
	var cached bsdecoder.RawTransaction
	if redis.RedisClient.Enabled {
		err := redis.GetCachedValue(txid+"_stats", &cached, nil)

		if err == nil && len(cached.TxID) == 64 {
			json.NewEncoder(w).Encode(cached)
			return
		}
	}

	txDetails, err := bstore.GetTxLight(txid)

	if err != nil {

		//Check if it is taal mapi tx
		if taalBitcoinProxyEnabled {

			tx, err := bitcoin.GetRawTransactionFromTaalNode(txid)

			if err == nil {
				// only few mempool txs will end up here
				// we have json but to put in same structure as bstore
				txDetails, err = bstore.ParseRawTx(tx.Hex)
				if err != nil {
					logger.Errorf("Unable to ParseRawTx %v", tx.Hex)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				txDetails.Src = "taalmapi"
			} else {
				logger.Warnf("unable to get details from the bstore and the TAAL node: %s , %s", txid, err)
				w.WriteHeader(http.StatusNotFound)
				return
			}

		} else {
			logger.Errorf("unable to get details of tx: %s , %s", txid, err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}

	//workout total input
	var totalInValue float64
	txMap := make(map[string]*bsdecoder.RawTransaction)

	for _, v := range txDetails.Vin {

		if len(v.Coinbase) == 0 {

			var txd *bsdecoder.RawTransaction
			_, ok := txMap[v.Txid]
			if !ok {
				txd, err = bstore.GetTxLight(v.Txid)
			} else {
				txd = txMap[v.Txid]
			}

			if err != nil {

				// get get inputs from taalmapi if main tx is from taalmapi
				if taalBitcoinProxyEnabled && txDetails.Src == "taalmapi" {
					tx, err := bitcoin.GetRawTransactionFromTaalNode(v.Txid)

					if err == nil {
						// only few mempool txs will end up here
						//mehhh... we have json but to put in same structure as bstore
						txd, err = bstore.ParseRawTx(tx.Hex)
						if err != nil {
							logger.Errorf("Unable to ParseRawTx %v", tx.Hex)
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
						// txDetails.Src = "taalmapi"
					} else {
						logger.Warnf("unable to get details from the bstore and the TAAL node: %s , %s", txid, err)
						w.WriteHeader(http.StatusNotFound)
						return
					}
				} else {

					logger.Errorf("unable to get details of tx from bstore: %s , %s", v.Txid, err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			if txd != nil {
				txMap[v.Txid] = txd
				var outIndex = int(*v.Vout)
				totalInValue += txd.Vout[outIndex].Value
			}

		} else if len(v.Coinbase) > 0 {
			var height = uint64(*txDetails.BlockHeight)
			totalInValue += utils.CalculateReward(height)

		}
	}

	txDetails.VinValue = &totalInValue

	//remove Vin & Vout details
	txDetails.Vout = nil
	txDetails.Vin = nil
	txDetails.Hex = ""

	// Cache this response
	if redis.RedisClient.Enabled && txDetails.VinCount > GetMaxVinCountForProcessing() {
		//TODO:Mo: Should use Expire?? on these keys
		//SetCacheValueWithExpire
		err = redis.SetCacheValue(txid+"_stats", txDetails, nil)

		if err != nil {
			logger.Errorf("Unable to cache GetTxStats response %+v\n", err)
		}
	}

	json.NewEncoder(w).Encode(txDetails)

}

// TODO: Remove bad txids
func GetTxDetails(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txid := vars["txid"]

	if len(txid) != 64 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Default order desc
	order := strings.ToLower(r.URL.Query().Get("order"))
	if order == "" || (order != "desc" && order != "asc") {
		order = "desc"
	}

	// limit
	limitStr := strings.ToLower(r.URL.Query().Get("iolimit"))
	if limitStr == "" {
		limitStr = "10" //TODO: config settings?
	}
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit < 1 || limit > 50 {
		limit = 10 //TODO: config settings?
	}

	// vinOffset
	vinOffsetStr := strings.ToLower(r.URL.Query().Get("vinOffset"))
	if vinOffsetStr == "" {
		vinOffsetStr = "0"
	}
	vinOffset, err := strconv.ParseInt(vinOffsetStr, 10, 64)
	if err != nil {
		vinOffset = 0
	}

	// voutOffset
	voutOffsetStr := strings.ToLower(r.URL.Query().Get("voutOffset"))
	if voutOffsetStr == "" {
		voutOffsetStr = "0"
	}
	voutOffset, err := strconv.ParseInt(voutOffsetStr, 10, 64)
	if err != nil {
		voutOffset = 0
	}

	//skip total inputs value if requested
	skipTotalVin := false
	skipTotalVinStr := strings.ToLower(r.URL.Query().Get("skipTotalVin"))

	if skipTotalVinStr == "true" {
		skipTotalVin = true
	}

	txDetails, err := bstore.GetTxWithRange(txid, int(vinOffset), int(vinOffset+limit), int(voutOffset), int(voutOffset+limit))
	if err != nil {

		//Check if it is taal mapi tx
		if taalBitcoinProxyEnabled {

			tx, err := bitcoin.GetRawTransactionFromTaalNode(txid)

			if err == nil {
				// only few mempool txs will end up here
				// we have json but to put in same structure as bstore
				txDetails, err = bstore.ParseRawTx(tx.Hex)
				if err != nil {
					logger.Errorf("Unable to ParseRawTx %v", tx.Hex)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				txDetails.Src = "taalmapi"

				//If it was not mempool tx
				if len(tx.BlockHash) > 0 {
					txDetails.BlockHash = tx.BlockHash
					txDetails.Confirmations = tx.Confirmations
					txDetails.Time = tx.Time
					txDetails.Blocktime = tx.Blocktime
					txDetails.BlockHeight = &tx.BlockHeight
					logger.Errorf("Unable to server a mined Tx from bstore. Investigate! Txid: %s", txid)
				}

			} else {
				logger.Infof("unable to get details from the bstore and the TAAL node: %s , %s", txid, err)
				w.WriteHeader(http.StatusNotFound)
				return
			}

		} else {
			logger.Errorf("unable to get details of tx: %s , %s", txid, err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}

	//check offset
	if vinOffset >= txDetails.VinCount || voutOffset >= txDetails.VoutCount {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if txDetails.VinCount <= GetMaxVinCountForProcessing() {
		skipTotalVin = false
	}

	maxTxHexLength := bstore.GetMaxTxHexLength()

	//workout total input
	var totalInValue float64
	txMapLight := make(map[string]*bsdecoder.RawTransaction)
	txMapFull := make(map[string]*bsdecoder.RawTransaction)

	for i, v := range txDetails.Vin {

		//pagination on vin
		loopIndex := int64(i)
		isInPage := loopIndex >= vinOffset && loopIndex < vinOffset+limit

		if skipTotalVin == true && !isInPage {
			continue
		}

		if len(v.Coinbase) == 0 {

			var outIndex = int(*v.Vout)

			if isInPage && v.ScriptSig != nil && len(v.ScriptSig.Hex) > maxTxHexLength {
				maxTxHexLength = 1000
				txDetails.Vin[i].ScriptSig.Hex = utils.TruncateStrings(v.ScriptSig.Hex, maxTxHexLength)
				txDetails.Vin[i].ScriptSig.ASM = utils.TruncateStrings(v.ScriptSig.ASM, maxTxHexLength)
				txDetails.Vin[i].ScriptSig.IsTruncated = true
			}

			if isInPage {
				// Paginated item — needs full decode for VoutDetails with hex/ASM
				var txd *bsdecoder.RawTransaction
				if cached, ok := txMapFull[v.Txid]; ok {
					txd = cached
				} else {
					txd, err = bstore.GetTx(v.Txid)
					if err != nil {
						if taalBitcoinProxyEnabled && txDetails.Src == "taalmapi" {
							tx, err := bitcoin.GetRawTransactionFromTaalNode(v.Txid)
							if err == nil {
								txd, err = bstore.ParseRawTx(tx.Hex)
								if err != nil {
									logger.Errorf("Unable to ParseRawTx %v", tx.Hex)
									w.WriteHeader(http.StatusInternalServerError)
									return
								}
							} else {
								logger.Warnf("unable to get details from the bstore and the TAAL node: %s , %s", txid, err)
								w.WriteHeader(http.StatusNotFound)
								return
							}
						} else {
							logger.Errorf("unable to get details of tx from bstore: %s , %s, while processing input for %s", v.Txid, err, txid)
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
					}
				}

				if txd != nil {
					txMapFull[v.Txid] = txd
					totalInValue += txd.Vout[outIndex].Value
					txDetails.Vin[i].VoutDetails = &txd.Vout[outIndex]

					if len(txd.Vout[outIndex].ScriptPubKey.Hex) > maxTxHexLength {
						maxTxHexLength = 1000
						txDetails.Vin[i].VoutDetails.ScriptPubKey.Hex = utils.TruncateStrings(txd.Vout[outIndex].ScriptPubKey.Hex, maxTxHexLength)
						txDetails.Vin[i].VoutDetails.ScriptPubKey.ASM = utils.TruncateStrings(txd.Vout[outIndex].ScriptPubKey.ASM, maxTxHexLength)
						txDetails.Vin[i].VoutDetails.ScriptPubKey.IsTruncated = true
					}
				}
			} else {
				// Not in page — only need value, use light decode
				var txd *bsdecoder.RawTransaction
				if cached, ok := txMapLight[v.Txid]; ok {
					txd = cached
				} else if cached, ok := txMapFull[v.Txid]; ok {
					txd = cached
				} else {
					txd, err = bstore.GetTxLight(v.Txid)
					if err != nil {
						if taalBitcoinProxyEnabled && txDetails.Src == "taalmapi" {
							tx, err := bitcoin.GetRawTransactionFromTaalNode(v.Txid)
							if err == nil {
								txd, err = bstore.ParseRawTx(tx.Hex)
								if err != nil {
									logger.Errorf("Unable to ParseRawTx %v", tx.Hex)
									w.WriteHeader(http.StatusInternalServerError)
									return
								}
							} else {
								logger.Warnf("unable to get details from the bstore and the TAAL node: %s , %s", txid, err)
								w.WriteHeader(http.StatusNotFound)
								return
							}
						} else {
							logger.Errorf("unable to get details of tx from bstore: %s , %s, while processing input for %s", v.Txid, err, txid)
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
					}
				}

				if txd != nil {
					txMapLight[v.Txid] = txd
					totalInValue += txd.Vout[outIndex].Value
				}
			}

		} else if len(v.Coinbase) > 0 {
			var height = uint64(*txDetails.BlockHeight)
			totalInValue += utils.CalculateReward(height)

			address := ""
			if len(txDetails.Vout) > 0 && txDetails.Vout[0].ScriptPubKey.Addresses != nil && len(txDetails.Vout[0].ScriptPubKey.Addresses) > 0 {
				address = txDetails.Vout[0].ScriptPubKey.Addresses[0]
			}
			minerDetails, err := pools.GetMinerTag(v.Coinbase, address)
			if err == nil {

				txDetails.Vin[i].MinerInfo = &bsdecoder.MinerDetails{}
				txDetails.Vin[i].MinerInfo.Name = minerDetails.Name
				txDetails.Vin[i].MinerInfo.Type = minerDetails.Type
				txDetails.Vin[i].MinerInfo.Link = minerDetails.Link
			}

		}
	}

	if skipTotalVin == false {
		txDetails.VinValue = &totalInValue
	}

	//slice vin and vout array based on pagination parameters
	txDetails.Vout = txDetails.Vout[voutOffset:utils.Min(txDetails.VoutCount, voutOffset+limit)]
	txDetails.Vin = txDetails.Vin[vinOffset:utils.Min(txDetails.VinCount, vinOffset+limit)]

	//add Op_Return tags and spent outs
	for i, vout := range txDetails.Vout {

		//add spent details
		spent := false
		if utxostore.IsEnabled() {
			resSpentIn, err := utxostore.GetConfirmedSpentInByTxIdOut(txid, uint32(vout.N))
			if err == nil {
				txDetails.Vout[i].Spent = &bsdecoder.Spent{Txid: resSpentIn.TxId, N: resSpentIn.Vin}
				spent = true
			}
		}

		if !spent && utxosmempool.IsEnabled() {
			resSpentIn, err := utxosmempool.GetMempoolSpentInByTxIdOut(txid, uint32(vout.N))
			if err == nil {
				txDetails.Vout[i].Spent = &bsdecoder.Spent{Txid: resSpentIn.TxId, N: resSpentIn.Vin}
			}
		}

		//add tags
		isTagged := false
		// if elasticSearchEnabled {
		// 	if strings.HasPrefix(vout.ScriptPubKey.ASM, "OP_FALSE") || strings.HasPrefix(vout.ScriptPubKey.ASM, "0 ") || strings.HasPrefix(vout.ScriptPubKey.ASM, "OP_RETURN") {
		// 		tag, err := search.GetTagByTxIdVoutIndex(txid, vout.N)
		// 		if err == nil && tag != nil && len(tag.Type) > 0 {
		// 			txDetails.Vout[i].ScriptPubKey.OpReturn = tag
		// 			isTagged = true
		// 		}
		// 	}
		// }

		if !isTagged {
			//tag nonstandard
			if vout.ScriptPubKey.Type == "nonstandard" {
				tag, err := bstore.GetNonStandardTag(vout.ScriptPubKey.Hex)
				if err == nil && tag != nil && len(tag.Type) > 0 {
					txDetails.Vout[i].ScriptPubKey.Tag = tag
				}
			} else {

				//dont't need full asm string
				asm := ""
				if len(vout.ScriptPubKey.ASM) > 50 {
					asm = vout.ScriptPubKey.ASM[0:50]
					vout.ScriptPubKey.ASM = ""
				} else {
					asm = vout.ScriptPubKey.ASM
				}

				//tag opreturn
				tag, err := bstore.GetOpReturnTag(asm, vout.ScriptPubKey.Hex)
				if err == nil && tag != nil && len(tag.Type) > 0 {
					txDetails.Vout[i].ScriptPubKey.OpReturn = tag
				}

			}
		}

		if internal.BadTxids.Has(txid) {
			txDetails.Vout[i].ScriptPubKey.ASM = "removed"
			txDetails.Vout[i].ScriptPubKey.Hex = "removed"
			continue
		}

		//truncate
		maxTxHexLength := bstore.GetMaxTxHexLength()
		if len(vout.ScriptPubKey.Hex) > maxTxHexLength {
			txDetails.Vout[i].ScriptPubKey.Hex = utils.TruncateStrings(vout.ScriptPubKey.Hex, maxTxHexLength)
			txDetails.Vout[i].ScriptPubKey.ASM = utils.TruncateStrings(vout.ScriptPubKey.ASM, maxTxHexLength)
			txDetails.Vout[i].ScriptPubKey.IsTruncated = true
		}
	}

	json.NewEncoder(w).Encode(txDetails)

}

// SendTxBody comment
type SendTxBody struct {
	TxHex string `json:"txhex"`
}

func DecodeRawTx(w http.ResponseWriter, r *http.Request) {

	var hexBody SendTxBody
	b, _ := io.ReadAll(r.Body)
	json.Unmarshal(b, &hexBody)
	var err error

	if hexBody.TxHex == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var raw = hexBody.TxHex

	rawHex, err := hex.DecodeString(raw)
	if err != nil {
		logger.Warnf("Invalid tx rejected: %s ", raw)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//few checks for valid tx
	reader := bytes.NewReader(rawHex)

	//version
	version := make([]byte, 4)
	if n, err := io.ReadFull(reader, version); n != 4 || err != nil {
		logger.Warnf("Invalid tx rejected: %s ", raw)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//should be 1 but making it future proof
	// if binary.LittleEndian.Uint32(version) > 10 {
	// 	w.WriteHeader(http.StatusBadRequest)
	// 	return
	// }

	inputCount, _, err := bt.DecodeVarIntFromReader(reader)
	if err != nil {
		logger.Warnf("Invalid tx rejected: %s ", raw)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//minimum 36 - 32 bytes previous txid and 4 bytes index
	if len(rawHex) < (int(inputCount) * 36) {
		logger.Warnf("Invalid tx rejected: %s ", raw)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	txDetails, err := bstore.ParseRawTx(raw)
	if err != nil {
		logger.Errorf("unable to get details raw tx: %s , %s", raw, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//check if bstore is aware of tx
	txDetailsFromBstore, err := bstore.GetTx(txDetails.Hash)
	if err == nil && txDetailsFromBstore.Hash == txDetails.Hash {
		//if yes take details from bstore as it is aware of block/mempool status
		txDetails = txDetailsFromBstore
	} else {
		txDetails.IsUnknown = true
	}

	containsUnknownUTXO := false

	//workout total input
	var totalInValue float64
	for i, v := range txDetails.Vin {

		if len(v.Coinbase) == 0 && v.VoutDetails == nil {
			txd, err := bstore.GetTx(v.Txid)
			if err != nil {
				containsUnknownUTXO = true
				break
			}

			var outIndex = int(*v.Vout)

			if outIndex >= len(txd.Vout) {
				containsUnknownUTXO = true
				break
			}
			totalInValue += txd.Vout[outIndex].Value
			txDetails.Vin[i].VoutDetails = &txd.Vout[outIndex]

		} else if len(v.Coinbase) > 0 && txDetails.BlockHeight != nil {
			var height = uint64(*txDetails.BlockHeight)
			totalInValue += utils.CalculateReward(height)

			address := ""
			if len(txDetails.Vout) > 0 && txDetails.Vout[0].ScriptPubKey.Addresses != nil && len(txDetails.Vout[0].ScriptPubKey.Addresses) > 0 {
				address = txDetails.Vout[0].ScriptPubKey.Addresses[0]
			}
			minerDetails, err := pools.GetMinerTag(v.Coinbase, address)
			if err == nil {

				txDetails.Vin[i].MinerInfo = &bsdecoder.MinerDetails{}
				txDetails.Vin[i].MinerInfo.Name = minerDetails.Name
				txDetails.Vin[i].MinerInfo.Type = minerDetails.Type
				txDetails.Vin[i].MinerInfo.Link = minerDetails.Link
			}

		}
	}

	if !containsUnknownUTXO {
		txDetails.VinValue = &totalInValue
	}

	//add Op_Return tags
	for i, vout := range txDetails.Vout {

		if vout.ScriptPubKey.Type == "nonstandard" {
			tag, err := bstore.GetNonStandardTag(vout.ScriptPubKey.Hex)
			if err == nil && tag != nil && len(tag.Type) > 0 {
				txDetails.Vout[i].ScriptPubKey.Tag = tag
			}
		} else {
			//getTag
			//dont't need full asm string
			asm := ""
			if len(vout.ScriptPubKey.ASM) > 50 {
				asm = vout.ScriptPubKey.ASM[0:50]
				vout.ScriptPubKey.ASM = ""
			} else {
				asm = vout.ScriptPubKey.ASM
			}

			tag, err := bstore.GetOpReturnTag(asm, vout.ScriptPubKey.Hex)
			if err == nil && tag != nil && len(tag.Type) > 0 {
				txDetails.Vout[i].ScriptPubKey.OpReturn = tag
			}
		}

		if internal.BadTxids.Has(txDetails.Hash) {
			txDetails.Vout[i].ScriptPubKey.ASM = "removed"
			txDetails.Vout[i].ScriptPubKey.Hex = "removed"
		}

		//truncate
		maxTxHexLength := bstore.GetMaxTxHexLength()
		if len(vout.ScriptPubKey.Hex) > maxTxHexLength {
			txDetails.Vout[i].ScriptPubKey.Hex = utils.TruncateStrings(vout.ScriptPubKey.Hex, maxTxHexLength)
			txDetails.Vout[i].ScriptPubKey.ASM = utils.TruncateStrings(vout.ScriptPubKey.ASM, maxTxHexLength)
			txDetails.Vout[i].ScriptPubKey.IsTruncated = true
		}
	}

	json.NewEncoder(w).Encode(txDetails)

}

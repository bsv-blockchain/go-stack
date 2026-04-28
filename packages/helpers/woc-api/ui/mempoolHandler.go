package ui

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/teranode-group/woc-api/bstore"
)

type MempoolDetail struct {
	Fee         float64 `json:"fee"`
	Height      int     `json:"height"`
	ModifiedFee float64 `json:"modifiedfee"`
	Size        int64   `json:"size"`
	Time        float64 `json:"time"`
}

type MempoolStatsSummary struct {
	MaxFee                  int                      `json:"maxFee,omitempty"`
	MaxFeePerByte           int                      `json:"maxFeePerByte,omitempty"`
	MaxAge                  int                      `json:"maxAge,omitempty"`
	MaxSize                 int                      `json:"maxSize,omitempty"`
	Ages                    []int                    `json:"ages,omitempty"`
	Sizes                   []int                    `json:"sizes,omitempty"`
	FeeStats                []MempoolFeeStats        `json:"feeStats"`
	TransactionsBySizeStats []TransactionBySizeStats `json:"transactionsBySizeStats"`
	TransactionsByAgeStats  []TransactionByAgeStats  `json:"transactionsByAgeStats"`
	ProcessMempoolData      bool                     `json:"processMempoolData"`
	TotalFees               float64                  `json:"totalFees"`
	TotalBytes              int                      `json:"totalBytes"`
	TotalCount              int                      `json:"totalCount"`
	AverageFee              float64                  `json:"averageFee"`
	AverageFeePerByte       float64                  `json:"averageFeePerByte"`
	MemoryUsage             int64                    `json:"memoryUsage"`
}

type UnconfirmedTxs struct {
	Count  int          `json:"txcount"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
	TxIds  []TxWithMeta `json:"tx,omitempty"`
}
type MempoolFeeStats struct {
	Label             string  `json:"name"`
	Count             int64   `json:"count"`
	TotalFees         float64 `json:"totalFees"`
	TotalBytes        int64   `json:"totalBytes"`
	AverageFee        float64 `json:"averageFee"`
	AverageFeePerByte float64 `json:"averageFeePerByte"`
}

type TransactionBySizeStats struct {
	Label string `json:"name"`
	Count int64  `json:"count"`
}

type TransactionByAgeStats struct {
	Label string `json:"name"`
	Count int64  `json:"count"`
}

type TxWithMeta struct {
	Txid string        `json:"txid"`
	Tags []bstore.Tags `json:"tags,omitempty"`
}

// GetMempoolStats returns homepage data
func GetMempoolStats(w http.ResponseWriter, r *http.Request) {
	info, err := bitcoinClient.GetMempoolInfo()
	if err != nil {
		logger.Errorf("GetMempoolInfo %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var ms = MempoolStatsSummary{}
	ms.ProcessMempoolData = true
	ms.TotalCount = info.Size
	ms.TotalBytes = info.Bytes
	ms.MemoryUsage = int64(info.Usage)

	// No mempoolstats for 300MB+ mempool or 30000 tx
	if info.Usage >= 300000001 {
		ms.ProcessMempoolData = false
		err = json.NewEncoder(w).Encode(ms)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			logger.Errorf("encoding mempoolSummary %+v\n", err)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	raw, err := bitcoinClient.GetRawMempool(true)
	if err != nil {
		logger.Errorf("getRawMempool %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var mempoolDetails map[string]MempoolDetail
	err = json.Unmarshal(raw, &mempoolDetails)
	if err != nil {
		logger.Errorf("unmarshalling mempoolDetails %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var maxAge int64
	var maxFee float64
	maxFee = 0
	var maxSize = 2000
	var totalFees float64
	var count int

	for _, txMempoolInfo := range mempoolDetails {
		now := time.Now() // current local time
		sec := now.Unix()
		age := sec - int64(txMempoolInfo.Time)
		fee := txMempoolInfo.ModifiedFee

		if age > maxAge {
			maxAge = age
		}
		if fee > maxFee {
			maxFee = fee
		}
	}

	satoshiPerByteBucketMaxima := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "15", "20", "30", "others"}
	bucketCount := len(satoshiPerByteBucketMaxima)

	satoshiPerByteBuckets := make([]MempoolFeeStats, bucketCount)

	satoshiPerByteBuckets[0].Label = ("[0 - " + satoshiPerByteBucketMaxima[0] + "]")
	for i, value := range satoshiPerByteBucketMaxima {
		if value == "unknown" {
			continue
		}

		if i > 0 && i < bucketCount-1 {
			satoshiPerByteBuckets[i].Label = ("[" + satoshiPerByteBucketMaxima[i-1] + " - " + satoshiPerByteBucketMaxima[i] + "]")
		}
	}
	satoshiPerByteBuckets[bucketCount-1].Label = satoshiPerByteBucketMaxima[bucketCount-1]

	var ageBucketCount = 100
	transactionsByAgeStats := make([]TransactionByAgeStats, ageBucketCount)

	for i := 0; i < ageBucketCount; i++ {
		transactionsByAgeStats[i].Label = fmt.Sprintf("%d%s%d", int64(i)*maxAge/int64(ageBucketCount), "-", int64(i)+int64(1)*maxAge/int64(ageBucketCount))
	}

	var sizeBucketCount = 100
	transactionsBySizeStats := make([]TransactionBySizeStats, sizeBucketCount)

	for i := 0; i < sizeBucketCount; i++ {
		if i == (sizeBucketCount - 1) {
			transactionsBySizeStats[i].Label = fmt.Sprintf("%d%s", int64(i)*int64(maxSize)/int64(sizeBucketCount), "+")
		} else {
			transactionsBySizeStats[i].Label = fmt.Sprintf("%d%s%d", int64(i)*int64(maxSize)/int64(sizeBucketCount), " - ", int64(i)+int64(1)*int64(maxSize)/int64(sizeBucketCount))
		}
	}

	for _, v := range mempoolDetails {
		fee := v.ModifiedFee
		feePerByte := float64(v.ModifiedFee) / float64(v.Size)
		satoshiPerByte := feePerByte * 100000000
		now := time.Now() // current local time
		sec := now.Unix()
		age := sec - int64(v.Time)

		isNeg := math.Signbit(float64(age))

		if isNeg {
			age = 0
		}

		size := v.Size

		var ageBucketIndex = Min(int64(ageBucketCount)-1, int64(math.Round((float64(age))/(float64(maxAge)/float64(ageBucketCount)))))

		transactionsByAgeStats[ageBucketIndex].Count++

		var sizeBucketIndex = Min(int64(sizeBucketCount)-1, int64(math.Round(float64(size)/(float64(maxSize)/float64(sizeBucketCount)))))
		transactionsBySizeStats[sizeBucketIndex].Count++

		totalFees += fee
		count++

		addedToBucket := false
		for i, value := range satoshiPerByteBucketMaxima {
			if value == "unknown" {
				continue
			}

			spb, _ := strconv.Atoi(satoshiPerByteBucketMaxima[i])

			// logger.Infof("-- Before %+v,%+v,%+v,%+v,%+v", i, float32(spb), float32(feePerByte), txid, addedToBucket)
			result := big.NewFloat(float64(satoshiPerByte)).Cmp(big.NewFloat(float64(spb)))
			if result < 1 {
				satoshiPerByteBuckets[i].Count++
				satoshiPerByteBuckets[i].TotalFees += fee
				satoshiPerByteBuckets[i].TotalBytes += int64(size)
				satoshiPerByteBuckets[i].AverageFee = satoshiPerByteBuckets[i].TotalFees / float64(satoshiPerByteBuckets[i].Count)
				satoshiPerByteBuckets[i].AverageFeePerByte = satoshiPerByteBuckets[i].TotalFees / float64(satoshiPerByteBuckets[i].TotalBytes)

				addedToBucket = true
				// logger.Infof("-- Added %+v,%+v,%+v,%+v,%+v", i, float32(spb), float64(feePerByte), txid, addedToBucket)
				break
			}

		}
		if !addedToBucket {
			satoshiPerByteBuckets[bucketCount-1].Count++
			satoshiPerByteBuckets[bucketCount-1].TotalFees += fee
			satoshiPerByteBuckets[bucketCount-1].TotalBytes += int64(size)
			satoshiPerByteBuckets[bucketCount-1].AverageFee = satoshiPerByteBuckets[bucketCount-1].TotalFees / float64(satoshiPerByteBuckets[bucketCount-1].Count)
			satoshiPerByteBuckets[bucketCount-1].AverageFeePerByte = satoshiPerByteBuckets[bucketCount-1].TotalFees / float64(satoshiPerByteBuckets[bucketCount-1].TotalBytes)
		}
	}

	ms.FeeStats = satoshiPerByteBuckets
	ms.MaxFee = int(maxFee)
	ms.MaxAge = int(maxAge)
	ms.MaxFeePerByte = maxSize
	ms.TransactionsByAgeStats = transactionsByAgeStats
	ms.TransactionsBySizeStats = transactionsBySizeStats
	ms.AverageFee = totalFees / float64(count)
	ms.AverageFeePerByte = totalFees / float64(ms.TotalBytes)
	ms.TotalFees = totalFees

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ms)
}

func GetMempoolTxs(w http.ResponseWriter, r *http.Request) {

	// limit
	limitStr := strings.ToLower(r.URL.Query().Get("limit"))
	if limitStr == "" {
		limitStr = "10" //TODO: const
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 50 {
		limit = 10 //TODO: const
	}

	// offset
	offsetStr := strings.ToLower(r.URL.Query().Get("offset"))
	if offsetStr == "" {
		offsetStr = "0"
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//skip skipTags if requested
	skipTags := false
	skipTagsStr := strings.ToLower(r.URL.Query().Get("skipTags"))

	if skipTagsStr == "true" {
		skipTags = true
	}

	resp := &UnconfirmedTxs{}

	raw, err := bitcoinClient.GetRawMempool(false)
	if err != nil {
		logger.Errorf("getRawMempool %+v\n", err)
		return
	}

	var txIds []string
	json.Unmarshal([]byte(raw), &txIds)

	resp.Count = len(txIds)
	resp.Limit = limit

	if offset <= resp.Count {
		txIds = txIds[offset:MinInt(resp.Count, offset+limit)]
		resp.Offset = offset
	} else {
		txIds = txIds[0:MinInt(resp.Count, limit)]
		resp.Offset = 0
	}

	txIdsWithMeta := make([]TxWithMeta, len(txIds))

	for i, id := range txIds {
		txIdsWithMeta[i].Txid = id
		var tagsArray []bstore.Tags

		if !skipTags {
			txDetails, err := bstore.GetTx(id)
			if err != nil {
				logger.Errorf("unable to get details of mempool tx: %s , %s", id, err)
				continue
			}

			for _, v := range txDetails.Vout {

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
			if len(tagsArray) > 0 {
				txIdsWithMeta[i].Tags = tagsArray
			}
		}
	}
	resp.TxIds = txIdsWithMeta
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)

}

func MinInt(value_0, value_1 int) int {
	if value_0 < value_1 {
		return value_0
	}
	return value_1
}

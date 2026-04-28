package ui

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/teranode-group/woc-api/redis"
	"github.com/teranode-group/woc-api/search"
)

const (
	KEY_GetTagsPerWeek        = "GetTagsPerWeek"
	KEY_GetTagsFromDay        = "GetTagsByDays/1"
	KEY_GetTagsFromSevenDays  = "GetTagsByDays/7"
	KEY_GetTagsFromThirtyDays = "GetTagsByDays/30"
	KEY_GetTagsFromNinetyDays = "GetTagsByDays/90"

	KEY_GetTagsSummaryFromDay        = "GetTagsSummaryByDays/1"
	KEY_GetTagsSummaryFromSevenDays  = "GetTagsSummaryByDays/7"
	KEY_GetTagsSummaryFromThirtyDays = "GetTagsSummaryByDays/30"
	KEY_GetTagsSummaryFromNinetyDays = "GetTagsSummaryByDays/90"
)

func GetTagCounts(w http.ResponseWriter, r *http.Request) {
	if !elasticSearchEnabled {
		// op return search not enabled. return
		logger.Errorf("opreturn search disabled in config")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	vars := mux.Vars(r)
	fromDate := vars["fromDate"]
	toDate := vars["toDate"]

	res, err := search.GetTagCount(fromDate, toDate)
	if err != nil {
		logger.Errorf("FindTagCount - %+v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

func GetTagCountsByBlockHeight(w http.ResponseWriter, r *http.Request) {
	if !elasticSearchEnabled {
		// op return search not enabled. return
		logger.Errorf("opreturn search disabled in config")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	vars := mux.Vars(r)
	height := vars["height"]

	res, err := search.GetTagCountByBlockHeight(height)
	if err != nil {
		logger.Errorf("GetTagCountsByBlockHeigh - %+v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

func GetTagsByDays(w http.ResponseWriter, r *http.Request) {
	if !elasticSearchEnabled {
		// op return search not enabled. return
		logger.Errorf("opreturn search disabled in config")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	vars := mux.Vars(r)
	days := vars["days"]

	if days == "" {
		logger.Errorf("GetTagsByDays - days is required")
		w.WriteHeader(http.StatusBadRequest)
		return

	}

	//var cachedforWeek map[string]map[string]int
	var cached map[string]map[string]search.TagDetails
	var err error
	// Get from cache
	if redis.RedisClient.Enabled {
		switch days {
		case "1":
			err = redis.GetCachedValue(KEY_GetTagsFromDay, &cached, nil)
		case "7":
			err = redis.GetCachedValue(KEY_GetTagsFromSevenDays, &cached, nil)
		case "30":
			err = redis.GetCachedValue(KEY_GetTagsFromThirtyDays, &cached, nil)
		case "90":
			err = redis.GetCachedValue(KEY_GetTagsFromNinetyDays, &cached, nil)
		}
	}

	// Get from indexing service
	if !redis.RedisClient.Enabled || err != nil || cached == nil {
		cached, err = search.GetTagsByDays(days)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(cached)
}

func GetTagsSummaryByDays(w http.ResponseWriter, r *http.Request) {
	if !elasticSearchEnabled {
		// op return search not enabled. return
		logger.Errorf("opreturn search disabled in config")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	vars := mux.Vars(r)
	days := vars["days"]

	if days == "" {
		logger.Errorf("GetTagsSummaryByDays - days is required")
		w.WriteHeader(http.StatusBadRequest)
		return

	}

	var cached map[string]search.TagDetails
	var err error
	// Get from cache
	if redis.RedisClient.Enabled {
		switch days {
		case "1":
			err = redis.GetCachedValue(KEY_GetTagsSummaryFromDay, &cached, nil)
		case "7":
			err = redis.GetCachedValue(KEY_GetTagsSummaryFromSevenDays, &cached, nil)
		case "30":
			err = redis.GetCachedValue(KEY_GetTagsSummaryFromThirtyDays, &cached, nil)
		case "90":
			err = redis.GetCachedValue(KEY_GetTagsSummaryFromNinetyDays, &cached, nil)
		}
	}

	// Get from indexing service
	if !redis.RedisClient.Enabled || err != nil || cached == nil {
		cached, err = search.GetTagsSummaryByDays(days)
		if err != nil {
			logger.Errorf("GetTagsBlockSummaryByDays - %+v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(cached)
}

func GetTagsPerWeek(w http.ResponseWriter, r *http.Request) {
	if !elasticSearchEnabled {
		// op return search not enabled. return
		logger.Errorf("opreturn search disabled in config")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var cached map[string]map[string]int
	var err error
	// Get from cache
	if redis.RedisClient.Enabled {
		err = redis.GetCachedValue(KEY_GetTagsPerWeek, &cached, nil)
	}

	// Get from indexing service
	if !redis.RedisClient.Enabled || err != nil || cached == nil {
		cached, err = search.GetTagsPerWeek()
		if err != nil {
			logger.Errorf("GetTagsPerWeek - %+v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(cached)
}

func SearchByTag(w http.ResponseWriter, r *http.Request) {

	query := r.URL.Query().Get("name")

	if query == "" {
		logger.Errorf("Name required")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// limit
	limitStr := strings.ToLower(r.URL.Query().Get("limit"))
	if limitStr == "" {
		limitStr = "10"
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	//offset
	offsetStr := strings.ToLower(r.URL.Query().Get("offset"))
	if offsetStr == "" {
		offsetStr = "0"
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		logger.Errorf("invalid offset")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	query = strings.TrimSpace(query)

	searchResult := SearchResult{}

	if !elasticSearchEnabled {
		// op return search not enabled. return
		logger.Errorf("opreturn search disabled in config")
		searchResult.Count = 0
		searchResult.Type = "not found"
		json.NewEncoder(w).Encode(searchResult)
		return
	}

	searchResult.OpReturns, err = search.SearchByFullTag(query, offset, limit)
	if err != nil {
		searchResult.Count = 0
		searchResult.Type = "not found"
		logger.Errorf("error searching for tag %+v - %+v\n", query, err)
		json.NewEncoder(w).Encode(searchResult)
		return
	} else {
		searchResult.Count = int(searchResult.OpReturns.Count)
		searchResult.Type = "opReturn"
		json.NewEncoder(w).Encode(searchResult)
		return
	}

}

func SearchLatestDetailsByTag(w http.ResponseWriter, r *http.Request) {

	query := r.URL.Query().Get("name")

	if query == "" {
		logger.Errorf("Name required")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var err error

	query = strings.TrimSpace(query)

	searchResult := SearchResult{}
	var cache search.Result

	if !elasticSearchEnabled {
		// op return search not enabled. return
		logger.Errorf("opreturn search disabled in config")
		searchResult.Count = 0
		searchResult.Type = "not found"
		json.NewEncoder(w).Encode(searchResult)
		return
	}

	var resultErr error
	// Get from cache
	if redis.RedisClient.Enabled {
		resultErr = redis.GetCachedValue(query, &cache, nil)
	}

	// Get from indexing service
	if !redis.RedisClient.Enabled || resultErr != nil {
		//Will fetch the latest 1000
		println("getting non cache")
		cache, err = search.SearchByFullTag(query, 0, 1000)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	if err != nil {
		searchResult.Count = 0
		searchResult.Type = "not found"
		logger.Errorf("error searching for tag %+v - %+v\n", query, err)
		json.NewEncoder(w).Encode(searchResult)
		return
	} else {
		searchResult.OpReturns = cache
		searchResult.Count = int(searchResult.OpReturns.Count)
		searchResult.Type = "opReturn"
		json.NewEncoder(w).Encode(searchResult)
		return
	}
}

func StartTagsStatsCache() {
	if !redis.RedisClient.Enabled {
		return
	}
	logger.Info("Starting Periodic Node Stats Caching")

	cacheData := func(cacheKey string, fetchFunc func() (interface{}, error), conn redis.Conn) {
		data, err := fetchFunc()
		if err != nil {
			logger.Errorf("Error fetching data for %s: %+v", cacheKey, err)
			return
		}

		if data == nil {
			logger.Warnf("No data to cache for %s", cacheKey)
			return
		}

		err = redis.SetCacheValue(cacheKey, data, conn)
		if err != nil {
			logger.Errorf("Unable to cache data for %s: %+v", cacheKey, err)
		}
	}

	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		conn := redis.RedisClient.ConnPool.Get()

		cacheData(KEY_GetTagsPerWeek, func() (interface{}, error) {
			return search.GetTagsPerWeek()
		}, conn)

		for days, cacheKey := range map[string]string{
			"1":  KEY_GetTagsFromDay,
			"7":  KEY_GetTagsFromSevenDays,
			"30": KEY_GetTagsFromThirtyDays,
			"90": KEY_GetTagsFromNinetyDays,
		} {
			cacheData(cacheKey, func() (interface{}, error) {
				return search.GetTagsByDays(days)
			}, conn)
		}

		for days, cacheKey := range map[string]string{
			"1":  KEY_GetTagsSummaryFromDay,
			"7":  KEY_GetTagsSummaryFromSevenDays,
			"30": KEY_GetTagsSummaryFromThirtyDays,
			"90": KEY_GetTagsSummaryFromNinetyDays,
		} {
			cacheData(cacheKey, func() (interface{}, error) {
				return search.GetTagsSummaryByDays(days)
			}, conn)
		}

		logger.Info("Periodic Node Stats Caching complete")
		conn.Flush()
		conn.Close()
	}
}

func StartTagsDetailsByTagCache() {

	if !redis.RedisClient.Enabled {
		return
	}

	logger.Info("Starting perodic tag details caching")

	lastKnownBlockHash := ""
	isBusy := false
	skipCounter := 0
	maxSkipCounter := 2
	ticker := time.NewTicker(3 * time.Minute) //update every 3 mins
	for ; true; <-ticker.C {
		var isErr bool
		if isBusy && skipCounter <= maxSkipCounter {
			logger.Warnf("Skipping TagsStatsCache update. isBusy:%+v, skipCounter:%+v/%+v\n", isBusy, skipCounter, maxSkipCounter)
			skipCounter++
			continue
		}
		isBusy = true
		skipCounter = 0

		chainInfo, err := bitcoinClient.GetBlockchainInfo()
		if err != nil {
			logger.Errorf("Unable to GetBlockchainInfo %+v\n", err)
			skipCounter++
			continue
		}
		if lastKnownBlockHash != chainInfo.BestBlockHash {
			conn := redis.RedisClient.ConnPool.Get()
			var cached map[string]map[string]search.TagDetails

			err := redis.GetCachedValue(KEY_GetTagsFromNinetyDays, &cached, nil)

			if err != nil || cached == nil {
				cached, err = search.GetTagsByDays("90")
				if err != nil {
					logger.Errorf("Unable to cache GetTagsByDays/90 %+v\n", err)
				}
			} else {
				tagTxCounts := calculateTagTxCounts(cached)
				// Filter out the tags with a total TxCount greater than 10000
				filteredTags := filterTags(tagTxCounts)
				for _, tagName := range filteredTags {
					result, err := search.SearchByFullTag(tagName, 0, 1000)
					if err != nil {
						logger.Errorf("Unable to get SearchByFullTag %+v\n", err)
						isErr = true
						break
					}
					err = redis.SetCacheValue(tagName, result, conn)
					if err != nil {
						logger.Errorf("Unable to cache SearchByFullTag  %+v\n", err)
					}
				}
			}
			lastKnownBlockHash = chainInfo.BestBlockHash
			conn.Flush()
			conn.Close()
			if !isErr {
				isBusy = false
			}
		}
	}
}

func calculateTagTxCounts(result map[string]map[string]search.TagDetails) map[string]int64 {
	tagTxCounts := make(map[string]int64)
	for _, tags := range result {
		for tag, details := range tags {
			tagTxCounts[tag] += details.TotalTxcount
		}
	}
	return tagTxCounts
}

// Filter out the tags with a total TxCount greater than 10000
func filterTags(tagTxCounts map[string]int64) []string {
	var filteredTags []string
	for tag, txCount := range tagTxCounts {
		if txCount > 100000 {
			filteredTags = append(filteredTags, tag)
		}
	}
	return filteredTags
}

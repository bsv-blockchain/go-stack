package pools

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/ordishs/gocore"
	"github.com/patrickmn/go-cache"
	"github.com/teranode-group/woc-api/models"
)

var logger = gocore.Log("woc-api")

var poolsCacheKey = "PoolTags"

// No expiry time
var poolsCache = cache.New(0, 0)

// Load pools data from poolConfigUrl
func CachePoolTags() error {

	poolTags := &models.PoolTags{}

	httpClient := http.Client{
		Timeout: time.Second * 4,
	}

	poolConfigURL, _ := gocore.Config().Get("poolConfigUrl")

	req, err := http.NewRequest(http.MethodGet, poolConfigURL, nil)
	if err != nil {
		logger.Errorf("can't create new http request %+v\n", err)
		return errors.New("can't create new http request")
	}

	res, getErr := httpClient.Do(req)
	if getErr != nil {
		logger.Errorf("can't get request %+v\n", getErr)
		return errors.New("can't get request")
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		logger.Errorf("can't read poolConfig file %+v\n", readErr)
		return errors.New("can't read poolConfig file")
	}
	poolTags.Mu.Lock()
	defer poolTags.Mu.Unlock()
	jsonErr := json.Unmarshal(body, &poolTags)
	if jsonErr != nil {
		logger.Errorf("can't unmarshal poolConfig file %+v\n", jsonErr)
		return errors.New("can't unmarshal poolConfig file")
	}

	poolsCache.Set(poolsCacheKey, poolTags, cache.NoExpiration)

	return nil
}

func GetPoolTags() (*models.PoolTags, error) {

	cachedPoolsInterface, found := poolsCache.Get(poolsCacheKey)

	if found {
		poolTags := cachedPoolsInterface.(*models.PoolTags)
		return poolTags, nil
	}

	err := CachePoolTags()
	if err != nil {
		return nil, errors.New("Unable to cache poolsTag")
	}

	cachedPoolsInterface, found = poolsCache.Get(poolsCacheKey)
	if found {
		poolTags := cachedPoolsInterface.(*models.PoolTags)
		return poolTags, nil
	}

	return nil, errors.New("Unable to read pools Tag from cache")
}

func GetMinerTag(coinbaseHex string, address string) (*models.MinerDetails, error) {

	poolTags, err := GetPoolTags()

	if err != nil {
		return nil, errors.New("Unable to get pool tabs")
	}

	if poolTags != nil {
		// check the payout addresses
		minerInfo, ok := poolTags.PayoutAddresses[address]
		if ok {
			minerInfo.Type = "address"
			return &minerInfo, nil
		}

		// check the coinbase tag
		bCoinbase, err := hex.DecodeString(coinbaseHex)
		if err != nil {
			logger.Errorf("Can't decode coinbase. %+v\n", err)
		}

		for key := range poolTags.CoinbaseTags {
			if strings.Contains(string(bCoinbase), key) {
				minerInfo, ok := poolTags.CoinbaseTags[key]
				if ok {
					minerInfo.Type = "tag"
					return &minerInfo, nil
				}
			}
		}

	}

	return nil, errors.New("No found")
}

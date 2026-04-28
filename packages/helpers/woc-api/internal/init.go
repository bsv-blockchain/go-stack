package internal

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	gobitcoin "github.com/ordishs/go-bitcoin"
	"github.com/teranode-group/common/utils"
	"github.com/teranode-group/woc-api/bitcoin"
	"github.com/teranode-group/woc-api/models"
	"github.com/teranode-group/woc-api/mongocache"

	"github.com/ordishs/gocore"
)

var logger = gocore.Log("woc-api")

// Save block to mongoDB if block size greater than blockCacheSizeMin
const blockCacheSizeMin = 2000000 // 2 MB

// Save block to mongoDB if block tx count greater than blockCacheTxCountMin
const blockCacheTxCountMin = 1000

// Save tx to mongoDB if tx vin or tx vout count is greater than txCacheVoutVinCountMin
const txCacheVoutVinCountMin = 1000

// Save tx to mongoDB if tx size is greater than txCacheSizeMin
const txCacheSizeMin = 80000 //80 KB

var useMongoCache bool
var bitcoinClient *bitcoin.Client
var poolTags *models.PoolTags = &models.PoolTags{}
var cacheWriteTransaction = make(chan *gobitcoin.RawTransaction, 10000)
var BadTxids = utils.NewSet()

func init() {
	useMongoCache = gocore.Config().GetBool("mongoCache", false)

	//Create bitcoin client
	var err error
	bitcoinClient, err = bitcoin.New()
	if err != nil {
		logger.Errorf("Unable get bitcoin client", err)
	}

	addBadTxids()
	go readMinerIDConfig()

}

func addBadTxids() {
	btxListStr, ok := gocore.Config().Get("badTxIds")
	if !ok {
		logger.Warn("No bad tx found in config file")
	}
	btxList := strings.Split(btxListStr, ",")
	for _, t := range btxList {
		logger.Infof("badtx: %s", t)
		BadTxids.Add(t)
	}
}

func readMinerIDConfig() {
	httpClient := http.Client{
		Timeout: time.Second * 4,
	}

	poolConfigURL, _ := gocore.Config().Get("poolConfigUrl")

	req, err := http.NewRequest(http.MethodGet, poolConfigURL, nil)
	if err != nil {
		logger.Errorf("can't create new http request %+v\n", err)
		return
	}

	res, getErr := httpClient.Do(req)
	if getErr != nil {
		logger.Errorf("can't get request %+v\n", getErr)
		return
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		logger.Errorf("can't read poolConfig file %+v\n", readErr)
		return
	}
	poolTags.Mu.Lock()
	defer poolTags.Mu.Unlock()
	jsonErr := json.Unmarshal(body, &poolTags)
	if jsonErr != nil {
		logger.Errorf("can't unmarshal poolConfig file %+v\n", jsonErr)
		return
	}
}

func StartCacheTxWrite() {
	for tx := range cacheWriteTransaction {
		go func(t *gobitcoin.RawTransaction) {
			mongocache.AddTransactionToCache(*t)
		}(tx)
	}
}

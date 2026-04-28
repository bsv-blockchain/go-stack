package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	electrumTypes "github.com/checksum0/go-electrum/electrum"
	"github.com/gorilla/mux"
	gobitcoin "github.com/ordishs/go-bitcoin"
	"github.com/teranode-group/common/parser"
	"github.com/teranode-group/woc-api/apikeys"
	"github.com/teranode-group/woc-api/bitcoin"
	"github.com/teranode-group/woc-api/bstore"
	"github.com/teranode-group/woc-api/electrum"
	"github.com/teranode-group/woc-api/internal"
	"github.com/teranode-group/woc-api/mongocache"
	"github.com/teranode-group/woc-api/redis"
	"github.com/teranode-group/woc-api/search"
	tokens "github.com/teranode-group/woc-api/tokens"
	"github.com/teranode-group/woc-api/ui"
	"github.com/teranode-group/woc-api/utxosmempool"

	"github.com/ordishs/gocore"
	"github.com/rs/cors"
	"github.com/teranode-group/common/utils"
)

type wrappedElectrumClient struct {
	mu             sync.RWMutex
	electrumClient *electrum.Client
}

var (
	isMainnet               bool
	network                 string
	useMongoCache           bool
	taalBitcoinProxyEnabled bool
	offlineMode             bool

	apiKeyCheckEnabled     bool
	apiKeyRateLimitEnabled bool
	apiKeySaveActivity     bool

	endpointWeightEnabled        bool
	endpointWeightMap            map[string]int
	apiKeyDefaultRateLimitPerSec int
	apiKeyDefaultRateLimitPerDay int

	merkleProofServiceEnabled bool
	merkleProofServiceAddress string

	wrappedEleC   *wrappedElectrumClient
	bitcoinClient *bitcoin.Client

	logger = gocore.Log("woc-api")
)

// Start the server
func Start() {

	apiPort, ok := gocore.Config().GetInt("port")
	if !ok {
		logger.Fatal("Error: Must have a port setting")
	}

	apiHost, ok := gocore.Config().Get("host")
	if !ok {
		logger.Fatal("Error: Must have a host setting")
	}

	logger.Infof("Starting server on port %d...", apiPort)

	useMongoCache = gocore.Config().GetBool("mongoCache", false)
	taalBitcoinProxyEnabled = gocore.Config().GetBool("taalBitcoinProxyEnabled", false)
	offlineMode = gocore.Config().GetBool("offlineMode", false)
	apiKeyCheckEnabled = gocore.Config().GetBool("apiKeyCheckEnabled", true)
	apiKeyRateLimitEnabled = gocore.Config().GetBool("apiKeyRateLimitEnabled", true)
	apiKeySaveActivity = gocore.Config().GetBool("apiKeySaveActivity", true)

	// In offline mode, disable activity store (no account-manager connection)
	if offlineMode {
		apiKeySaveActivity = false
		logger.Info("offlineMode enabled: activity store disabled")
	}

	merkleProofServiceEnabled = gocore.Config().GetBool("woc_merkle_service_enabled", true)
	merkleProofServiceAddress, ok = gocore.Config().Get("woc_merkle_service_address")
	if merkleProofServiceEnabled && !ok {
		logger.Fatal("Error: Must have a woc_merkle_service_address setting")
	}

	if apiKeyCheckEnabled {
		if offlineMode {
			apikeys.StartAPIKeysFromCache()
			logger.Info("offlineMode: Using cached API keys only (no account-manager connection)")
		} else {
			apikeys.StartAPIKeysListener()
		}
		logger.Infof("ApiKey Check is set to %v in settings", apiKeyCheckEnabled)
	}

	if apiKeyRateLimitEnabled {

		logger.Infof("ApiKey Rate Limiter is set to %v in settings", apiKeyRateLimitEnabled)

		apiKeyDefaultRateLimitPerSec, _ = gocore.Config().GetInt("apiKeyDefaultRateLimitPerSec", 3)
		apiKeyDefaultRateLimitPerDay, _ = gocore.Config().GetInt("apiKeyDefaultRateLimitPerDay", 100000)

		endpointWeightEnabled = gocore.Config().GetBool("endpointWeightEnabled", true)
		endpointWeightMap = make(map[string]int)

		if endpointWeightEnabled {

			logger.Infof("Endpoints Weight is set to %v in settings", endpointWeightEnabled)

			endpointWeightMapString, ok := gocore.Config().Get("endpointWeightMap")

			if !ok {
				logger.Error("error getting endpointWeightMap")
			}

			for _, path := range strings.Split(endpointWeightMapString, ",") {
				parts := strings.Split(path, ":")
				weight, err := strconv.Atoi(parts[1])
				if err == nil {
					endpointWeightMap[parts[0]] = weight
				}
			}
		}
	}

	if apiKeySaveActivity {
		logger.Infof("ApiKey Save Activity is set to %v in settings", apiKeySaveActivity)
	}

	//Initialize redis cache
	//redis.Start()

	//rate limiter depends on redis
	if apiKeyRateLimitEnabled && !redis.RedisClient.Enabled {
		logger.Fatal("Error: apiKeyRateLimitEnabled is set to true in settings which depends on redis. Redis is Disable!")
	}

	isMainnet = gocore.Config().GetBool("isMainnet", true)
	network, _ = gocore.Config().Get("network")
	logger.Infof("isMainnet is set to %v in settings", isMainnet)

	//electrumX settings
	electrumURL, ok := gocore.Config().Get("electrumUrl")
	if !ok {
		logger.Fatal("Error: Must have an electrumUrl host setting")
	}

	var err error
	wrappedEleC = &wrappedElectrumClient{}
	wrappedEleC.electrumClient, err = electrum.New(electrumURL)
	if err != nil {
		logger.Errorf("Could not start electrum client", err)
	}

	ticker := time.NewTicker(60 * time.Second)
	go func() {
		for range ticker.C {

			// If the electrumClient is not nil, the following || will call PrintOrError
			if wrappedEleC.electrumClient == nil || wrappedEleC.electrumClient.PingOrError() != nil {
				wrappedEleC.mu.Lock()
				wrappedEleC.electrumClient, err = electrum.New(electrumURL)
				wrappedEleC.mu.Unlock()

				if err != nil {
					logger.Errorf("Onping - Could not start electrum client: %v", err)
				}
			}
		}
	}()

	//Create bitcoin client
	bitcoinClient, err = bitcoin.New()
	if err != nil {
		logger.Errorf("Unable to create bitcoin client", err)
	}

	//Start node cache for stats
	go bitcoin.StartNodeStatsCache()

	opReturnSearch := gocore.Config().GetBool("opReturnSearch", false)

	if opReturnSearch {
		go ui.StartTagsStatsCache()
		go ui.StartTagsDetailsByTagCache()
	}

	go utxosmempool.StartMempoolStatsCache()

	serverAndPort := fmt.Sprintf("%s:%d", apiHost, apiPort)
	writeTimeout, _ := gocore.Config().Get("writeTimeout")
	readTimeout, _ := gocore.Config().Get("readTimeout")
	idleTimeout, _ := gocore.Config().Get("idleTimeout")

	wt, _ := time.ParseDuration(writeTimeout)
	rt, _ := time.ParseDuration(readTimeout)
	it, _ := time.ParseDuration(idleTimeout)

	listen(serverAndPort, wt, rt, it)
}

func listen(bindAddr string, wt time.Duration, rt time.Duration, it time.Duration) {
	wait, _ := time.ParseDuration("15s")
	r := mux.NewRouter()

	r.Use(commonMiddleware)
	r.Use(TaalAPIKeyMiddleware)

	//**** API endpoints ***
	// health - only tells this serice is running, not checking dependencies. For clients to check if API is up and running
	r.HandleFunc("/woc", getWoc)
	// health - For internal use alerting and monitoring - check dependencies
	r.HandleFunc("/internal/health", healthCheck)

	r.HandleFunc("/feerecommendation", proxyToServer("http://localhost:8085"))

	// block
	r.HandleFunc("/block/headers", getBlockHeadersList).Methods("GET")
	r.HandleFunc("/block/{hash}", getBlock).Methods("GET")
	r.HandleFunc("/block/{heightOrhash}/header", getBlockHeader).Methods("GET")

	r.HandleFunc("/block/height/{height}", getBlockByHeight).Methods("GET")
	r.HandleFunc("/block/{hash}/tx/{skip}/{limit}", getBlockTxids).Methods("GET")
	r.HandleFunc("/block/{hash}/page/{number}", getBlockPage).Methods("GET")
	r.HandleFunc("/blocks/{height}/{count}/header", bstore.GetBlocksHandler).Methods("GET") //new using bstore
	r.HandleFunc("/blocks/orphans", bstore.GetBlocksAtHeightIncludeOrphans).Methods("GET")

	// tx
	r.HandleFunc("/tx/raw", postSendRawTransaction).Methods("POST")
	r.HandleFunc("/tx/decode", postDecodeRawTransaction).Methods("POST")
	r.HandleFunc("/tx/{txid}", getRawTransaction).Methods("GET")
	r.HandleFunc("/tx/{txid}/opreturn", bstore.GetOpreturnData).Methods("GET")
	r.HandleFunc("/tx/{txid}/ancestors", getAncestorsTransaction).Methods("GET")
	r.HandleFunc("/tx/{txid}/descendants", getDescendantsTransaction).Methods("GET")
	r.HandleFunc("/tx/{txid}/hex", getRawTransactionHex).Methods("GET")
	r.HandleFunc("/tx/{txid}/bin", getRawTransactionBin).Methods("GET")

	r.HandleFunc("/mapi/tx/{txid}", getMapiRawTransaction).Methods("GET")
	r.HandleFunc("/mapi/tx/{txid}/hex", getMapiRawTransactionHex).Methods("GET")
	r.HandleFunc("/mapi/tx/{txid}/ancestors", getMapiAncestorsTransaction).Methods("GET")
	r.HandleFunc("/mapi/tx/{txid}/descendants", getMapiDescendantsTransaction).Methods("GET")

	r.HandleFunc("/tx/{txid}/out/{index}/hex", getRawTransactionOutputHex).Methods("GET")
	r.HandleFunc("/txout/{txid}/{vout}", getTxOut).Methods("GET")
	r.HandleFunc("/txout/{txid}/{vout}/{includeMempool}", getTxOut).Methods("GET")
	r.HandleFunc("/tx/{txid}/merkleproof", getMerkleProof).Methods("GET")
	r.HandleFunc("/tx/{txid}/proof/tsc", getMerkleProofWithNodeAsBackup).Methods("GET")
	r.HandleFunc("/tx/{txid}/{height}/merkleproof", getMerkleProof).Methods("GET")

	r.HandleFunc("/txs", postRawTransactionsWithLimit).Methods("POST")
	r.HandleFunc("/txs/hex", bstore.BulkTxHexHandler).Methods("POST")
	r.HandleFunc("/txs/status", bstore.BulkTxStatusHandler).Methods("POST")
	r.HandleFunc("/txs/vouts/hex", bstore.BulkTxVoutHex).Methods("POST")

	// chain
	r.HandleFunc("/blockchain/info", getBlockchainInfo).Methods("GET")
	r.HandleFunc("/circulatingsupply", getCirculatingSupply).Methods("GET")

	r.HandleFunc("/chain/info", getBlockchainInfo).Methods("GET")
	r.HandleFunc("/chain/stats", ui.GetHomepage).Methods("GET") //handler is also used by /ui/homepage (for now response is same)
	r.HandleFunc("/chain/tips", getBlockchainTips).Methods("GET")
	r.HandleFunc("/chain/tips/headers", getBlockchainTipsWithDetails).Methods("GET")
	r.HandleFunc("/tx/hash/{txid}", getRawTransaction).Methods("GET")
	r.HandleFunc("/tx/{txid}", getRawTransaction).Methods("GET")
	r.HandleFunc("/block/hash/{hash}", getBlock).Methods("GET")
	r.HandleFunc("/block/hash/{hash}/tx/{skip}/{limit}", getBlockTxids).Methods("GET")
	r.HandleFunc("/block/hash/{hash}/page/{number}", getBlockPage).Methods("GET")

	// mempool
	r.HandleFunc("/mempool/info", getMempoolInfo).Methods("GET")
	r.HandleFunc("/mempool/raw/{details}", getRawMempool).Methods("GET")
	r.HandleFunc("/mempool/raw", getRawMempool).Methods("GET")
	r.HandleFunc("/nonfinalmempool/raw", getNonFinalMempoolTxList).Methods("GET")

	// address or scripthash - data source utxo-store
	r.HandleFunc("/address/{addressOrScripthash}/confirmed/history", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/script/{addressOrScripthash}/confirmed/history", proxyToServer("http://localhost:8085"))

	r.HandleFunc("/address/{addressOrScripthash}/confirmed/unspent", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/script/{addressOrScripthash}/confirmed/unspent", proxyToServer("http://localhost:8085"))

	r.HandleFunc("/address/{addressOrScripthash}/confirmed/balance", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/script/{addressOrScripthash}/confirmed/balance", proxyToServer("http://localhost:8085"))

	r.HandleFunc("/tx/{txid}/{vout}/confirmed/spent", proxyToServer("http://localhost:8085"))

	// bulk
	r.HandleFunc("/addresses/confirmed/unspent", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/addresses/confirmed/history", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/addresses/confirmed/balance", proxyToServer("http://localhost:8085"))

	r.HandleFunc("/scripts/confirmed/unspent", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/scripts/confirmed/history", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/scripts/confirmed/balance", proxyToServer("http://localhost:8085"))

	r.HandleFunc("/addresses/unconfirmed/unspent", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/addresses/unconfirmed/history", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/addresses/unconfirmed/balance", proxyToServer("http://localhost:8085"))

	r.HandleFunc("/scripts/unconfirmed/unspent", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/scripts/unconfirmed/history", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/scripts/unconfirmed/balance", proxyToServer("http://localhost:8085"))

	r.HandleFunc("/addresses/history/all", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/utxos/spent", proxyToServer("http://localhost:8085"))

	// address or scripthash - data source utxos-mempool
	r.HandleFunc("/address/{addressOrScripthash}/unconfirmed/history", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/script/{addressOrScripthash}/unconfirmed/history", proxyToServer("http://localhost:8085"))

	r.HandleFunc("/address/{addressOrScripthash}/unconfirmed/unspent", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/script/{addressOrScripthash}/unconfirmed/unspent", proxyToServer("http://localhost:8085"))

	r.HandleFunc("/address/{addressOrScripthash}/unconfirmed/balance", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/script/{addressOrScripthash}/unconfirmed/balance", proxyToServer("http://localhost:8085"))

	r.HandleFunc("/tx/{txid}/{vout}/unconfirmed/spent", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/tx/{txid}/beef", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/tx/{txid}/proof/bump", proxyToServer("http://localhost:8085"))

	// combined (utxo-store and utxos-mempool)
	r.HandleFunc("/tx/{txid}/{vout}/spent", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/address/{addressOrScripthash}/unspent/all", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/script/{addressOrScripthash}/unspent/all", proxyToServer("http://localhost:8085"))

	// address

	r.HandleFunc("/address/{address}/info", validateAddress).Methods("GET")
	r.HandleFunc("/address/hash/{scriptHash}/balance", legacyProxy(scriptHashPath("/address/{sh}/confirmed/balance"), legacyBalance)).Methods("GET")
	r.HandleFunc("/address/hash/{scriptHash}/history", legacyProxy(scriptHashPath("/address/{sh}/confirmed/history"), legacyHistory)).Methods("GET")
	r.HandleFunc("/address/hash/{scriptHash}/unspent", legacyProxy(scriptHashPath("/address/{sh}/unspent/all"), legacyUnspent)).Methods("GET")
	r.HandleFunc("/address/{address}/used", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/address/{address}/scripts", proxyToServer("http://localhost:8085"))

	// /balance passes the address through unchanged so the downstream handler
	// populates `address` and runs the associated-scripthash merge logic.
	// /history and /unspent remain strict P2PKH-only (pre-conversion) for now.
	r.HandleFunc("/address/{address}/balance", legacyProxy(addressPassthroughPath("/address/{addr}/confirmed/balance"), legacyBalance)).Methods("GET")
	r.HandleFunc("/address/{address}/history", legacyProxy(addressPassthroughPath("/address/{addr}/confirmed/history"), legacyHistory)).Methods("GET")
	r.HandleFunc("/address/{address}/unspent", legacyProxy(addressToScriptHashPath("/address/{sh}/unspent/all"), legacyUnspent)).Methods("GET")
	r.HandleFunc("/addresses/unspent", legacyProxy(staticPath("/addresses/confirmed/unspent"), legacyBulkUnspent)).Methods("POST")
	r.HandleFunc("/addresses/balance", legacyProxy(staticPath("/addresses/confirmed/balance"), legacyBulkBalance)).Methods("POST")
	r.HandleFunc("/addresses/history", legacyProxy(staticPath("/addresses/history/all"), legacyBulkHistory)).Methods("POST")

	// script
	r.HandleFunc("/script/{scriptHash}/balance", legacyProxy(scriptHashPath("/address/{sh}/confirmed/balance"), legacyBalance)).Methods("GET")
	r.HandleFunc("/script/{scriptHash}/history", legacyProxy(scriptHashPath("/address/{sh}/confirmed/history"), legacyHistory)).Methods("GET")
	r.HandleFunc("/script/{scriptHash}/unspent", legacyProxy(scriptHashPath("/address/{sh}/unspent/all"), legacyUnspent)).Methods("GET")
	r.HandleFunc("/script/{addressOrScripthash}/used", isAddressOrScriptHashUsed).Methods("GET")
	r.HandleFunc("/scripts/unspent", legacyProxy(staticPath("/scripts/confirmed/unspent"), legacyBulkUnspent)).Methods("POST")
	r.HandleFunc("/scripts/balance", legacyProxy(staticPath("/scripts/confirmed/balance"), legacyBulkBalance)).Methods("POST")
	r.HandleFunc("/scripts/history", legacyProxy(staticPath("/scripts/history/all"), legacyBulkHistory)).Methods("POST")

	// search links
	r.HandleFunc("/search/links", postSearchLinks).Methods("POST")

	// ** tokens **

	// bsv-21
	r.HandleFunc("/token/bsv21/id/{id}", tokens.GetBsv21TokenByID).Methods(http.MethodGet)
	r.HandleFunc("/token/bsv21/id/{id}/owners", tokens.GetBsv21TokenOwners).Methods(http.MethodGet)
	r.HandleFunc("/token/bsv21/outpoint/{outpoint}", tokens.GetBSV21Inscription).Methods(http.MethodGet)
	r.HandleFunc("/token/bsv21/txid/{txid}/spent", tokens.GetBSV21TxSpent).Methods(http.MethodGet)
	r.HandleFunc("/token/bsv21/txid/{txid}", tokens.GetBsv21TokensByTxid).Methods(http.MethodGet)
	r.HandleFunc("/token/bsv21/{address}/id/{id}/depth", tokens.GetBSV21Depth).Methods(http.MethodGet)
	r.HandleFunc("/token/bsv21/{address}/id/{id}/history", tokens.GetBSV21HistoryByAddress).Methods(http.MethodGet)
	r.HandleFunc("/token/bsv21/{address}/balance", tokens.GetBSV21AddressBalance).Methods(http.MethodGet)
	r.HandleFunc("/token/bsv21/{address}/unspent", tokens.GetBsv21UnspentByAddress).Methods(http.MethodGet)

	// 1satordinals
	r.HandleFunc("/token/1satordinals/{outpoint}", tokens.Get1SatOrdinalsTokenByOutpoint).Methods(http.MethodGet)
	r.HandleFunc("/token/1satordinals/{origin}/origin", tokens.Get1SatOrdinalsTokenByID).Methods(http.MethodGet)
	r.HandleFunc("/token/1satordinals/{outpoint}/latest", tokens.Get1SatOrdinalsLatest).Methods(http.MethodGet)
	r.HandleFunc("/token/1satordinals/{outpoint}/history", tokens.Get1SatOrdinalsHistory).Methods(http.MethodGet)
	r.HandleFunc("/token/1satordinals/{outpoint}/content", tokens.Get1SatOrdinalsContent).Methods(http.MethodGet)
	r.HandleFunc("/token/1satordinals/tx/{txid}", tokens.Get1SatOrdinalsTokensByTxID).Methods(http.MethodGet)
	r.HandleFunc("/tokens/1satordinals", tokens.Get1SatOrdinalsProtocolInfo).Methods(http.MethodGet)

	r.HandleFunc("/address/{address}/tokens", tokens.GetAddressTokensHandler).Methods("GET")
	r.HandleFunc("/addresses/tokens", tokens.PostBulkTokensByAddress).Methods("POST")

	r.HandleFunc("/address/{address}/tokens/unspent", tokens.GetAddressUnspentTokensHandler).Methods("GET")
	r.HandleFunc("/addresses/tokens/unspent", tokens.PostBulkUnspentTokensByAddress).Methods("POST")

	r.HandleFunc("/tokens/stas", tokens.GetStasProtocolInfo).Methods("GET")
	r.HandleFunc("/tokens/{skip}/{limit}", tokens.GetTokensHandler).Methods("GET")
	r.HandleFunc("/tokens", tokens.GetTokensHandler).Methods("GET")
	r.HandleFunc("/token/{redeemAddr}/{symbol}/tx/{skip}/{limit}", tokens.GetTxByTokenID).Methods("GET")
	r.HandleFunc("/token/{redeemAddr}/{symbol}/tx", tokens.GetTxByTokenID).Methods("GET")
	r.HandleFunc("/token/{redeemAddr}/{symbol}", tokens.GetTokenByID).Methods("GET")
	r.HandleFunc("/token/tx/{txid}/out/{index}", tokens.GetTokenTxVout).Methods("GET")

	//**** new UI endpoints ****
	r.HandleFunc("/ui/homepage", ui.GetHomepage).Methods("GET")         //handler is also used by chain/summary endpoints (for now response is same)
	r.HandleFunc("/ui/homepage24hr", ui.GetHomepage24Hr).Methods("GET") //Temp endpoing to the 1b test
	r.HandleFunc("/ui/nodestatus", ui.GetNodeStatus).Methods("GET")
	r.HandleFunc("/ui/search/{query}", ui.Search).Methods("GET")

	r.HandleFunc("/ui/tx/decode", ui.DecodeRawTx).Methods("POST")
	r.HandleFunc("/ui/tx/{txid}", ui.GetTxDetails).Methods("GET")
	r.HandleFunc("/ui/tx/{txid}/stats", ui.GetTxStats).Methods("GET")                      //new using bstore
	r.HandleFunc("/ui/block/{heightOrhash}/header", bstore.GetBlockHandler).Methods("GET") //new using bstore
	r.HandleFunc("/ui/block/{heightOrhash}/tx", bstore.GetBlockTxHandler).Methods("GET")   //new using bstore
	r.HandleFunc("/ui/address/{addressOrScripthash}", ui.GetAddressOrScripthashPage).Methods("GET")
	r.HandleFunc("/ui/script/{addressOrScripthash}", ui.GetAddressOrScripthashPage).Methods("GET")
	r.HandleFunc("/ui/address/{addressOrScripthash}/mempool", ui.GetMempoolHistoryByScripthash).Methods("GET")
	r.HandleFunc("/ui/address/{addressOrScripthash}/confirmed", ui.GetConfirmedHistoryByScripthash).Methods("GET")
	r.HandleFunc("/ui/script/{addressOrScripthash}/mempool", ui.GetMempoolHistoryByScripthash).Methods("GET")
	r.HandleFunc("/ui/script/{addressOrScripthash}/confirmed", ui.GetConfirmedHistoryByScripthash).Methods("GET")
	r.HandleFunc("/ui/script/blockbalance/{scripthash}", ui.GetScripthashBlockBalance).Methods("GET")
	r.HandleFunc("/ui/report/tx", ui.RecaptchaMiddleware(ui.ReportTxHandler)).Methods("POST")
	r.HandleFunc("/ui/tagscount/{fromDate}/{toDate}", ui.GetTagCounts).Methods("GET")
	r.HandleFunc("/ui/tagscountbyblockheight/{height}", ui.GetTagCountsByBlockHeight).Methods("GET")
	r.HandleFunc("/ui/tagsbydays/{days}", ui.GetTagsByDays).Methods("GET")
	r.HandleFunc("/ui/tagssummarybydays/{days}", ui.GetTagsSummaryByDays).Methods("GET")
	r.HandleFunc("/ui/tagsperweek", ui.GetTagsPerWeek).Methods("GET")
	r.HandleFunc("/ui/searchbyfulltag", ui.SearchByTag).Methods("GET")
	r.HandleFunc("/ui/searchlatestdetailsbyfulltag", ui.SearchLatestDetailsByTag).Methods("GET")
	r.HandleFunc("/ui/mempool/stats", ui.GetMempoolStats).Methods("GET")
	r.HandleFunc("/ui/mempool/tx", ui.GetMempoolTxs).Methods("GET")

	r.HandleFunc("/ui/address/{addressOrScripthash}/stats", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/ui/script/{addressOrScripthash}/stats", proxyToServer("http://localhost:8085"))

	// proxying to new server on folder serverfiber/server.go
	r.HandleFunc("/ui/tx/propagation/{txid}", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/tx/hash/{txid}/propagation", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/ui/stats/chart/blocks", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/ui/stats/chart/summary", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/ui/stats/chart/dailysummary", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/ui/stats/blocks", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/ui/stats/minertags", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/ui/stats/mempool", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/ui/stats/tagssummary/{days}", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/ui/stats/tagssummary/height/{height}", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/ui/homepage/stats", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/ui/taggedoutputs", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/ui/searchtagoutput", proxyToServer("http://localhost:8085"))

	r.HandleFunc("/exchangerate/latest", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/exchangerate", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/exchangerate/historical", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/block/height/{height}/stats", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/block/height/{height}/txindex/{txindex}", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/block/hash/{hash}/stats", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/block/tagcount/height/{height}/stats", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/block/{from}-{to}/stats", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/block/headers/resources", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/block/headers/latest", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/block/headers/{filename}", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/ui/stats/query", proxyToServer("http://localhost:8085"))

	r.HandleFunc("/tx/{txid}/beef", proxyToServer("http://localhost:8085"))

	r.HandleFunc("/miner/blocks/stats", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/miner/summary/stats", proxyToServer("http://localhost:8085"))
	r.HandleFunc("/miner/fees", proxyToServer("http://localhost:8085"))

	//**** current old UI endpoints ****
	r.HandleFunc("/network/info", getNetworkInfo).Methods("GET")
	r.HandleFunc("/getnettotals", getNetTotals).Methods("GET")
	r.HandleFunc("/mining/info", getMiningInfo).Methods("GET")
	r.HandleFunc("/uptime", getUptime).Methods("GET")
	r.HandleFunc("/peer/info", getPeerInfo).Methods("GET")
	r.HandleFunc("/getrawmempool/{details}", getRawMempool).Methods("GET")
	r.HandleFunc("/getchaintxstats/{blockcount}", getChainTxStats).Methods("GET")
	r.HandleFunc("/validateaddress/{address}", validateAddress).Methods("GET")
	r.HandleFunc("/help", getHelp)

	r.HandleFunc("/getblocks", postBlocks)
	r.HandleFunc("/getblocksbyheight", postBlocksByHeight).Methods("POST", "OPTIONS")
	r.HandleFunc("/getblocktxids/{hash}", getBlockTxids).Methods("GET")
	r.HandleFunc("/getblocktxids/{hash}/{skip}/{limit}", getBlockTxids)
	r.HandleFunc("/gettransactions", postRawTransactionsNoLimit).Methods("POST")
	r.HandleFunc("/sendrawtransaction", postSendRawTransaction).Methods("POST")
	r.HandleFunc("/searchOpReturn", postSearchOpReturn).Methods("POST")
	r.HandleFunc("/searchForContentType", postSearchForContentType).Methods("POST")
	r.HandleFunc("/getTagCounts/{fromDate}/{toDate}", ui.GetTagCounts).Methods("GET")
	r.HandleFunc("/getTagsPerWeek", ui.GetTagsPerWeek).Methods("GET")

	//r.HandleFunc("/exchangerate", getExchangeRate).Methods("GET")

	// deprecated
	r.HandleFunc("/getblockchaininfo", getBlockchainInfo).Methods("GET")
	r.HandleFunc("/getnetworkinfo", getNetworkInfo).Methods("GET")
	r.HandleFunc("/getmempoolinfo", getMempoolInfo).Methods("GET")
	r.HandleFunc("/getmininginfo", getMiningInfo).Methods("GET")
	r.HandleFunc("/getuptime", getUptime).Methods("GET")
	r.HandleFunc("/getpeerinfo", getPeerInfo).Methods("GET")
	r.HandleFunc("/getrawmempool", getRawMempool).Methods("GET")
	r.HandleFunc("/gethelp", getHelp).Methods("GET")
	r.HandleFunc("/getblock/{hash}", getBlock).Methods("GET")
	r.HandleFunc("/getblockbyheight/{height}", getBlockByHeight).Methods("GET")
	r.HandleFunc("/gettransaction/{txid}", getRawTransaction).Methods("GET")

	// deprecated
	r.HandleFunc("/getAddressHistory/hash/{scriptHash}", legacyProxy(scriptHashPath("/address/{sh}/confirmed/history"), legacyHistory)).Methods("GET")
	r.HandleFunc("/getAddressBalance/hash/{scriptHash}", legacyProxy(scriptHashPath("/address/{sh}/confirmed/balance"), legacyBalance)).Methods("GET")
	r.HandleFunc("/getAddressHistory/{address}", legacyProxy(addressPassthroughPath("/address/{addr}/confirmed/history"), legacyHistory)).Methods("GET")
	r.HandleFunc("/getAddressBalance/{address}", legacyProxy(addressPassthroughPath("/address/{addr}/confirmed/balance"), legacyBalance)).Methods("GET")
	// electrum
	r.HandleFunc("/addresshistory/hash/{scriptHash}", legacyProxy(scriptHashPath("/address/{sh}/confirmed/history"), legacyHistory)).Methods("GET")
	r.HandleFunc("/addressbalance/hash/{scriptHash}", legacyProxy(scriptHashPath("/address/{sh}/confirmed/balance"), legacyBalance)).Methods("GET")
	r.HandleFunc("/addresshistory/{address}", legacyProxy(addressPassthroughPath("/address/{addr}/confirmed/history"), legacyHistory)).Methods("GET")
	r.HandleFunc("/addressbalance/{address}", legacyProxy(addressPassthroughPath("/address/{addr}/confirmed/balance"), legacyBalance)).Methods("GET")
	// merkle proofs
	r.HandleFunc("/proofs/{txid}", proxyToServer("http://localhost:8889"))
	r.HandleFunc("/verify/{txid}", proxyToServer("http://localhost:8889"))
	// r.HandleFunc("/tx/{txid}", proxyToServer("http://localhost:8889"))
	r.HandleFunc("/tx/{txid}/out/{index}", proxyToServer("http://localhost:8889"))

	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./www/")))

	handler := cors.Default().Handler(r)

	srv := &http.Server{
		Addr: bindAddr,

		WriteTimeout: wt,
		ReadTimeout:  rt,
		IdleTimeout:  it,
		Handler:      handler, // Pass our instance of gorilla/mux in.
	}

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			logger.Error(err)
		}
	}()

	internal.StartCacheTxWrite()
	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	srv.Shutdown(ctx)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	logger.Info("Shutting down")
	os.Exit(0)
}

func getWoc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Whats On Chain"))
}

func getBlockchainInfo(w http.ResponseWriter, r *http.Request) {
	var err error

	info, err := bitcoinClient.GetBlockchainInfo()
	if err != nil {
		logger.Errorf("GetBlockchainInfo %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(info)
}

func getBlockchainTips(w http.ResponseWriter, r *http.Request) {
	var err error

	info, err := bitcoinClient.GetChainTips()
	if err != nil {
		logger.Errorf("GetChainTips %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(info)
}

type HeaderDetails struct {
	Hash              string  `json:"hash"`
	Version           uint32  `json:"version"`
	PreviousBlockHash string  `json:"prevBlockHash"`
	NextBlockHash     string  `json:"nextblockhash,omitempty"`
	MerkleRoot        string  `json:"merkleroot"`
	Time              uint64  `json:"creationTimestamp"`
	Difficulty        float64 `json:"difficultyTarget"`
	Nonce             uint64  `json:"nonce"`
	TxCount           uint64  `json:"transactionCount"`
	//"work": 275007828391552868683
}

// block-headers-client https://github.com/bitcoin-sv/block-headers-client format
type TipDetails struct {
	Header        HeaderDetails `json:"Header"`
	Chainwork     string        `json:"work"`
	Height        uint64        `json:"height"`
	Confirmations uint64        `json:"confirmations"`
	Status        string        `json:"status,omitempty"`
	BranchLen     uint32        `json:"branchlen,omitempty"`
}

// "Possible values for status:\n"
// "1.  \"invalid\"               This branch contains at least one invalid block\n"
// "2.  \"headers-only\"          Not all blocks for this branch are available, but the headers are valid\n"
// "3.  \"valid-headers\"         All blocks are available for this branch, but they were never fully validated\n"
// "4.  \"valid-fork\"            This branch is not part of the active chain, but is fully validated\n"
// "5.  \"active\"                This is the tip of the active main chain, which is certainly valid\n"

func getBlockchainTipsWithDetails(w http.ResponseWriter, r *http.Request) {
	var err error

	tips, err := bitcoinClient.GetChainTips()
	if err != nil {
		logger.Errorf("GetChainTips %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	results := make([]TipDetails, 0)

	for _, tip := range tips {

		// Only return top 5 tips
		if len(results) > 5 {
			break
		}

		// 1: Try to get details from bstore first
		if bstore.IsEnabled() {
			block, err := bstore.GetBlockDetails(tip.Hash, 0)
			if err == nil && block != nil {
				logger.Infof("getBlockchainTipsWithDetails - Block not found in bstore %+v - %+v\n", tip.Hash, err)

				var confirmations = uint64(0)
				if confirmations > 0 && !block.Orphaned {
					confirmations = uint64(block.Confirmations)
				}

				results = append(results, TipDetails{
					Header: HeaderDetails{
						Hash:              block.Hash,
						Version:           uint32(block.Version),
						PreviousBlockHash: block.PreviousBlockHash,
						NextBlockHash:     block.NextBlockHash,
						MerkleRoot:        block.MerkleRoot,
						Time:              block.Time,
						Difficulty:        block.Difficulty,
						Nonce:             block.Nonce,
						TxCount:           block.TxCount,
					},
					Chainwork:     block.Chainwork,
					Height:        block.Height,
					Confirmations: confirmations,
					Status:        tip.Status,
					BranchLen:     tip.BranchLen,
				})
				continue
			}

		}

		//2: try mongoDB , 3: bitcoin
		//internal.GetBlock will try mongoDB first, if failed , try  bitcoin node
		block, err := internal.GetBlock(tip.Hash)
		if err != nil {
			logger.Errorf("getBlockchainTipsWithDetails - internalGetBlock for hash %+v - %+v\n", tip.Hash, err)
			continue
		}

		var confirmations = uint64(0)
		if block.Confirmations > 0 {
			confirmations = uint64(block.Confirmations)
		}

		results = append(results, TipDetails{
			Header: HeaderDetails{
				Hash:              block.Hash,
				Version:           uint32(block.Version),
				PreviousBlockHash: block.PreviousBlockHash,
				NextBlockHash:     block.NextBlockHash,
				MerkleRoot:        block.MerkleRoot,
				Time:              block.Time,
				Difficulty:        block.Difficulty,
				Nonce:             block.Nonce,
				TxCount:           block.TxCount,
			},
			Chainwork:     block.Chainwork,
			Height:        block.Height,
			Confirmations: confirmations,
			Status:        tip.Status,
			BranchLen:     tip.BranchLen,
		})

	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func getCirculatingSupply(w http.ResponseWriter, r *http.Request) {
	var err error

	info, err := bitcoinClient.GetBlockchainInfo()
	if err != nil {
		logger.Errorf("GetNetworkInfo %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	h := info.Blocks

	cs := utils.CirculatingSupply(h)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(cs)
}

func getNetworkInfo(w http.ResponseWriter, r *http.Request) {
	var err error

	info, err := bitcoinClient.GetNetworkInfo()
	if err != nil {
		logger.Errorf("GetNetworkInfo %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(info)
}

func getNetTotals(w http.ResponseWriter, r *http.Request) {
	var err error

	totals, err := bitcoinClient.GetNetTotals()
	if err != nil {
		logger.Errorf("GetNetTotals %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(totals)
}

func getMempoolInfo(w http.ResponseWriter, r *http.Request) {
	var err error

	info, err := bitcoinClient.GetMempoolInfo()
	if err != nil {
		logger.Errorf("GetMempoolInfo %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(info)
}

func getNonFinalMempoolTxList(w http.ResponseWriter, r *http.Request) {

	var err error
	txList, err := bitcoinClient.GetRawNonFinalMempool()

	if err != nil {
		logger.Errorf("GetRawNonFinalMempool -   %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(txList)
}

func getAncestorsTransaction(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	txid := vars["txid"]

	if len(txid) != 64 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var err error
	info, err := bitcoinClient.GetMempoolAncestors(txid, false)

	if err != nil {
		logger.Infof("GetMempoolAncestors for txid:%+v - %+v\n", txid, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var txIds []string
	json.Unmarshal([]byte(info), &txIds)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(txIds)
}

func getDescendantsTransaction(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	txid := vars["txid"]

	if len(txid) != 64 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var err error

	info, err := bitcoinClient.GetMempoolDescendants(txid, false)

	if err != nil {
		logger.Infof("GetMempoolDescendants for txid:%+v - %+v\n", txid, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var txIds []string
	json.Unmarshal([]byte(info), &txIds)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(txIds)
}

func getMapiAncestorsTransaction(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	txid := vars["txid"]

	if len(txid) != 64 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var err error
	info, err := bitcoin.GetMempoolAncestorsFromTaalNode(txid)

	if err != nil {
		logger.Infof("GetMempoolAncestorsFromTaalNode for txid:%+v - %+v\n", txid, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if len(info) == 0 {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]string{})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(info)
}

func getMapiDescendantsTransaction(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	txid := vars["txid"]

	if len(txid) != 64 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var err error

	info, err := bitcoin.GetMempoolDescendantsFromTaalNode(txid)

	if err != nil {
		logger.Infof("GetMempoolDescendantsFromTaalNode for txid:%+v - %+v\n", txid, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if len(info) == 0 {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]string{})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(info)
}

func getMiningInfo(w http.ResponseWriter, r *http.Request) {
	var err error

	info, err := bitcoinClient.GetMiningInfo()
	if err != nil {
		logger.Errorf("GetMiningInfo %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(info)
}

func getUptime(w http.ResponseWriter, r *http.Request) {
	var err error

	uptime, err := bitcoinClient.Uptime()
	if err != nil {
		logger.Errorf("GetUptime %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(uptime)
}

func getPeerInfo(w http.ResponseWriter, r *http.Request) {
	var err error

	info, err := bitcoinClient.GetPeerInfo()
	if err != nil {
		logger.Errorf("getPeerInfo %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(info)
}

func getRawMempool(w http.ResponseWriter, r *http.Request) {
	var err error

	vars := mux.Vars(r)
	details := vars["details"]
	var d = false
	d, _ = strconv.ParseBool(details)

	raw, err := bitcoinClient.GetRawMempool(d)
	if err != nil {
		logger.Errorf("getRawMempool %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !d {
		var txIds []string
		json.Unmarshal([]byte(raw), &txIds)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(txIds)
	} else {
		var jsonMap map[string]interface{}
		json.Unmarshal(raw, &jsonMap)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(jsonMap)
	}
}

func getChainTxStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	blockCount := vars["blockcount"]
	count, err := strconv.Atoi(blockCount)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	stats, err := bitcoinClient.GetChainTxStats(count)
	if err != nil {
		logger.Errorf("getChainTxStats %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(stats)
}

func validateAddress(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]
	var err error

	addr, err := bitcoinClient.ValidateAddress(address)
	if err != nil {
		logger.Errorf("getAddress %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(addr)
}

func isAddressOrScriptHashUsed(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	scriptHash := vars["addressOrScripthash"]
	var err error

	if len(scriptHash) > 64 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// if address convert to script hash
	if len(scriptHash) != 64 && len(scriptHash) < 64 {
		scriptHash, err = utils.AddressToScriptHash(scriptHash, network)
		if err != nil {
			logger.Errorf("AddressToScriptHash request failure for %s, %v", scriptHash, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// check electrumx - cheap to ask utxos-mempool first
	if utxosmempool.IsEnabled() {
		hasHistoryInMempool, err := utxosmempool.HasHistoryInMempool(scriptHash)
		if err != nil {
			logger.Errorf("GetConfirmedHistoryByScriptHash request failure%s, %v", scriptHash, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if hasHistoryInMempool {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(true)
			return
		}
	}

	// TODO:check utxo-store when ready

	// check electrumX
	h, err := wrappedEleC.electrumClient.HasHistory(scriptHash)
	if err != nil {
		logger.Errorf("GetConfirmedHistoryByScriptHash request failure%s, %v", scriptHash, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(h)
}

func getHelp(w http.ResponseWriter, r *http.Request) {
	var err error

	jsonBytes, err := bitcoinClient.GetHelp()
	if err != nil {
		logger.Errorf("getHelp %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var jsonMap map[string]interface{}
	json.Unmarshal(jsonBytes, &jsonMap)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(jsonMap)
}

func getTxOut(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txid := vars["txid"]
	voutStr := vars["vout"]
	incMempoolStr := vars["includeMempool"]
	incMempool := false
	var err error
	if incMempoolStr == "true" {
		incMempool = true
	}
	vout, err := strconv.Atoi(voutStr)
	if err != nil {
		logger.Errorf("getTxOut %+v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info, err := bitcoinClient.GetTxOut(txid, vout, incMempool)
	if err != nil {
		logger.Errorf("getTxOut %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(info)
}

func getBlock(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hash := vars["hash"]

	var err error

	block, err := internal.GetBlock(hash)
	if err != nil {
		logger.Errorf("getblockhash for hash %+v - %+v\n", hash, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(*block)
}

// BlockHashesBody comment
type BlockHashesBody struct {
	BlockHashes []string `json:"hashes"`
}

func postBlocks(w http.ResponseWriter, r *http.Request) {
	var blockhashesBody BlockHashesBody
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &blockhashesBody)
	hashes := blockhashesBody.BlockHashes

	results := make([]*gobitcoin.Block, 0)
	for _, hash := range hashes {

		block, err := internal.GetBlock(hash)
		if err != nil {
			logger.Errorf("postBlocks - error getblockhash for hash %+v - %+v\n", hash, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if block != nil {
			results = append(results, block)
		}
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func getBlockByHeight(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sHeight := vars["height"]
	height, err := strconv.Atoi(sHeight)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// var block *models.Block
	hash, err := bitcoinClient.GetBlockHash(height)
	if err != nil {
		logger.Errorf("getblockhash for height %+v - %+v\n", height, err)
		if strings.Contains(err.Error(), "out of range") {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if len(hash) != 64 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	block, err := internal.GetBlock(hash)
	if err != nil {
		logger.Errorf("getblockhash for height %+v - %+v\n", height, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(*block)
}

// BlockHeightsBody comment
type BlockHeightsBody struct {
	BlockHeights []int `json:"heights"`
}

func postBlocksByHeight(w http.ResponseWriter, r *http.Request) {
	var blockHeightsBody BlockHeightsBody
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &blockHeightsBody)
	heights := blockHeightsBody.BlockHeights

	results := make([]*gobitcoin.Block, 0)
	for _, height := range heights {
		//heightInteger, err := strconv.Atoi(height)

		if height < 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		hash, err := bitcoinClient.GetBlockHash(height)

		if err != nil {
			logger.Errorf("getblockhash for height %+v - %+v\n", height, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if len(hash) != 64 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		block, err := internal.GetBlock(hash)
		if err != nil {
			logger.Errorf("getblockhash for height %+v - %+v\n", height, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if block != nil {
			results = append(results, block)
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func getBlockHeader(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hash := vars["heightOrhash"]
	format := r.URL.Query().Get("format")
	var err error

	if len(hash) != 64 {
		height, err := strconv.Atoi(hash)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// var block *models.Block
		hash, err = bitcoinClient.GetBlockHash(height)
		if err != nil {
			logger.Errorf("getblockhash for height %+v - %+v\n", height, err)
			if strings.Contains(err.Error(), "out of range") {
				w.WriteHeader(http.StatusNotFound)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		if len(hash) != 64 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}

	blockHeader, err := internal.GetBlockHeader(hash)
	if err != nil {
		logger.Errorf("getblockhash for hash %+v - %+v\n", hash, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if format == "block-headers-client" {

		var status = "active"
		//check that for that height, bitcoin returns the same blockhash if not its a orphaned block header
		hash, err := bitcoinClient.GetBlockHash(int(blockHeader.Height))
		if err != nil {
			logger.Errorf("getBlockHeader - getblockhash for height %+v - %+v\n", blockHeader.Height, err)
			status = "orphaned"
		}

		if hash != blockHeader.Hash {
			status = "orphaned"
		}

		var confirmations = uint64(0)
		if blockHeader.Confirmations > 0 {
			confirmations = uint64(blockHeader.Confirmations)
		}

		var header = TipDetails{
			Header: HeaderDetails{
				Hash:              blockHeader.Hash,
				Version:           uint32(blockHeader.Version),
				PreviousBlockHash: blockHeader.PreviousBlockHash,
				NextBlockHash:     blockHeader.NextBlockHash,
				MerkleRoot:        blockHeader.MerkleRoot,
				Time:              blockHeader.Time,
				Difficulty:        blockHeader.Difficulty,
				Nonce:             blockHeader.Nonce,
				TxCount:           uint64(blockHeader.TxCount),
			},
			Chainwork:     blockHeader.Chainwork,
			Height:        blockHeader.Height,
			Confirmations: confirmations,
			Status:        status,
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(header)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(*blockHeader)
}

func getBlockHeadersList(w http.ResponseWriter, r *http.Request) {
	// blocks?hash=xxx&order=desc&limit=10

	qs := r.URL.Query()

	if len(qs) == 0 {
		headers, err := bitcoinClient.GetCachedLatestHeaders()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(headers)
		return
	}

	hash := qs.Get("hash")
	order := strings.ToLower(qs.Get("order"))
	limitStr := qs.Get("limit")

	// Default order desc
	if order == "" {
		order = "desc"
	}

	// Default limit 10
	if limitStr == "" {
		limitStr = "10"
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
	if limit < 1 || limit > 20 {
		mapD := map[string]string{"error": "limit can't be less than 1 or greater than 20"}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(mapD)
		return
	}

	// return array
	blocks := make([]*gobitcoin.BlockHeader, limit)
	itemsInArray := 0
	nextHash := ""
	if hash != "" {
		nextHash = hash
	}
	if order == "desc" {
		if nextHash == "" {
			// getbestblockhash
			nextHash, err = bitcoinClient.GetBestBlockHash()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
		for i := 0; i < limit; i++ {
			block, err := internal.GetBlockHeader(nextHash)
			if err != nil {
				logger.Errorf("getblockhash for hash %+v - %+v\n", nextHash, err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			blocks[i] = block
			itemsInArray++
			nextHash = block.PreviousBlockHash
			if nextHash == "" {
				break
			}
		}
	} else {
		if nextHash == "" {
			genesisBlock, ok := gocore.Config().Get("genesisBlock")
			if ok {
				nextHash = genesisBlock
			}
		}
		for i := 0; i < limit; i++ {
			block, err := internal.GetBlockHeader(nextHash)
			if err != nil {
				logger.Errorf("getblockhash for hash %+v - %+v\n", nextHash, err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			blocks[i] = block
			itemsInArray++
			nextHash = block.NextBlockHash
			if nextHash == "" {
				break
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	if itemsInArray < limit {
		json.NewEncoder(w).Encode(blocks[:itemsInArray])
	} else {
		json.NewEncoder(w).Encode(blocks)
	}
}

func getBlockPage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hash := vars["hash"]

	page, err := strconv.Atoi(vars["number"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if page <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// first 100 ids are save with the block
	skip := 100

	if page > 1 {
		skip = 100 + mongocache.BlockTxidCollectionMaxTx*(page-1)
	}

	txids, err := mongocache.GetTxidsForAPIBlockPage(hash, skip, mongocache.BlockTxidCollectionMaxTx)
	if err != nil {
		logger.Errorf("getting txids for block %+s: %+v\n", hash, err)
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(txids)
}

func getBlockTxids(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hash := vars["hash"]

	skip, err := strconv.Atoi(vars["skip"])
	if err != nil {
		skip = 0
	}

	limit, err := strconv.Atoi(vars["limit"])
	if err != nil {
		limit = 10000
	}

	txids, err := mongocache.GetTxidsForBlock(hash, skip, limit)
	if err != nil {
		logger.Errorf("failed to get txids for block %+s: %+v\n", hash, err)
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(txids)
}

func getRawTransactionHex(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txid := vars["txid"]

	if len(txid) != 64 {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("txid must be 64 hex characters in length"))
		return
	}

	var err error
	var tx *string

	//try bstore first
	if bstore.IsEnabled() {
		// from bstore
		tx, _ = bstore.GetTxHex(txid)
		if tx != nil {
			w.Header().Set("content-disposition", "attachment;filename="+txid+".hex")
			w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(*tx))
			return
		}

		logger.Warnf("Failed to gettransactionhex from bstore for txid %+v - %+v\n", txid, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// from node
	tx, err = bitcoinClient.GetRawTransactionHex(txid)
	if err != nil {
		logger.Errorf("gettransactionhex for txid %+v - %+v\n", txid, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if tx != nil {
		w.Header().Set("content-disposition", "attachment;filename="+txid+".hex")
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(*tx))
	}
}
func getRawTransactionBin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txid := vars["txid"]

	if len(txid) != 64 {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("txid must be 64 hex characters in length"))
		return
	}

	var err error
	var tx *string

	//try bstore first
	if bstore.IsEnabled() {
		// from bstore
		tx, _ = bstore.GetTxHex(txid)
		if tx != nil {
			w.Header().Set("content-disposition", "attachment;filename="+txid+".bin")
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			binaryData, _ := hex.DecodeString(*tx)
			w.Write(binaryData)
			return
		}

		logger.Warnf("Failed to gettransactionhex from bstore for txid %+v - %+v\n", txid, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// from node
	tx, err = bitcoinClient.GetRawTransactionHex(txid)
	if err != nil {
		logger.Errorf("gettransactionhex for txid %+v - %+v\n", txid, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if tx != nil {
		w.Header().Set("content-disposition", "attachment;filename="+txid+".bin")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		binaryData, _ := hex.DecodeString(*tx)
		w.Write(binaryData)
	}
}

func getMapiRawTransactionHex(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txid := vars["txid"]

	if len(txid) != 64 {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("txid must be 64 hex characters in length"))
		return
	}

	var err error
	var tx *gobitcoin.RawTransaction

	// from node
	tx, err = bitcoin.GetRawTransactionFromTaalNode(txid)
	if err != nil {
		logger.Warnf("gettransactionhex for txid %+v - %+v\n", txid, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if tx != nil {
		w.Header().Set("content-disposition", "attachment;filename="+txid+".hex")
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(tx.Hex))
	}
}

func getRawTransactionOutputHex(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txid := vars["txid"]
	indexStr := vars["index"]

	if len(txid) != 64 {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("txid must be 64 hex characters in length"))
		return
	}

	index, indexErr := strconv.Atoi(indexStr)
	if indexErr != nil {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("vout must be a number"))
		return
	}

	if bstore.IsEnabled() {
		//from bstore
		voutHex, err := bstore.GetTxVoutHex(txid, int64(index))

		if voutHex != nil {
			w.Header().Set("content-disposition", "attachment;filename="+txid+"_"+strconv.Itoa(index)+".hex")
			w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(*voutHex))
			return
		}

		logger.Warnf("Failed to get gettransactionIndexhex from bstore for txid and index %+v , vout %+v - %+v\n", txid, index, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//from node
	var err error
	var tx *gobitcoin.RawTransaction
	tx, err = bitcoinClient.GetRawTransaction(txid, false, true)
	if err != nil {
		logger.Errorf("gettransactionhex for txid %+v, vout index %+v - %+v\n", txid, index, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if tx != nil {
		if len(tx.Vout) > 0 && (len(tx.Vout) >= (index + 1)) {
			w.Header().Set("content-disposition", "attachment;filename="+txid+"_"+strconv.Itoa(index)+".hex")
			w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(tx.Vout[index].ScriptPubKey.Hex))
			return
		} else {
			w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("vout must be less than " + strconv.Itoa(len(tx.Vout))))
			return
		}
	}

	logger.Warnf("gettransactionIndexhex for txid and index %+v , vout %+v - %+v\n", txid, index, err)
	w.WriteHeader(http.StatusBadRequest)
}

func getRawTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txid := vars["txid"]

	tx, err := internal.GetTransaction(txid)
	if err != nil {
		logger.Errorf("gettransaction for txid %+v - %+v\n", txid, err)
	}

	//patch to fix confirmation for satoshi tx - block 0
	if txid == "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b" {
		//Get tx in block 1 to get correct confirmations
		txPatch, err := internal.GetTransaction("0e3e2357e806b6cdb1f70b54c3a3a17b6714ee1f0e68bebb44a74b1efd512098")
		if err != nil {
			logger.Errorf("gettransaction for txid %+v - %+v\n", txid, err)
		}
		tx.Confirmations = txPatch.Confirmations + 1
	}

	if tx != nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(*tx)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func getMapiRawTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txid := vars["txid"]

	tx, err := bitcoin.GetRawTransactionFromTaalNode(txid)
	if err != nil {
		logger.Errorf("GetRawTransactionFromTaalNode for txid %+v - %+v\n", txid, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	//patch to fix confirmation for satoshi tx - block 0
	if txid == "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b" {
		//Get tx in block 1 to get correct confirmations
		txPatch, err := internal.GetTransaction("0e3e2357e806b6cdb1f70b54c3a3a17b6714ee1f0e68bebb44a74b1efd512098")
		if err != nil {
			logger.Errorf("getTransaction for txid %+v - %+v\n", txid, err)
		}
		tx.Confirmations = txPatch.Confirmations + 1
	}

	//Add tags
	if len(tx.Vout) > 0 {
		for i, vout := range tx.Vout {

			if strings.HasPrefix(vout.ScriptPubKey.ASM, "0 ") {
				vout.ScriptPubKey.ASM = strings.TrimPrefix(vout.ScriptPubKey.ASM, "0 ")
				vout.ScriptPubKey.Hex = vout.ScriptPubKey.Hex[2:]
			}

			if strings.HasPrefix(vout.ScriptPubKey.ASM, "OP_RETURN") {
				buf, err := hex.DecodeString(vout.ScriptPubKey.Hex)
				if err != nil {
					logger.Errorf("parsing op_return script %+v", err)
					continue
				}
				tag, subtag, parts, err := parser.ParseOpReturn(buf)

				if err == nil {
					ps := &gobitcoin.OpReturn{}
					ps.Type = tag
					ps.Action = subtag

					if parts != nil && *parts != nil && len(*parts) > 0 && !internal.BadTxids.Has(txid) {
						ps.Text = (*parts)[0].URI
						var up []string
						for _, p := range *parts {
							up = append(up, p.UTF8)
						}
						ps.Parts = up
					}
					tx.Vout[i].ScriptPubKey.OpReturn = ps
				}
				if internal.BadTxids.Has(txid) {
					tx.Vout[i].ScriptPubKey.ASM = "removed"
					tx.Vout[i].ScriptPubKey.Hex = "removed"
				}

			} else if vout.ScriptPubKey.Type == "nonstandard" {

				buf, err := hex.DecodeString(vout.ScriptPubKey.Hex)
				if err != nil {
					logger.Errorf("parsing op_return script %+v", err)
					continue
				}

				tag, subtag, err := parser.ParseNonStandard(buf)
				if err == nil {
					t := &gobitcoin.Tag{}
					t.Type = tag
					t.Action = subtag
					tx.Vout[i].ScriptPubKey.Tag = t
				}
			}
		}
	}

	if tx != nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(*tx)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

// TxidBody comment
type TxidBody struct {
	Txids []string `json:"txids"`
}

// used by api
func postRawTransactionsWithLimit(w http.ResponseWriter, r *http.Request) {
	var txidBody TxidBody
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &txidBody)
	txids := txidBody.Txids

	if len(txids) > 20 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode("Maximum number of transactions per request has been exceeded")
		return
	}
	postRawTransactions(w, txids)
}

// used by ui
func postRawTransactionsNoLimit(w http.ResponseWriter, r *http.Request) {
	var txidBody TxidBody
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &txidBody)
	txids := txidBody.Txids

	postRawTransactions(w, txids)
}

func postRawTransactions(w http.ResponseWriter, txids []string) {

	var err error

	confirmationsMap := make(map[string]int64)
	txIDResultsIndexMap := make(map[string]int64)

	results := make([]*gobitcoin.RawTransaction, 0)
	for _, txid := range txids {
		if len(txid) == 64 {

			//Check if this txid was already processed
			index, duplicate := txIDResultsIndexMap[txid]
			if duplicate {
				results = append(results, results[index])
				continue
			}

			var tx *gobitcoin.RawTransaction
			ok := false
			if useMongoCache {
				tx, ok = mongocache.GetTransactionFromCache(txid)
			}
			if !ok { // no transaction in cache
				tx, err = bitcoinClient.GetRawTransaction(txid, true, true)
				if err != nil {

					if taalBitcoinProxyEnabled {
						tx, err = bitcoin.GetRawTransactionFromTaalNode(txid)

						if err != nil {
							logger.Errorf("GetRawTransactionFromTaalNode for txid %+v - %+v\n", txid, err)
							continue
						}

					} else {
						logger.Errorf("GetRawTransaction for txid %+v - %+v\n", txid, err)
						continue
					}
				}

				// todo: extract to common method
				if len(tx.Vout) > 0 {
					for i, vout := range tx.Vout {

						if strings.HasPrefix(vout.ScriptPubKey.ASM, "0 ") {
							vout.ScriptPubKey.ASM = strings.TrimPrefix(vout.ScriptPubKey.ASM, "0 ")
							vout.ScriptPubKey.Hex = vout.ScriptPubKey.Hex[2:]

						}
						if strings.HasPrefix(vout.ScriptPubKey.ASM, "OP_RETURN") {
							buf, err := hex.DecodeString(vout.ScriptPubKey.Hex)
							if err != nil {
								logger.Errorf("parsing op_return script %+v", err)
								continue
							}
							tag, subtag, parts, err := parser.ParseOpReturn(buf)
							if err == nil {
								ps := &gobitcoin.OpReturn{}
								ps.Type = tag
								ps.Action = subtag
								if parts != nil && *parts != nil && len(*parts) > 0 && !internal.BadTxids.Has(txid) {
									ps.Text = (*parts)[0].URI
								}
								tx.Vout[i].ScriptPubKey.OpReturn = ps
							}
							if internal.BadTxids.Has(txid) {
								tx.Vout[i].ScriptPubKey.ASM = "removed"
								tx.Vout[i].ScriptPubKey.Hex = "removed"
							}

						} else if vout.ScriptPubKey.Type == "nonstandard" {

							buf, err := hex.DecodeString(vout.ScriptPubKey.Hex)
							if err != nil {
								logger.Errorf("parsing op_return script %+v", err)
								continue
							}

							tag, subtag, err := parser.ParseNonStandard(buf)
							if err == nil {
								t := &gobitcoin.Tag{}
								t.Type = tag
								t.Action = subtag
								tx.Vout[i].ScriptPubKey.Tag = t
							}
						}
					}
				}

				//this will cache if chache condition are meet
				internal.CacheTx(tx)

			} else {
				//check if it exists in map
				confirmations, ok := confirmationsMap[tx.BlockHash]
				if !ok {
					//tx from cache, its cheap to get confirmation from blockheader
					blockHeader, err := bitcoinClient.GetBlockHeader(tx.BlockHash)
					if err != nil {
						logger.Errorf("GetBlockHeader for txid %+v - %+v\n", txid, err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					confirmations := blockHeader.Confirmations
					if confirmations > -1 {
						tx.Confirmations = uint32(confirmations)
					}
				} else {
					tx.Confirmations = uint32(confirmations)
				}
			}
			if tx != nil {
				results = append(results, tx)
				//Add to map
				confirmationsMap[tx.BlockHash] = int64(tx.Confirmations)
				//save processed txId and its index in results map
				txIDResultsIndexMap[txid] = int64(len(results) - 1)
			}
		} // end txix != ""
	} // end tx range
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

// SendTxBody comment
type SendTxBody struct {
	TxHex string `json:"txhex"`
}

func postSendRawTransaction(w http.ResponseWriter, r *http.Request) {

	var hexBody SendTxBody
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &hexBody)
	var txid string
	var err error
	if hexBody.TxHex != "" {
		txid, err = bitcoinClient.SendRawTransaction(hexBody.TxHex)
		if err != nil {
			// logger.Errorf("sendrawtransaction for hex %+v - %+v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(fmt.Sprintf("%v", err))
			return
		}
	}

	results := txid
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)

}
func postDecodeRawTransaction(w http.ResponseWriter, r *http.Request) {
	var hexBody SendTxBody
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &hexBody)
	var decodedTx string
	var err error
	if hexBody.TxHex != "" {
		decodedTx, err = bitcoinClient.DecodeRawTransaction(hexBody.TxHex)
		if err != nil {
			logger.Errorf("decoderawtransaction: %+v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(fmt.Sprintf("%v", err))
			return
		}
	}

	results := decodedTx

	var jsonResult map[string]interface{}
	_ = json.Unmarshal([]byte(decodedTx), &jsonResult)
	txid := jsonResult["txid"].(string)
	if internal.BadTxids.Has(txid) {
		ra := r.RemoteAddr
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			ra = ra + ", " + xff
		}
		logger.Errorf("someone [%s] tried decoding bad tx: %s\n", ra, txid)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(fmt.Sprintf("Your IP address [%s] has been recorded.", ra))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(results))
}

// SearchRequest comment
type SearchRequest struct {
	Term    string   `json:"term"`
	Phrases []string `json:"phrases"`
	From    int      `json:"from"`
	Size    int      `json:"size"`
}

func postSearchOpReturn(w http.ResponseWriter, r *http.Request) {
	opReturnSearch := gocore.Config().GetBool("opReturnSearch", false)
	if !opReturnSearch {
		// op return search not enabled. return
		logger.Warn("opreturn search disabled in config")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	var searchRequest SearchRequest
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &searchRequest)

	result, err := search.Find(searchRequest.Term, searchRequest.From, searchRequest.Size)
	if err != nil {
		logger.Errorf("searching for term %+v - %+v\n", searchRequest.Term, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// SearchLinksRequest comments
type SearchLinksRequest struct {
	Query string `json:"query"`
}

// SearchLinksResponse comments
type SearchLinksResponse struct {
	Links []LinkTypeMap `json:"results"`
}

// LinkTypeMap comments
type LinkTypeMap struct {
	TYPE string `json:"type"`
	URI  string `json:"url"`
}

func postSearchLinks(w http.ResponseWriter, r *http.Request) {

	var searchRequest SearchLinksRequest
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &searchRequest)

	query := strings.TrimSpace(strings.ToLower(searchRequest.Query))
	rawCaseQuery := strings.TrimSpace(searchRequest.Query)

	var searchResponse SearchLinksResponse
	if len(query) == 64 {

		//Is it tx hash
		var tx *gobitcoin.RawTransaction
		ok := false
		if useMongoCache {
			tx, ok = mongocache.GetTransactionFromCache(query)
		}
		if !ok {
			tx, _ = bitcoinClient.GetRawTransaction(query, true, true)
		}

		if tx != nil {
			var link LinkTypeMap
			link.TYPE = "tx"
			link.URI = "https://whatsonchain.com/tx/" + query
			searchResponse.Links = append(searchResponse.Links, link)
		}

		//Is it block hash
		block, _ := internal.GetBlock(query)
		if block != nil {
			var link LinkTypeMap
			link.TYPE = "block"
			link.URI = "https://whatsonchain.com/block/" + query
			searchResponse.Links = append(searchResponse.Links, link)
		}

	} else {

		// Is it block heigh
		height, err := strconv.Atoi(query)
		if err == nil {
			hash, err := bitcoinClient.GetBlockHash(height)
			if err == nil {
				block, _ := internal.GetBlock(hash)
				if block != nil {
					var link LinkTypeMap
					link.TYPE = "block"
					link.URI = "https://whatsonchain.com/block-height/" + query
					searchResponse.Links = append(searchResponse.Links, link)
				}
			}
		}
	}

	addr, err := bitcoinClient.ValidateAddress(rawCaseQuery)
	if err != nil {
		logger.Errorf("getAddress %s %+v\n", err, rawCaseQuery)
	} else {
		if addr.IsValid {
			var link LinkTypeMap
			link.TYPE = "address"
			link.URI = "https://whatsonchain.com/address/" + rawCaseQuery
			searchResponse.Links = append(searchResponse.Links, link)
		}
	}

	// op_return search
	opReturnSearch := gocore.Config().GetBool("opReturnSearch", false)
	if opReturnSearch {
		result, err := search.Find(query, 0, 1)
		if err == nil && result.Count > 0 {
			var link LinkTypeMap
			link.TYPE = "op_return"
			link.URI = "https://whatsonchain.com/opreturn-query?term=" + query + "&size=10&offset=0"
			searchResponse.Links = append(searchResponse.Links, link)
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(searchResponse)
}

func postSearchForContentType(w http.ResponseWriter, r *http.Request) {
	opReturnSearch := gocore.Config().GetBool("opReturnSearch", false)
	if !opReturnSearch {
		// op return search not enabled. return
		logger.Warn("opreturn search disabled in config")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	var searchRequest SearchRequest
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &searchRequest)

	result, err := search.FindContentTypes(searchRequest.Phrases, searchRequest.From, searchRequest.Size)
	if err != nil {
		logger.Errorf("searching for content types %+v - %+v\n", searchRequest.Phrases, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

type mapiTxJSON struct {
	RawTX string `json:"rawtx"`
}

// electrum
func getAddressHistoryByScriptHash(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	scriptHash := vars["scriptHash"]

	if wrappedEleC.electrumClient == nil {
		logger.Errorf("Couldn't get history for script %s. electrumClient is nil, check electrumX connection", scriptHash)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h, err := wrappedEleC.electrumClient.GetAddressHistory(scriptHash)
	if err != nil {
		logger.Warnf("Couldn't get script history for script %s, %+v", scriptHash, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(h)
}

func getAddressBalanceByScriptHash(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	scriptHash := vars["scriptHash"]

	if wrappedEleC.electrumClient == nil {
		logger.Errorf("Can't get balance for script %s. electrumClient is nil, check electrumX connection", scriptHash)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	b, err := wrappedEleC.electrumClient.GetAddressBalance(scriptHash)
	if err != nil {
		logger.Warnf("Couldn't get balance for address %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(b)
}

func getAddressUnspentByScriptHash(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	scriptHash := vars["scriptHash"]

	if wrappedEleC.electrumClient == nil {
		logger.Errorf("Can't get unspent for script %s. electrumClient is nil, check electrumX connection", scriptHash)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	b, err := wrappedEleC.electrumClient.ListUnspent(scriptHash)
	if err != nil {
		logger.Warnf("Couldn't get unspent for address %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(b)
}

func getAddressHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]

	if wrappedEleC.electrumClient == nil {
		logger.Errorf("Can't get history for address %s. electrumClient is nil, check electrumX connection", address)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	scriptHash, err := utils.AddressToScriptHash(address, network)
	if err != nil {
		logger.Errorf("AddressToScriptHash failed for address %s , %+v\n", address, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h, err := wrappedEleC.electrumClient.GetAddressHistory(scriptHash)
	if err != nil {
		logger.Warnf("Couldn't get balance for address %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(h)
}

func getAddressBalance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]

	if wrappedEleC.electrumClient == nil {
		logger.Errorf("Can't get balance for address %s. electrumClient is nil, check electrumX connection", address)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	scriptHash, err := utils.AddressToScriptHash(address, network)
	if err != nil {
		logger.Errorf("AddressToScriptHash failed for address %s , %+v\n", address, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	b, err := wrappedEleC.electrumClient.GetAddressBalance(scriptHash)
	if err != nil {
		logger.Warnf("Couldn't get balance for address %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(b)

}

func getMerkleProofWithNodeAsBackup(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	txid := vars["txid"]

	//try merkle proof service
	if merkleProofServiceEnabled {
		client := &http.Client{}
		req, err := http.NewRequest("GET", merkleProofServiceAddress+"/proofs/"+txid, nil)
		if err != nil {
			logger.Errorf("Failed to create request: " + err.Error())
		} else {

			resp, err := client.Do(req)
			if err != nil {
				logger.Errorf("Failed to call client: " + err.Error())
			} else {
				defer resp.Body.Close()

				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					logger.Errorf("Failed to read response body: " + err.Error())
				}

				if len(body) > 20 {
					w.Header().Set("Content-Type", "application/json")

					// Historical behaviour from the standalone merkle service is to return an array
					// of proofs. However, some txs were answered as single objects. We normalise
					// those to a singleton array so API consumers always see the same shape.
					var arr []json.RawMessage
					if err := json.Unmarshal(body, &arr); err == nil {
						w.Write(body)
						return
					}

					var obj map[string]interface{}
					if err := json.Unmarshal(body, &obj); err == nil {
						normalized, marshalErr := json.Marshal([]map[string]interface{}{obj})
						if marshalErr == nil {
							w.Write(normalized)
							return
						}
					}

					// If the payload is neither array nor object we keep the service response as-is.
					w.Write(body)
					return
				}
			}
		}
	}

	// Service could not provide a proof, so revert to the node to preserve backwards compatibility.
	tx, err := internal.GetTransaction(txid)
	if err != nil {
		// Legacy clients depended on the string "null" instead of a 404.
		w.Write([]byte("null"))
		return
	}

	if len(tx.BlockHash) > 2 {
		merkle, err := bitcoinClient.GetMerkleProof(tx.BlockHash, txid)
		if err != nil {
			logger.Errorf("Failed to GetMerkleProof from bitcoin: %v", err)

			// If the node cannot find the tx inside the block we preserve the
			// historical behaviour: respond with a single-element array whose
			// Nodes entry is null. Callers rely on this to distinguish between
			// “not in block” and other errors.
			type merkleFallback struct {
				Index  int         `json:"index"`
				TxOrId string      `json:"txOrId"`
				Target string      `json:"target"`
				Nodes  interface{} `json:"nodes"`
			}

			if strings.Contains(err.Error(), "Transaction(s) not found in provided block") {
				fallback := []merkleFallback{
					{
						Index:  1,
						TxOrId: txid,
						Target: tx.BlockHash,
						Nodes:  nil,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(fallback)
				return
			}
			// otherwise, keep the old “null” behavior
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("null"))
			return
		}
		// For consistency with the service payload, always return an array—even when the
		// proof originated from the node fallback path.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*gobitcoin.MerkleProof{merkle})
		return
	}

	//null to make it compatible with old endpoint and not break users apps ... :(
	w.Write([]byte("null"))
	return

}

func getMerkleProof(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txid := vars["txid"]
	h := vars["height"]

	if wrappedEleC.electrumClient == nil {
		logger.Errorf("Can't get merkleproof for tx %s. electrumClient is nil, check electrumX connection", txid)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// if height is not given get block hash from tx and get height from blockbyhash
	if h == "" && txid != "" {
		tx, err := bitcoinClient.GetRawTransaction(txid, true, true)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if len(tx.BlockHash) > 2 {
			block, err := internal.GetBlock(tx.BlockHash)
			if err != nil {
				logger.Errorf("getblockhash for hash %+v - %+v\n", tx.BlockHash, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			h = strconv.FormatInt(int64(block.Height), 10)
		} else {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}
	height, err := strconv.ParseUint(h, 10, 32)
	if err != nil {
		logger.Errorf("height %q not an integer %+v\n", height, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	b, err := wrappedEleC.electrumClient.GetMerkleProof(txid, uint32(height))
	if err != nil {
		logger.Errorf("Couldn't get merkle proof for txid %s and height %d: %+v", txid, height, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(b)
}

func getAddressUnspent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]

	if wrappedEleC.electrumClient == nil {
		logger.Errorf("Can't get unspent for address %s. electrumClient is nil, check electrumX connection", address)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	scriptHash, err := utils.AddressToScriptHash(address, network)
	if err != nil {
		logger.Errorf("AddressToScriptHash failed for address %s , %+v\n", address, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	b, err := wrappedEleC.electrumClient.ListUnspent(scriptHash)
	if err != nil || b == nil {

		logger.Errorf("Error from electrumX (getAddressUnspent): Failed to get Unspent data - 1/2 try response:%v, err:%v", b, err)

		b, err = wrappedEleC.electrumClient.ListUnspent(scriptHash)
		if err != nil || b == nil {
			logger.Errorf("Error from electrumX (getAddressUnspent): Failed to get Unspent data - 2/2 try response:%v, err:%v", b, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(b)

}

type bulkAddressesRequest struct {
	Addresses []string `json:"addresses"`
}

type bulkAddressesResponse struct {
	Address string                             `json:"address"`
	Unspend []*electrumTypes.ListUnspentResult `json:"unspent"`
	Error   string                             `json:"error"`
}

func postBulkUnspentByAddress(w http.ResponseWriter, r *http.Request) {
	var reqBody bulkAddressesRequest
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &reqBody)
	addrs := utils.Unique(reqBody.Addresses)

	if len(addrs) > 20 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode("Maximum number of addresses per request has been exceeded")
		return
	}

	if wrappedEleC.electrumClient == nil {
		logger.Errorf("Can't get unspent for address list. electrumClient is nil, check electrumX connection")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var response [20]bulkAddressesResponse

	for index, address := range addrs {
		response[index].Address = address

		scriptHash, err := utils.AddressToScriptHash(address, network)
		if err != nil {
			response[index].Error = "Unable to convert address to scripthash"
			continue
		}

		b, err := wrappedEleC.electrumClient.ListUnspent(scriptHash)

		if err != nil || b == nil {
			// response[index].Error = "Failed to get Unspent data"
			logger.Errorf("Error from electrumX (postBulkUnspentByAddress): Failed to get Unspent data - 1/2 try response:%v, err:%v", b, err)

			//try again
			b, err = wrappedEleC.electrumClient.ListUnspent(scriptHash)
			if err != nil || b == nil {
				logger.Errorf("Error from electrumX (postBulkUnspentByAddress): Failed to get Unspent data - 2/2 try response:%v, err:%v", b, err)
				response[index].Error = "Failed to get Unspent data"
				continue
			}

		}
		response[index].Unspend = b

	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response[0:len(addrs)])
}

type bulkAddressesHistoryResponse struct {
	Address string                            `json:"address"`
	History []*electrumTypes.GetMempoolResult `json:"history"`
	Error   string                            `json:"error"`
}

func postBulkHistoryByAddress(w http.ResponseWriter, r *http.Request) {
	var reqBody bulkAddressesRequest
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &reqBody)
	addrs := utils.Unique(reqBody.Addresses)

	if len(addrs) > 20 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if wrappedEleC.electrumClient == nil {
		logger.Errorf("Can't get history for address list. electrumClient is nil, check electrumX connection")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var response [20]bulkAddressesHistoryResponse

	for index, address := range addrs {
		response[index].Address = address

		scriptHash, err := utils.AddressToScriptHash(address, network)
		if err != nil {
			response[index].Error = "Unable to convery address to scripthash"
			continue
		}

		b, err := wrappedEleC.electrumClient.GetAddressHistoryOrTooLargeError(scriptHash)
		if err != nil {
			response[index].Error = err.Error()
			continue
		}

		if b == nil {
			response[index].Error = "Failed to get history"
			continue
		}

		response[index].History = b

	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response[0:len(addrs)])
}

type bulkAddressesBalanceResponse struct {
	Address string                   `json:"address"`
	Balance *electrum.AddressBalance `json:"balance"`
	Error   string                   `json:"error"`
}

func postBulkBalanceByAddress(w http.ResponseWriter, r *http.Request) {
	var reqBody bulkAddressesRequest
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &reqBody)
	addrs := utils.Unique(reqBody.Addresses)

	if len(addrs) > 20 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode("Maximum number of addresses per request has been exceeded")
		return
	}

	if wrappedEleC.electrumClient == nil {
		logger.Errorf("Can't get blanace for address list. electrumClient is nil, check electrumX connection")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var response [20]bulkAddressesBalanceResponse

	for index, address := range addrs {
		response[index].Address = address

		scriptHash, err := utils.AddressToScriptHash(address, network)
		if err != nil {
			response[index].Error = "Unable to convery address to scripthash"
			continue
		}

		b, err := wrappedEleC.electrumClient.GetAddressBalance(scriptHash)
		if err != nil || b == nil {
			response[index].Error = "Failed to get balance data"
			continue
		}
		response[index].Balance = b

	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response[0:len(addrs)])
}

type bulkScriptRequest struct {
	Scripts []string `json:"scripts"`
}

type bulkScriptUnspentResponse struct {
	Script  string                             `json:"script"`
	Unspend []*electrumTypes.ListUnspentResult `json:"unspent"`
	Error   string                             `json:"error"`
}

// used by api
func postBulkUnspentByScriptHash(w http.ResponseWriter, r *http.Request) {
	var reqBody bulkScriptRequest
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &reqBody)
	scripts := utils.Unique(reqBody.Scripts)

	if len(scripts) > 20 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode("Maximum number of scripts per request has been exceeded")
		return
	}

	if wrappedEleC.electrumClient == nil {
		logger.Errorf("Can't get unspent for scripthash list. electrumClient is nil, check electrumX connection")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var response [20]bulkScriptUnspentResponse

	for index, scriptHash := range scripts {
		response[index].Script = scriptHash

		if len(scriptHash) != 64 {
			response[index].Error = "Invalid Script"
			continue
		}

		b, err := wrappedEleC.electrumClient.ListUnspent(scriptHash)
		if err != nil {
			response[index].Error = "Failed to get Unspent data"
			continue
		}
		response[index].Unspend = b

	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response[0:len(scripts)])
}

type bulkScriptBalanceResponse struct {
	Script  string                   `json:"script"`
	Balance *electrum.AddressBalance `json:"balance"`
	Error   string                   `json:"error"`
}

// used by api
func postBulkBalanceByScriptHash(w http.ResponseWriter, r *http.Request) {
	var reqBody bulkScriptRequest
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &reqBody)
	scripts := utils.Unique(reqBody.Scripts)

	if len(scripts) > 20 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if wrappedEleC.electrumClient == nil {
		logger.Errorf("Can't get balance for scripthash list. electrumClient is nil, check electrumX connection")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var response [20]bulkScriptBalanceResponse

	for index, scriptHash := range scripts {
		response[index].Script = scriptHash

		if len(scriptHash) != 64 {
			response[index].Error = "Invalid Script"
			continue
		}

		b, err := wrappedEleC.electrumClient.GetAddressBalance(scriptHash)
		if err != nil {
			response[index].Error = "Failed to get balance"
			continue
		}
		response[index].Balance = b

	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response[0:len(scripts)])
}

type bulkScriptHistoryResponse struct {
	Script  string                            `json:"script"`
	History []*electrumTypes.GetMempoolResult `json:"History"`
	Error   string                            `json:"error"`
}

// used by api
func postBulkHistoryByScriptHash(w http.ResponseWriter, r *http.Request) {
	var reqBody bulkScriptRequest
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &reqBody)
	scripts := utils.Unique(reqBody.Scripts)

	if len(scripts) > 20 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if wrappedEleC.electrumClient == nil {
		logger.Errorf("Can't get history for scripthash list. electrumClient is nil, check electrumX connection")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var response [20]bulkScriptHistoryResponse

	for index, scriptHash := range scripts {
		response[index].Script = scriptHash

		if len(scriptHash) != 64 {
			response[index].Error = "Invalid Script"
			continue
		}

		b, err := wrappedEleC.electrumClient.GetAddressHistoryOrTooLargeError(scriptHash)
		if err != nil {
			response[index].Error = err.Error()
			continue
		}
		response[index].History = b

	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response[0:len(scripts)])
}

func proxyToServer(target string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		url, _ := url.Parse(target)

		proxy := httputil.NewSingleHostReverseProxy(url)

		// Update the headers to allow for SSL redirection
		r.URL.Host = url.Host
		r.URL.Scheme = url.Scheme
		r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
		r.Host = url.Host

		proxy.ServeHTTP(w, r)
	}

}

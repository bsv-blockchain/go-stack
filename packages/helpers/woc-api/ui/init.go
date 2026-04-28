package ui

import (
	"sync"
	"time"

	"github.com/ordishs/gocore"
	"github.com/teranode-group/woc-api/bitcoin"
	"github.com/teranode-group/woc-api/electrum"
)

var logger = gocore.Log("woc-api")

// var electrumClient *electrum.Client
type wrappedElectrumClient struct {
	mu             sync.RWMutex
	electrumClient *electrum.Client
}

var wrappedEleC *wrappedElectrumClient
var bitcoinClient *bitcoin.Client
var taalBitcoinProxyEnabled bool
var isMainnet bool
var network string
var elasticSearchEnabled bool
var nonFinalMempoolSearchEnabled bool
var utxoStorePriorityForUI bool

func init() {
	electrumURL, ok := gocore.Config().Get("electrumUrl")
	if !ok {
		logger.Fatal("Must have an electrumUrl host setting")
	}

	var err error
	// electrumClient, err = electrum.New(electrumURL)
	wrappedEleC = &wrappedElectrumClient{}
	wrappedEleC.electrumClient, err = electrum.New(electrumURL)

	if err != nil {
		logger.Errorf("Failed to create electrum client ", err)
	}

	// To keep the connection alive we need to schedule the ping
	ticker := time.NewTicker(60 * time.Second)
	go func() {
		for range ticker.C {
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

	bitcoinClient, err = bitcoin.New()
	if err != nil {
		logger.Errorf("Failed to create bitcoinClient -", err)
		return
	}

	taalBitcoinProxyEnabled = gocore.Config().GetBool("taalBitcoinProxyEnabled", false)

	isMainnet = gocore.Config().GetBool("isMainnet", true)
	network, _ = gocore.Config().Get("network")

	elasticSearchEnabled = gocore.Config().GetBool("opReturnSearch", false)
	if !elasticSearchEnabled {
		// op return search not enabled. return
		logger.Info("opReturnSearch (Elastic search) disabled in settings")
	}

	nonFinalMempoolSearchEnabled = gocore.Config().GetBool("nonFinalMempoolSearchEnabled", true)
	if !nonFinalMempoolSearchEnabled {
		logger.Info("nonFinalMempoolSearchEnabled disabled in settings")
	}

	utxoStorePriorityForUI = gocore.Config().GetBool("utxoStorePriorityForUI", true)

}

func GetMaxVinCountForProcessing() int64 {
	//TODO: move to settings
	return 500
}

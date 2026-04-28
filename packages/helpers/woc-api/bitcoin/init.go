package bitcoin

import (
	"github.com/ordishs/gocore"
)

var logger = gocore.Log("woc-api")

var taalBitcoinProxyEnabled bool
var tApiURL string
var tApiKey string

func init() {

	taalBitcoinProxyEnabled = gocore.Config().GetBool("taalBitcoinProxyEnabled", false)

	if taalBitcoinProxyEnabled {
		logger.Info("Taal node proxy enabled")
	} else {
		logger.Info("Taal node proxy disabled")
	}

	var ok bool
	tApiURL, ok = gocore.Config().Get("taalTapiURL")
	if !ok {
		logger.Fatal("Error: No taalTapiURL settings in config")
	}

	tApiKey, ok = gocore.Config().Get("taalTapiKey")
	if !ok {
		logger.Fatal("Error: No taalTapiKey settings in config")
	}
}

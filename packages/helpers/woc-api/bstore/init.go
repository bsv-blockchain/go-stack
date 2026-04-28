package bstore

import (
	"github.com/ordishs/gocore"
	"github.com/teranode-group/woc-api/bitcoin"
)

var logger = gocore.Log("woc-api")
var bstoreEnabled bool
var isMainnet bool
var bstoreBadgerVersion bool
var nodeAsBackupEnabled bool
var bitcoinClient *bitcoin.Client
var testNewBlockTxStatus bool
var bstoreGrpcMaxCallRecvMsgSizeMB int

func init() {
	bstoreEnabled = gocore.Config().GetBool("bstoreEnabled", true)

	if bstoreEnabled {
		logger.Info("bStore is Enabled")
	} else {
		logger.Info("bStore is Disabled")
	}

	isMainnet = gocore.Config().GetBool("isMainnet", true)

	if isMainnet {
		logger.Info("bStore Address format is set to mainnet")
	} else {
		logger.Info("bStore Address format is set to test")
	}

	bstoreBadgerVersion = gocore.Config().GetBool("bstoreBadgerVersion", true)

	if bstoreEnabled {
		logger.Info("bStore configured to use  BadgerDB")
	} else {
		logger.Info("bStore configured to NOT use  BadgerDB")
	}

	nodeAsBackupEnabled = gocore.Config().GetBool("bstoreNodeAsBackupEnabled", true)

	var err error
	bitcoinClient, err = bitcoin.New()
	if err != nil {
		logger.Errorf("Failed to create new bitcoinClient -", err)
		return
	}

	testNewBlockTxStatus = gocore.Config().GetBool("bstoreBlockTxStatusCheckEnabled", false)

	bstoreGrpcMaxCallRecvMsgSizeMB, _ = gocore.Config().GetInt("bstoreGrpcMaxCallRecvMsgSize_mb", 500)

}

func IsEnabled() bool {
	return bstoreEnabled
}

// Used to get the correct address format.
func IsMainnet() bool {
	return isMainnet
}

func NewBlockTxStatusTestEnabled() bool {
	return testNewBlockTxStatus
}

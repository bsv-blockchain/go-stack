package configs

import (
	"github.com/teranode-group/common/utils/gocorehelper"
)

var Settings Config

type Config struct {
	FiberPort                     int
	LogLevel                      string
	PrettifyLog                   bool
	WocStatsAddress               string
	WocStatsEnabled               bool
	WocExchangeRateAddress        string
	WocExchangeRateEnabled        bool
	BlockHeadersPath              string
	BlockHeadersFileUrl           string
	BlockHeadersSaveEnabled       bool
	BlockHeadersSaveLatestEnabled bool
	BlockHeadersSaveToFileTimer   int
	P2pServiceAddress             string
	P2pServiceEnabled             bool
	HomePageStatsCacheExpiry      int
	HomePageStatsCacheEnabled     bool
	WocMerkleServiceEnabled       bool
	WocMerkleServiceAddress       string
	IsMainnet                     bool
	Network                       string
	FeeUnit                       string
	FeeRate                       int
	MinFee                        int
	WocStatsMcpClientUrl          string
	ProfilerAddress               string
}

func Load() error {
	return gocorehelper.ParseAndValidate(&Settings)
}

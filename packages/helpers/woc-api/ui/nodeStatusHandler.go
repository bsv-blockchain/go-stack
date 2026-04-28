package ui

import (
	"encoding/json"
	"net/http"
)

type nodeStatusData struct {
	Chain                string  `json:"chain"`
	Version              int     `json:"version"`
	SubVersion           string  `json:"subversion"`
	ProtocolVersion      int     `json:"protocolversion"`
	Pruned               bool    `json:"pruned"`
	Blocks               int32   `json:"blocks"`
	Headers              int32   `json:"headers"`
	Difficulty           float64 `json:"difficulty"`
	Connections          int     `json:"connections"`
	TotalBytesRecv       int     `json:"totalbytesrecv"`
	TotalBytesSent       int     `json:"totalbytessent"`
	Uptime               uint64  `json:"uptime"`
	VerificationProgress float64 `json:"verificationprogress,omitempty"`

	Warnings string `json:"warnings"`
}

// GetNodeStatus returns node status data
func GetNodeStatus(w http.ResponseWriter, r *http.Request) {
	var err error

	nsd := nodeStatusData{}

	// use goroutines to get info in parallel
	bci, err := bitcoinClient.GetBlockchainInfo()
	if err != nil {
		logger.Errorf("GetBlockchainInfo %+v\n", err)
		return
	}

	uptime, err := bitcoinClient.Uptime()
	if err != nil {
		logger.Errorf("GetUptime %+v\n", err)
		return
	}

	totals, err := bitcoinClient.GetNetTotals()
	if err != nil {
		logger.Errorf("GetNetTotals %+v\n", err)
		return
	}

	ni, err := bitcoinClient.GetNetworkInfo()
	if err != nil {
		logger.Errorf("GetNetworkInfo %+v\n", err)
		return
	}

	nsd.Chain = bci.Chain
	nsd.Version = ni.Version
	nsd.SubVersion = ni.SubVersion
	nsd.ProtocolVersion = ni.ProtocolVersion
	nsd.Connections = ni.Connections
	nsd.Pruned = bci.Pruned
	nsd.Blocks = bci.Blocks
	nsd.Headers = bci.Headers
	nsd.Difficulty = bci.Difficulty
	nsd.TotalBytesRecv = totals.TotalBytesRecv
	nsd.TotalBytesSent = totals.TotalBytesSent
	nsd.VerificationProgress = bci.VerificationProgress

	nsd.Uptime = uptime

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(nsd)
}

package p2pservice

import (
	"context"
	"fmt"

	commongrpc "github.com/teranode-group/common/grpc"
	"github.com/teranode-group/common/logger"
	p2p_service "github.com/teranode-group/proto/p2p-service"
	"go.uber.org/zap"
)

type Status struct {
	Status         string          `json:"status"`
	ConnectedPeers int64           `json:"connectedPeers"`
	UpTime         string          `json:"upTime"`
	MainNode       *MainNodeStatus `json:"mainNode"`
}

type MainNodeStatus struct {
	IsConnected    bool   `json:"isConnected"`
	IsReconnecting bool   `json:"isReconnecting"`
	LastBlockHash  string `json:"lastBlockHash"`
	TxsCached      uint64 `json:"txsCached"`
}

func HealthCheck(ctx context.Context, address string) (*Status, error) {
	p2pServiceConnection, err := commongrpc.NewClientConnection(address, logger.Log, nil)
	if err != nil {
		errMsg := "failed to connect to p2p-service"
		logger.Log.Info(errMsg, zap.Error(err))
		return &Status{}, fmt.Errorf("%s: %s", errMsg, err.Error())
	}

	defer p2pServiceConnection.Close()

	healthCheckClient := p2p_service.NewHealthClient(p2pServiceConnection)

	clientResponse, err := healthCheckClient.Check(
		ctx,
		&p2p_service.HealthCheckRequest{
			Service: "p2p-service",
		},
	)
	if err != nil {
		errMsg := "failed to get p2p-service health check"
		logger.Log.Info(errMsg, zap.Error(err))
		return &Status{}, fmt.Errorf("%s: %s", errMsg, err.Error())
	}
	status := &Status{
		ConnectedPeers: clientResponse.ConnectedPeers,
		Status:         "OK",
	}
	if clientResponse.Map == nil {
		return status, nil
	}

	if val, ok := clientResponse.Map.Fields["upTime"]; ok {
		status.UpTime = val.GetStringValue()
	}

	mainNode, err := extractMainNodeInfo(clientResponse)
	if err != nil {
		errMsg := "failed to extract main node info"
		logger.Log.Error(errMsg, zap.Error(err))
		return &Status{}, fmt.Errorf("%s: %s", errMsg, err.Error())
	}
	status.MainNode = mainNode

	return status, nil
}

func extractMainNodeInfo(response *p2p_service.HealthCheckResponse) (*MainNodeStatus, error) {
	if response.Map == nil {
		return nil, fmt.Errorf("map is nil")
	}

	pearsStatusValue, exists := response.Map.Fields["pears_status"]
	if !exists {
		return nil, fmt.Errorf("pears_status field not found")
	}
	peersList := pearsStatusValue.GetListValue()
	if peersList == nil {
		return nil, fmt.Errorf("pears_status is not a list")
	}

	for _, peerValue := range peersList.Values {
		peerStruct := peerValue.GetStructValue()
		if peerStruct == nil {
			continue
		}

		isMain, exists := peerStruct.Fields["is_main_node"]
		if !exists || !isMain.GetBoolValue() {
			continue
		}

		node := &MainNodeStatus{}
		if txsValue, exists := peerStruct.Fields["txs_cache_stored"]; exists {
			node.TxsCached = uint64(txsValue.GetNumberValue())
		}
		if connectedValue, exists := peerStruct.Fields["is_connected"]; exists {
			node.IsConnected = connectedValue.GetBoolValue()
		}
		if reconnectingValue, exists := peerStruct.Fields["is_reconnecting"]; exists {
			node.IsReconnecting = reconnectingValue.GetBoolValue()
		}
		if hashValue, exists := peerStruct.Fields["last_block_hash"]; exists {
			node.LastBlockHash = hashValue.GetStringValue()
		}

		return node, nil
	}

	return nil, fmt.Errorf("main node info is not found")
}

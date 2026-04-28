package tokens

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/teranode-group/common"
	token_service "github.com/teranode-group/proto/token-service"
)

// - get token details
// https://api.whatsonchain.com/v1/bsv/<network>/token/1SatOrdinals/<outpoint>
// // rpc Get1SatOrdinalsTokenByID(Get1SatOrdinalsTokenByIDRequest) returns (Get1SatOrdinalsTokenByIDResponse) {}
func Get1SatOrdinalsTokenByOutpoint(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	outpoint := vars["outpoint"]

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	tsReq := &token_service.Get1SatOrdinalsTokenByIDRequest{
		Outpoint: outpoint,
	}

	tsRes, err := tokenServiceClient.Get1SatOrdinalsTokenByID(ctx, tsReq)
	if err != nil {
		if oneSatOrdinalsContainsBSV20Error(err) {
			write1SatOrdinalsNotImplementedBSV20(w)
			return
		}
		logger.Errorf("Unable to GetTokens for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	//TODO: should copy to custom type before response
	if err := json.NewEncoder(w).Encode(tsRes); err != nil {
		logger.Errorf("Could not encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// - get token details by origin
// .../token/1SatOrdinals/<origin>/origin
// rpc Get1SatOrdinalsTokenByID(Get1SatOrdinalsTokenByIDRequest) returns (Get1SatOrdinalsTokenByIDResponse) {}
func Get1SatOrdinalsTokenByID(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	origin := vars["origin"]

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	tsReq := &token_service.Get1SatOrdinalsTokenByIDRequest{
		OrdinalNumber: origin,
	}

	tsRes, err := tokenServiceClient.Get1SatOrdinalsTokenByID(ctx, tsReq)
	if err != nil {
		if oneSatOrdinalsContainsBSV20Error(err) {
			write1SatOrdinalsNotImplementedBSV20(w)
			return
		}
		logger.Errorf("Unable to GetTokens for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	//TODO: should copy to custom type before response
	if err := json.NewEncoder(w).Encode(tsRes); err != nil {
		logger.Errorf("Could not encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// - get latest transaction of the token
// .../token/1SatOrdinals/<outpoint>/latest
// rpc Get1SatOrdinalsLatest(Get1SatOrdinalsLatestRequest) returns (Get1SatOrdinalsLatestResponse) {}
func Get1SatOrdinalsLatest(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	outpoint := vars["outpoint"]

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	tsReq := &token_service.Get1SatOrdinalsLatestRequest{
		Outpoint: outpoint,
	}

	tsRes, err := tokenServiceClient.Get1SatOrdinalsLatest(ctx, tsReq)
	if err != nil {
		if oneSatOrdinalsContainsBSV20Error(err) {
			write1SatOrdinalsNotImplementedBSV20(w)
			return
		}
		logger.Errorf("Unable to GetTokens for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	//TODO: should copy to custom type before response
	if err := json.NewEncoder(w).Encode(tsRes); err != nil {
		logger.Errorf("Could not encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// - get token history
// .../token/1SatOrdinals/<outpoint>/history
// rpc Get1SatOrdinalsHistory(Get1SatOrdinalsHistoryRequest) returns (Get1SatOrdinalsHistoryResponse) {}
func Get1SatOrdinalsHistory(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	varsQuery := r.URL.Query()
	outpoint := vars["outpoint"]

	skip, err := strconv.Atoi(varsQuery.Get("skip"))
	if err != nil {
		skip = 0
	}

	limit, err := strconv.Atoi(varsQuery.Get("limit"))
	if err != nil {
		limit = 30
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	tsReq := &token_service.Get1SatOrdinalsHistoryRequest{
		Outpoint: outpoint,
		Skip:     uint64(skip),
		Limit:    uint64(limit),
	}

	tsRes, err := tokenServiceClient.Get1SatOrdinalsHistory(ctx, tsReq)
	if err != nil {
		if oneSatOrdinalsContainsBSV20Error(err) {
			write1SatOrdinalsNotImplementedBSV20(w)
			return
		}
		logger.Errorf("Unable to GetTokens for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	//TODO: should copy to custom type before response
	if err := json.NewEncoder(w).Encode(tsRes); err != nil {
		logger.Errorf("Could not encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// - get token content
// .../token/1SatOrdinals/<outpoint>/content
// rpc Get1SatOrdinalsContent(Get1SatOrdinalsContentRequest) returns (Get1SatOrdinalsContentResponse) {}
func Get1SatOrdinalsContent(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	outpoint := vars["outpoint"]

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	tsReq := &token_service.Get1SatOrdinalsContentRequest{
		Outpoint: outpoint,
	}

	tsRes, err := tokenServiceClient.Get1SatOrdinalsContent(ctx, tsReq)
	if err != nil {
		if oneSatOrdinalsContainsBSV20Error(err) {
			write1SatOrdinalsNotImplementedBSV20(w)
			return
		}
		logger.Errorf("Unable to GetTokens for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if tsRes.Type != "" {
		w.Header().Set("Content-Type", tsRes.Type)
	}
	if _, err := w.Write(tsRes.Content); err != nil {
		logger.Errorf("Could not write response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// - get token/s by tx id
// .../token/1SatOrdinals/tx/<txid>
// rpc Get1SatOrdinalsTokensByTxID(Get1SatOrdinalsTokensByTxIDRequest) returns (Get1SatOrdinalsTokensByTxIDResponse) {}
func Get1SatOrdinalsTokensByTxID(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	varsQuery := r.URL.Query()
	txid := vars["txid"]

	if len(txid) != 64 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	skip, err := strconv.Atoi(varsQuery.Get("skip"))
	if err != nil {
		skip = 0
	}

	limit, err := strconv.Atoi(varsQuery.Get("limit"))
	if err != nil {
		limit = 30
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	tsReq := &token_service.Get1SatOrdinalsTokensByTxIDRequest{
		Txid:  txid,
		Skip:  uint64(skip),
		Limit: uint64(limit),
	}

	tsRes, err := tokenServiceClient.Get1SatOrdinalsTokensByTxID(ctx, tsReq)
	if err != nil {
		if oneSatOrdinalsContainsBSV20Error(err) {
			write1SatOrdinalsNotImplementedBSV20(w)
			return
		}
		logger.Errorf("Unable to GetTokens for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	//TODO: should copy to custom type before response
	if err := json.NewEncoder(w).Encode(tsRes); err != nil {
		logger.Errorf("Could not encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// - 1Sat Ordinals Info
// .../token/1SatOrdinals
// rpc Get1SatOrdinalsProtocolInfo(Get1SatOrdinalsProtocolInfoRequest) returns (Get1SatOrdinalsProtocolInfoResponse) {}
// GetStasTokenTxCount :
func Get1SatOrdinalsProtocolInfo(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	tsReq := &token_service.Get1SatOrdinalsProtocolInfoRequest{}

	tsRes, err := tokenServiceClient.Get1SatOrdinalsProtocolInfo(ctx, tsReq)
	if err != nil {
		logger.Errorf("Unable to GetTokens for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	//TODO: should copy to custom type before response
	if err := json.NewEncoder(w).Encode(tsRes); err != nil {
		logger.Errorf("Could not encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// TODO: remove when BSV-20/21 will be supported on token-service
func oneSatOrdinalsContainsBSV20Error(err error) bool {
	bsv20NotImplementedError := "Not Implemented - BSV20"
	return strings.Contains(err.Error(), bsv20NotImplementedError)
}

// TODO: remove when BSV-20/21 will be supported on token-service
func write1SatOrdinalsNotImplementedBSV20(w http.ResponseWriter) {
	resp := struct {
		Message string `json:"message"`
	}{
		Message: "The token indexer only provides data for 1Sat ordinals NFTs, at the moment, not BSV-20/21 fungible tokens. We’re working on adding them.",
	}

	respBt, err := json.Marshal(resp)
	//TODO: should copy to custom type before response
	if err != nil {
		logger.Errorf("Could not encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.Error(w, string(respBt), http.StatusNotImplemented)
}

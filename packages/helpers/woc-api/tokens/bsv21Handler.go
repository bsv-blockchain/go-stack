package tokens

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/teranode-group/common"
	token_service "github.com/teranode-group/proto/token-service"
)

func GetBSV21AddressBalance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	varsQuery := r.URL.Query()
	addr := vars["address"]

	skip, err := strconv.Atoi(varsQuery.Get("skip"))
	if err != nil {
		skip = 0
	}

	limit, err := strconv.Atoi(varsQuery.Get("limit"))
	if err != nil {
		limit = defaultItemsLimit
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("failed connecting %s %+v", tokenServiceName, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tsReq := &token_service.GetBSV21AddressBalanceRequest{
		Address:       addr,
		FilterMempool: varsQuery.Get("filterMempool"),
		Skip:          uint64(skip),
		Limit:         uint64(limit),
	}

	tokenSrvCli := token_service.NewTokenServiceClient(conn)
	itemsRes, err := tokenSrvCli.GetBSV21AddressBalance(ctx, tsReq)
	if err != nil {
		if oneSatOrdinalsContainsBSV20Error(err) {
			write1SatOrdinalsNotImplementedBSV20(w)
			return
		}
		logger.Errorf("failed calling GetBSV21AddressBalance for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(itemsRes); err != nil {
		logger.Errorf("failed to encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func GetBSV21Depth(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	varsQuery := r.URL.Query()
	addr := vars["address"]
	outpoint := vars["id"]

	skip, err := strconv.Atoi(varsQuery.Get("skip"))
	if err != nil {
		skip = 0
	}

	limit, err := strconv.Atoi(varsQuery.Get("limit"))
	if err != nil {
		limit = defaultItemsLimit
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("failed connecting %s %+v", tokenServiceName, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tsReq := &token_service.GetBSV21DepthRequest{
		Address:  addr,
		Outpoint: outpoint,
		Skip:     uint64(skip),
		Limit:    uint64(limit),
	}

	tokenSrvCli := token_service.NewTokenServiceClient(conn)
	itemsRes, err := tokenSrvCli.GetBSV21Depth(ctx, tsReq)
	if err != nil {
		if oneSatOrdinalsContainsBSV20Error(err) {
			write1SatOrdinalsNotImplementedBSV20(w)
			return
		}
		logger.Errorf("failed calling GetBSV21Depth for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(itemsRes); err != nil {
		logger.Errorf("failed to encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func GetBSV21HistoryByAddress(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	varsQuery := r.URL.Query()
	addr := vars["address"]
	outpoint := vars["id"]

	skip, err := strconv.Atoi(varsQuery.Get("skip"))
	if err != nil {
		skip = 0
	}

	limit, err := strconv.Atoi(varsQuery.Get("limit"))
	if err != nil {
		limit = defaultItemsLimit
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("failed connecting %s %+v", tokenServiceName, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tsReq := &token_service.GetBSV21HistoryByAddressRequest{
		Address:  addr,
		Outpoint: outpoint,
		Skip:     uint64(skip),
		Limit:    uint64(limit),
	}

	tokenSrvCli := token_service.NewTokenServiceClient(conn)
	itemsRes, err := tokenSrvCli.GetBSV21HistoryByAddress(ctx, tsReq)
	if err != nil {
		if oneSatOrdinalsContainsBSV20Error(err) {
			write1SatOrdinalsNotImplementedBSV20(w)
			return
		}
		logger.Errorf("failed calling GetBSV21HistoryByAddress for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(itemsRes); err != nil {
		logger.Errorf("failed to encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func GetBSV21Inscription(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	outpoint := vars["outpoint"]

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("failed connecting %s %+v", tokenServiceName, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tsReq := &token_service.GetBSV21InscriptionRequest{
		Outpoint: outpoint,
	}

	tokenSrvCli := token_service.NewTokenServiceClient(conn)
	ins, err := tokenSrvCli.GetBSV21Inscription(ctx, tsReq)
	if err != nil {
		if oneSatOrdinalsContainsBSV20Error(err) {
			write1SatOrdinalsNotImplementedBSV20(w)
			return
		}
		logger.Errorf("failed calling GetBSV21Inscription for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(ins); err != nil {
		logger.Errorf("failed to encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func GetBsv21TokenByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	outpoint := vars["id"]

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("failed connecting %s %+v", tokenServiceName, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tsReq := &token_service.GetBSV21TokenByIDRequest{
		Outpoint: outpoint,
	}

	tokenSrvCli := token_service.NewTokenServiceClient(conn)
	tsRes, err := tokenSrvCli.GetBSV21TokenByID(ctx, tsReq)
	if err != nil {
		if oneSatOrdinalsContainsBSV20Error(err) {
			write1SatOrdinalsNotImplementedBSV20(w)
			return
		}
		logger.Errorf("failed calling GetBSV21TokenByID for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(tsRes); err != nil {
		logger.Errorf("failed to encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func GetBsv21TokenOwners(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	varsQuery := r.URL.Query()
	outpoint := vars["id"]

	skip, err := strconv.Atoi(varsQuery.Get("skip"))
	if err != nil {
		skip = 0
	}

	limit, err := strconv.Atoi(varsQuery.Get("limit"))
	if err != nil {
		limit = defaultItemsLimit
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("failed connecting %s %+v", tokenServiceName, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tsReq := &token_service.GetBSV21TokenOwnersRequest{
		Outpoint: outpoint,
		Skip:     uint64(skip),
		Limit:    uint64(limit),
	}

	tokenSrvCli := token_service.NewTokenServiceClient(conn)
	tsRes, err := tokenSrvCli.GetBSV21TokenOwners(ctx, tsReq)
	if err != nil {
		if oneSatOrdinalsContainsBSV20Error(err) {
			write1SatOrdinalsNotImplementedBSV20(w)
			return
		}
		logger.Errorf("failed calling GetBSV21TokenOwners for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(tsRes); err != nil {
		logger.Errorf("failed to encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func GetBsv21TokensByTxid(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	varsQuery := r.URL.Query()
	txid := vars["txid"]

	skip, err := strconv.Atoi(varsQuery.Get("skip"))
	if err != nil {
		skip = 0
	}

	limit, err := strconv.Atoi(varsQuery.Get("limit"))
	if err != nil {
		limit = defaultItemsLimit
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("failed connecting %s %+v", tokenServiceName, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tsReq := &token_service.GetBSV21TokensByTxIDRequest{
		Txid:  txid,
		Skip:  uint64(skip),
		Limit: uint64(limit),
	}

	tokenSrvCli := token_service.NewTokenServiceClient(conn)
	tsRes, err := tokenSrvCli.GetBSV21TokensByTxID(ctx, tsReq)
	if err != nil {
		if oneSatOrdinalsContainsBSV20Error(err) {
			write1SatOrdinalsNotImplementedBSV20(w)
			return
		}
		logger.Errorf("failed calling GetBSV21TokensByTxID for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(tsRes); err != nil {
		logger.Errorf("failed to encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func GetBSV21TxSpent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	varsQuery := r.URL.Query()
	txid := vars["txid"]

	skip, err := strconv.Atoi(varsQuery.Get("skip"))
	if err != nil {
		skip = 0
	}

	limit, err := strconv.Atoi(varsQuery.Get("limit"))
	if err != nil {
		limit = defaultItemsLimit
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("failed connecting %s %+v", tokenServiceName, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tsReq := &token_service.GetBSV21TxSpentRequest{
		Txid:  txid,
		Skip:  uint64(skip),
		Limit: uint64(limit),
	}

	tokenSrvCli := token_service.NewTokenServiceClient(conn)
	itemsRes, err := tokenSrvCli.GetBSV21TxSpent(ctx, tsReq)
	if err != nil {
		if oneSatOrdinalsContainsBSV20Error(err) {
			write1SatOrdinalsNotImplementedBSV20(w)
			return
		}
		logger.Errorf("failed calling GetBSV21TxSpent for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(itemsRes); err != nil {
		logger.Errorf("failed to encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func GetBsv21UnspentByAddress(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	varsQuery := r.URL.Query()
	addr := vars["address"]

	skip, err := strconv.Atoi(varsQuery.Get("skip"))
	if err != nil {
		skip = 0
	}

	limit, err := strconv.Atoi(varsQuery.Get("limit"))
	if err != nil {
		limit = defaultItemsLimit
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, tokenServiceName)
	if err != nil {
		logger.Errorf("failed connecting %s %+v", tokenServiceName, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tsReq := &token_service.GetBSV21TokensByAddressRequest{
		Address:   addr,
		IsUnspent: true,
		Skip:      uint64(skip),
		Limit:     uint64(limit),
	}

	tokenSrvCli := token_service.NewTokenServiceClient(conn)
	tsRes, err := tokenSrvCli.GetBSV21TokensByAddress(ctx, tsReq)
	if err != nil {
		if oneSatOrdinalsContainsBSV20Error(err) {
			write1SatOrdinalsNotImplementedBSV20(w)
			return
		}
		logger.Errorf("failed calling GetBSV21TokensByAddress for: %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(tsRes); err != nil {
		logger.Errorf("failed to encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

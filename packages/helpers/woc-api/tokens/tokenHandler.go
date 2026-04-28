package tokens

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/ordishs/gocore"
	"github.com/teranode-group/common"
	token_service "github.com/teranode-group/proto/token-service"
	"github.com/teranode-group/woc-api/bstore"
)

var logger = gocore.Log("woc-api")

type TokenInfo struct {
	Protocol     string `json:"protocol"`
	RedeemAddr   string `json:"redeemAddr"`
	Symbol       string `json:"symbol"`
	Image        string `json:"image"`
	Balance      int64  `json:"balance"`
	TokenBalance int64  `json:"tokenBalance"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	IsFungible   bool   `json:"isFungible"`
}

type AddresssTokenBalanceResponse struct {
	Address string      `json:"address"`
	Tokens  []TokenInfo `json:"tokens"`
}

func GetAddressTokensHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]

	wait, _ := time.ParseDuration("10s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, "tokenService")
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	tsReq := &token_service.GetBalanceByAddressRequest{
		Address: address,
	}

	tsRes, err := tokenServiceClient.GetBalanceByAddress(ctx, tsReq)
	if err != nil {
		logger.Errorf("Unable to GetBalanceByAddress for %s: %+v", address, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var tokens []TokenInfo

	for _, currentToken := range tsRes.Tokens {

		req := &token_service.GetByRedeemAddrAndSymbolRequest{
			RedeemAddr: currentToken.RedeemAddr,
			Symbol:     currentToken.Symbol,
		}

		//Get token by id
		resp, err := tokenServiceClient.GetTokenByRedeemAddrAndSymbol(ctx, req)
		if err != nil {
			logger.Errorf("GetAddressTokensHandler - Unable to GetTokenByRedeemAddr %s AndSymbol %s: %+v", currentToken.RedeemAddr, currentToken.Symbol, err)
			continue
		}

		var tokenBalance int64
		if resp.Token.SatsPerToken > 0 {
			tokenBalance = currentToken.Balance / resp.Token.SatsPerToken
		}

		token := TokenInfo{
			Protocol:     resp.Token.Protocol,
			RedeemAddr:   resp.Token.TokenId,
			Symbol:       resp.Token.Symbol,
			Image:        resp.Token.Image,
			Balance:      currentToken.Balance,
			TokenBalance: tokenBalance,
			Name:         resp.Token.Name,
			Description:  resp.Token.Description,
			IsFungible:   resp.Token.IsFungible,
		}
		tokens = append(tokens, token)
	}

	resp := AddresssTokenBalanceResponse{
		Address: tsRes.Address,
		Tokens:  tokens,
	}

	//TODO: should copy to custom type before response
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Errorf("Could not encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

type bulkAddressesTokenResponse struct {
	Address string      `json:"address"`
	Tokens  []TokenInfo `json:"tokens"`
	Error   string      `json:"error"`
}

func PostBulkTokensByAddress(w http.ResponseWriter, r *http.Request) {
	var reqBody bulkAddressesTokenRequest
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &reqBody)
	addrs := unique(reqBody.Addresses)

	if len(addrs) > 20 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var response [20]bulkAddressesTokenResponse

	wait, _ := time.ParseDuration("200s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, "tokenService")
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	for index, address := range addrs {
		response[index].Address = address

		tsReq := &token_service.GetBalanceByAddressRequest{
			Address: address,
		}

		tsRes, err := tokenServiceClient.GetBalanceByAddress(ctx, tsReq)
		if err != nil {
			logger.Errorf("unable to GetBalanceByAddress for %s: %+v", address, err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var tokens []TokenInfo

		for _, currentToken := range tsRes.Tokens {
			req := &token_service.GetByRedeemAddrAndSymbolRequest{
				RedeemAddr: currentToken.RedeemAddr,
				Symbol:     currentToken.Symbol,
			}

			// Get token by id
			resp, err := tokenServiceClient.GetTokenByRedeemAddrAndSymbol(ctx, req)
			if err != nil {
				logger.Errorf("GetAddressTokensHandler - Unable to GetTokenByRedeemAddr %s AndSymbol %s: %+v", currentToken.RedeemAddr, currentToken.Symbol, err)
				continue
			}

			var tokenBalance int64
			if resp.Token.SatsPerToken > 0 {
				tokenBalance = currentToken.Balance / resp.Token.SatsPerToken
			}

			token := TokenInfo{
				Protocol:     resp.Token.Protocol,
				RedeemAddr:   resp.Token.TokenId,
				Symbol:       resp.Token.Symbol,
				Image:        resp.Token.Image,
				Balance:      currentToken.Balance,
				TokenBalance: tokenBalance,
			}
			tokens = append(tokens, token)
		}
		response[index].Tokens = tokens
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response[0:len(addrs)])

	//TODO: should copy to custom type before response
	// if err := json.NewEncoder(w).Encode(resp); err != nil {
	// 	logger.Errorf("Error: Could not encode response - %+v", err)
	// 	w.WriteHeader(http.StatusInternalServerError)
	// }
}

type TokenUtxo struct {
	RedeemAddr string  `json:"redeemAddr"`
	Symbol     string  `json:"symbol"`
	Txid       string  `json:"txid"`
	Index      int64   `json:"index"`
	Amount     int64   `json:"amount"`
	Script     *string `json:"script,omitempty"`
}

type AddresssTokenUnspentResponse struct {
	Address string      `json:"address"`
	Utxos   []TokenUtxo `json:"utxos"`
}

func GetAddressUnspentTokensHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]
	includeScriptStr := strings.ToLower(r.URL.Query().Get("script"))
	includeScript, _ := strconv.ParseBool(includeScriptStr)

	wait, _ := time.ParseDuration("10s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, "tokenService")
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	tsReq := &token_service.GetUtxosByAddressRequest{
		Address: address,
	}

	tsRes, err := tokenServiceClient.GetUtxosByAddress(ctx, tsReq)
	if err != nil {
		logger.Errorf("Unable to GetUtxosByAddress for %s: %+v", address, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var utxos []TokenUtxo

	for _, currentUtxo := range tsRes.Utxos {
		utxo := TokenUtxo{
			RedeemAddr: currentUtxo.RedeemAddr,
			Symbol:     currentUtxo.Symbol,
			Txid:       currentUtxo.Txid,
			Index:      currentUtxo.Index,
			Amount:     currentUtxo.Value,
		}

		if includeScript {
			script, err := bstore.GetTxVoutHex(currentUtxo.Txid, currentUtxo.Index)

			if err == nil {
				utxo.Script = script
			} else {
				logger.Errorf("Unable to GetTxVoutHex for  %s, %+v: %+v", currentUtxo.Txid, currentUtxo.Index, err)
			}
		}

		utxos = append(utxos, utxo)

	}

	resp := AddresssTokenUnspentResponse{
		Address: tsRes.Address,
		Utxos:   utxos,
	}

	//TODO: should copy to custom type before response
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Errorf("Could not encode response - %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

type bulkAddressesTokenRequest struct {
	Addresses []string `json:"addresses"`
}

type bulkAddressesUnspentTokenResponse struct {
	Address string      `json:"address"`
	Utxos   []TokenUtxo `json:"utxos"`
	Error   string      `json:"error"`
}

func PostBulkUnspentTokensByAddress(w http.ResponseWriter, r *http.Request) {
	var reqBody bulkAddressesTokenRequest
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &reqBody)
	addrs := unique(reqBody.Addresses)

	if len(addrs) > 20 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var response [20]bulkAddressesUnspentTokenResponse

	wait, _ := time.ParseDuration("200s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, "tokenService")
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	for index, address := range addrs {
		response[index].Address = address

		tsReq := &token_service.GetUtxosByAddressRequest{
			Address: address,
		}

		tsRes, err := tokenServiceClient.GetUtxosByAddress(ctx, tsReq)
		if err != nil {
			response[index].Error = "Failed to get Unspent data"
			continue
		}

		var utxos []TokenUtxo

		for _, currentUtxo := range tsRes.Utxos {
			utxo := TokenUtxo{
				RedeemAddr: currentUtxo.RedeemAddr,
				Symbol:     currentUtxo.Symbol,
				Txid:       currentUtxo.Txid,
				Index:      currentUtxo.Index,
				Amount:     currentUtxo.Value,
			}
			utxos = append(utxos, utxo)
		}
		response[index].Utxos = utxos

	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response[0:len(addrs)])

	//TODO: should copy to custom type before response
	// if err := json.NewEncoder(w).Encode(resp); err != nil {
	// 	logger.Errorf("Error: Could not encode response - %+v", err)
	// 	w.WriteHeader(http.StatusInternalServerError)
	// }
}

// GetTokensHandler :
func GetTokensHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	skip, err := strconv.Atoi(vars["skip"])
	if err != nil {
		skip = 0
	}

	limit, err := strconv.Atoi(vars["limit"])
	if err != nil {
		limit = 30
	}
	wait, _ := time.ParseDuration("10s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, "tokenService")
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	tsReq := &token_service.GetTokensRequest{
		Skip:  uint64(skip),
		Limit: uint64(limit),
	}

	tsRes, err := tokenServiceClient.GetTokens(ctx, tsReq)
	if err != nil {
		logger.Errorf("Unable to GetTokens %+v", err)
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

// GetTokenByID :
func GetTokenByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	redeemAddr := vars["redeemAddr"]
	symbol := vars["symbol"]

	//TODO: validate tokenID

	wait, _ := time.ParseDuration("10s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, "tokenService")
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)
	tsReq := &token_service.GetByRedeemAddrAndSymbolRequest{
		RedeemAddr: redeemAddr,
		Symbol:     symbol,
	}

	tsRes, err := tokenServiceClient.GetTokenByRedeemAddrAndSymbol(ctx, tsReq)
	if err != nil {
		logger.Errorf("Unable to GetTokens for redeemAddr %s, symbol %s: %+v", redeemAddr, symbol, err)
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

// GetTxByTokenID :
func GetTokenTxVout(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txid := vars["txid"]

	if len(txid) != 64 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	indexStr := vars["index"]
	if len(indexStr) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	index, err := strconv.ParseInt(indexStr, 10, 64)
	if err != nil || index < 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	wait, _ := time.ParseDuration("10s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, "tokenService")
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	tsReq := &token_service.GetTokenVoutRequest{
		Txid:  txid,
		Index: index,
	}

	tsRes, err := tokenServiceClient.GetTokenVout(ctx, tsReq)
	if err != nil {
		logger.Errorf("Unable to GetTokenVout for %s / %d", txid, index)
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

func GetTxByTokenID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	redeemAddr := vars["redeemAddr"]
	symbol := vars["symbol"]

	skip, err := strconv.Atoi(vars["skip"])
	if err != nil {
		skip = 0
	}

	limit, err := strconv.Atoi(vars["limit"])
	if err != nil {
		limit = 30
	}

	//TODO: validate tokenID

	wait, _ := time.ParseDuration("10s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, "tokenService")
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	tsReq := &token_service.GetByRedeemAddrAndSymbolRequest{
		RedeemAddr: redeemAddr,
		Symbol:     symbol,
		Skip:       uint64(skip),
		Limit:      uint64(limit),
	}

	tsRes, err := tokenServiceClient.GetTxByRedeemAddrAndSymbol(ctx, tsReq)
	if err != nil {
		logger.Errorf("Unable to GetTokens for redeemAddr %s, symbol %s: %+v", redeemAddr, symbol, err)
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

// GetStasTokenTxCount :
func GetStasProtocolInfo(w http.ResponseWriter, r *http.Request) {

	wait, _ := time.ParseDuration("10s")
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, "tokenService")
	if err != nil {
		logger.Errorf("Unable to connect tokenService %+v", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer conn.Close()

	tokenServiceClient := token_service.NewTokenServiceClient(conn)

	tsReq := &token_service.GetStasProtocolInfoRequest{}

	tsRes, err := tokenServiceClient.GetStasProtocolInfo(ctx, tsReq)
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

// Returns list with unique values
func unique(strSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range strSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

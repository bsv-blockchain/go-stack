package bitcoin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	gobitcoin "github.com/ordishs/go-bitcoin"
)

type taalNodeProxyRequest struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

type taalNodeRawTxProxyResponse struct {
	Result *gobitcoin.RawTransaction `json:"result"`
}

//send commad to taal nodes using proxy TAPI
func GetRawTransactionFromTaalNode(txID string) (rawTx *gobitcoin.RawTransaction, err error) {

	if !taalBitcoinProxyEnabled {
		return nil, fmt.Errorf("Error: GetRawTransactionFromTaalNode - taalBitcoinProxy is disabled in config")
	}

	if len(txID) != 64 {
		return nil, fmt.Errorf("Error: GetRawTransactionFromTaalNode - Invalid txID: %v", txID)
	}

	tApiClient := http.Client{
		Timeout: time.Second * 10,
	}

	payloadBuffer := &bytes.Buffer{}
	jsonEncoder := json.NewEncoder(payloadBuffer)
	postData := taalNodeProxyRequest{Method: "getrawtransaction", Params: []interface{}{txID, 1}}
	err = jsonEncoder.Encode(postData)
	if err != nil {
		logger.Errorf("Failed while creating payload %v,%v", postData, err)
		return nil, fmt.Errorf("Error: GetRawTransactionFromTaalNode - Failed while creating payload")
	}

	req, err := http.NewRequest(http.MethodPost, tApiURL, payloadBuffer)
	if err != nil {
		logger.Errorf("Failed to create request %v,%v", req, err)
		return nil, fmt.Errorf("Error: GetRawTransactionFromTaalNode - Failed to create request")
	}

	req.Header.Add("Authorization", tApiKey)
	req.Header.Add("Content-Type", "application/json")

	res, err := tApiClient.Do(req)
	if err != nil {
		logger.Errorf("Failed on http call as a tApiClient %v", err)
		return nil, fmt.Errorf("Error: GetRawTransactionFromTaalNode - Failed on http call as a tApiClient")
	}

	bodyBytes, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		logger.Errorf("Failed on read body %v", readErr)
		return nil, fmt.Errorf("Error: GetRawTransactionFromTaalNode - Failed on read body")
	}

	var bodyJSON taalNodeRawTxProxyResponse

	err = json.Unmarshal([]byte(bodyBytes), &bodyJSON)
	if err != nil {
		logger.Errorf("Failed on json.Unmarshal %v", err)
		return nil, fmt.Errorf("Error: GetRawTransactionFromTaalNode - Failed on json.Unmarshal")
	}

	if bodyJSON.Result == nil || len(bodyJSON.Result.Hex) == 0 {
		return nil, fmt.Errorf("Error: GetRawTransactionFromTaalNode - Not found")
	}

	return bodyJSON.Result, nil

}

type taalNodeTxListProxyResponse struct {
	Result []string `json:"result"`
}

func GetMempoolAncestorsFromTaalNode(txID string) (txList []string, err error) {

	if !taalBitcoinProxyEnabled {
		return nil, fmt.Errorf("Error: GetMempoolAncestorsFromTaalNode - taalBitcoinProxy is disabled in config")
	}

	if len(txID) != 64 {
		return nil, fmt.Errorf("Error: GetMempoolAncestorsFromTaalNode - Invalid txID: %v", txID)
	}

	tApiClient := http.Client{
		Timeout: time.Second * 10,
	}

	payloadBuffer := &bytes.Buffer{}
	jsonEncoder := json.NewEncoder(payloadBuffer)
	postData := taalNodeProxyRequest{Method: "getmempoolancestors", Params: []interface{}{txID, false}}
	err = jsonEncoder.Encode(postData)
	if err != nil {
		logger.Errorf("Failed while creating payload %v,%v", postData, err)
		return nil, fmt.Errorf("Error: GetMempoolAncestorsFromTaalNode - Failed while creating payload")
	}

	req, err := http.NewRequest(http.MethodPost, tApiURL, payloadBuffer)
	if err != nil {
		logger.Errorf("Failed to create request %v,%v", req, err)
		return nil, fmt.Errorf("Error: GetMempoolAncestorsFromTaalNode - Failed to create request")
	}

	req.Header.Add("Authorization", tApiKey)
	req.Header.Add("Content-Type", "application/json")

	res, err := tApiClient.Do(req)
	if err != nil {
		logger.Errorf("Failed on http call as a tApiClient %v", err)
		return nil, fmt.Errorf("Error: GetMempoolAncestorsFromTaalNode - Failed on http call as a tApiClient")
	}

	bodyBytes, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		logger.Errorf("Failed on read body %v", readErr)
		return nil, fmt.Errorf("Error: GetMempoolAncestorsFromTaalNode - Failed on read body")
	}

	var bodyJSON taalNodeTxListProxyResponse
	err = json.Unmarshal([]byte(bodyBytes), &bodyJSON)
	if err != nil {
		logger.Errorf("Failed on json.Unmarshal %v", err)
		return nil, fmt.Errorf("Error: GetMempoolAncestorsFromTaalNode - Failed on json.Unmarshal")
	}
	return bodyJSON.Result, nil

}
func GetMempoolDescendantsFromTaalNode(txID string) (txList []string, err error) {

	if !taalBitcoinProxyEnabled {
		return nil, fmt.Errorf("Error: GetMempoolDescendantsFromTaalNode - taalBitcoinProxy is disabled in config")
	}

	if len(txID) != 64 {
		return nil, fmt.Errorf("Error: GetMempoolDescendantsFromTaalNode - Invalid txID: %v", txID)
	}

	tApiClient := http.Client{
		Timeout: time.Second * 10,
	}

	payloadBuffer := &bytes.Buffer{}
	jsonEncoder := json.NewEncoder(payloadBuffer)
	postData := taalNodeProxyRequest{Method: "getmempooldescendants", Params: []interface{}{txID, false}}
	err = jsonEncoder.Encode(postData)
	if err != nil {
		logger.Errorf("Failed while creating payload %v,%v", postData, err)
		return nil, fmt.Errorf("Error: GetMempoolDescendantsFromTaalNode -  Failed while creating payload")
	}

	req, err := http.NewRequest(http.MethodPost, tApiURL, payloadBuffer)
	if err != nil {
		logger.Errorf("Failed to create request %v,%v", req, err)
		return nil, fmt.Errorf("Error: GetMempoolDescendantsFromTaalNode - Failed to create request")
	}

	req.Header.Add("Authorization", tApiKey)
	req.Header.Add("Content-Type", "application/json")

	res, err := tApiClient.Do(req)
	if err != nil {
		logger.Errorf("Failed on http call as a tApiClient %v", err)
		return nil, fmt.Errorf("Error: GetMempoolDescendantsFromTaalNode - Failed on http call as a tApiClient")
	}

	bodyBytes, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		logger.Errorf("Failed on read body %v", readErr)
		return nil, fmt.Errorf("Error: GetMempoolDescendantsFromTaalNode - Failed on read body")
	}

	var bodyJSON taalNodeTxListProxyResponse
	err = json.Unmarshal([]byte(bodyBytes), &bodyJSON)
	if err != nil {
		logger.Errorf("Failed on json.Unmarshal %v", err)
		return nil, fmt.Errorf("Error: GetMempoolDescendantsFromTaalNode - Failed on json.Unmarshal")
	}

	return bodyJSON.Result, nil

}

package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var ep string = "https://api.whatsonchain.com/v1/bsv/test/tx/hash/"
var txHash string = "e76e42509f98409ca2174524747ba42850b41d3a6e2aceccb7afcc165a05b9cd"
var txHashInvalid string = "invalid"

func TestGetTransactionHash(t *testing.T) {

	url := fmt.Sprintf("%v/%v", ep, txHash)
	method := "GET"

	res, body := httpRequest(url, method)

	assert.Equal(t, 200, res.StatusCode)
	assert.Contains(t, string(body), "{\"txid\":\"e76e42509f98409ca2174524747ba42850b41d3a6e2aceccb7afcc165a05b9cd\"")

}

func TestTransactionHash_Invalid(t *testing.T) {

	url := fmt.Sprintf("%v/%v", ep, txHashInvalid)
	method := "GET"

	res, _ := httpRequest(url, method)

	assert.Equal(t, 404, res.StatusCode)

}

func TestTransactionHash_InvalidMethod(t *testing.T) {

	url := fmt.Sprintf("%v/%v", ep, txHashInvalid)
	method := "POST"

	res, _ := httpRequest(url, method)

	assert.Equal(t, 404, res.StatusCode)

}

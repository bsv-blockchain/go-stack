package test

import (
	"fmt"
	"testing"

	"github.com/teranode-group/woc-api/test"

	"github.com/stretchr/testify/assert"
)

var ep string = "https://mapi.taal.com/mapi"
var validTestnetKey = "testnet_16c64a607e21394ce01ddd8932e6f87e"

func TestGetFeeQuote(t *testing.T) {
	var term string = "feeQuote"
	url := fmt.Sprintf("%v/%v", ep, term)
	method := "GET"

	res, _ := test.HttpRequestDH(url, method, validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestGetFeeQuote_InvalidMethod(t *testing.T) {
	var term string = "feeQuote"
	url := fmt.Sprintf("%v/%v", ep, term)
	method := "PUT"

	res, _ := test.HttpRequestDH(url, method, validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 405)
}

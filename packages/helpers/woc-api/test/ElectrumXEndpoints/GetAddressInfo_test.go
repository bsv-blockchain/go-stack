package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/teranode-group/woc-api/test"
)

var ep string = "https://api.taal.com/api/v1"
var testnetKey = "testnet_16c64a607e21394ce01ddd8932e6f87e"
var mainnetKey = "mainnet_aed60d39b52b6049d7f881e4028ae194"

func TestGetAddressInfo(t *testing.T) {
	var term string = "address/1HZJ3kKqhsHgz7oyK52GG2QkzyrtPPwcvx/info"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "GET", mainnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, 200, res.StatusCode)
}

func TestGetAddressInfo_InvalidMethod(t *testing.T) {
	var term string = "address/1HZJ3kKqhsHgz7oyK52GG2QkzyrtPPwcvx/info"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "PATCH", mainnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, 405, res.StatusCode)
}

func TestGetAddressInfo_InvalidKey(t *testing.T) {
	var term string = "address/1HZJ3kKqhsHgz7oyK52GG2QkzyrtPPwcvx/info"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "GET", testnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, 400, res.StatusCode)
}

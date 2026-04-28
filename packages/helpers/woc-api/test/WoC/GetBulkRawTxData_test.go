package test

import (
	"fmt"
	"testing"

	"github.com/teranode-group/woc-api/test"

	"github.com/stretchr/testify/assert"
)

func TestGetBulkRaTxData(t *testing.T) {
	var term string = "txs"

	url := fmt.Sprintf("%v/%v", ep, term)

	var jsonStream string = "{ \"txids\": [\"e76e42509f98409ca2174524747ba42850b41d3a6e2aceccb7afcc165a05b9cd\", \"cfb2289a453b213c3fb72b491dbf439d9873d4cfbd6ae8813fa62d1a947712c7\" ]}"
	res, _ := test.HttpRequestDH_RQBody(url, "POST", validTestnetKey, jsonStream)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestGetBulkRaTxData_InvalidMethod(t *testing.T) {
	var term string = "txs"

	url := fmt.Sprintf("%v/%v", ep, term)

	var jsonStream string = "{ \"txids\": [\"e76e42509f98409ca2174524747ba42850b41d3a6e2aceccb7afcc165a05b9cd\", \"cfb2289a453b213c3fb72b491dbf439d9873d4cfbd6ae8813fa62d1a947712c7\" ]}"
	res, _ := test.HttpRequestDH_RQBody(url, "PATCH", validTestnetKey, jsonStream)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 404)
}

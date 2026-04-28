package test

import (
	"fmt"
	"testing"

	"github.com/teranode-group/woc-api/test"

	"github.com/stretchr/testify/assert"
)

func TestGetRawTxData(t *testing.T) {
	var term string = "tx/e76e42509f98409ca2174524747ba42850b41d3a6e2aceccb7afcc165a05b9cd/hex"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "GET", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestGetRawTxData_InvalidMethod(t *testing.T) {
	var term string = "tx/e76e42509f98409ca2174524747ba42850b41d3a6e2aceccb7afcc165a05b9cd/hex"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "PATCH", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 404)
}

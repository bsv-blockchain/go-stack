package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/teranode-group/woc-api/test"
)

func TestQueryTx(t *testing.T) {
	var term string = "tx"
	var txId string = "da18439f502b2891102ac3a9d1fa0648ddda8afa77c2be7c3707e492db3b9043"

	url := fmt.Sprintf("%v/%v/%v", ep, term, txId)

	res, _ := test.HttpRequestDH(url, "GET", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)

}

func TestQueryTx_BadMethod(t *testing.T) {
	var term string = "tx"
	var txId string = "da18439f502b2891102ac3a9d1fa0648ddda8afa77c2be7c3707e492db3b9043"

	url := fmt.Sprintf("%v/%v/%v", ep, term, txId)

	res, _ := test.HttpRequestDH(url, "PATCH", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 405)

}

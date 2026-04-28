package test

import (
	"fmt"
	"testing"

	"github.com/teranode-group/woc-api/test"

	"github.com/stretchr/testify/assert"
)

func TestGetBulkScriptUnspentTx(t *testing.T) {
	var term string = "scripts/unspent"

	url := fmt.Sprintf("%v/%v", ep, term)

	var jsonStream string = "{ \"scripts\": [\"933e2ab80b99749dbf8c22904899e8279056ce38644f64ab7313b8372c865ffb\"]}"
	res, _ := test.HttpRequestDH_RQBody(url, "POST", validTestnetKey, jsonStream)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestGetBulkScriptUnspentTx_InvalidMethod(t *testing.T) {
	var term string = "scripts/unspent"

	url := fmt.Sprintf("%v/%v", ep, term)

	var jsonStream string = "{ \"scripts\": [\"933e2ab80b99749dbf8c22904899e8279056ce38644f64ab7313b8372c865ffb\"]}"
	res, _ := test.HttpRequestDH_RQBody(url, "PATCH", validTestnetKey, jsonStream)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 404)
}

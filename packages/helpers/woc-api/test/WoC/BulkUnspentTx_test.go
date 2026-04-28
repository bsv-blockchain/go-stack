package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/teranode-group/woc-api/test"
)

func TestBulkUnspentTx(t *testing.T) {
	var term string = "addresses/unspent"

	url := fmt.Sprintf("%v/%v", ep, term)

	var jsonStream string = "{ \"addresses\": [\"mxKoPHekXHr5hdxXZEyzpjYXdpnH4KUjbK\"]}"
	res, _ := test.HttpRequestDH_RQBody(url, "POST", validTestnetKey, jsonStream)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestBulkUnspentTx_InvalidMethod(t *testing.T) {
	var term string = "addresses/unspent"

	url := fmt.Sprintf("%v/%v", ep, term)

	var jsonStream string = "{ \"addresses\": [\"mxKoPHekXHr5hdxXZEyzpjYXdpnH4KUjbK\"]}"
	res, _ := test.HttpRequestDH_RQBody(url, "PATCH", validTestnetKey, jsonStream)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 404)
}

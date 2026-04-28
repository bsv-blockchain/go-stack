package test

import (
	"fmt"
	"testing"

	"github.com/teranode-group/woc-api/test"

	"github.com/stretchr/testify/assert"
)

func TestGetScriptUnspentTx(t *testing.T) {
	var term string = "address/mxKoPHekXHr5hdxXZEyzpjYXdpnH4KUjbK/unspent"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "GET", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestGetScriptUnspentTx_InvalidMethod(t *testing.T) {
	var term string = "address/mxKoPHekXHr5hdxXZEyzpjYXdpnH4KUjbK/unspent"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "PATCH", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 404)
}

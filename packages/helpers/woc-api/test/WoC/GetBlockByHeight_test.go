package test

import (
	"fmt"
	"testing"

	"github.com/teranode-group/woc-api/test"

	"github.com/stretchr/testify/assert"
)

var ep string = "https://api.whatsonchain.com/v1/bsv/test"
var validTestnetKey = "testnet_16c64a607e21394ce01ddd8932e6f87e"

func TestGetBlockByHeight(t *testing.T) {
	var term string = "block/height/744713"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "GET", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestGetBlockByHeight_BadMethod(t *testing.T) {
	var term string = "block/height/744713"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "PATCH", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 404)
}

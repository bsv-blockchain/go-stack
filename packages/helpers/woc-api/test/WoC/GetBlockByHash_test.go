package test

import (
	"fmt"
	"testing"

	"github.com/teranode-group/woc-api/test"

	"github.com/stretchr/testify/assert"
)

func TestGetBlockByHash(t *testing.T) {
	var term string = "block/hash/00000000000000a9ce6691b3c773b7e5649d839f5a0985507b1b6f115c623fb5"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "GET", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestGetBlockByHash_InvalidMethod(t *testing.T) {
	var term string = "block/hash/00000000000000a9ce6691b3c773b7e5649d839f5a0985507b1b6f115c623fb5"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "PATCH", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 404)
}

package test

import (
	"fmt"
	"testing"

	"github.com/teranode-group/woc-api/test"

	"github.com/stretchr/testify/assert"
)

func TestGetHeaderByHash(t *testing.T) {
	var term string = "block/000000000000004bbc5de6eda059d56c6ebc6de699e77a1ba5cb504b1c2d8355/header"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "GET", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestGetHeaderByHash_InvalidMethod(t *testing.T) {
	var term string = "block/000000000000004bbc5de6eda059d56c6ebc6de699e77a1ba5cb504b1c2d8355/header"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "PATCH", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 404)
}

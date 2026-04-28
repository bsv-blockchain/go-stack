package test

import (
	"fmt"
	"testing"

	"github.com/teranode-group/woc-api/test"

	"github.com/stretchr/testify/assert"
)

func TestGetMerkleProof(t *testing.T) {
	var term string = "tx/76ef10f447d4ff41f68950c323e8387beae21382ac5c47f5bb52296a670e2e7e/proof"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "GET", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestGetMerkleProof_InvalidMethod(t *testing.T) {
	var term string = "tx/76ef10f447d4ff41f68950c323e8387beae21382ac5c47f5bb52296a670e2e7e/proof"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "PATCH", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 404)
}

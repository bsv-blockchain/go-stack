package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/teranode-group/woc-api/test"
)

//var ep string = "https://api.taal.com/api/v1"

func TestGetAddressBalance(t *testing.T) {
	var term string = "address/1HRADRLckTpFJJskkihZp16X6jR8JVRJcr/balance"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "GET", mainnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestGetAddressBalance_InvalidMethod(t *testing.T) {
	var term string = "address/1HRADRLckTpFJJskkihZp16X6jR8JVRJcr/balance"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, err := test.HttpRequestDH(url, "PATCH", mainnetKey)

	assert.NotNil(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 405)
}

func TestGetAddressBalance_InvalidKey(t *testing.T) {
	var term string = "address/1HRADRLckTpFJJskkihZp16X6jR8JVRJcr/balance"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, err := test.HttpRequestDH(url, "GET", testnetKey)

	assert.NotNil(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, res.StatusCode, 400)
}

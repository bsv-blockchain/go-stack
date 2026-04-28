package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/teranode-group/woc-api/test"
)

//var ep string = "https://api.taal.com/api/v1"

func TestGetUnspent(t *testing.T) {
	var term string = "address/1HRADRLckTpFJJskkihZp16X6jR8JVRJcr/unspent"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "GET", mainnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)

}

func TestGetUnspent_InvalidMethod(t *testing.T) {
	var term string = "address/1HRADRLckTpFJJskkihZp16X6jR8JVRJcr/unspent"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "PATCH", mainnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 405)

}

func TestGetUnspent_InvalidKey(t *testing.T) {
	var term string = "address/1HRADRLckTpFJJskkihZp16X6jR8JVRJcr/unspent"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "GET", testnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 400)

}

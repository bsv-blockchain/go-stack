package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/teranode-group/woc-api/test"
)

//var ep string = "https://api.taal.com/api/v1"

func TestGetAddressHistory(t *testing.T) {
	var term string = "address/1HZJ3kKqhsHgz7oyK52GG2QkzyrtPPwcvx/history"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, err := test.HttpRequestDH(url, "GET", mainnetKey)

	assert.NotNil(t, err)
	assert.Equal(t, res.StatusCode, 200)
}

func TestGetAddressHistory_InvalidMethod(t *testing.T) {
	var term string = "address/1HZJ3kKqhsHgz7oyK52GG2QkzyrtPPwcvx/history"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, err := test.HttpRequestDH(url, "PATCH", mainnetKey)

	assert.NotNil(t, err)
	assert.Equal(t, res.StatusCode, 405)

}

func TestGetAddressHistory_InvalidKey(t *testing.T) {
	var term string = "address/1HZJ3kKqhsHgz7oyK52GG2QkzyrtPPwcvx/history"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, err := test.HttpRequestDH(url, "GET", testnetKey)

	assert.NotNil(t, err)
	assert.Equal(t, res.StatusCode, 400)
}

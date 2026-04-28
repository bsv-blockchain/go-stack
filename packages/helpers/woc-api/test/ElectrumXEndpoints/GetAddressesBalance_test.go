package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/teranode-group/woc-api/test"
)

//var ep string = "https://api.taal.com/api/v1"

func TestGetAddressesBalance(t *testing.T) {
	var term string = "addresses/balance"

	url := fmt.Sprintf("%v/%v", ep, term)

	var jsonStream string = "{ \"addresses\": [\"16ZBEb7pp6mx5EAGrdeKivztd5eRJFuvYP\", \"1KGHhLTQaPr4LErrvbAuGE62yPpDoRwrob\"] }"

	res, _ := test.HttpRequestDH_RQBody(url, "POST", mainnetKey, jsonStream)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestGetAddressesBalance_InvalidMethod(t *testing.T) {
	var term string = "addresses/balance"

	url := fmt.Sprintf("%v/%v", ep, term)

	var jsonStream string = "{ \"addresses\": [\"16ZBEb7pp6mx5EAGrdeKivztd5eRJFuvYP\", \"1KGHhLTQaPr4LErrvbAuGE62yPpDoRwrob\"] }"

	res, err := test.HttpRequestDH_RQBody(url, "PATCH", mainnetKey, jsonStream)

	assert.NotNil(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 405)
}

func TestGetAddressesBalance_InvalidKey(t *testing.T) {
	var term string = "addresses/balance"

	url := fmt.Sprintf("%v/%v", ep, term)

	var jsonStream string = "{ \"addresses\": [\"16ZBEb7pp6mx5EAGrdeKivztd5eRJFuvYP\", \"1KGHhLTQaPr4LErrvbAuGE62yPpDoRwrob\"] }"

	res, err := test.HttpRequestDH_RQBody(url, "PATCH", mainnetKey, jsonStream)

	assert.NotNil(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, res.StatusCode, 405)
}

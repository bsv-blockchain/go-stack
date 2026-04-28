package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/teranode-group/woc-api/test"
)

//var ep string = "https://api.taal.com/api/v1"

func TestGetScriptHashHistory(t *testing.T) {
	var term string = "address/hash/933e2ab80b99749dbf8c22904899e8279056ce38644f64ab7313b8372c865ffb/history"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "GET", mainnetKey)

	assert.NotNil(t, res)
	assert.Equal(t, res.StatusCode, 200)

}

func TestGetScriptHashHistory_InvalidMethod(t *testing.T) {
	var term string = "address/hash/933e2ab80b99749dbf8c22904899e8279056ce38644f64ab7313b8372c865ffb/history"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, err := test.HttpRequestDH(url, "PATCH", mainnetKey)

	assert.NotNil(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, res.StatusCode, 405)

}

func TestGetScriptHashHistory_InvalidKey(t *testing.T) {
	var term string = "address/hash/933e2ab80b99749dbf8c22904899e8279056ce38644f64ab7313b8372c865ffb/history"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, err := test.HttpRequestDH(url, "GET", testnetKey)

	assert.NotNil(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, res.StatusCode, 400)

}

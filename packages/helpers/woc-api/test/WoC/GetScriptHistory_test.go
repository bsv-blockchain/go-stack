package test

import (
	"fmt"
	"testing"

	"github.com/teranode-group/woc-api/test"

	"github.com/stretchr/testify/assert"
)

func TestGetScriptHistory(t *testing.T) {
	var term string = "script/933e2ab80b99749dbf8c22904899e8279056ce38644f64ab7313b8372c865ffb/history"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "GET", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestGetScriptHistory_InvalidMethod(t *testing.T) {
	var term string = "script/933e2ab80b99749dbf8c22904899e8279056ce38644f64ab7313b8372c865ffb/history"

	url := fmt.Sprintf("%v/%v", ep, term)

	res, _ := test.HttpRequestDH(url, "PATCH", validTestnetKey)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 404)
}

package test

import (
	"fmt"
	"testing"

	"github.com/teranode-group/woc-api/test"

	"github.com/stretchr/testify/assert"
)

func TestGetExplorerLinks(t *testing.T) {
	var term string = "search/links"

	url := fmt.Sprintf("%v/%v", ep, term)

	var jsonStream string = "{ \"query\": \"mxKoPHekXHr5hdxXZEyzpjYXdpnH4KUjbK\" }"
	res, _ := test.HttpRequestDH_RQBody(url, "POST", validTestnetKey, jsonStream)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestGetExplorerLinks_InvalidMethod(t *testing.T) {
	var term string = "search/links"

	url := fmt.Sprintf("%v/%v", ep, term)

	var jsonStream string = "{ \"query\": \"mxKoPHekXHr5hdxXZEyzpjYXdpnH4KUjbK\" }"
	res, _ := test.HttpRequestDH_RQBody(url, "PATCH", validTestnetKey, jsonStream)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 404)
}

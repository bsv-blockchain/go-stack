package test

import (
	"fmt"
	"testing"

	"github.com/teranode-group/woc-api/test"

	"github.com/stretchr/testify/assert"
)

func TestDecodeTxDetails(t *testing.T) {
	var term string = "tx/decode"

	url := fmt.Sprintf("%v/%v", ep, term)

	var jsonStream string = "{ \"txhex\": \"02000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1d03fcf915105361746f736869204e616b616d6f746f30000001eee2aa2affffffff02062c9c04000000001976a914b85e201070afbb3e14893b1eeb0385d952d87cf088acc2eb0b00000000001976a91445ac90190d0c95e3f14ddcda1a7902d5070536cc88ac00000000\"}"
	res, _ := test.HttpRequestDH_RQBody(url, "POST", validTestnetKey, jsonStream)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestDecodeTxDetails_InvalidMethod(t *testing.T) {
	var term string = "tx/decode"

	url := fmt.Sprintf("%v/%v", ep, term)

	var jsonStream string = "{ \"txhex\": \"02000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1d03fcf915105361746f736869204e616b616d6f746f30000001eee2aa2affffffff02062c9c04000000001976a914b85e201070afbb3e14893b1eeb0385d952d87cf088acc2eb0b00000000001976a91445ac90190d0c95e3f14ddcda1a7902d5070536cc88ac00000000\"}"
	res, _ := test.HttpRequestDH_RQBody(url, "GET", validTestnetKey, jsonStream)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 404)
}

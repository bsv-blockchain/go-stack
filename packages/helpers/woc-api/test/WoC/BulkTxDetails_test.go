package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/teranode-group/woc-api/test"
)

func TestBulkTxDetails(t *testing.T) {
	var term string = "txs"

	url := fmt.Sprintf("%v/%v", ep, term)

	var jsonStream string = "{ \"txids\": [ \"6c941114fbc53a2e8b9c42597a100361a3529b3d498efe4275ac4734e69eeee2\", \"e76e42509f98409ca2174524747ba42850b41d3a6e2aceccb7afcc165a05b9cd\", \"3ea69621d4678b84d2974726d84acdbfd9457ba9a76cc7deb7f89c40adb1aabf\", \"b992bc98271304c8a963932d4ff7cb3c88725b759516605cf83f892bfa5d507c\", \"d06c572c68646e4bc931ee35bd3635be8675b4783daf70249493a91737d6f20b\", \"3524b77eabb7ebe82c1d7f22555bf9baf9fdcee370dca5130a5cf6d19ea94cfb\", \"6fb0ecd7770dae48732563df8a080890ce573f376474f960aa013b87ad9a4abe\", \"6e392818910691f225e9b5e5393464d24eb337aa2858564a8ac9e495926c392a\", \"a176345257b5b672e869cb29f2cb29911a28b8b786882a5e7fa236482789c3bd\", \"ca9c4679dde284706421cbc74e9b043ef26f3dd813884709c0ffe060a892a219\", \"ee0f2c1e3da9311ad84e2d9887501f3d7d3c14fd7d443c7ccdce37200345410c\", \"aef0be689f5c69ad2315d24e77246c728d73a7dc3843afce178ba547e10d644d\", \"eddbc9912928c5900a97c64d2206d8a42bffe760f1c1e88b19cb8d2ad7fa177a\", \"1a62d2909e473d985d537a16c577ddab1f0eba21a90d17363702c38a0e192608\", \"b077b6fc8e0f6f03edc040caac5cbf1a5f63a9a11f30200bf718b67df19ca8f1\", \"a4ae47a2c067f2f7845f10df6fd8178f544c50acb9476deb763b4297dbacc942\", \"5e68adee59457d6f5ea998ee53bdde17556ad70d9da46078c76d6b12de33bc5f\", \"74a27df57f2240b4be0f4cea2dca8492e3a1f7ce467367c9ec6d7f11143c0bad\", \"94323a3fbfa9ec30ced34b11f536dbafdd2c4402f034ebf85ddfa44e562c0188\", \"1376dd5a0a7fe367004615b264f01db18cf977b24af7ff0b430d00bffd1a1720\" ] }"
	res, _ := test.HttpRequestDH_RQBody(url, "POST", validTestnetKey, jsonStream)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)
}

func TestBulkTxDetails_InvalidMethod(t *testing.T) {
	var term string = "txs"

	url := fmt.Sprintf("%v/%v", ep, term)

	var jsonStream string = "{ \"txids\": [ \"6c941114fbc53a2e8b9c42597a100361a3529b3d498efe4275ac4734e69eeee2\", \"e76e42509f98409ca2174524747ba42850b41d3a6e2aceccb7afcc165a05b9cd\", \"3ea69621d4678b84d2974726d84acdbfd9457ba9a76cc7deb7f89c40adb1aabf\", \"b992bc98271304c8a963932d4ff7cb3c88725b759516605cf83f892bfa5d507c\", \"d06c572c68646e4bc931ee35bd3635be8675b4783daf70249493a91737d6f20b\", \"3524b77eabb7ebe82c1d7f22555bf9baf9fdcee370dca5130a5cf6d19ea94cfb\", \"6fb0ecd7770dae48732563df8a080890ce573f376474f960aa013b87ad9a4abe\", \"6e392818910691f225e9b5e5393464d24eb337aa2858564a8ac9e495926c392a\", \"a176345257b5b672e869cb29f2cb29911a28b8b786882a5e7fa236482789c3bd\", \"ca9c4679dde284706421cbc74e9b043ef26f3dd813884709c0ffe060a892a219\", \"ee0f2c1e3da9311ad84e2d9887501f3d7d3c14fd7d443c7ccdce37200345410c\", \"aef0be689f5c69ad2315d24e77246c728d73a7dc3843afce178ba547e10d644d\", \"eddbc9912928c5900a97c64d2206d8a42bffe760f1c1e88b19cb8d2ad7fa177a\", \"1a62d2909e473d985d537a16c577ddab1f0eba21a90d17363702c38a0e192608\", \"b077b6fc8e0f6f03edc040caac5cbf1a5f63a9a11f30200bf718b67df19ca8f1\", \"a4ae47a2c067f2f7845f10df6fd8178f544c50acb9476deb763b4297dbacc942\", \"5e68adee59457d6f5ea998ee53bdde17556ad70d9da46078c76d6b12de33bc5f\", \"74a27df57f2240b4be0f4cea2dca8492e3a1f7ce467367c9ec6d7f11143c0bad\", \"94323a3fbfa9ec30ced34b11f536dbafdd2c4402f034ebf85ddfa44e562c0188\", \"1376dd5a0a7fe367004615b264f01db18cf977b24af7ff0b430d00bffd1a1720\" ] }"
	res, _ := test.HttpRequestDH_RQBody(url, "PATCH", validTestnetKey, jsonStream)

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 404)
}

package test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubmitMultipleTx(t *testing.T) {
	var term string = "txs"

	url := fmt.Sprintf("%v/%v", ep, term)

	reqBody, err := json.Marshal(map[string]string{
		"rawTx": "010000000185c575d1e73bd0192a33d3dd9573e75081fd1753736f147afaf2054f515e2340010000006b4830450221008ef4654ded749d36b62feac875c0b51b10927f1b8c263f9731328125ce33597802201353bdb8bd3e8c9f4e246616904f61d67d70faf33be1977125dfa934517f858141210243fb28c5258f9f5936f58eab245d27bd11d7b814970a142f2e94f2d6b196a50effffffff0288130000000000001976a91472cfef298560fac0d986fbff42dae2e5ee79a92188ac7e711400000000001976a91472cfef298560fac0d986fbff42dae2e5ee79a92188ac00000000",
	})
	assert.Nil(t, err)

	res, _ := HttpRequestDH_Post(url, validTestnetKey, reqBody, "")

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 200)

}

func TestSubmitMultipleTx_BadMethod(t *testing.T) {
	var term string = "txs"

	url := fmt.Sprintf("%v/%v", ep, term)

	reqBody, err := json.Marshal(map[string]string{
		"rawTx": "010000000185c575d1e73bd0192a33d3dd9573e75081fd1753736f147afaf2054f515e2340010000006b4830450221008ef4654ded749d36b62feac875c0b51b10927f1b8c263f9731328125ce33597802201353bdb8bd3e8c9f4e246616904f61d67d70faf33be1977125dfa934517f858141210243fb28c5258f9f5936f58eab245d27bd11d7b814970a142f2e94f2d6b196a50effffffff0288130000000000001976a91472cfef298560fac0d986fbff42dae2e5ee79a92188ac7e711400000000001976a91472cfef298560fac0d986fbff42dae2e5ee79a92188ac00000000",
	})
	assert.Nil(t, err)

	res, _ := HttpRequestDH_Post(url, validTestnetKey, reqBody, "GET")

	assert.NotNil(t, res)
	assert.NotNil(t, res.StatusCode)
	assert.Equal(t, res.StatusCode, 405)

}

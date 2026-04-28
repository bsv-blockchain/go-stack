package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bitcoinsv/bsvd/bsvec"
	"github.com/libsv/go-bk/bec"
	"github.com/libsv/go-bk/chaincfg"
	"github.com/libsv/go-bk/wif"
	"github.com/libsv/go-bt/bscript"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/unlocker"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
)

//const broadcastTimeoutInSeconds = 10

func ConvertToSats(num float64) uint64 {
	s := fmt.Sprintf("%.8f", num)
	s = strings.Replace(s, ".", "", 1)
	u, _ := strconv.ParseUint(s, 10, 64)
	return u
}

func HttpRequest(url string, method string, payload string, apikey string) []byte {

	newString := fmt.Sprintf(`{ "rawTx": "%v" }`, payload)
	body := strings.NewReader(newString)
	client := &http.Client{}
	req, err := http.NewRequest(method, url, body)

	if err != nil {
		fmt.Println(err)

	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", apikey)

	response, err := client.Do(req)
	if err != nil {
		fmt.Println(err)

	}

	responseBody, err := ioutil.ReadAll(response.Body)

	if err != nil {
		fmt.Println(err)
	}

	return responseBody
}

func HttpRequestWithoutApiKey(url string, method string, payload string) []byte {

	newString := fmt.Sprintf(`{ "rawTx": "%v" }`, payload)
	body := strings.NewReader(newString)
	client := &http.Client{}
	req, err := http.NewRequest(method, url, body)

	if err != nil {
		fmt.Println(err)

	}
	req.Header.Add("Content-Type", "application/json")

	response, err := client.Do(req)
	if err != nil {
		fmt.Println(err)

	}

	responseBody, err := ioutil.ReadAll(response.Body)

	if err != nil {
		fmt.Println(err)
	}

	return responseBody
}

func GetFundsFromFaucet(address string) string {

	url := fmt.Sprintf("https://api-test.whatsonchain.com/v1/bsv/test/faucet/send/%s", address)
	body := strings.NewReader("")
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, body)

	if err != nil {
		fmt.Println(err)

	}
	req.Header.Add("Content-Type", "application/json")

	response, err := client.Do(req)
	if err != nil {
		fmt.Println(err)

	}

	responseBody, err := ioutil.ReadAll(response.Body)

	if err != nil {
		fmt.Println(err)
	}
	txid := string(responseBody[:])
	fmt.Println("Txid =", txid)
	return txid
}

func CreatePK(t *testing.T) *bsvec.PrivateKey {

	privateKey, err := bsvec.NewPrivateKey(bsvec.S256())
	if err != nil {
		t.Fatal(err)
	}
	return privateKey
}

func CreateAddress(t *testing.T, publicKey *bsvec.PublicKey) string {
	destAddressT, err := bscript.NewAddressFromPublicKey(publicKey, false) // false means "not mainnet"
	if err != nil {
		return ""
	}
	return destAddressT.AddressString
}

func CreatePK1() *bsvec.PrivateKey {

	privateKey, err := bsvec.NewPrivateKey(bsvec.S256())
	if err != nil {
		Fatal(err)
	}
	return privateKey
}

func Fatal(err error) {
	panic("unimplemented")
}

func CreateAddress1(publicKey *bsvec.PublicKey) string {
	destAddressT, err := bscript.NewAddressFromPublicKey(publicKey, false) // false means "not mainnet"
	if err != nil {
		return ""
	}
	return destAddressT.AddressString
}

// returns tx details from woc which can then be used as input for another tx
func GetTransaction(txid string) (*Transaction, error) {
	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/test/tx/hash/%s", txid)

	client := &http.Client{}
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var tx Transaction
	if err := json.NewDecoder(resp.Body).Decode(&tx); err != nil {
		return nil, err
	}

	return &tx, err
}

type Transaction struct {
	Vouts []Out `json:"vout"`
}

type Out struct {
	Value        float64 `json:"value"`
	Vout         uint32  `json:"n"`
	ScriptPubKey Script  `json:"scriptPubKey"`
}

type Script struct {
	Hex string `json:"hex"`
}

type MapiBody struct {
	Payload   string `json:"Payload"`
	Signature string `json:"signature"`
	PublicKey string `json:"publicKey"`
	Encoding  string `json:"encoding"`
	Mimetype  string `json:"mimetype"`
}

func HttpRequestDH(url string, method string, apiKey string) (res *http.Response, body []byte) {

	client := &http.Client{}

	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		fmt.Println(err)
		return
	}

	var bearer = "Bearer " + apiKey

	req.Header.Set("Content-Type", "application/json, charset=UTF-8")
	req.Header.Add("Authorization", bearer)

	res, err = client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer res.Body.Close()

	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	return res, body
}

func HttpRequestDH_RQBody(url string, method string, apiKey string, jsonStream string) (res *http.Response, body []byte) {

	client := &http.Client{}

	var req *http.Request
	var err error

	if jsonStream == "" {
		req, err = http.NewRequest(method, url, nil)
	} else {
		req, err = http.NewRequest(method, url, strings.NewReader(jsonStream))
	}

	if err != nil {
		fmt.Println(err)
		return
	}

	var bearer = "Bearer " + apiKey

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", bearer)

	res, err = client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer res.Body.Close()

	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	return res, body
}

func GetVoutIndex(tran Transaction, value float64) int {
	var inputVout int
	for i := 0; i < len(tran.Vouts); i++ {
		if tran.Vouts[i].Value == value {
			inputVout = i
		}

	}
	return inputVout

}

func CreateNewTransactionAndSign(t *testing.T, txid string, inputTx Transaction, inputVout int, ToAddress string, ChangeAddress string, signatureKey *bsvec.PrivateKey, satsAmount int) *bt.Tx {
	tx := bt.NewTx()

	_ = tx.From(
		txid,
		inputTx.Vouts[inputVout].Vout,
		inputTx.Vouts[inputVout].ScriptPubKey.Hex,
		ConvertToSats(inputTx.Vouts[inputVout].Value),
	)

	_ = tx.PayToAddress(ToAddress, uint64(satsAmount))
	_ = tx.ChangeToAddress(ChangeAddress, bt.NewFeeQuote())

	wifNew, _ := wif.NewWIF((*bec.PrivateKey)(signatureKey), &chaincfg.MainNet, false)
	decodedWif, err := wif.DecodeWIF(wifNew.String())
	assert.Nil(t, err, "Error returned %v", err)

	if err := tx.FillAllInputs(context.Background(), &unlocker.Getter{PrivateKey: decodedWif.PrivKey}); err != nil {
		log.Fatal("error here--- " + err.Error())
	}
	log.Printf("tx: %s\n", tx)

	return tx

}

func CreateNewTransactionAndSignNew(txid string, inputTx Transaction, inputVout int, ToAddress string, ChangeAddress string, signatureKey *bsvec.PrivateKey, satsAmount int) *bt.Tx {
	tx := bt.NewTx()

	_ = tx.From(
		txid,
		inputTx.Vouts[inputVout].Vout,
		inputTx.Vouts[inputVout].ScriptPubKey.Hex,
		ConvertToSats(inputTx.Vouts[inputVout].Value),
	)

	_ = tx.PayToAddress(ToAddress, uint64(satsAmount))
	_ = tx.ChangeToAddress(ChangeAddress, bt.NewFeeQuote())

	wifNew, _ := wif.NewWIF((*bec.PrivateKey)(signatureKey), &chaincfg.MainNet, false)
	decodedWif, err := wif.DecodeWIF(wifNew.String())
	Expect(err).ShouldNot(HaveOccurred())

	if err := tx.FillAllInputs(context.Background(), &unlocker.Getter{PrivateKey: decodedWif.PrivKey}); err != nil {
		log.Fatal("error here--- " + err.Error())
	}
	log.Printf("tx: %s\n", tx)

	return tx

}

func HttpRequestDH_Post(url string, apiKey string, reqBody []byte, method string) (res *http.Response, body []byte) {

	client := &http.Client{}

	var req *http.Request
	var err error
	if method == "" {
		req, err = http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	} else {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	}

	if err != nil {
		fmt.Println(err)
		return
	}

	var bearer = "Bearer " + apiKey

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", bearer)

	res, err = client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer res.Body.Close()

	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	return res, body
}

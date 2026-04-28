package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"reflect"
	"regexp"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
)

var endpoint string = "https://taalnet.whatsonchain.com/v1/bsv/taalnet/"
var token string = "e2e63b1f793aca847e6954765fa1599df6d1e1a7"
var address string = "mq7psuJ7Z9h1w4H3YtcCoHx7cPmhVM9UsV"
var missingAddress string = "m87psuJ7Z9h1w4H3YtcCoHx7cPmhVM9UsV"
var missingToken string = "47e63b1f793aca847e6954765fa1523df6d1e1a7"
var txid string = "c50d6798dbbe04e51cf08ab99793faae3514b77bbc2e0e2ae83bfe2597a28d35"
var missingTxid string = "sdfsd6798dbbe04e51cf08ab99793faae3514b77bbc2e0e2ae83bfe2597a28d35"
var symbol string = "TAALT"
var missingSymbol = "DOES_NOT_EXIST"

func Test_TokenEndpoints(t *testing.T) {
	RegisterFailHandler(Fail)
	var r []Reporter
	reportDir := "../allure-results/"
	reportFileName := "allure-report"
	if reportDir != "" {
		r = append(r, reporters.NewJUnitReporter(path.Join(reportDir, fmt.Sprintf("%s_%02d.xml", reportFileName, config.GinkgoConfig.ParallelNode))))
	}
	RunSpecsWithDefaultAndCustomReporters(t, "TestTokenEndpoints", r)
}

var _ = Describe("TestTokenEndpoints", func() {

	Context("Token Endpoint Tests", func() {

		It("TestGetAllTokens", func() {

			url := fmt.Sprintf("%v/tokens", endpoint)
			method := "GET"

			res, body := httpRequest(url, method)
			Expect(res.StatusCode).To(Equal(200))
			Expect(string(body)).To(ContainSubstring("{\"token_id\":\"1dc0f091dda936b86266adbabfd1d3f4a9dfee16\",\"symbol\":\"TAALT\",\"name\":\"Taal Token\""))

		})

		It("TestGetAllTokensPost_ThrowsError", func() {
			url := fmt.Sprintf("%v/tokens", endpoint)
			method := "POST"
			res, body := httpRequest(url, method)

			Expect(res.StatusCode).To(Equal(405))
			Expect(string(body)).To(ContainSubstring("Method Not Allowed"))

		})

		It("TestGetTokenDetails", func() {
			url := fmt.Sprintf("%v/token/%v/%v", endpoint, token, symbol)
			method := "GET"
			res, body := httpRequest(url, method)

			Expect(res.StatusCode).To(Equal(200))
			content, _ := ReadFile("test_structs/getTokenData.json")
			isPayloadCorrect, _ := JSONBytesEqual([]byte(content), body)
			Expect(isPayloadCorrect).To(BeTrue(), "Payload should be equal")
		})

		It("TestGetTokenDetailsPost_ThrowsError", func() {

			url := fmt.Sprintf("%v/token/%v/%v", endpoint, token, symbol)
			method := "POST"

			res, body := httpRequest(url, method)
			Expect(res.StatusCode).To(Equal(405))
			Expect(string(body)).To(ContainSubstring("Method Not Allowed"))

		})

		It("TestGetTokenDetails_WrongTokenID", func() {

			url := fmt.Sprintf("%v/token/%v/%v", endpoint, missingToken, symbol)
			method := "GET"

			res, body := httpRequest(url, method)
			Expect(res.StatusCode).To(Equal(404))
			Expect(string(body)).To(ContainSubstring("Some Error"))
		})

		It("TestGetTokenDetails_WrongSymbol", func() {

			url := fmt.Sprintf("%v/token/%v/%v", endpoint, token, missingSymbol)
			method := "GET"

			res, body := httpRequest(url, method)
			Expect(res.StatusCode).To(Equal(404))
			Expect(string(body)).To(ContainSubstring("Some Error"))
		})

		It("TestGetTokenDetails_WrongSymbol_WrongTokenID", func() {

			url := fmt.Sprintf("%v/token/%v/%v", endpoint, missingToken, missingSymbol)
			method := "GET"

			res, body := httpRequest(url, method)
			Expect(res.StatusCode).To(Equal(404))
			Expect(string(body)).To(ContainSubstring("Some Error"))
		})

		It("TestGetAddressTokenUtxos", func() {

			url := fmt.Sprintf("%v/address/%v/tokens/unspent", endpoint, address)
			method := "GET"
			res, body := httpRequest(url, method)

			Expect(res.StatusCode).To(Equal(200))
			content, _ := ReadFile("test_structs/getTokenUnspent.json")
			isPayloadCorrect, _ := JSONBytesEqual([]byte(content), body)
			Expect(isPayloadCorrect).To(BeTrue(), "Payload should be equal")
		})

		It("TestPostAddressTokenUtxos_ThrowsError", func() {
			url := fmt.Sprintf("%v/address/%v/tokens/unspent", endpoint, address)
			method := "POST"

			res, body := httpRequest(url, method)
			Expect(res.StatusCode).To(Equal(405))
			Expect(string(body)).To(ContainSubstring("Method Not Allowed"))
		})

		It("TestGetAddressTokenUtxos_MissingAddress", func() {

			url := fmt.Sprintf("%v/address/%v/tokens/unspent", endpoint, address)
			method := "POST"

			res, body := httpRequest(url, method)
			Expect(res.StatusCode).To(Equal(404))
			Expect(string(body)).To(ContainSubstring("Some Error"))
		})

		It("TestGetAddressTokens", func() {

			url := fmt.Sprintf("%v/address/%v/tokens", endpoint, address)
			method := "GET"
			res, body := httpRequest(url, method)

			Expect(res.StatusCode).To(Equal(200))
			content, _ := ReadFile("test_structs/getTokens.json")
			isPayloadCorrect, _ := JSONBytesEqual([]byte(content), body)
			Expect(isPayloadCorrect).To(BeTrue(), "Payload should be equal")
		})

		It("TestPostAddressTokens_ThrowsError", func() {
			url := fmt.Sprintf("%v/address/%v/tokens", endpoint, address)
			method := "POST"
			res, body := httpRequest(url, method)

			Expect(res.StatusCode).To(Equal(405))
			Expect(string(body)).To(ContainSubstring("Method Not Allowed"))
		})

		It("TestGetAddressTokens_MissingAddress", func() {

			url := fmt.Sprintf("%v/address/%v/tokens", endpoint, missingAddress)
			method := "GET"

			res, body := httpRequest(url, method)
			Expect(res.StatusCode).To(Equal(200))
			Expect(string(body)).To(ContainSubstring("{\"address\":\"m87psuJ7Z9h1w4H3YtcCoHx7cPmhVM9UsV\",\"tokens\":null}"))
		})

		It("TestGetTxByTokenId", func() {

			url := fmt.Sprintf("%v/token/%v/%v/tx", endpoint, token, symbol)
			method := "GET"
			res, body := httpRequest(url, method)

			Expect(res.StatusCode).To(Equal(200))
			content, _ := ReadFile("test_structs/getTxByTokenID.json")
			isPayloadCorrect, _ := JSONBytesEqual([]byte(content), body)
			Expect(isPayloadCorrect).To(BeTrue(), "Payload should be equal")
		})

		It("TestPostTxByTokenId_ThrowsError", func() {

			url := fmt.Sprintf("%v/token/%v/%v/tx", endpoint, token, symbol)
			method := "POST"
			res, body := httpRequest(url, method)

			Expect(res.StatusCode).To(Equal(405))
			Expect(string(body)).To(ContainSubstring("Method Not Allowed"))
		})

		It("TestGetTxByTokenId_MissingToken", func() {

			url := fmt.Sprintf("%v/token/%v/%v/tx", endpoint, missingToken, symbol)
			method := "GET"
			res, body := httpRequest(url, method)

			Expect(res.StatusCode).To(Equal(404))
			Expect(string(body)).To(ContainSubstring("Some Error"))
		})

		It("TestGetTxByTokenId_MissingSymbol", func() {

			url := fmt.Sprintf("%v/token/%v/%v/tx", endpoint, missingToken, symbol)
			method := "GET"
			res, body := httpRequest(url, method)

			Expect(res.StatusCode).To(Equal(404))
			Expect(string(body)).To(ContainSubstring("Some Error"))
		})

		It("TestGetTxByTokenId_MissingSymbolMissingToken", func() {

			url := fmt.Sprintf("%v/token/%v/%v/tx", endpoint, missingToken, missingSymbol)
			method := "GET"
			res, body := httpRequest(url, method)

			Expect(res.StatusCode).To(Equal(404))
			Expect(string(body)).To(ContainSubstring("Some Error"))
		})

		It("TestGetTokenVout", func() {
			index := 1
			url := fmt.Sprintf("%v/token/tx/%v/out/%v", endpoint, txid, index)
			method := "GET"

			res, body := httpRequest(url, method)
			Expect(res.StatusCode).To(Equal(200))
			content, _ := ReadFile("test_structs/getTokenVout.json")
			isPayloadCorrect, _ := JSONBytesEqual([]byte(content), body)
			Expect(isPayloadCorrect).To(BeTrue(), "Payload should be equal")
		})

		It("TestPostTokenVout_ThrowsError", func() {

			index := 1
			url := fmt.Sprintf("%v/token/tx/%v/out/%v", endpoint, txid, index)
			method := "POST"
			res, body := httpRequest(url, method)

			Expect(res.StatusCode).To(Equal(405))
			Expect(string(body)).To(ContainSubstring("Method Not Allowed"))
		})

		It("TestGetTokenVout_MissingTxID", func() {

			index := 1
			url := fmt.Sprintf("%v/token/tx/%v/out/%v", endpoint, missingTxid, index)
			method := "GET"
			res, body := httpRequest(url, method)

			Expect(res.StatusCode).To(Equal(404))
			Expect(string(body)).To(ContainSubstring("Some Error"))
		})

		It("TestGetTokenVout_MissingIndex", func() {
			missingIndex := 5
			url := fmt.Sprintf("%v/token/tx/%v/out/%v", endpoint, txid, missingIndex)
			method := "GET"
			res, body := httpRequest(url, method)

			Expect(res.StatusCode).To(Equal(404))
			Expect(string(body)).To(ContainSubstring("Some Error"))
		})

		It("TestGetTokenVout_MissingTxId_MissingIndex", func() {

			missingIndex := 5
			url := fmt.Sprintf("%v/token/tx/%v/out/%v", endpoint, missingTxid, missingIndex)
			method := "GET"
			res, body := httpRequest(url, method)

			Expect(res.StatusCode).To(Equal(404))
			Expect(string(body)).To(ContainSubstring("Some Error"))

		})

	})

})

func httpRequest(url string, method string) (res *http.Response, body []byte) {

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		fmt.Println(err)
		return
	}
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

func ReadFile(partialfilepath string) (string, error) {
	data, err := os.ReadFile(partialfilepath)
	if err != nil {
		log.Println(" File not found")
		return "", err
	}
	temp := string(data)
	re := regexp.MustCompile(` +\r?\n +`)
	temp1 := re.ReplaceAllString(temp, "")
	log.Println(temp1)
	return temp1, nil
}

func JSONBytesEqual(a, b []byte) (bool, error) {
	var j, j2 interface{}
	if err := json.Unmarshal(a, &j); err != nil {
		return false, err
	}
	if err := json.Unmarshal(b, &j2); err != nil {
		return false, err
	}
	return reflect.DeepEqual(j2, j), nil
}

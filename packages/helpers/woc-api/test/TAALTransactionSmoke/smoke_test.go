package test

import (
	"encoding/json"
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/teranode-group/woc-api/test"
)

func TestSmokeTaalTransaction(t *testing.T) {
	RegisterFailHandler(Fail) // connects gingko to gomega
	var r []ginkgo.Reporter
	reportDir := "../../allure-results/"
	reportFileName := "allure-report"
	if reportDir != "" {
		r = append(r, reporters.NewJUnitReporter(path.Join(reportDir, fmt.Sprintf("%s_%02d.xml", reportFileName, config.GinkgoConfig.ParallelNode))))
	}
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "Smoke test Suite", r)

}

var _ = Describe("Smoke test API Suite", func() {
	var mainnetKey = "mainnet_aed60d39b52b6049d7f881e4028ae194"
	var ElectrumEndpoint = "https://api.taal.com/api/v1"
	var TapiBroadcast = "https://api.taal.com/api/v1/broadcast"
	var BitcoinEndpoint = "https://api.taal.com/api/v1/bitcoin"

	var validTestnetKey = "testnet_4aef81fcd8f87a12d8f36f2cf5528733"

	Context("Bitcoin Get txout", func() {

		It(" Bitcoin Get Get txout with valid key", func() {

			url := fmt.Sprintf("%v", BitcoinEndpoint)

			var jsonStream string = "{\"jsonrpc\": \"1.0\", \"id\":\"postmantest\", \"method\": \"gettxout\", \"params\": [\"5ae748b3493dee130e9de12dc394089e8c21a99ff91b3cf795c95dc776bc1723\", 0]}"

			res, _ := test.HttpRequestDH_RQBody(url, "POST", mainnetKey, jsonStream)

			Expect(res.StatusCode).To(Equal(200))

		})

	})

	Context("Bitcoin Get Best Block hash", func() {

		It("Bitcoin Get Best block hash with valid key", func() {

			url := fmt.Sprintf("%v", BitcoinEndpoint)

			var jsonStream string = "{\"jsonrpc\": \"1.0\", \"id\": \"postmantest\", \"method\": \"getbestblockhash\", \"params\": []}"

			res, _ := test.HttpRequestDH_RQBody(url, "POST", mainnetKey, jsonStream)

			Expect(res.StatusCode).To(Equal(200))

		})

	})
	Context("Electrum Get Address Balance ", func() {
		var AddressBalance string = "address/1HRADRLckTpFJJskkihZp16X6jR8JVRJcr/balance"

		It(" Electrum Get Address Balance with valid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressBalance)
			res, body := test.HttpRequestDH(url, "GET", mainnetKey)

			Expect(res.StatusCode).To(Equal(200))
			Expect(string(body)).To(ContainSubstring("confirmed"))

		})

	})

	Context(" Electrum Get Addresses Balance ", func() {
		var AddressesBalance string = "addresses/balance"

		It(" Electrum Get Addresses Balance with valid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressesBalance)

			var jsonStream string = "{ \"addresses\": [\"16ZBEb7pp6mx5EAGrdeKivztd5eRJFuvYP\", \"1KGHhLTQaPr4LErrvbAuGE62yPpDoRwrob\"] }"

			res, _ := test.HttpRequestDH_RQBody(url, "POST", mainnetKey, jsonStream)

			Expect(res.StatusCode).To(Equal(200))

		})

	})

	Context("MAPI Fee Quote", func() {
		var MapiFeeQuote string = "https://api.taal.com/mapi/feeQuote"
		var validTestnetKey = "testnet_4aef81fcd8f87a12d8f36f2cf5528733"

		It("MAPI get Fee Quote successfull", func() {
			res, body := test.HttpRequestDH(MapiFeeQuote, "GET", validTestnetKey)

			time.Sleep(10 * time.Second)
			Expect(res.StatusCode).To(Equal(200))
			var output test.MapiBody
			json.Unmarshal(body, &output)
			Expect(output.Payload).Should(ContainSubstring("apiVersion"))
			Expect(output.Payload).Should(ContainSubstring("timestamp"))
		})

	})

	Context("Test Broadcast Tapi", func() {

		It("Tapi Broadcast 100 sats sucessfull", func() {

			satsAmount2 := 1000 //amount of sats we are sending in tx
			johnPrivateKey := test.CreatePK1()
			johnAddress := test.CreateAddress1(johnPrivateKey.PubKey())
			toPrivateKey := test.CreatePK1()
			toAddress := test.CreateAddress1(toPrivateKey.PubKey())
			time.Sleep(10 * time.Second)
			txid2 := test.GetFundsFromFaucet(johnAddress)

			inputTx2, _ := test.GetTransaction(txid2)

			inputVout2 := test.GetVoutIndex(*inputTx2, 0.01)

			time.Sleep(5 * time.Second)
			tx2 := test.CreateNewTransactionAndSignNew(txid2, *inputTx2, inputVout2, toAddress, toAddress, johnPrivateKey, satsAmount2)

			response2 := test.HttpRequest(TapiBroadcast, "POST", tx2.String(), validTestnetKey)

			time.Sleep(5 * time.Second) //wait for tx to propagate
			txResult2, err := test.GetTransaction(string(response2))
			time.Sleep(5 * time.Second)
			if err != nil {
				Expect(string(err.Error())).Should(ContainSubstring("400"))
				response2 := test.HttpRequest(TapiBroadcast, "POST", tx2.String(), validTestnetKey)

				time.Sleep(5 * time.Second) //wait for tx to propagate
				txResult2, err := test.GetTransaction(string(response2))
				Expect(err).ShouldNot(HaveOccurred())
				Expect(txResult2.Vouts[0].Value).To(Equal(1e-05))
				Expect(txResult2.Vouts[1].Value).To(Equal(0.00998887))
			} else {
				Expect(err).ShouldNot(HaveOccurred())
				Expect(txResult2.Vouts[0].Value).To(Equal(1e-05))
				Expect(txResult2.Vouts[1].Value).To(Equal(0.00998887))
				Expect(test.ConvertToSats(txResult2.Vouts[0].Value)).To(Equal(uint64(satsAmount2)))

			}

		})

	})

})

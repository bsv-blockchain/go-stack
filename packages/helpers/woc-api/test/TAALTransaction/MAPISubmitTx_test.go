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

var MapiBroadcast = "https://api.taal.com/mapi/tx"

func TestSubmitTx(t *testing.T) {
	RegisterFailHandler(Fail) // connects gingko to gomega
	var r []ginkgo.Reporter
	reportDir := "../../allure-results/"
	reportFileName := "allure-report"
	if reportDir != "" {
		r = append(r, reporters.NewJUnitReporter(path.Join(reportDir, fmt.Sprintf("%s_%02d.xml", reportFileName, config.GinkgoConfig.ParallelNode))))
	}
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "MAPI Submit Transaction API Suite", r)

}

var _ = Describe("MAPI Submit Transaction API Suite", func() {
	var SubmitTx = "https://api.taal.com/mapi/tx"
	// var SubmitTxs = "https://api.taal.com/mapi/txs"
	var validTestnetKey = "testnet_16c64a607e21394ce01ddd8932e6f87e"

	Context("Submit Transaction ", func() {

		It("Submit Transaction with invalid key", func() {
			satsAmount := 2000 //amount of sats we are sending in tx
			alicePrivateKey := test.CreatePK1()
			aliceAddress := test.CreateAddress1(alicePrivateKey.PubKey())
			ToPrivateKey := test.CreatePK1()
			ToAddress := test.CreateAddress1(ToPrivateKey.PubKey())

			txid := test.GetFundsFromFaucet(aliceAddress)

			inputTx, _ := test.GetTransaction(txid)

			inputVout := test.GetVoutIndex(*inputTx, 0.01)

			tx := test.CreateNewTransactionAndSignNew(txid, *inputTx, inputVout, ToAddress, ToAddress, alicePrivateKey, satsAmount)

			reqBody, err := json.Marshal(map[string]string{
				"rawTx": tx.String(),
			})
			Expect(err).ShouldNot(HaveOccurred())

			response, body := test.HttpRequestDH_Post(SubmitTx, invalidTestnetKey, reqBody, "POST")
			time.Sleep(10 * time.Second)

			Expect(response.StatusCode).To(Equal(401))
			Expect(response.Status).To(Equal("401 Unauthorized"))
			Expect(string(body)).To(ContainSubstring("Account not found"))

		})

		It("Submit Transaction with invalid tx", func() {

			var invalidTx = "invalidhex"
			reqBody, err := json.Marshal(map[string]string{
				"rawTx": invalidTx,
			})
			Expect(err).ShouldNot(HaveOccurred())

			response, body := test.HttpRequestDH_Post(SubmitTx, validTestnetKey, reqBody, "POST")
			time.Sleep(10 * time.Second)

			Expect(response.StatusCode).To(Equal(400))
			Expect(response.Status).To(Equal("400 Bad Request"))
			Expect(string(body)).To(ContainSubstring("Failed to parse raw tx"))

		})

		It("Submit Transaction  successfull", func() {
			satsAmount := 2000 //amount of sats we are sending in tx
			alicePrivateKey := test.CreatePK1()
			aliceAddress := test.CreateAddress1(alicePrivateKey.PubKey())
			ToPrivateKey := test.CreatePK1()
			ToAddress := test.CreateAddress1(ToPrivateKey.PubKey())

			txid := test.GetFundsFromFaucet(aliceAddress)

			inputTx, _ := test.GetTransaction(txid)

			inputVout := test.GetVoutIndex(*inputTx, 0.01)

			time.Sleep(10 * time.Second)
			tx := test.CreateNewTransactionAndSignNew(txid, *inputTx, inputVout, ToAddress, ToAddress, alicePrivateKey, satsAmount)

			reqBody, err := json.Marshal(map[string]string{
				"rawTx": tx.String(),
			})
			Expect(err).ShouldNot(HaveOccurred())

			_, _ = test.HttpRequestDH_Post(SubmitTx, validTestnetKey, reqBody, "POST")

			response, body := test.HttpRequestDH_Post(SubmitTx, validTestnetKey, reqBody, "POST")
			time.Sleep(10 * time.Second)

			Expect(response.StatusCode).To(Equal(200))

			var output test.MapiBody
			json.Unmarshal(body, &output)

			Expect(output.Payload).Should(ContainSubstring("apiVersion"))
			Expect(output.Payload).Should(ContainSubstring("timestamp"))

		})

	})
})

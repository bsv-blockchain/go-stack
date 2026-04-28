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

func TestMAPIQueryTx(t *testing.T) {
	RegisterFailHandler(Fail) // connects gingko to gomega
	var r []ginkgo.Reporter
	reportDir := "../../allure-results/"
	reportFileName := "allure-report"
	if reportDir != "" {
		r = append(r, reporters.NewJUnitReporter(path.Join(reportDir, fmt.Sprintf("%s_%02d.xml", reportFileName, config.GinkgoConfig.ParallelNode))))
	}
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "MAPI Query Transaction API Suite", r)

}

var _ = Describe("MAPI Query Transaction API Suite", func() {
	var MapiTransactionQuery = "https://api.taal.com/mapi/tx"
	var validTestnetKey = "testnet_4aef81fcd8f87a12d8f36f2cf5528733"

	Context("Policy Quote", func() {

		It("Query Transaction with Valid TXid", func() {
			alicePrivateKey := test.CreatePK1()
			aliceAddress := test.CreateAddress1(alicePrivateKey.PubKey())

			txid := test.GetFundsFromFaucet(aliceAddress)

			endpoint := fmt.Sprintf("%v/%v", MapiTransactionQuery, txid)

			res, body := test.HttpRequestDH(endpoint, "GET", validTestnetKey)

			time.Sleep(5 * time.Second)
			Expect(res.StatusCode).To(Equal(200))
			var output test.MapiBody
			json.Unmarshal(body, &output)

			Expect(output.Payload).Should(ContainSubstring("apiVersion"))
			Expect(output.Payload).Should(ContainSubstring("timestamp"))
		})

		It("Mapi Query Transaction with Bad Method", func() {
			alicePrivateKey := test.CreatePK1()
			aliceAddress := test.CreateAddress1(alicePrivateKey.PubKey())

			txid := test.GetFundsFromFaucet(aliceAddress)

			endpoint := fmt.Sprintf("%v/%v", MapiTransactionQuery, txid)

			res, _ := test.HttpRequestDH(endpoint, "PATCH", validTestnetKey)
			Expect(res.StatusCode).To(Equal(405))

		})

		It("Query Transaction with Invalid TXid", func() {
			txidInvalid := "invalid"

			endpoint := fmt.Sprintf("%v/%v", MapiTransactionQuery, txidInvalid)

			res, body := test.HttpRequestDH(endpoint, "GET", validTestnetKey)
			Expect(res.StatusCode).To(Equal(400))
			Expect(string(body)).Should(ContainSubstring("Invalid format of TransactionId"))

		})

	})
})

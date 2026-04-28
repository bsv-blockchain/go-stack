package test

import (
	"fmt"
	"path"
	"testing"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/teranode-group/woc-api/test"
)

var BitcoinEndpoint = "https://api.taal.com/api/v1/bitcoin"
var mainnetKey = "mainnet_aed60d39b52b6049d7f881e4028ae194"
var invalidKey = "invalidhex"

func TestBitcoinEndpoints(t *testing.T) {
	RegisterFailHandler(Fail) // connects gingko to gomega
	var r []ginkgo.Reporter
	reportDir := "../../allure-results/"
	reportFileName := "allure-report"
	if reportDir != "" {
		r = append(r, reporters.NewJUnitReporter(path.Join(reportDir, fmt.Sprintf("%s_%02d.xml", reportFileName, config.GinkgoConfig.ParallelNode))))
	}
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "Bitcoin endpoints API Suite", r)

}

var _ = Describe("Bitcoin endpoints API Suite", func() {

	Context("Get txout", func() {

		It("Get Get txout with valid key", func() {

			url := fmt.Sprintf("%v", BitcoinEndpoint)

			var jsonStream string = "{\"jsonrpc\": \"1.0\", \"id\":\"postmantest\", \"method\": \"gettxout\", \"params\": [\"5ae748b3493dee130e9de12dc394089e8c21a99ff91b3cf795c95dc776bc1723\", 0]}"

			res, _ := test.HttpRequestDH_RQBody(url, "POST", mainnetKey, jsonStream)

			Expect(res.StatusCode).To(Equal(200))

		})

		It("Get txout with invalid key", func() {

			url := fmt.Sprintf("%v", BitcoinEndpoint)

			var jsonStream string = "{\"jsonrpc\": \"1.0\", \"id\":\"postmantest\", \"method\": \"gettxout\", \"params\": [\"5ae748b3493dee130e9de12dc394089e8c21a99ff91b3cf795c95dc776bc1723\", 0]}"

			res, _ := test.HttpRequestDH_RQBody(url, "POST", invalidKey, jsonStream)

			Expect(res.StatusCode).To(Equal(401))
			Expect(res.Status).To(Equal("401 Unauthorized"))

		})

	})

	Context("Get Best Block hash", func() {

		It("Get Best block hash with valid key", func() {

			url := fmt.Sprintf("%v", BitcoinEndpoint)

			var jsonStream string = "{\"jsonrpc\": \"1.0\", \"id\": \"postmantest\", \"method\": \"getbestblockhash\", \"params\": []}"

			res, _ := test.HttpRequestDH_RQBody(url, "POST", mainnetKey, jsonStream)

			Expect(res.StatusCode).To(Equal(200))

		})

		It("Get Best block hash with invalid key", func() {

			url := fmt.Sprintf("%v", BitcoinEndpoint)

			var jsonStream string = "{\"jsonrpc\": \"1.0\", \"id\": \"postmantest\", \"method\": \"getbestblockhash\", \"params\": []}"

			res, _ := test.HttpRequestDH_RQBody(url, "POST", invalidKey, jsonStream)

			Expect(res.StatusCode).To(Equal(401))
			Expect(res.Status).To(Equal("401 Unauthorized"))

		})

	})

	Context("Get raw transaction", func() {

		It("Get raw transaction with valid key", func() {

			url := fmt.Sprintf("%v", BitcoinEndpoint)

			var jsonStream string = " {\"jsonrpc\": \"1.0\", \"id\": \"postmantest\", \"method\": \"getrawtransaction\", \"params\": [\"5ae748b3493dee130e9de12dc394089e8c21a99ff91b3cf795c95dc776bc1723\", true]}"

			res, _ := test.HttpRequestDH_RQBody(url, "POST", mainnetKey, jsonStream)

			Expect(res.StatusCode).To(Equal(200))

		})

		It("Get raw transaction with invalid key", func() {

			url := fmt.Sprintf("%v", BitcoinEndpoint)

			var jsonStream string = " {\"jsonrpc\": \"1.0\", \"id\": \"postmantest\", \"method\": \"getrawtransaction\", \"params\": [\"5ae748b3493dee130e9de12dc394089e8c21a99ff91b3cf795c95dc776bc1723\", true]}"

			res, _ := test.HttpRequestDH_RQBody(url, "POST", invalidKey, jsonStream)

			Expect(res.StatusCode).To(Equal(401))
			Expect(res.Status).To(Equal("401 Unauthorized"))

		})

	})

})

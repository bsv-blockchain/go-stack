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

var ElectrumEndpoint = "https://api.taal.com/api/v1"

func TestElectrumAddress(t *testing.T) {
	RegisterFailHandler(Fail) // connects gingko to gomega
	var r []ginkgo.Reporter
	reportDir := "../../allure-results/"
	reportFileName := "allure-report"
	if reportDir != "" {
		r = append(r, reporters.NewJUnitReporter(path.Join(reportDir, fmt.Sprintf("%s_%02d.xml", reportFileName, config.GinkgoConfig.ParallelNode))))
	}
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "Electrum Address API Suite", r)

}

var _ = Describe("Electrum Address API Suite", func() {
	var AddressBalance string = "address/1HRADRLckTpFJJskkihZp16X6jR8JVRJcr/balance"

	Context("Get Address Balance ", func() {

		It("Get Address Balance with valid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressBalance)
			res, body := test.HttpRequestDH(url, "GET", mainnetKey)

			Expect(res.StatusCode).To(Equal(200))
			Expect(string(body)).To(ContainSubstring("confirmed"))

		})

		It("Submit Transaction with invalid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressBalance)
			res, body := test.HttpRequestDH(url, "GET", invalidKey)

			Expect(res.StatusCode).To(Equal(401))
			Expect(res.Status).To(Equal("401 Unauthorized"))
			Expect(string(body)).To(ContainSubstring("Account not found"))

		})

	})

	Context("Get Addresses Balance ", func() {
		var AddressesBalance string = "addresses/balance"

		It("Get Addresses Balance with valid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressesBalance)

			var jsonStream string = "{ \"addresses\": [\"16ZBEb7pp6mx5EAGrdeKivztd5eRJFuvYP\", \"1KGHhLTQaPr4LErrvbAuGE62yPpDoRwrob\"] }"

			res, _ := test.HttpRequestDH_RQBody(url, "POST", mainnetKey, jsonStream)

			Expect(res.StatusCode).To(Equal(200))

		})

		It("Submit Transaction with invalid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressesBalance)

			var jsonStream string = "{ \"addresses\": [\"16ZBEb7pp6mx5EAGrdeKivztd5eRJFuvYP\", \"1KGHhLTQaPr4LErrvbAuGE62yPpDoRwrob\"] }"
			res, _ := test.HttpRequestDH_RQBody(url, "POST", invalidKey, jsonStream)

			Expect(res.StatusCode).To(Equal(401))
			Expect(res.Status).To(Equal("401 Unauthorized"))

		})

	})

	Context("Get Addresses History ", func() {
		var AddressesHistory string = "addresses/history"

		It("Get Addresses History with valid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressesHistory)

			var jsonStream string = "{ \"addresses\": [\"16ZBEb7pp6mx5EAGrdeKivztd5eRJFuvYP\", \"1KGHhLTQaPr4LErrvbAuGE62yPpDoRwrob\"] }"
			res, _ := test.HttpRequestDH_RQBody(url, "POST", mainnetKey, jsonStream)
			Expect(res.StatusCode).To(Equal(200))

		})

		It("Get Addresses History with invalid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressesHistory)

			var jsonStream string = "{ \"addresses\": [\"16ZBEb7pp6mx5EAGrdeKivztd5eRJFuvYP\", \"1KGHhLTQaPr4LErrvbAuGE62yPpDoRwrob\"] }"
			res, _ := test.HttpRequestDH_RQBody(url, "POST", invalidKey, jsonStream)

			Expect(res.StatusCode).To(Equal(401))
			Expect(res.Status).To(Equal("401 Unauthorized"))

		})

	})

	Context("Get Address History ", func() {
		var AddressHistory string = "address/1HZJ3kKqhsHgz7oyK52GG2QkzyrtPPwcvx/history"

		It("Get Addresses History with valid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressHistory)

			res, _ := test.HttpRequestDH(url, "GET", mainnetKey)

			Expect(res.StatusCode).To(Equal(200))

		})

		It("Get Address History with invalid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressHistory)

			res, _ := test.HttpRequestDH(url, "GET", invalidKey)

			Expect(res.StatusCode).To(Equal(401))
			Expect(res.Status).To(Equal("401 Unauthorized"))

		})

	})

	Context("Get Address Info ", func() {
		var AddressInfo string = "address/1HZJ3kKqhsHgz7oyK52GG2QkzyrtPPwcvx/info"

		It("Get Addresses History with valid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressInfo)

			res, _ := test.HttpRequestDH(url, "GET", mainnetKey)

			Expect(res.StatusCode).To(Equal(200))

		})

		It("Get Address History with invalid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressInfo)

			res, _ := test.HttpRequestDH(url, "GET", invalidKey)

			Expect(res.StatusCode).To(Equal(401))
			Expect(res.Status).To(Equal("401 Unauthorized"))

		})

	})

	Context("Get Address Script Hash Balance ", func() {
		var AddressHashBalance string = "address/hash/933e2ab80b99749dbf8c22904899e8279056ce38644f64ab7313b8372c865ffb/balance"

		It("Get Addresses Script Hash Balance with valid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressHashBalance)

			res, _ := test.HttpRequestDH(url, "GET", mainnetKey)

			Expect(res.StatusCode).To(Equal(200))

		})

		It("Get Address History with invalid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressHashBalance)

			res, _ := test.HttpRequestDH(url, "GET", invalidKey)

			Expect(res.StatusCode).To(Equal(401))
			Expect(res.Status).To(Equal("401 Unauthorized"))

		})

	})

	Context("Get Address Script Hash History", func() {
		var AddressHashHistory string = "address/hash/933e2ab80b99749dbf8c22904899e8279056ce38644f64ab7313b8372c865ffb/history"

		It("Get Addresses Script Hash Balance with valid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressHashHistory)

			res, _ := test.HttpRequestDH(url, "GET", mainnetKey)

			Expect(res.StatusCode).To(Equal(200))

		})

		It("Get Address History with invalid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressHashHistory)

			res, _ := test.HttpRequestDH(url, "GET", invalidKey)

			Expect(res.StatusCode).To(Equal(401))
			Expect(res.Status).To(Equal("401 Unauthorized"))

		})

	})

	Context("Get Address Script Hash unspent", func() {
		var AddressHashUnspent string = "address/hash/933e2ab80b99749dbf8c22904899e8279056ce38644f64ab7313b8372c865ffb/unspent"

		It("Get Addresses Script Hash Balance with valid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressHashUnspent)

			res, _ := test.HttpRequestDH(url, "GET", mainnetKey)

			Expect(res.StatusCode).To(Equal(200))

		})

		It("Get Address History with invalid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressHashUnspent)

			res, _ := test.HttpRequestDH(url, "GET", invalidKey)

			Expect(res.StatusCode).To(Equal(401))
			Expect(res.Status).To(Equal("401 Unauthorized"))

		})

	})

	Context("Get Address unspent", func() {
		var AddressUnspent string = "address/1HRADRLckTpFJJskkihZp16X6jR8JVRJcr/unspent"

		It("Get Addresses unspent with valid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressUnspent)

			res, _ := test.HttpRequestDH(url, "GET", mainnetKey)

			Expect(res.StatusCode).To(Equal(200))

		})

		It("Get Address unspent with invalid key", func() {

			url := fmt.Sprintf("%v/%v", ElectrumEndpoint, AddressUnspent)

			res, _ := test.HttpRequestDH(url, "GET", invalidKey)

			Expect(res.StatusCode).To(Equal(401))
			Expect(res.Status).To(Equal("401 Unauthorized"))

		})

	})

})

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

func TestMAPIPolicyQuote(t *testing.T) {
	RegisterFailHandler(Fail) // connects gingko to gomega
	var r []ginkgo.Reporter
	reportDir := "../../allure-results/"
	reportFileName := "allure-report"
	if reportDir != "" {
		r = append(r, reporters.NewJUnitReporter(path.Join(reportDir, fmt.Sprintf("%s_%02d.xml", reportFileName, config.GinkgoConfig.ParallelNode))))
	}
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "MAPI Policy Quote API Suite", r)

}

var _ = Describe("MAPI Policy Quote API Suite", func() {
	var MapiPolicyQuote string = "https://api.taal.com/mapi/policyQuote"
	var validTestnetKey = "testnet_4aef81fcd8f87a12d8f36f2cf5528733"

	Context("Policy Quote", func() {

		It("get Policy Quote successfull", func() {
			res, body := test.HttpRequestDH(MapiPolicyQuote, "GET", validTestnetKey)

			time.Sleep(5 * time.Second)
			Expect(res.StatusCode).To(Equal(200))
			var output test.MapiBody
			json.Unmarshal(body, &output)

			Expect(output.Payload).Should(ContainSubstring("apiVersion"))
			Expect(output.Payload).Should(ContainSubstring("timestamp"))
		})
		It("get Policy Quote with wrong Method", func() {
			res, _ := test.HttpRequestDH(MapiPolicyQuote, "PATCH", validTestnetKey)
			Expect(res.StatusCode).To(Equal(405))
		})
	})
})

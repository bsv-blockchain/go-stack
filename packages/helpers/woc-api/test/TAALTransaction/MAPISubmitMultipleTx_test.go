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

func TestMAPIMultipleTx(t *testing.T) {
	RegisterFailHandler(Fail) // connects gingko to gomega
	var r []ginkgo.Reporter
	reportDir := "../../allure-results/"
	reportFileName := "allure-report"
	if reportDir != "" {
		r = append(r, reporters.NewJUnitReporter(path.Join(reportDir, fmt.Sprintf("%s_%02d.xml", reportFileName, config.GinkgoConfig.ParallelNode))))
	}
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "MAPI submit multiple tx API Suite", r)

}

var _ = Describe("MAPI submit multiple txs Suite", func() {
	var SubmitTxs = "https://api.taal.com/mapi/txs"
	var validTestnetKey = "testnet_4aef81fcd8f87a12d8f36f2cf5528733"

	Context("submit multiple txs", func() {

		It("submit multiple txs successfull", func() {

			satsAmount := 2000 //amount of sats we are sending in tx
			alicePrivateKey := test.CreatePK1()
			aliceAddress := test.CreateAddress1(alicePrivateKey.PubKey())
			ToPrivateKey := test.CreatePK1()
			ToAddress := test.CreateAddress1(ToPrivateKey.PubKey())
			JohnPrivateKey := test.CreatePK1()
			JohnAddress := test.CreateAddress1(JohnPrivateKey.PubKey())
			ToPrivateKey2 := test.CreatePK1()
			ToAddress2 := test.CreateAddress1(ToPrivateKey2.PubKey())

			txid1 := test.GetFundsFromFaucet(aliceAddress)

			txid2 := test.GetFundsFromFaucet(JohnAddress)

			inputTx1, _ := test.GetTransaction(txid1)

			inputVout1 := test.GetVoutIndex(*inputTx1, 0.01)

			inputTx2, _ := test.GetTransaction(txid2)

			inputVout2 := test.GetVoutIndex(*inputTx2, 0.01)

			time.Sleep(10 * time.Second)
			tx1 := test.CreateNewTransactionAndSignNew(txid1, *inputTx1, inputVout1, ToAddress, ToAddress, alicePrivateKey, satsAmount)

			time.Sleep(10 * time.Second)
			tx2 := test.CreateNewTransactionAndSignNew(txid2, *inputTx2, inputVout2, ToAddress2, ToAddress2, JohnPrivateKey, satsAmount)

			fmt.Sprintf(tx2.String())

			type RawTx struct {
				RawTx string `json:"rawTx"`
			}

			reqBody := []RawTx{
				{RawTx: fmt.Sprintf(`%v`, tx1.String())},
				{RawTx: fmt.Sprintf(`%v`, tx2.String())},
			}

			body, _ := json.Marshal(&reqBody)

			_, _ = test.HttpRequestDH_Post(SubmitTxs, validTestnetKey, body, "POST")
			time.Sleep(20 * time.Second)
			response, body := test.HttpRequestDH_Post(SubmitTxs, validTestnetKey, body, "POST")
			time.Sleep(10 * time.Second)
			fmt.Println(response)
			fmt.Println(string(body))
			Expect(response.StatusCode).To(Equal(200))

			var output test.MapiBody
			json.Unmarshal(body, &output)

			Expect(output.Payload).Should(ContainSubstring("apiVersion"))
			Expect(output.Payload).Should(ContainSubstring("timestamp"))

		})

	})
})

package test

import (
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

var TapiBroadcast = "https://api.taal.com/api/v1/broadcast"

var validTestnetKey = "testnet_4aef81fcd8f87a12d8f36f2cf5528733"
var invalidTestnetKey = "testnet_4aef81fcd8f87a12d8f36f2cf552"
var invalidTx = "0100000001b5f0b144e5706b18d2dc95cbb3844c545528f361fa4af8b4ab3a0006648462d2010000006a4730440220445ad646fb54cd893a6b9f6b10fb3de8fbb952cf7196d492211f43c8e41757c1022043c3cc9a8e12820658599eb918f94da814258bc85abd56aaafe653019ce3d42341210259b4153cf16f487749d036a8b34c5e5840a849d8e182bd082c6acc6efd950cc4ffffffff02e8030000000000001976a9149eb0b6cdb004195fd9b2ed8907e84d02a4ec48d788ace73d0f00000000001976a9149eb0b6cdb004195fd9b2ed8907e84d02a4ec48d788ac00000001"

func TestBroadcastTapi(t *testing.T) {
	RegisterFailHandler(Fail) // connects gingko to gomega
	var r []ginkgo.Reporter
	reportDir := "../../allure-results/"
	reportFileName := "allure-report"
	if reportDir != "" {
		r = append(r, reporters.NewJUnitReporter(path.Join(reportDir, fmt.Sprintf("%s_%02d.xml", reportFileName, config.GinkgoConfig.ParallelNode))))
	}
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "TAPI Broadcast API Suite", r)

}

var _ = Describe("TAPI Broadcast API Suite", func() {
	var TapiBroadcast = "https://api.taal.com/api/v1/broadcast"

	// var validTestnetKey = "testnet_4aef81fcd8f87a12d8f36f2cf5528733"
	// var invalidTestnetKey = "testnet_4aef81fcd8f87a12d8f36f2cf552"
	// var invalidTx = "0100000001b5f0b144e5706b18d2dc95cbb3844c545528f361fa4af8b4ab3a0006648462d2010000006a4730440220445ad646fb54cd893a6b9f6b10fb3de8fbb952cf7196d492211f43c8e41757c1022043c3cc9a8e12820658599eb918f94da814258bc85abd56aaafe653019ce3d42341210259b4153cf16f487749d036a8b34c5e5840a849d8e182bd082c6acc6efd950cc4ffffffff02e8030000000000001976a9149eb0b6cdb004195fd9b2ed8907e84d02a4ec48d788ace73d0f00000000001976a9149eb0b6cdb004195fd9b2ed8907e84d02a4ec48d788ac00000001"

	Context("Test Broadcast Tapi", func() {

		It("Tapi Broadcast with invalid key", func() {
			satsAmount := 2000 //amount of sats we are sending in tx
			kameshPrivateKey := test.CreatePK1()
			kameshAddress := test.CreateAddress1(kameshPrivateKey.PubKey())
			alicePrivateKey := test.CreatePK1()
			aliceAddress := test.CreateAddress1(alicePrivateKey.PubKey())
			txid := test.GetFundsFromFaucet(aliceAddress)

			inputTx, err := test.GetTransaction(txid)
			Expect(err).ShouldNot(HaveOccurred())

			inputVout := test.GetVoutIndex(*inputTx, 0.01)

			tx := test.CreateNewTransactionAndSignNew(txid, *inputTx, inputVout, kameshAddress, kameshAddress, alicePrivateKey, satsAmount)

			response := test.HttpRequest(TapiBroadcast, "POST", tx.String(), invalidTestnetKey)
			time.Sleep(10 * time.Second)

			Expect(string(response)).Should(ContainSubstring("Account not found"))

		})

		It("Tapi Broadcast 100 sats sucessfull", func() {

			satsAmount2 := 1000 //amount of sats we are sending in tx
			johnPrivateKey := test.CreatePK1()
			johnAddress := test.CreateAddress1(johnPrivateKey.PubKey())
			toPrivateKey := test.CreatePK1()
			toAddress := test.CreateAddress1(toPrivateKey.PubKey())
			time.Sleep(5 * time.Second)
			txid2 := test.GetFundsFromFaucet(johnAddress)

			inputTx2, _ := test.GetTransaction(txid2)

			inputVout2 := test.GetVoutIndex(*inputTx2, 0.01)

			tx2 := test.CreateNewTransactionAndSignNew(txid2, *inputTx2, inputVout2, toAddress, toAddress, johnPrivateKey, satsAmount2)

			response2 := test.HttpRequest(TapiBroadcast, "POST", tx2.String(), validTestnetKey)

			time.Sleep(10 * time.Second) //wait for tx to propagate
			txResult2, err := test.GetTransaction(string(response2))
			time.Sleep(10 * time.Second)
			if err != nil {
				Expect(string(err.Error())).Should(ContainSubstring("400"))

				response3 := test.HttpRequest(TapiBroadcast, "POST", tx2.String(), validTestnetKey)

				time.Sleep(10 * time.Second) //wait for tx to propagate
				txResult3, err := test.GetTransaction(string(response3))
				Expect(err).ShouldNot(HaveOccurred())
				Expect(txResult3.Vouts[0].Value).To(Equal(1e-05))
				Expect(txResult3.Vouts[1].Value).To(Equal(0.00998887))
			} else {
				Expect(err).ShouldNot(HaveOccurred())
				Expect(txResult2.Vouts[0].Value).To(Equal(1e-05))
				Expect(txResult2.Vouts[1].Value).To(Equal(0.00998887))
				Expect(test.ConvertToSats(txResult2.Vouts[0].Value)).To(Equal(uint64(satsAmount2)))
			}

		})

		It("Tapi Broadcast with no api key", func() {
			satsAmount := 2000 //amount of sats we are sending in tx
			kameshPrivateKey := test.CreatePK1()
			kameshAddress := test.CreateAddress1(kameshPrivateKey.PubKey())
			alicePrivateKey := test.CreatePK1()
			aliceAddress := test.CreateAddress1(alicePrivateKey.PubKey())
			txid := test.GetFundsFromFaucet(aliceAddress)

			inputTx, err := test.GetTransaction(txid)
			Expect(err).ShouldNot(HaveOccurred())

			inputVout := test.GetVoutIndex(*inputTx, 0.01)

			tx := test.CreateNewTransactionAndSignNew(txid, *inputTx, inputVout, kameshAddress, kameshAddress, alicePrivateKey, satsAmount)

			response := test.HttpRequestWithoutApiKey(TapiBroadcast, "POST", tx.String())
			time.Sleep(10 * time.Second)

			Expect(string(response)).Should(ContainSubstring("Anonymous access not allowed"))

		})

		It("Tapi Broadcast incorrect hex", func() {

			response := test.HttpRequest(TapiBroadcast, "POST", invalidTx, validTestnetKey)
			Expect(string(response)).Should(ContainSubstring("missing-inputs"))

		})

		It("Tapi Broadcast incorrect signature", func() {

			satsAmount := 2000 //amount of sats we are sending in tx
			alicePrivateKey := test.CreatePK1()
			aliceAddress := test.CreateAddress1(alicePrivateKey.PubKey())
			kameshPrivateKey := test.CreatePK1()
			kameshAddress := test.CreateAddress1(kameshPrivateKey.PubKey())

			txid := test.GetFundsFromFaucet(aliceAddress)

			inputTx, _ := test.GetTransaction(txid)

			inputVout := test.GetVoutIndex(*inputTx, 0.01)

			tx := test.CreateNewTransactionAndSignNew(txid, *inputTx, inputVout, kameshAddress, kameshAddress, kameshPrivateKey, satsAmount)
			time.Sleep(10 * time.Second)
			response := test.HttpRequest(TapiBroadcast, "POST", tx.String(), validTestnetKey)
			time.Sleep(20 * time.Second)
			Expect(string(response)).Should(ContainSubstring("mandatory-script-verify-flag-failed"))

		})
		It("Tapi Broadcast 2000 sats successfull", func() {
			satsAmount := 2000 //amount of sats we are sending in tx
			alicePrivateKey := test.CreatePK1()
			aliceAddress := test.CreateAddress1(alicePrivateKey.PubKey())
			kameshPrivateKey := test.CreatePK1()
			kameshAddress := test.CreateAddress1(kameshPrivateKey.PubKey())

			txid := test.GetFundsFromFaucet(aliceAddress)

			inputTx, _ := test.GetTransaction(txid)

			inputVout := test.GetVoutIndex(*inputTx, 0.01)

			tx := test.CreateNewTransactionAndSignNew(txid, *inputTx, inputVout, kameshAddress, kameshAddress, alicePrivateKey, satsAmount)

			// t.Log("tx" + tx.String())
			response := test.HttpRequest(TapiBroadcast, "POST", tx.String(), validTestnetKey)
			fmt.Print(string(response))

			time.Sleep(20 * time.Second) //wait for tx to propagate
			txResult, err := test.GetTransaction(string(response))
			time.Sleep(5 * time.Second)
			if err != nil {
				Expect(string(err.Error())).Should(ContainSubstring("400"))
				response2 := test.HttpRequest(TapiBroadcast, "POST", tx.String(), validTestnetKey)

				time.Sleep(5 * time.Second) //wait for tx to propagate
				txResult2, err := test.GetTransaction(string(response2))
				Expect(err).ShouldNot(HaveOccurred())
				Expect(txResult2.Vouts[0].Value).To(Equal(2e-05))
				Expect(txResult2.Vouts[1].Value).To(Equal(0.00997887))
			} else {
				Expect(err).ShouldNot(HaveOccurred())
				Expect(txResult.Vouts[0].Value).To(Equal(2e-05))
				Expect(txResult.Vouts[1].Value).To(Equal(0.00997887))
				Expect(test.ConvertToSats(txResult.Vouts[0].Value)).To(Equal(uint64(satsAmount)))
			}

		})

	})

})

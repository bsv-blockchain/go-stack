package test

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/libsv/go-bk/bec"
	"github.com/libsv/go-bk/chaincfg"
	"github.com/libsv/go-bk/wif"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/unlocker"
	"github.com/stretchr/testify/assert"
)

var mapiBroadcast = "https://mapi.taal.com/api/v1/broadcast"
var validTestnetKey = "testnet_4aef81fcd8f87a12d8f36f2cf5528733"
var invalidTestnetKey = "testnet_4aef81fcd8f87a12d8f36f2cf552"

/*
	Further Tests;
	broadcast Low sat amounts (around 100 sats)
	broadcast 1 sat
	broadcast zero sat
	Missing API key
	Incorrect API Key
	Missing Tx Hex
	Incorrect Tx Hex
	Duplicate Tx
	Incorrect Private Key
	WOC broadcast
*/
func TestMapiBroadcast(t *testing.T) {

	t.Run("Mapi Broadcast", func(t *testing.T) {

		satsAmount := 2000 //amount of sats we are sending in tx
		alicePrivateKey := CreatePK(t)
		aliceAddress := CreateAddress(t, alicePrivateKey.PubKey())
		kameshPrivateKey := CreatePK(t)
		kameshAddress := CreateAddress(t, kameshPrivateKey.PubKey())

		txid := GetFundsFromFaucet(aliceAddress)
		t.Log(txid)

		inputTx, _ := GetTransaction(txid)
		t.Log(inputTx.Vouts[0].Value)
		var inputVout int
		for i := 0; i < len(inputTx.Vouts); i++ {
			if inputTx.Vouts[i].Value == 0.01 {
				inputVout = i
			}

		}
		tx := bt.NewTx()

		_ = tx.From(
			txid,
			inputTx.Vouts[inputVout].Vout,
			inputTx.Vouts[inputVout].ScriptPubKey.Hex,
			ConvertToSats(inputTx.Vouts[inputVout].Value),
		)

		_ = tx.PayToAddress(kameshAddress, uint64(satsAmount))
		_ = tx.ChangeToAddress(kameshAddress, bt.NewFeeQuote())

		wifNew, _ := wif.NewWIF((*bec.PrivateKey)(alicePrivateKey), &chaincfg.MainNet, false)
		decodedWif, err := wif.DecodeWIF(wifNew.String())
		assert.Nil(t, err, "Error returned %v", err)

		if err := tx.FillAllInputs(context.Background(), &unlocker.Getter{PrivateKey: decodedWif.PrivKey}); err != nil {
			log.Fatal("error here--- " + err.Error())
		}
		log.Printf("tx: %s\n", tx)

		response := HttpRequest(mapiBroadcast, "POST", tx.String(), validTestnetKey)
		time.Sleep(5 * time.Second) //wait for tx to propagate
		txResult, err := GetTransaction(string(response))
		assert.Nil(t, err, "Error: %v", err)
		if err == nil {
			assert.Equal(t, ConvertToSats(txResult.Vouts[0].Value), uint64(satsAmount))
		}
	})

	t.Run("Mapi Broadcast with invalid key", func(t *testing.T) {

		//for now we use hardcoded txid, when we get a testnet faucet we can do this programatically
		txid := "d38dbe649e9e92fc577e2b03f3ab131f72505909c798d4e4cd755d9581e76b94"
		txVout := 1        // the output index for above tx id
		satsAmount := 2000 //amount of sats we are sending in tx
		inputTx, err := GetTransaction(txid)
		assert.Nil(t, err, "Error retrieving transaction %v", err)
		t.Log(inputTx)

		tx := bt.NewTx()

		_ = tx.From(
			txid,
			inputTx.Vouts[txVout].Vout,
			inputTx.Vouts[txVout].ScriptPubKey.Hex,
			ConvertToSats(inputTx.Vouts[txVout].Value),
		)

		_ = tx.PayToAddress("mqz2RSpt6cH4u1VQpRVQf8iYKMuSrZvX9W", uint64(satsAmount))
		_ = tx.ChangeToAddress("mqz2RSpt6cH4u1VQpRVQf8iYKMuSrZvX9W", bt.NewFeeQuote())

		decodedWif, err := wif.DecodeWIF("KzLCYEAopK8RxmLnQTACS6oUirmzSqpNMwDcv7KgdLwUTijNbr7k")
		assert.Nil(t, err, "Error returned %v", err)

		if err := tx.FillAllInputs(context.Background(), &unlocker.Getter{PrivateKey: decodedWif.PrivKey}); err != nil {
			log.Fatal("error here--- " + err.Error())
		}
		log.Printf("tx: %s\n", tx)

		response := HttpRequest(mapiBroadcast, "POST", tx.String(), invalidTestnetKey)
		t.Log(string(response))
		assert.Contains(t, string(response), "Account not found")
	})

}

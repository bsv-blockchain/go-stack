package slack

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/ordishs/gocore"
)

var logger = gocore.Log("woc-api")

type TextMessage struct {
	Msg string `json:"text"`
}

var slackMsgEnabled bool
var slackWebhookUrl string

func init() {

	slackMsgEnabled = gocore.Config().GetBool("slackWebhookEnabled", false)

	if slackMsgEnabled {
		logger.Info("INFO: slackMsg Enabled for testing")
	} else {
		logger.Info("INFO: slackMsg Disabled for testing")
	}

	slackWebhookUrl, _ = gocore.Config().Get("slackWebhookUrl")
}

func SendTextMsg(msg string) {

	if !slackMsgEnabled {
		logger.Errorf("SendTextMsg - slack msg requested but not enabled in settings.")
		return
	}

	msgToSend := &TextMessage{Msg: msg}
	jsonStr, err := json.Marshal(msgToSend)
	if err != nil {
		logger.Errorf("SendTextMsg - Failed to marshal json  %s, %s", jsonStr, err)
	}

	req, _ := http.NewRequest("POST", slackWebhookUrl, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("SendTextMsg - Failed to send slack Msg %s, %s", jsonStr, err)
	}
	defer resp.Body.Close()

}

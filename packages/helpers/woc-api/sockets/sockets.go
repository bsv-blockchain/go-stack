package sockets

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type Status struct {
	Status                string `json:"status"`
	LastBlockHeight       uint64 `json:"lastBlockHeight"`
	LastTimeBlockReceived string `json:"lastTimeBlockReceived"`
	LastTimeTxReceived    string `json:"lastTimeTxReceived"`
	UpTime                string `json:"upTime"`
}

func HealthCheck(ctx context.Context, healthUrl string) (Status, error) {
	var resp Status
	httpClient := http.Client{
		Timeout: time.Second * 3,
	}
	reqHttp, err := http.NewRequest(http.MethodGet, healthUrl, nil)
	if err != nil {
		return resp, fmt.Errorf("can't create new http request %+v", err)
	}

	resHttp, getErr := httpClient.Do(reqHttp)
	if getErr != nil {
		return resp, fmt.Errorf("can't get request %+v", getErr)
	}

	if resHttp.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("wrong sockets health response status code %d", resHttp.StatusCode)
	}
	body, readErr := ioutil.ReadAll(resHttp.Body)
	if readErr != nil {
		return resp, fmt.Errorf("can't read poolConfig file %+v", readErr)
	}
	jsonErr := json.Unmarshal(body, &resp)
	if jsonErr != nil {
		return resp, fmt.Errorf("can't unmarshal poolConfig file %+v", jsonErr)
	}

	return resp, nil
}

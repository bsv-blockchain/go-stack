package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/ordishs/gocore"
)

type ReportRequestBody struct {
	TxId    string `json:"txid"`
	Message string `json:"message"`
}

type RequestFieldValues struct {
	Summary     string `json:"summary"`
	Description string `json:"description"`
}

type ServiceRequest struct {
	ServiceDeskID      int                `json:"serviceDeskId"`
	RequestTypeID      int                `json:"requestTypeId"`
	RequestFieldValues RequestFieldValues `json:"requestFieldValues"`
	RaisedOnBehalfOf   string             `json:"raiseOnBehalfOf"`
}

type jiraCustomer struct {
	Email    string `json:"email"`
	FullName string `json:"fullName"`
}

type jiraResponse struct {
	IssueKey     string `json:"issueKey,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

func ReportTxHandler(w http.ResponseWriter, r *http.Request) {

	clientEmail, _ := gocore.Config().Get("jiraClientEmail")
	clientName, _ := gocore.Config().Get("jiraClientName")
	subject, _ := gocore.Config().Get("jiraSummary")

	var reportBody ReportRequestBody
	b, _ := ioutil.ReadAll(r.Body)
	err := json.Unmarshal(b, &reportBody)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(reportBody.TxId) != 64 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := createJiraCustomer(&jiraCustomer{
		Email:    clientEmail,
		FullName: clientName,
	}); err != nil {
		logger.Errorf("Unable to create Jira Customer for request :%+v , %s", reportBody, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	_, err = createJiraServiceRequest(&ServiceRequest{
		ServiceDeskID: 1,
		RequestTypeID: 2,
		RequestFieldValues: RequestFieldValues{
			Summary:     subject,
			Description: fmt.Sprintf("As a WoC user i would like to report \n TxId: \n %s \n Reason: \n %s", reportBody.TxId, reportBody.Message),
		},
		RaisedOnBehalfOf: clientEmail,
	})

	logger.Infof("%s :%+v", subject, reportBody)

	if err != nil {
		logger.Errorf("Unable to create Jira Customer for request :%+v , %s", reportBody, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func createJiraServiceRequest(payload *ServiceRequest) (*jiraResponse, error) {
	baseUrl, _ := gocore.Config().Get("jiraBaseUrl")
	serviceRequestUrl, _ := gocore.Config().Get("jiraServiceRequestUrl")
	url := baseUrl + serviceRequestUrl

	username, _ := gocore.Config().Get("jiraUser")
	password, _ := gocore.Config().Get("jiraApiKey")

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	payloadBuf := new(bytes.Buffer)
	if err := json.NewEncoder(payloadBuf).Encode(payload); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, payloadBuf)
	if err != nil {
		return nil, fmt.Errorf("Got error %s", err.Error())
	}

	req.SetBasicAuth(username, password)
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Got error %s", err.Error())
	}
	defer res.Body.Close()

	// fmt.Println("response Status:", res.Status)
	// fmt.Println("response Headers:", res.Header)

	var response jiraResponse
	decoder := json.NewDecoder(res.Body)
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}

	if res.StatusCode == 201 {
		return &response, nil
	}

	return nil, fmt.Errorf("Error %d creating jira customer: %s", res.StatusCode, response.ErrorMessage)
}

func createJiraCustomer(customer *jiraCustomer) error {
	baseUrl, _ := gocore.Config().Get("jiraBaseUrl")
	customerUrl, _ := gocore.Config().Get("jiraCustomerUrl")
	url := baseUrl + customerUrl

	username, _ := gocore.Config().Get("jiraUser")
	password, _ := gocore.Config().Get("jiraApiKey")

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	payloadBuf := new(bytes.Buffer)
	if err := json.NewEncoder(payloadBuf).Encode(customer); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, payloadBuf)
	if err != nil {
		return fmt.Errorf("Got error %s", err.Error())
	}

	req.SetBasicAuth(username, password)
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Got error %s", err.Error())
	}
	defer res.Body.Close()

	if res.StatusCode == 201 {
		return nil
	}

	// fmt.Println("response Status:", res.Status)
	// fmt.Println("response Headers:", res.Header)

	var response jiraResponse
	decoder := json.NewDecoder(res.Body)
	if err := decoder.Decode(&response); err != nil {
		return err
	}

	if res.StatusCode == 400 && strings.Contains(response.ErrorMessage, "An account already exists for this email") {
		return nil // Customer already exists - so no error
	}

	return fmt.Errorf("Error %d creating jira customer: %s", res.StatusCode, response.ErrorMessage)
}

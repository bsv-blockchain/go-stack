package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/ordishs/gocore"
)

const siteVerifyURL = "https://www.google.com/recaptcha/api/siteverify"

type SiteVerifyRequest struct {
	RecaptchaResponse string `json:"g-recaptcha-response"`
}

type SiteVerifyResponse struct {
	Success     bool      `json:"success"`
	Score       float64   `json:"score"`
	Action      string    `json:"action"`
	ChallengeTS time.Time `json:"challenge_ts"`
	Hostname    string    `json:"hostname"`
	ErrorCodes  []string  `json:"error-codes"`
}

func RecaptchaMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		if !gocore.Config().GetBool("recaptcha_enabled", true) {
			next(w, req)
			return
		}

		bodyBytes, err := ioutil.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode("Unauthorized")
			return
		}

		req.Body.Close()

		// Restore request body to read more than once.
		req.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

		// Unmarshal body into struct.
		var body SiteVerifyRequest
		if err := json.Unmarshal(bodyBytes, &body); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode("Unauthorized")
			return
		}

		// Check and verify the recaptcha response token.
		if err := checkRecaptcha(body.RecaptchaResponse); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode("Unauthorized")
			return
		}

		next(w, req)
	})
}

func checkRecaptcha(response string) error {

	secret, found := gocore.Config().Get("recaptcha_secret")
	if !found {
		return errors.New("No recaptcha_secret setting provided")
	}

	scoreThresholdPercent, found := gocore.Config().GetInt("recaptcha_score_threshold_percent")
	if !found {
		return errors.New("No recaptcha_score_threshold_percent setting provided")
	}

	req, err := http.NewRequest(http.MethodPost, siteVerifyURL, nil)
	if err != nil {
		return err
	}

	// Add necessary request parameters.
	q := req.URL.Query()
	q.Add("secret", secret)
	q.Add("response", response)
	req.URL.RawQuery = q.Encode()

	// Make request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Decode response.
	var body SiteVerifyResponse
	if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return err
	}

	// Check recaptcha verification success.
	if !body.Success {
		return errors.New("unsuccessful recaptcha verify request")
	}

	// Check response score.
	if body.Score < float64(scoreThresholdPercent/100) {
		return errors.New("lower received score than expected")
	}

	return nil
}

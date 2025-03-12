package exchange

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// retryDelays — exponential backoff при HTTP 429
var retryDelays = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

// doRequest виконує запит через фабрику (для коректного retry без читання body двічі).
// dest — вказівник на структуру для json.Unmarshal.
func doRequest(buildReq func() (*http.Request, error), dest interface{}) error {
	var lastErr error

	for attempt := 0; attempt <= len(retryDelays); attempt++ {
		req, err := buildReq()
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < len(retryDelays) {
			resp.Body.Close()
			time.Sleep(retryDelays[attempt])
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("read body: %w", err)
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
		}

		return json.Unmarshal(body, dest)
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// getPublic — простий GET без авторизації
func getPublic(url string, dest interface{}) error {
	return doRequest(func() (*http.Request, error) {
		return http.NewRequest(http.MethodGet, url, nil)
	}, dest)
}

// getJSON — GET з довільними заголовками
func getJSON(url string, headers map[string]string, dest interface{}) error {
	return doRequest(func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		return req, nil
	}, dest)
}

// postJSON — POST з JSON body і заголовками
func postJSON(url string, body string, headers map[string]string, dest interface{}) error {
	return doRequest(func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		return req, nil
	}, dest)
}

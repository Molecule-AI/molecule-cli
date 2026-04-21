package cmd

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// runHTTP does a raw HTTP call.
func runHTTP(method, url string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(method, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d — %s", resp.StatusCode, string(b))
	}
	return b, nil
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// paymentPointerHost extracts the host from a payment pointer string.
// E.g. "$ilp.example.com/alice" => "ilp.example.com"
func paymentPointerHost(pp string) (string, error) {
	pp = strings.TrimSpace(pp)
	if pp == "" {
		return "", fmt.Errorf("empty payment pointer")
	}
	if strings.HasPrefix(pp, "$") {
		pp = "https://" + strings.TrimPrefix(pp, "$")
	}
	u, err := url.Parse(pp)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", fmt.Errorf("no host")
	}
	return u.Host, nil
}

// paymentPointerURL converts a payment pointer string to an HTTPS URL for Open Payments.
func paymentPointerURL(pp string) (string, error) {
	pp = strings.TrimSpace(pp)
	if pp == "" {
		return "", fmt.Errorf("empty payment pointer")
	}
	if strings.HasPrefix(pp, "$") {
		pp = "https://" + strings.TrimPrefix(pp, "$")
	}
	u, err := url.Parse(pp)
	if err != nil {
		return "", err
	}
	if u.Scheme != "https" || u.Host == "" {
		return "", fmt.Errorf("invalid pointer url")
	}
	return u.String(), nil
}

// fetchWalletAsset fetches the wallet address JSON and returns its asset code/scale.
func fetchWalletAsset(pp string) (string, int, error) {
	urlStr, err := paymentPointerURL(pp)
	if err != nil {
		return "", 0, err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, urlStr, nil)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, fmt.Errorf("wallet fetch failed: %s", resp.Status)
	}
	var payload struct {
		AssetCode  string `json:"assetCode"`
		AssetScale int    `json:"assetScale"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", 0, err
	}
	return payload.AssetCode, payload.AssetScale, nil
}

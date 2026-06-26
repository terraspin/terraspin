package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// doJSON performs an authenticated JSON HTTP request and unmarshals the response.
// ponytail: shared helper cuts ~30 lines across github/gitlab clients.
func doJSON(method, url, token, tokenHeader string, payload, result any) error {
	var body io.Reader
	if payload != nil {
		raw, _ := json.Marshal(payload)
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(tokenHeader, token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	respRaw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respRaw))
	}
	if result != nil {
		if err := json.Unmarshal(respRaw, result); err != nil {
			return fmt.Errorf("parse: %w", err)
		}
	}
	return nil
}

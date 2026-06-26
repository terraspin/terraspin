package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var anthropicAPI = "https://api.anthropic.com/v1/messages"

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []claudeContent `json:"content"`
}

type claudeContent struct {
	Text string `json:"text"`
}

type claudeError struct {
	Message string `json:"message"`
}

// QueryClaude sends a prompt to the Anthropic Claude API using the
// Messages API and returns the raw text response.
// The caller should wrap ctx with a timeout: context.WithTimeout(ctx, 30*time.Second).
func QueryClaude(ctx context.Context, apiKey, prompt string) (string, error) {
	body := claudeRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Messages:  []claudeMessage{{Role: "user", Content: prompt}},
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("claude: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPI, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("claude: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("claude: send request: %w", err)
	}
	defer resp.Body.Close()

	respRaw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("claude: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var cerr claudeError
		if json.Unmarshal(respRaw, &cerr) == nil && cerr.Message != "" {
			return "", fmt.Errorf("claude: HTTP %d: %s", resp.StatusCode, cerr.Message)
		}
		return "", fmt.Errorf("claude: HTTP %d: %s", resp.StatusCode, string(respRaw))
	}

	var cresp claudeResponse
	if err := json.Unmarshal(respRaw, &cresp); err != nil {
		return "", fmt.Errorf("claude: parse response: %w", err)
	}

	if len(cresp.Content) == 0 {
		return "", fmt.Errorf("claude: no text content in response")
	}
	return cresp.Content[0].Text, nil
}

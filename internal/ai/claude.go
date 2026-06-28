// Package ai provides LLM clients (Claude, OpenAI, Ollama), prompt building,
// rule-based narrative generation, and sensitive value redaction.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

var anthropicAPI = "https://api.anthropic.com/v1/messages"

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []claudeContent `json:"content"`
	Usage   *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
}

type claudeContent struct {
	Text string `json:"text"`
}

const SystemPrompt = `You are Terraspin, an AI infrastructure reliability specialist.

You think like a senior SRE who has been paged at 3am because someone applied a plan without reading it. You produce specific, actionable intelligence — not generic risk advice.

Rules you never break:
- Stateful resource deletions (databases, volumes, storage with force_destroy) are CRITICAL until evidence proves otherwise
- Security group changes adding 0.0.0.0/0 are always HIGH or CRITICAL
- Rollback steps must be executable by a junior engineer without research
- Sensitive values are pre-redacted. Never ask for them.

Output format: Valid JSON only. No markdown fences. No prose outside JSON.`

type claudeError struct {
	Message string `json:"message"`
}

// QueryClaude sends a prompt to the Anthropic Claude API using the
// Messages API and returns the raw text response along with total token count.
// model defaults to CLAUDE_MODEL env var, then "claude-sonnet-4-6".
// apiKey falls back to ANTHROPIC_API_KEY env var when empty.
// Endpoint URL falls back to ANTHROPIC_BASE_URL env var, then the default.
// The caller should wrap ctx with a timeout: context.WithTimeout(ctx, 30*time.Second).
func QueryClaude(ctx context.Context, apiKey, model, prompt string) (string, int, error) {
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if model == "" {
		model = os.Getenv("CLAUDE_MODEL")
		if model == "" {
			model = "claude-sonnet-4-6"
		}
	}
	endpoint := anthropicAPI
	if v := os.Getenv("ANTHROPIC_BASE_URL"); v != "" {
		endpoint = v
	}

	body := claudeRequest{
		Model:     model,
		MaxTokens: 1024,
		System:    SystemPrompt,
		Messages:  []claudeMessage{{Role: "user", Content: prompt}},
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return "", 0, fmt.Errorf("claude: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", 0, fmt.Errorf("claude: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("claude: send request: %w", err)
	}
	defer resp.Body.Close()

	respRaw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("claude: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var cerr claudeError
		if json.Unmarshal(respRaw, &cerr) == nil && cerr.Message != "" {
			return "", 0, fmt.Errorf("claude: HTTP %d: %s", resp.StatusCode, cerr.Message)
		}
		return "", 0, fmt.Errorf("claude: HTTP %d: %s", resp.StatusCode, string(respRaw))
	}

	var cresp claudeResponse
	if err := json.Unmarshal(respRaw, &cresp); err != nil {
		return "", 0, fmt.Errorf("claude: parse response: %w", err)
	}

	if len(cresp.Content) == 0 {
		return "", 0, fmt.Errorf("claude: no text content in response")
	}
	tokens := 0
	if cresp.Usage != nil {
		tokens = cresp.Usage.InputTokens + cresp.Usage.OutputTokens
	}
	return cresp.Content[0].Text, tokens, nil
}

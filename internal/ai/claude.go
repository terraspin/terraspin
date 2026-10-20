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

type claudeProvider struct{}

func init() { registerProvider(claudeProvider{}) }

func (claudeProvider) Name() string   { return "claude" }
func (claudeProvider) NeedsKey() bool { return true }
func (claudeProvider) KeyEnv() string { return "ANTHROPIC_API_KEY" }

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

// QueryClaude sends a prompt to the Anthropic Claude API using the Messages API.
func QueryClaude(ctx context.Context, apiKey, model, prompt string) (string, int, error) {
	response, err := (claudeProvider{}).Query(ctx, Request{Model: model, System: SystemPrompt, Prompt: prompt, APIKey: apiKey})
	return response.Text, response.Tokens, err
}

func (claudeProvider) Query(ctx context.Context, request Request) (Response, error) {
	if request.APIKey == "" {
		request.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if request.Model == "" {
		request.Model = os.Getenv("CLAUDE_MODEL")
		if request.Model == "" {
			request.Model = "claude-sonnet-4-6"
		}
	}
	endpoint := anthropicAPI
	if value := os.Getenv("ANTHROPIC_BASE_URL"); value != "" {
		endpoint = value
	}

	raw, err := json.Marshal(claudeRequest{
		Model: request.Model, MaxTokens: 1024, System: request.System,
		Messages: []claudeMessage{{Role: "user", Content: request.Prompt}},
	})
	if err != nil {
		return Response{}, fmt.Errorf("claude: marshal request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return Response{}, fmt.Errorf("claude: create request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("x-api-key", request.APIKey)
	httpRequest.Header.Set("anthropic-version", "2023-06-01")

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return Response{}, fmt.Errorf("claude: send request: %w", err)
	}
	defer response.Body.Close()

	responseRaw, err := io.ReadAll(response.Body)
	if err != nil {
		return Response{}, fmt.Errorf("claude: read response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		message := string(responseRaw)
		var claudeErrorResponse claudeError
		if json.Unmarshal(responseRaw, &claudeErrorResponse) == nil && claudeErrorResponse.Message != "" {
			message = claudeErrorResponse.Message
		}
		return Response{}, &HTTPError{Provider: "claude", StatusCode: response.StatusCode, Message: message, RetryAfter: parseRetryAfter(response.Header.Get("Retry-After"))}
	}

	var parsed claudeResponse
	if err := json.Unmarshal(responseRaw, &parsed); err != nil {
		return Response{}, fmt.Errorf("claude: parse response: %w", err)
	}
	if len(parsed.Content) == 0 {
		return Response{}, fmt.Errorf("claude: no text content in response")
	}
	tokens := 0
	if parsed.Usage != nil {
		tokens = parsed.Usage.InputTokens + parsed.Usage.OutputTokens
	}
	return Response{Text: parsed.Content[0].Text, Tokens: tokens}, nil
}

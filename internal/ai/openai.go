package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var openAIAPI = "https://api.openai.com/v1/chat/completions"

type openAIProvider struct{}

func init() { registerProvider(openAIProvider{}) }

func (openAIProvider) Name() string   { return "openai" }
func (openAIProvider) NeedsKey() bool { return true }
func (openAIProvider) KeyEnv() string { return "OPENAI_API_KEY" }

type openAIRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

// QueryOpenAI sends a prompt to the OpenAI Chat Completions API.
func QueryOpenAI(ctx context.Context, apiKey, model, prompt string) (string, int, error) {
	response, err := (openAIProvider{}).Query(ctx, Request{Model: model, System: SystemPrompt, Prompt: prompt, APIKey: apiKey})
	return response.Text, response.Tokens, err
}

func (openAIProvider) Query(ctx context.Context, request Request) (Response, error) {
	if request.Model == "" {
		request.Model = "gpt-4o"
	}
	raw, _ := json.Marshal(openAIRequest{
		Model: request.Model, MaxTokens: 2048,
		Messages: []openAIMessage{{Role: "system", Content: request.System}, {Role: "user", Content: request.Prompt}},
	})

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIAPI, bytes.NewReader(raw))
	if err != nil {
		return Response{}, fmt.Errorf("openai: create request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+request.APIKey)

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return Response{}, fmt.Errorf("openai: send request: %w", err)
	}
	defer response.Body.Close()

	responseRaw, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		return Response{}, &HTTPError{Provider: "openai", StatusCode: response.StatusCode, Message: string(responseRaw), RetryAfter: parseRetryAfter(response.Header.Get("Retry-After"))}
	}

	var parsed openAIResponse
	if err := json.Unmarshal(responseRaw, &parsed); err != nil {
		return Response{}, fmt.Errorf("openai: parse response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return Response{}, fmt.Errorf("openai: no choices in response")
	}
	tokens := 0
	if parsed.Usage != nil {
		tokens = parsed.Usage.TotalTokens
	}
	return Response{Text: parsed.Choices[0].Message.Content, Tokens: tokens}, nil
}

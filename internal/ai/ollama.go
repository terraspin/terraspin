package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ollamaProvider struct{}

func init() { registerProvider(ollamaProvider{}) }

func (ollamaProvider) Name() string   { return "ollama" }
func (ollamaProvider) NeedsKey() bool { return false }
func (ollamaProvider) KeyEnv() string { return "" }

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Tokens   int    `json:"eval_count"`
}

// QueryOllama sends a prompt to a local Ollama instance.
func QueryOllama(ctx context.Context, host, model, prompt string) (string, int, error) {
	response, err := (ollamaProvider{}).Query(ctx, Request{Model: model, Prompt: prompt, Extra: map[string]string{"ollama_host": host}})
	return response.Text, response.Tokens, err
}

func (ollamaProvider) Query(ctx context.Context, request Request) (Response, error) {
	host := request.Extra["ollama_host"]
	if host == "" {
		host = "http://localhost:11434"
	}
	if request.Model == "" {
		request.Model = "llama3.2"
	}
	raw, _ := json.Marshal(ollamaRequest{Model: request.Model, Prompt: request.Prompt, Stream: false})

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(host, "/")+"/api/generate", bytes.NewReader(raw))
	if err != nil {
		return Response{}, fmt.Errorf("ollama: create request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return Response{}, fmt.Errorf("ollama: send request: %w", err)
	}
	defer response.Body.Close()

	responseRaw, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		return Response{}, &HTTPError{Provider: "ollama", StatusCode: response.StatusCode, Message: string(responseRaw), RetryAfter: parseRetryAfter(response.Header.Get("Retry-After"))}
	}

	var parsed ollamaResponse
	if err := json.Unmarshal(responseRaw, &parsed); err != nil {
		return Response{}, fmt.Errorf("ollama: parse response: %w", err)
	}
	return Response{Text: parsed.Response, Tokens: parsed.Tokens}, nil
}

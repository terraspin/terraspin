package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProviderPayloadsContainOnlyPromptMetadata(t *testing.T) {
	prompt := BuildAnalysisPrompt([]PlanChangeSummary{{Address: "example.internal", Action: "update", Tier: "low"}}, "low", 20, map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 1})
	for _, secret := range []string{"SYNTHETIC_SENSITIVE", "SYNTHETIC_UNMARKED"} {
		if strings.Contains(prompt, secret) {
			t.Fatal("analysis prompt contains an attribute value")
		}
	}

	t.Run("claude", func(t *testing.T) {
		defer withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			var body claudeRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.System != SystemPrompt || strings.Contains(body.Messages[0].Content, "SYNTHETIC_") {
				t.Fatal("unexpected Claude payload")
			}
			claudeSuccess(w)
		})()
		_, _, err := Execute(context.Background(), LLMOptions{Provider: "claude", APIKey: "key", Timeout: time.Second}, prompt)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("openai", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body openAIRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body.Messages) != 2 || body.Messages[0].Content != SystemPrompt || strings.Contains(body.Messages[1].Content, "SYNTHETIC_") {
				t.Fatal("unexpected OpenAI payload")
			}
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
		}))
		defer srv.Close()
		old := openAIAPI
		openAIAPI = srv.URL
		defer func() { openAIAPI = old }()
		_, _, err := Execute(context.Background(), LLMOptions{Provider: "openai", APIKey: "key", Timeout: time.Second}, prompt)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("ollama", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body ollamaRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			if strings.Contains(body.Prompt, "SYNTHETIC_") {
				t.Fatal("unexpected Ollama payload")
			}
			_, _ = w.Write([]byte(`{"response":"{}"}`))
		}))
		defer srv.Close()
		_, _, err := Execute(context.Background(), LLMOptions{Provider: "ollama", Extra: map[string]string{"ollama_host": srv.URL}, Timeout: time.Second}, prompt)
		if err != nil {
			t.Fatal(err)
		}
	})
}

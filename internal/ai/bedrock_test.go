package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setBedrockEnv(t *testing.T, endpoint string) {
	t.Helper()
	t.Setenv("AWS_ENDPOINT_URL_BEDROCK_RUNTIME", endpoint)
	t.Setenv("AWS_ACCESS_KEY_ID", "bedrock-test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "bedrock-test-secret-key")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "credentials"))
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "")
	t.Setenv("AWS_ROLE_ARN", "")
	t.Setenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "")
	t.Setenv("AWS_CONTAINER_CREDENTIALS_FULL_URI", "")
}

func newBedrockServer(t *testing.T, status int, handler func(http.ResponseWriter, *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
		}
		handler(w, r)
	}))
}

func assertBedrockRequest(t *testing.T, r *http.Request, model, system, prompt string) {
	t.Helper()
	if r.Method != http.MethodPost || r.URL.Path != "/model/"+model+"/converse" || !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256") {
		t.Fatalf("unexpected Bedrock request: method=%s path=%s signed=%t", r.Method, r.URL.Path, r.Header.Get("Authorization") != "")
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode Bedrock request: %v", err)
	}
	systemBlocks, ok := payload["system"].([]any)
	if !ok || len(systemBlocks) != 1 || systemBlocks[0].(map[string]any)["text"] != system {
		t.Fatalf("system=%v, want %q", payload["system"], system)
	}
	messages, ok := payload["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("messages=%v, want one user message", payload["messages"])
	}
	message, ok := messages[0].(map[string]any)
	if !ok || message["role"] != "user" {
		t.Fatalf("message=%v, want user message", messages[0])
	}
	content, ok := message["content"].([]any)
	if !ok || len(content) != 1 || content[0].(map[string]any)["text"] != prompt {
		t.Fatalf("content=%v, want %q", message["content"], prompt)
	}
	inference, ok := payload["inferenceConfig"].(map[string]any)
	if !ok || inference["maxTokens"] != float64(1024) {
		t.Fatalf("inferenceConfig=%v, want maxTokens 1024", payload["inferenceConfig"])
	}
}

func bedrockSuccess(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"output":{"message":{"role":"assistant","content":[{"text":"{\"summary\":\"ok\"}"}]}},"usage":{"totalTokens":5}}`))
}

func TestBedrockRetriesHTTPStatusThroughExecutor(t *testing.T) {
	for _, tc := range []struct {
		name       string
		maxRetries int
		handler    func(http.ResponseWriter, *http.Request, int)
		wantCalls  int
		wantError  bool
	}{
		{
			name:       "429 then success",
			maxRetries: 1,
			handler: func(w http.ResponseWriter, _ *http.Request, calls int) {
				if calls == 1 {
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				bedrockSuccess(w)
			},
			wantCalls: 2,
		},
		{
			name:       "400 does not retry",
			maxRetries: 2,
			handler: func(w http.ResponseWriter, _ *http.Request, _ int) {
				w.WriteHeader(http.StatusBadRequest)
			},
			wantCalls: 1,
			wantError: true,
		},
		{
			name:       "500 reaches attempt limit",
			maxRetries: 2,
			handler: func(w http.ResponseWriter, _ *http.Request, _ int) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantCalls: 3,
			wantError: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				assertBedrockRequest(t, r, "bedrock-test-model", SystemPrompt, "prompt")
				tc.handler(w, r, calls)
			}))
			defer server.Close()
			setBedrockEnv(t, server.URL)
			withRetryHooks(t, func(context.Context, time.Duration) error { return nil }, func(time.Duration) time.Duration { return 0 })

			_, _, err := Execute(context.Background(), LLMOptions{Provider: "bedrock", Model: "bedrock-test-model", Timeout: time.Second, MaxRetries: tc.maxRetries}, "prompt")
			if (err != nil) != tc.wantError || calls != tc.wantCalls {
				t.Fatalf("err=%v calls=%d, want error=%t calls=%d", err, calls, tc.wantError, tc.wantCalls)
			}
		})
	}
}

func TestBedrockRejectsMissingModelWithoutRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	setBedrockEnv(t, server.URL)

	_, _, err := Execute(context.Background(), LLMOptions{Provider: "bedrock", Timeout: time.Second}, "prompt")
	if err == nil || !strings.Contains(err.Error(), "bedrock: model is required") || requests != 0 {
		t.Fatalf("err=%v requests=%d, want required-model error without request", err, requests)
	}
}

func TestBedrockRejectsMissingAWSConfigurationWithoutRequest(t *testing.T) {
	for _, tc := range []struct {
		name  string
		unset []string
	}{
		{"region", []string{"AWS_REGION", "AWS_DEFAULT_REGION"}},
		{"credentials", []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
			defer server.Close()
			setBedrockEnv(t, server.URL)
			for _, name := range tc.unset {
				t.Setenv(name, "")
			}

			_, _, err := Execute(context.Background(), LLMOptions{Provider: "bedrock", Model: "bedrock-test-model", Timeout: time.Second}, "prompt")
			if err == nil || !strings.Contains(err.Error(), "bedrock: could not load AWS configuration:") || !strings.Contains(err.Error(), "set AWS_REGION and credentials") || requests != 0 {
				t.Fatalf("err=%v requests=%d, want AWS configuration error without request", err, requests)
			}
		})
	}
}

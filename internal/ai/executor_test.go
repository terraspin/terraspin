package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func withRetryHooks(t *testing.T, sleep func(context.Context, time.Duration) error, jitter func(time.Duration) time.Duration) {
	t.Helper()
	oldSleep, oldJitter := retrySleep, retryJitter
	retrySleep, retryJitter = sleep, jitter
	t.Cleanup(func() { retrySleep, retryJitter = oldSleep, oldJitter })
}

func claudeSuccess(w http.ResponseWriter) {
	_ = json.NewEncoder(w).Encode(claudeResponse{Content: []claudeContent{{Text: `{"summary":"ok"}`}}})
}

func TestExecuteRetriesRetryableErrors(t *testing.T) {
	var requests int
	defer withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		claudeSuccess(w)
	})()
	withRetryHooks(t, func(context.Context, time.Duration) error { return nil }, func(time.Duration) time.Duration { return 0 })

	_, _, err := Execute(context.Background(), LLMOptions{Provider: "claude", APIKey: "key", Timeout: time.Second, MaxRetries: 1}, "prompt")
	if err != nil || requests != 2 {
		t.Fatalf("err=%v requests=%d, want success after 2 attempts", err, requests)
	}
}

func TestExecuteDoesNotRetryClientErrors(t *testing.T) {
	var requests int
	defer withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusUnauthorized)
	})()
	withRetryHooks(t, func(context.Context, time.Duration) error { t.Fatal("should not sleep"); return nil }, func(time.Duration) time.Duration { return 0 })

	_, _, err := Execute(context.Background(), LLMOptions{Provider: "claude", APIKey: "key", Timeout: time.Second, MaxRetries: 7}, "prompt")
	if err == nil || requests != 1 {
		t.Fatalf("err=%v requests=%d, want one 401 attempt", err, requests)
	}
}

func TestExecuteHonorsRetryAfterAndAttemptLimit(t *testing.T) {
	var requests int
	var delays []time.Duration
	defer withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	})()
	withRetryHooks(t, func(_ context.Context, delay time.Duration) error { delays = append(delays, delay); return nil }, func(time.Duration) time.Duration { return 0 })

	_, _, err := Execute(context.Background(), LLMOptions{Provider: "claude", APIKey: "key", Timeout: time.Second, MaxRetries: 2}, "prompt")
	if err == nil || requests != 3 {
		t.Fatalf("err=%v requests=%d, want 3 attempts", err, requests)
	}
	if len(delays) != 2 || delays[0] != 10*time.Second || delays[1] != 10*time.Second {
		t.Fatalf("delays=%v, want capped Retry-After", delays)
	}
}

func TestExecuteAppliesAttemptTimeout(t *testing.T) {
	var requests atomic.Int32
	defer withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		time.Sleep(10 * time.Millisecond)
	})()

	_, _, err := Execute(context.Background(), LLMOptions{Provider: "claude", APIKey: "key", Timeout: 5 * time.Millisecond}, "prompt")
	if err == nil || requests.Load() < 1 || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("err=%v requests=%d, want a timed-out attempt", err, requests.Load())
	}
}

func TestExecuteRejectsMissingKeyWithoutRequest(t *testing.T) {
	var requests int
	defer withTestServer(t, func(w http.ResponseWriter, r *http.Request) { requests++ })()

	_, _, err := Execute(context.Background(), LLMOptions{Provider: "claude", Timeout: time.Second}, "prompt")
	if err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") || requests != 0 {
		t.Fatalf("err=%v requests=%d, want fail-fast without request", err, requests)
	}
}

func TestExecuteAllowsOllamaWithoutKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ollamaResponse{Response: `{"summary":"ok"}`})
	}))
	defer srv.Close()

	_, _, err := Execute(context.Background(), LLMOptions{Provider: "ollama", OllamaHost: srv.URL, Timeout: time.Second}, "prompt")
	if err != nil {
		t.Fatal(err)
	}
}

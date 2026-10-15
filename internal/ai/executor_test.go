package ai

import (
	"context"
	"encoding/json"
	"errors"
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

func TestExecuteRetryAfterValues(t *testing.T) {
	cases := []struct {
		name   string
		header string
		check  func(t *testing.T, delay time.Duration)
	}{
		{"seconds", "2", func(t *testing.T, delay time.Duration) {
			if delay != 2*time.Second {
				t.Errorf("delay=%v, want 2s", delay)
			}
		}},
		{"negative", "-1", func(t *testing.T, delay time.Duration) {
			if delay != 250*time.Millisecond {
				t.Errorf("delay=%v, want fallback", delay)
			}
		}},
		{"nonnumeric", "tomorrow", func(t *testing.T, delay time.Duration) {
			if delay != 250*time.Millisecond {
				t.Errorf("delay=%v, want fallback", delay)
			}
		}},
		{"over cap", "30", func(t *testing.T, delay time.Duration) {
			if delay != 10*time.Second {
				t.Errorf("delay=%v, want 10s cap", delay)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests int
			defer withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				if requests == 1 {
					w.Header().Set("Retry-After", tc.header)
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				claudeSuccess(w)
			})()
			var delays []time.Duration
			withRetryHooks(t, func(_ context.Context, delay time.Duration) error { delays = append(delays, delay); return nil }, func(time.Duration) time.Duration { return 0 })

			if _, _, err := Execute(context.Background(), LLMOptions{Provider: "claude", APIKey: "key", Timeout: time.Second, MaxRetries: 1}, "prompt"); err != nil {
				t.Fatal(err)
			}
			if requests != 2 || len(delays) != 1 {
				t.Fatalf("requests=%d delays=%v, want one retry", requests, delays)
			}
			tc.check(t, delays[0])
		})
	}
}

func TestExecuteHonorsRetryAfterHTTPDate(t *testing.T) {
	var requests int
	defer withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("Retry-After", time.Now().Add(3*time.Second).UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		claudeSuccess(w)
	})()
	var delays []time.Duration
	withRetryHooks(t, func(_ context.Context, delay time.Duration) error { delays = append(delays, delay); return nil }, func(time.Duration) time.Duration { return 0 })

	if _, _, err := Execute(context.Background(), LLMOptions{Provider: "claude", APIKey: "key", Timeout: time.Second, MaxRetries: 1}, "prompt"); err != nil {
		t.Fatal(err)
	}
	if len(delays) != 1 || delays[0] <= time.Second || delays[0] > 3*time.Second {
		t.Fatalf("delays=%v, want parsed HTTP date between 1s and 3s", delays)
	}
}

func TestExecuteStopsWhenCancelledDuringBackoff(t *testing.T) {
	defer withTestServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })()
	started := make(chan struct{})
	stopped := errors.New("backoff stopped by cancelled context")
	withRetryHooks(t, func(ctx context.Context, _ time.Duration) error {
		close(started)
		<-ctx.Done()
		return stopped
	}, func(time.Duration) time.Duration { return 0 })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, _, err := Execute(ctx, LLMOptions{Provider: "claude", APIKey: "key", Timeout: time.Second, MaxRetries: 1}, "prompt")
		result <- err
	}()
	<-started
	cancel()
	if err := <-result; !errors.Is(err, stopped) {
		t.Fatalf("err=%v, want cancellation to stop the backoff", err)
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

func TestExecuteGatewayStillRequiresClaudeKey(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got := r.Header.Get("x-api-key"); got != "gateway-placeholder" {
			t.Errorf("x-api-key=%q, want placeholder", got)
		}
		claudeSuccess(w)
	}))
	defer srv.Close()
	t.Setenv("ANTHROPIC_BASE_URL", srv.URL)

	_, _, err := Execute(context.Background(), LLMOptions{Provider: "claude", Timeout: time.Second}, "prompt")
	if err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") || !strings.Contains(err.Error(), "gateway/proxy without authentication can use a placeholder value") || requests != 0 {
		t.Fatalf("err=%v requests=%d, want gateway key failure without request", err, requests)
	}

	if _, _, err := Execute(context.Background(), LLMOptions{Provider: "claude", APIKey: "gateway-placeholder", Timeout: time.Second}, "prompt"); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests=%d, want one placeholder-key request", requests)
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

package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func withTestServer(t *testing.T, handler http.HandlerFunc) func() {
	t.Helper()
	srv := httptest.NewServer(handler)
	orig := anthropicAPI
	anthropicAPI = srv.URL
	return func() { srv.Close(); anthropicAPI = orig }
}

func TestQueryClaude_success(t *testing.T) {
	defer withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(claudeResponse{
			Content: []claudeContent{{Text: "analysis result"}},
		})
	})()

	got, err := QueryClaude(context.Background(), "test-key", "", "analyze this plan")
	if err != nil {
		t.Fatal(err)
	}
	if got != "analysis result" {
		t.Errorf("got %q, want %q", got, "analysis result")
	}
}

func TestQueryClaude_httpError(t *testing.T) {
	defer withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(claudeError{Message: "invalid API key"})
	})()

	_, err := QueryClaude(context.Background(), "bad-key", "", "prompt")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestQueryClaude_timeout(t *testing.T) {
	defer withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	})()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := QueryClaude(ctx, "key", "", "prompt")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

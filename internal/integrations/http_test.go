package integrations

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoJSON_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.Header.Get("Authorization") != "test-token" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": 42}`))
	}))
	defer srv.Close()

	var result struct {
		ID int `json:"id"`
	}
	err := doJSON(http.MethodPost, srv.URL, "test-token", "Authorization", nil, &result)
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != 42 {
		t.Errorf("id = %d, want 42", result.ID)
	}
}

func TestDoJSON_httpError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message": "bad token"}`))
	}))
	defer srv.Close()

	err := doJSON(http.MethodGet, srv.URL, "", "", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsAny(err.Error(), "401", "Unauthorized") {
		t.Errorf("error = %v, should mention 401", err)
	}
}

func TestDoJSON_sendsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	payload := map[string]string{"body": "hello"}
	err := doJSON(http.MethodPost, srv.URL, "", "", payload, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

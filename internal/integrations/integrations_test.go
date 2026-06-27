package integrations

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// GitHub
// ---------------------------------------------------------------------------

func TestNewGitHubClientFromEnv_readsEnv(t *testing.T) {
	os.Setenv("GITHUB_TOKEN", "gh-test-token")
	os.Setenv("GITHUB_REPOSITORY", "owner/repo")
	os.Setenv("GITHUB_REF", "refs/pull/42/merge")
	defer func() {
		os.Unsetenv("GITHUB_TOKEN")
		os.Unsetenv("GITHUB_REPOSITORY")
		os.Unsetenv("GITHUB_REF")
	}()

	c := NewGitHubClientFromEnv()
	if c.Token != "gh-test-token" {
		t.Errorf("token = %q", c.Token)
	}
	if c.Repo != "owner/repo" {
		t.Errorf("repo = %q", c.Repo)
	}
	if c.PRNum != 42 {
		t.Errorf("pr = %d, want 42", c.PRNum)
	}
}

func TestGitHubClient_FindCommentByTag_findsExisting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 123, "body": "some comment"},
			{"id": 456, "body": "<!-- terraspin --> analysis here"},
		})
	}))
	defer srv.Close()

	c := &GitHubClient{Token: "t", Repo: "o/r", PRNum: 1, APIBase: srv.URL}
	id, err := c.FindCommentByTag("<!-- terraspin -->")
	if err != nil {
		t.Fatal(err)
	}
	if id != 456 {
		t.Errorf("id = %d, want 456", id)
	}
}

func TestGitHubClient_FindCommentByTag_notFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "body": "nothing here"},
		})
	}))
	defer srv.Close()

	c := &GitHubClient{Token: "t", Repo: "o/r", PRNum: 1, APIBase: srv.URL}
	id, err := c.FindCommentByTag("<!-- terraspin -->")
	if err != nil {
		t.Fatal(err)
	}
	if id != 0 {
		t.Errorf("id = %d, want 0", id)
	}
}

func TestGitHubClient_FindCommentByTag_emptyFields(t *testing.T) {
	c := &GitHubClient{}
	id, err := c.FindCommentByTag("tag")
	if err != nil {
		t.Fatal(err)
	}
	if id != 0 {
		t.Errorf("id = %d, want 0", id)
	}
}

func TestGitHubClient_PostComment_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": 789}`))
	}))
	defer srv.Close()

	c := &GitHubClient{Token: "t", Repo: "o/r", PRNum: 1, APIBase: srv.URL}
	id, err := c.PostComment("hello")
	if err != nil {
		t.Fatal(err)
	}
	if id != 789 {
		t.Errorf("id = %d, want 789", id)
	}
}

func TestGitHubClient_PostComment_missingFields(t *testing.T) {
	c := &GitHubClient{}
	_, err := c.PostComment("body")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGitHubClient_UpdateComment_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &GitHubClient{Token: "t", Repo: "o/r", PRNum: 1, APIBase: srv.URL}
	err := c.UpdateComment(42, "updated body")
	if err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// GitLab
// ---------------------------------------------------------------------------

func TestNewGitLabClientFromEnv_readsEnv(t *testing.T) {
	os.Setenv("GITLAB_TOKEN", "gl-token")
	os.Setenv("CI_PROJECT_ID", "12345")
	os.Setenv("CI_MERGE_REQUEST_IID", "99")
	defer func() {
		os.Unsetenv("GITLAB_TOKEN")
		os.Unsetenv("CI_PROJECT_ID")
		os.Unsetenv("CI_MERGE_REQUEST_IID")
	}()

	c := NewGitLabClientFromEnv()
	if c.Token != "gl-token" {
		t.Errorf("token = %q", c.Token)
	}
	if c.PID != "12345" {
		t.Errorf("pid = %q", c.PID)
	}
	if c.MRID != "99" {
		t.Errorf("mrid = %q", c.MRID)
	}
}

func TestGitLabClient_PostNote_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.Header.Get("PRIVATE-TOKEN") != "gl-token" {
			t.Errorf("auth = %q", r.Header.Get("PRIVATE-TOKEN"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": 555}`))
	}))
	defer srv.Close()

	c := &GitLabClient{BaseURL: srv.URL, Token: "gl-token", PID: "1", MRID: "2"}
	id, err := c.PostNote("test note")
	if err != nil {
		t.Fatal(err)
	}
	if id != 555 {
		t.Errorf("id = %d, want 555", id)
	}
}

func TestGitLabClient_PostNote_missingToken(t *testing.T) {
	c := &GitLabClient{}
	_, err := c.PostNote("body")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGitLabClient_UpdateNote_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &GitLabClient{BaseURL: srv.URL, Token: "t", PID: "1", MRID: "2"}
	err := c.UpdateNote(100, "updated")
	if err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Slack
// ---------------------------------------------------------------------------

func TestSlackWebhook_SendRiskNotification_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type = %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sw := &SlackWebhook{WebhookURL: srv.URL}
	err := sw.SendRiskNotification("critical", 95, "summary", map[string]int{"create": 1})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSlackWebhook_SendRiskNotification_emptyURL(t *testing.T) {
	sw := &SlackWebhook{}
	err := sw.SendRiskNotification("high", 70, "", nil)
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestSlackWebhook_SendRiskNotification_httpError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sw := &SlackWebhook{WebhookURL: srv.URL}
	err := sw.SendRiskNotification("high", 70, "test", map[string]int{"update": 1})
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %v, should mention 500", err)
	}
}

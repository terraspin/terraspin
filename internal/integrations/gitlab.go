package integrations

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

// GitLabClient posts MR notes to GitLab.
type GitLabClient struct {
	BaseURL string
	Token   string
	PID     string // project ID
	MRID    string // merge request IID
}

// NewGitLabClientFromEnv builds a client from CI environment variables.
func NewGitLabClientFromEnv() *GitLabClient {
	baseURL := os.Getenv("CI_SERVER_URL")
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	token := os.Getenv("GITLAB_TOKEN")
	if token == "" {
		token = os.Getenv("CI_JOB_TOKEN")
	}
	return &GitLabClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		PID:     os.Getenv("CI_PROJECT_ID"),
		MRID:    os.Getenv("CI_MERGE_REQUEST_IID"),
	}
}

// PostNote posts a markdown note to the MR. Returns the note ID.
func (c *GitLabClient) PostNote(body string) (int, error) {
	if c.Token == "" {
		return 0, fmt.Errorf("gitlab: no token (set GITLAB_TOKEN or CI_JOB_TOKEN)")
	}
	if c.PID == "" || c.MRID == "" {
		return 0, fmt.Errorf("gitlab: CI_PROJECT_ID or CI_MERGE_REQUEST_IID not set")
	}
	url := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%s/notes", c.BaseURL, c.PID, c.MRID)
	var result struct{ ID int }
	if err := doJSON(http.MethodPost, url, c.Token, "PRIVATE-TOKEN", map[string]string{"body": body}, &result); err != nil {
		return 0, fmt.Errorf("gitlab: post note: %w", err)
	}
	return result.ID, nil
}

// UpdateNote updates an existing note by ID.
func (c *GitLabClient) UpdateNote(noteID int, body string) error {
	url := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%s/notes/%d", c.BaseURL, c.PID, c.MRID, noteID)
	return doJSON(http.MethodPut, url, c.Token, "PRIVATE-TOKEN", map[string]string{"body": body}, nil)
}

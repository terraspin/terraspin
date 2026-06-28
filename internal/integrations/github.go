// Package integrations provides GitHub PR comments, GitLab MR notes, Slack webhooks,
// and the MCP server for AI coding assistants.
package integrations

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// GitHubClient posts and updates PR comments (raw HTTP, no SDK).
type GitHubClient struct {
	Token   string
	Repo    string // "owner/repo"
	PRNum   int
	APIBase string
}

// NewGitHubClientFromEnv builds a client from CI environment variables.
func NewGitHubClientFromEnv() *GitHubClient {
	return &GitHubClient{
		Token:   os.Getenv("GITHUB_TOKEN"),
		Repo:    os.Getenv("GITHUB_REPOSITORY"),
		PRNum:   prNumFromEnv(),
		APIBase: "https://api.github.com",
	}
}

func prNumFromEnv() int {
	ref := os.Getenv("GITHUB_REF")
	if ref == "" {
		ref = os.Getenv("GITHUB_REF_NAME")
	}
	var n int
	if _, err := fmt.Sscanf(ref, "%d/merge", &n); err == nil {
		return n
	}
	if _, err := fmt.Sscanf(ref, "refs/pull/%d/merge", &n); err == nil {
		return n
	}
	return 0
}

const ghTokenHeader = "Authorization"

func ghBearer(token string) string { return "Bearer " + token }

// FindCommentByTag searches for an existing Terraspin comment.
func (c *GitHubClient) FindCommentByTag(tag string) (int, error) {
	if c.Token == "" || c.Repo == "" || c.PRNum == 0 {
		return 0, nil
	}
	url := fmt.Sprintf("%s/repos/%s/issues/%d/comments", c.APIBase, c.Repo, c.PRNum)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", ghBearer(c.Token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("github: list comments: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, nil
	}

	var comments []struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
	}
	json.NewDecoder(resp.Body).Decode(&comments)
	for _, c := range comments {
		if strings.Contains(c.Body, tag) {
			return c.ID, nil
		}
	}
	return 0, nil
}

// PostComment creates a new PR comment. Returns the comment ID.
func (c *GitHubClient) PostComment(body string) (int, error) {
	if c.Token == "" || c.Repo == "" || c.PRNum == 0 {
		return 0, fmt.Errorf("github: missing token, repo, or PR number")
	}
	url := fmt.Sprintf("%s/repos/%s/issues/%d/comments", c.APIBase, c.Repo, c.PRNum)
	var result struct{ ID int }
	if err := doJSON(http.MethodPost, url, ghBearer(c.Token), ghTokenHeader, map[string]string{"body": body}, &result); err != nil {
		return 0, fmt.Errorf("github: post comment: %w", err)
	}
	return result.ID, nil
}

// UpdateComment edits an existing comment by ID.
func (c *GitHubClient) UpdateComment(commentID int, body string) error {
	url := fmt.Sprintf("%s/repos/%s/issues/comments/%d", c.APIBase, c.Repo, commentID)
	return doJSON(http.MethodPatch, url, ghBearer(c.Token), ghTokenHeader, map[string]string{"body": body}, nil)
}

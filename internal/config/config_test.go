package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultsWhenFileMissing(t *testing.T) {
	cfg, err := Load("/nonexistent/.terraspin.yml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != 1 {
		t.Errorf("version = %d, want 1", cfg.Version)
	}
	if cfg.LLM != nil {
		t.Errorf("llm should be nil when file missing, got %v", cfg.LLM)
	}
}

func TestLoad_ActualFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".terraspin.yml")
	content := `
version: 1
llm:
  provider: openai
  model: gpt-4o
rules:
  - id: test-rule
    severity: critical
    description: "test"
    match:
      resource_type_pattern: "aws_instance"
      action: delete
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.Provider != "openai" {
		t.Errorf("llm provider = %q", cfg.LLM.Provider)
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(cfg.Rules))
	}
	if cfg.Rules[0].ID != "test-rule" {
		t.Errorf("rule id = %q", cfg.Rules[0].ID)
	}
	if cfg.Rules[0].Match.ResourceTypePattern != "aws_instance" {
		t.Errorf("match.resource_type_pattern = %q", cfg.Rules[0].Match.ResourceTypePattern)
	}
}

func TestValidate_InvalidVersion(t *testing.T) {
	cfg := &Config{Version: 2}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for version 2")
	}
}

func TestValidate_MissingRuleID(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Rules: []Rule{
			{Severity: "high", Description: "no id"},
		},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for missing rule id")
	}
}

func TestValidate_InvalidSeverity(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Rules: []Rule{
			{ID: "bad", Severity: "urgent", Description: "bad"},
		},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for invalid severity")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".terraspin.yml")
	os.WriteFile(path, []byte(":: invalid yaml ::"), 0644)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_FullConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".terraspin.yml")
	content := `
version: 1

llm:
  provider: claude
  model: claude-sonnet-4-20250514
  timeout: 30s
  max_retries: 2
  fallback_to_rules: true

risk:
  fail_on: high

rules:
  - id: no-public-rds
    severity: critical
    description: "Database must not be publicly accessible"
    match:
      resource_type_pattern: "*_db_instance"
      attribute_path: "publicly_accessible"
      value: true

  - id: prod-deletion-gate
    severity: critical
    description: "Deletions in prod require manual review"
    match:
      action: delete
      workspace_pattern: "prod-*"

  - id: open-ssh
    severity: high
    description: "SSH open to 0.0.0.0/0"
    match:
      resource_type_pattern: "aws_security_group_rule"
      attribute_path: "cidr_blocks"
      contains: "0.0.0.0/0"

slack:
  webhook_url_env: SLACK_WEBHOOK_URL
  notify_on: [critical, high]
  channel: "#infra-changes"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Version != 1 {
		t.Errorf("version = %d", cfg.Version)
	}
	if cfg.Risk.FailOn != "high" {
		t.Errorf("fail_on = %q", cfg.Risk.FailOn)
	}
	if len(cfg.Rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(cfg.Rules))
	}
	if cfg.Slack.Channel != "#infra-changes" {
		t.Errorf("slack channel = %q", cfg.Slack.Channel)
	}
}

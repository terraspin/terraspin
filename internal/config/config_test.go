package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_DefaultsWhenFileMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), ".terraspin.yml"))
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

func TestLoad_ConfigWithoutLLMUsesDefaults(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{name: "version only", content: "version: 1\n"},
		{name: "risk config", content: "version: 1\nrisk:\n  fail_on: high\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".terraspin.yml")
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.LLM == nil || *cfg.LLM != *DefaultConfig().LLM {
				t.Errorf("llm = %#v, want defaults %#v", cfg.LLM, DefaultConfig().LLM)
			}
		})
	}
}

func TestLoad_ValidatesLLMTimeoutAndTracksExplicitModel(t *testing.T) {
	dir := t.TempDir()
	invalid := filepath.Join(dir, "invalid.yml")
	if err := os.WriteFile(invalid, []byte("version: 1\nllm:\n  timeout: nope\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(invalid); err == nil || !strings.Contains(err.Error(), "llm.timeout") {
		t.Fatalf("err=%v, want llm.timeout validation error", err)
	}

	explicit := filepath.Join(dir, "explicit.yml")
	if err := os.WriteFile(explicit, []byte("version: 1\nllm:\n  provider: openai\n  model: custom-model\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.ExplicitModel() != "custom-model" {
		t.Errorf("explicit model = %q", cfg.LLM.ExplicitModel())
	}

	implicit := filepath.Join(dir, "implicit.yml")
	if err := os.WriteFile(implicit, []byte("version: 1\nllm:\n  provider: openai\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(implicit)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.ExplicitModel() != "" {
		t.Errorf("implicit model = %q, want empty", cfg.LLM.ExplicitModel())
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

func TestLoad_ValidatesRuleAttributePaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".terraspin.yml")
	content := `
version: 1
rules:
  - id: bad-path
    severity: high
    match:
      resource_type_pattern: aws_security_group
      attribute_path: ingress[*].cidr_blocks[*]
      attribute_path_2: ingress[-1].from_port
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected invalid attribute path error")
	}
	for _, want := range []string{"bad-path", "attribute_path", `ingress[*].cidr_blocks[*]`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want %q", err, want)
		}
	}
}

func TestLoad_ValidatesSecondaryRuleAttributePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".terraspin.yml")
	content := `
version: 1
rules:
  - id: bad-secondary-path
    severity: high
    match:
      resource_type_pattern: aws_security_group
      attribute_path: ingress[0].from_port
      attribute_path_2: ingress[*].cidr_blocks[*]
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected invalid secondary attribute path error")
	}
	for _, want := range []string{"bad-secondary-path", "attribute_path_2", `ingress[*].cidr_blocks[*]`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want %q", err, want)
		}
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
	_ = os.WriteFile(path, []byte(":: invalid yaml ::"), 0644)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_BuiltinRuleControls(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		want    string
	}{
		{"unknown disabled rule", "version: 1\ndisable_rules: [NO-SUCH-RULE]\n", "NO-SUCH-RULE"},
		{"custom builtin ID conflict", "version: 1\nrules:\n  - id: AWS-SG-001\n    severity: high\n    match:\n      action: create\n", "AWS-SG-001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".terraspin.yml")
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoad_FullConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".terraspin.yml")
	content := `
version: 1

llm:
  provider: claude
  model: claude-sonnet-4-6
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

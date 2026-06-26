// Package config loads, validates, and exposes .terraspin.yml configuration.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level .terraspin.yml structure.
type Config struct {
	Version int          `yaml:"version"`
	LLM     *LLMConfig   `yaml:"llm,omitempty"`
	Risk    *RiskConfig  `yaml:"risk,omitempty"`
	Rules   []Rule       `yaml:"rules,omitempty"`
	Slack   *SlackConfig `yaml:"slack,omitempty"`
}

// LLMConfig holds LLM provider configuration.
type LLMConfig struct {
	Provider        string `yaml:"provider"`
	Model           string `yaml:"model"`
	Timeout         string `yaml:"timeout"`
	MaxRetries      int    `yaml:"max_retries"`
	FallbackToRules bool   `yaml:"fallback_to_rules"`
}

// RiskConfig holds risk behavior settings.
type RiskConfig struct {
	FailOn string `yaml:"fail_on"`
}

// SlackConfig holds Slack webhook settings.
type SlackConfig struct {
	WebhookURLEnv string   `yaml:"webhook_url_env"`
	NotifyOn      []string `yaml:"notify_on"`
	Channel       string   `yaml:"channel"`
}

// Rule defines a custom risk rule.
type Rule struct {
	ID          string    `yaml:"id"`
	Severity    string    `yaml:"severity"`
	Description string    `yaml:"description"`
	Match       RuleMatch `yaml:"match"`
}

// RuleMatch defines the conditions for a rule to trigger.
type RuleMatch struct {
	ResourceTypePattern string `yaml:"resource_type_pattern,omitempty"`
	AttributePath       string `yaml:"attribute_path,omitempty"`
	AttributePath2      string `yaml:"attribute_path_2,omitempty"`
	Value               any    `yaml:"value,omitempty"`
	Value2              any    `yaml:"value_2,omitempty"`
	Contains            string `yaml:"contains,omitempty"`
	Action              string `yaml:"action,omitempty"`
	WorkspacePattern    string `yaml:"workspace_pattern,omitempty"`
}

// RuleMatchResult describes a single rule match on a resource.
type RuleMatchResult struct {
	RuleID      string `json:"rule_id"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Address     string `json:"address"`
}

// DefaultConfig returns a config with LLM defaults (the only defaults main.go needs).
func DefaultConfig() *Config {
	return &Config{
		Version: 1,
		LLM: &LLMConfig{
			Provider:        "claude",
			Model:           "claude-sonnet-4-20250514",
			Timeout:         "30s",
			MaxRetries:      2,
			FallbackToRules: true,
		},
	}
}

// Load reads and validates a .terraspin.yml file.
// Returns bare config (version only) if file doesn't exist.
func Load(path string) (*Config, error) {
	if path == "" {
		path = ".terraspin.yml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Version: 1}, nil
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: validate %s: %w", path, err)
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported version %d", c.Version)
	}
	for i, r := range c.Rules {
		if r.ID == "" {
			return fmt.Errorf("rules[%d]: id is required", i)
		}
		switch r.Severity {
		case "critical", "high", "medium", "low":
		default:
			return fmt.Errorf("rules[%q]: invalid severity %q", r.ID, r.Severity)
		}
		if r.Match.ResourceTypePattern == "" && r.Match.Action == "" {
			return fmt.Errorf("rules[%q]: at least one match condition required", r.ID)
		}
	}
	return nil
}

package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/terraspin/terraspin/internal/parser"
)

// EvaluateRules checks all custom rules against a plan and returns matches.
func EvaluateRules(cfg *Config, ast *parser.PlanAST) []RuleMatchResult {
	if cfg == nil || len(cfg.Rules) == 0 {
		return nil
	}
	var results []RuleMatchResult
	for _, rc := range ast.Changes {
		if rc.Action == parser.ActionNoOp || rc.Action == parser.ActionRead {
			continue
		}
		for _, rule := range cfg.Rules {
			if matchRule(rule, rc, ast.Workspace) {
				results = append(results, RuleMatchResult{
					RuleID:      rule.ID,
					Severity:    rule.Severity,
					Description: rule.Description,
					Address:     rc.Address,
				})
			}
		}
	}
	return results
}

// matchRule checks if a rule matches a resource change.
func matchRule(rule Rule, rc parser.ResourceChange, workspace string) bool {
	m := rule.Match

	// Resource type glob match (exact when no wildcards)
	if m.ResourceTypePattern != "" {
		matched, _ := filepath.Match(m.ResourceTypePattern, rc.Type)
		if !matched {
			return false
		}
	}
	// Action match
	if m.Action != "" && string(rc.Action) != m.Action {
		return false
	}
	// Workspace glob match
	if m.WorkspacePattern != "" {
		matched, _ := filepath.Match(m.WorkspacePattern, workspace)
		if !matched {
			return false
		}
	}
	if m.AttributePath != "" && !matchesAttribute(rc.After, rc.Before, m.AttributePath, m.Value, m.Contains) {
		return false
	}
	if m.AttributePath2 != "" && m.Value2 != nil && !matchesAttribute(rc.After, rc.Before, m.AttributePath2, m.Value2, "") {
		return false
	}
	return true
}

func matchesAttribute(after, before map[string]any, rawPath string, expected any, contains string) bool {
	path, err := ParsePath(rawPath)
	if err != nil {
		return false
	}
	values := path.Resolve(after)
	if len(values) == 0 {
		values = path.Resolve(before)
	}
	for _, value := range values {
		if contains != "" && containsValue(value, contains) {
			return true
		}
		if contains == "" && expected != nil && valuesEqual(value, expected) {
			return true
		}
		if contains == "" && expected == nil {
			return true
		}
	}
	return false
}

func containsValue(value any, expected string) bool {
	switch value := value.(type) {
	case string:
		return strings.Contains(value, expected)
	case []any:
		for _, element := range value {
			if text, ok := element.(string); ok && text == expected {
				return true
			}
		}
	}
	return false
}

// getNestedValue preserves the legacy helper used by existing tests.
func getNestedValue(data map[string]any, rawPath string) any {
	path, err := ParsePath(rawPath)
	if err != nil {
		return nil
	}
	values := path.Resolve(data)
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

// valuesEqual compares two values for rule matching.
// ponytail: fmt.Sprintf handles cross-type (int vs float64 vs string).
func valuesEqual(a, b any) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

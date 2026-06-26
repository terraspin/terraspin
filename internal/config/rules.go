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
	// Attribute path + value/contains match
	if m.AttributePath != "" {
		val := getNestedValue(rc.After, m.AttributePath)
		if val == nil {
			val = getNestedValue(rc.Before, m.AttributePath)
		}
		if val == nil {
			return false
		}
		if m.Contains != "" {
			s, ok := val.(string)
			if !ok || !strings.Contains(s, m.Contains) {
				return false
			}
		} else if m.Value != nil {
			if !valuesEqual(val, m.Value) {
				return false
			}
		}
		// Secondary attribute match (AND with primary)
		if m.AttributePath2 != "" && m.Value2 != nil {
			val2 := getNestedValue(rc.After, m.AttributePath2)
			if val2 == nil {
				val2 = getNestedValue(rc.Before, m.AttributePath2)
			}
			if val2 == nil || !valuesEqual(val2, m.Value2) {
				return false
			}
		}
	}
	return true
}

// getNestedValue walks a dotted path into a nested map.
// ponytail: flat dot-separated paths, no array indexing.
func getNestedValue(data map[string]any, path string) any {
	if data == nil {
		return nil
	}
	for _, key := range strings.Split(path, ".") {
		v, ok := data[key]
		if !ok {
			return nil
		}
		sub, ok := v.(map[string]any)
		if !ok {
			return v
		}
		data = sub
	}
	return nil
}

// valuesEqual compares two values for rule matching.
// ponytail: fmt.Sprintf handles cross-type (int vs float64 vs string).
func valuesEqual(a, b any) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

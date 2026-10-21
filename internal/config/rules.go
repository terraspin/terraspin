package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/terraspin/terraspin/internal/parser"
)

// EvaluateRules checks configured and built-in rules against a plan and returns matches.
func EvaluateRules(cfg *Config, ast *parser.PlanAST) []RuleMatchResult {
	if cfg == nil {
		return nil
	}
	type configuredRule struct {
		rule   Rule
		source string
	}
	var rules []configuredRule
	if cfg.builtinRulesEnabled() {
		builtins, err := builtinRules()
		if err == nil {
			disabled := make(map[string]bool, len(cfg.DisableRules))
			for _, id := range cfg.DisableRules {
				disabled[id] = true
			}
			for _, rule := range builtins {
				if !disabled[rule.ID] {
					rules = append(rules, configuredRule{rule: rule, source: "builtin"})
				}
			}
		}
	}
	for _, rule := range cfg.Rules {
		rules = append(rules, configuredRule{rule: rule, source: "custom"})
	}

	var results []RuleMatchResult
	for _, rc := range ast.Changes {
		if rc.Action == parser.ActionNoOp || rc.Action == parser.ActionRead {
			continue
		}
		for _, configured := range rules {
			if matchRule(configured.rule, rc, ast.Workspace) {
				results = append(results, RuleMatchResult{
					RuleID:      configured.rule.ID,
					Severity:    configured.rule.Severity,
					Description: configured.rule.Description,
					Remediation: configured.rule.Remediation,
					Source:      configured.source,
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
	if m.AttributePath != "" && m.AttributePath2 != "" && (m.Value2 != nil || m.Contains2 != "") {
		if matched, correlated := matchesCorrelatedAttributes(rc.After, rc.Before, m); correlated {
			return matched
		}
	}
	if m.AttributePath != "" && !matchesAttribute(rc.After, rc.Before, m.AttributePath, m.Value, m.Contains) {
		return false
	}
	if m.AttributePath2 != "" && (m.Value2 != nil || m.Contains2 != "") && !matchesAttribute(rc.After, rc.Before, m.AttributePath2, m.Value2, m.Contains2) {
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
		if attributeValueMatches(value, expected, contains) {
			return true
		}
	}
	return false
}

func matchesCorrelatedAttributes(after, before map[string]any, match RuleMatch) (bool, bool) {
	first, err := ParsePath(match.AttributePath)
	if err != nil {
		return false, false
	}
	second, err := ParsePath(match.AttributePath2)
	if err != nil || first.wildcardPrefix == "" || first.wildcardPrefix != second.wildcardPrefix {
		return false, false
	}

	firstValues := resolveIndexed(first, after, before)
	secondValues := resolveIndexed(second, after, before)
	for _, firstValue := range firstValues {
		if !attributeValueMatches(firstValue.Value, match.Value, match.Contains) {
			continue
		}
		for _, secondValue := range secondValues {
			if firstValue.Index == secondValue.Index && attributeValueMatches(secondValue.Value, match.Value2, match.Contains2) {
				return true, true
			}
		}
	}
	return false, true
}

func resolveIndexed(path Path, after, before map[string]any) []IndexedValue {
	values := path.ResolveIndexed(after)
	if len(values) == 0 {
		values = path.ResolveIndexed(before)
	}
	return values
}

func attributeValueMatches(value, expected any, contains string) bool {
	if contains != "" {
		return containsValue(value, contains)
	}
	return expected == nil || valuesEqual(value, expected)
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

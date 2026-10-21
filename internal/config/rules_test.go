package config

import (
	"testing"

	"github.com/terraspin/terraspin/internal/parser"
)

func TestEvaluateRules_noRules(t *testing.T) {
	cfg := &Config{Version: 1}
	ast := &parser.PlanAST{}
	results := EvaluateRules(cfg, ast)
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestEvaluateRules_nilConfig(t *testing.T) {
	results := EvaluateRules(nil, &parser.PlanAST{})
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestEvaluateRules_resourceTypeMatch(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Rules: []Rule{
			{
				ID:          "kill-db",
				Severity:    "critical",
				Description: "DB changes are critical",
				Match: RuleMatch{
					ResourceTypePattern: "aws_db_instance",
				},
			},
		},
	}
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{Address: "aws_db_instance.primary", Type: "aws_db_instance", Action: parser.ActionDelete},
			{Address: "aws_s3_bucket.logs", Type: "aws_s3_bucket", Action: parser.ActionUpdate},
		},
	}
	results := EvaluateRules(cfg, ast)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Address != "aws_db_instance.primary" {
		t.Errorf("address = %q", results[0].Address)
	}
	if results[0].RuleID != "kill-db" {
		t.Errorf("rule id = %q", results[0].RuleID)
	}
}

func TestEvaluateRules_globPattern(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Rules: []Rule{
			{
				ID:       "db-pattern",
				Severity: "high",
				Match: RuleMatch{
					ResourceTypePattern: "*_db_*",
				},
			},
		},
	}
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{Address: "aws_db_instance.x", Type: "aws_db_instance", Action: parser.ActionUpdate},
			{Address: "aws_s3_bucket.y", Type: "aws_s3_bucket", Action: parser.ActionUpdate},
		},
	}
	results := EvaluateRules(cfg, ast)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Address != "aws_db_instance.x" {
		t.Errorf("address = %q", results[0].Address)
	}
}

func TestEvaluateRules_actionMatch(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Rules: []Rule{
			{
				ID:       "no-delete",
				Severity: "critical",
				Match: RuleMatch{
					Action: "delete",
				},
			},
		},
	}
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{Address: "a", Type: "aws_s3_bucket", Action: parser.ActionDelete},
			{Address: "b", Type: "aws_s3_bucket", Action: parser.ActionUpdate},
		},
	}
	results := EvaluateRules(cfg, ast)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Address != "a" {
		t.Errorf("address = %q", results[0].Address)
	}
}

func TestEvaluateRules_noopSkipped(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Rules: []Rule{
			{
				ID:       "catch-all",
				Severity: "high",
				Match:    RuleMatch{Action: "no-op"},
			},
		},
	}
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{Address: "a", Type: "aws_s3_bucket", Action: parser.ActionNoOp},
		},
	}
	results := EvaluateRules(cfg, ast)
	if len(results) != 0 {
		t.Errorf("no-op should be skipped, got %d results", len(results))
	}
}

func TestEvaluateRules_attributeValue(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Rules: []Rule{
			{
				ID:       "public-rds",
				Severity: "critical",
				Match: RuleMatch{
					ResourceTypePattern: "aws_db_instance",
					AttributePath:       "publicly_accessible",
					Value:               true,
				},
			},
		},
	}
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{
				Address: "aws_db_instance.x",
				Type:    "aws_db_instance",
				Action:  parser.ActionUpdate,
				After:   map[string]any{"publicly_accessible": true, "engine": "postgres"},
			},
			{
				Address: "aws_db_instance.y",
				Type:    "aws_db_instance",
				Action:  parser.ActionUpdate,
				After:   map[string]any{"publicly_accessible": false},
			},
		},
	}
	results := EvaluateRules(cfg, ast)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Address != "aws_db_instance.x" {
		t.Errorf("address = %q", results[0].Address)
	}
}

func TestEvaluateRules_contains(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Rules: []Rule{
			{
				ID:       "open-ssh",
				Severity: "high",
				Match: RuleMatch{
					ResourceTypePattern: "aws_security_group_rule",
					AttributePath:       "cidr_blocks",
					Contains:            "0.0.0.0/0",
				},
			},
		},
	}
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{
				Address: "aws_security_group_rule.ssh",
				Type:    "aws_security_group_rule",
				Action:  parser.ActionUpdate,
				After:   map[string]any{"cidr_blocks": "0.0.0.0/0", "from_port": 22},
			},
		},
	}
	results := EvaluateRules(cfg, ast)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
}

func TestEvaluateRules_workspaceMatch(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Rules: []Rule{
			{
				ID:       "prod-gate",
				Severity: "critical",
				Match: RuleMatch{
					Action:           "delete",
					WorkspacePattern: "prod-*",
				},
			},
		},
	}
	ast := &parser.PlanAST{
		Workspace: "prod-eu-west-1",
		Changes: []parser.ResourceChange{
			{Address: "a", Type: "aws_s3_bucket", Action: parser.ActionDelete},
		},
	}
	results := EvaluateRules(cfg, ast)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	// Non-matching workspace
	ast.Workspace = "staging"
	results2 := EvaluateRules(cfg, ast)
	if len(results2) != 0 {
		t.Errorf("staging workspace shouldn't match, got %d", len(results2))
	}
}

func TestGetNestedValue_flat(t *testing.T) {
	data := map[string]any{"foo": "bar", "num": 42}
	if v := getNestedValue(data, "foo"); v != "bar" {
		t.Errorf("got %v", v)
	}
	if v := getNestedValue(data, "num"); v != 42 {
		t.Errorf("got %v", v)
	}
	if v := getNestedValue(data, "missing"); v != nil {
		t.Errorf("got %v", v)
	}
}

func TestGetNestedValue_nested(t *testing.T) {
	data := map[string]any{
		"tags": map[string]any{
			"Name": "my-app",
		},
	}
	if v := getNestedValue(data, "tags.Name"); v != "my-app" {
		t.Errorf("got %v", v)
	}
	if v := getNestedValue(data, "tags.missing"); v != nil {
		t.Errorf("got %v", v)
	}
	if v := getNestedValue(nil, "anything"); v != nil {
		t.Errorf("got %v", v)
	}
}

func TestEvaluateRulesListPaths(t *testing.T) {
	resource := parser.ResourceChange{
		Address: "aws_security_group.web",
		Type:    "aws_security_group",
		Action:  parser.ActionUpdate,
		After: map[string]any{
			"ingress": []any{
				map[string]any{"cidr_blocks": []any{"10.0.0.0/8"}, "from_port": float64(22)},
				map[string]any{"cidr_blocks": []any{"0.0.0.0/0"}, "from_port": float64(80)},
			},
			"protocol": "tcp",
		},
	}
	for _, tc := range []struct {
		name  string
		match RuleMatch
		want  bool
	}{
		{"wildcard list membership", RuleMatch{AttributePath: "ingress[*].cidr_blocks", Contains: "0.0.0.0/0"}, true},
		{"indexed value", RuleMatch{AttributePath: "ingress[0].from_port", Value: 22}, true},
		{"out of range", RuleMatch{AttributePath: "ingress[2].from_port", Value: 22}, false},
		{"wildcard non-list", RuleMatch{AttributePath: "protocol[*]", Contains: "tcp"}, false},
		{"secondary path is independent", RuleMatch{AttributePath: "ingress[*].cidr_blocks", Contains: "0.0.0.0/0", AttributePath2: "ingress[*].from_port", Value2: 22}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Rules: []Rule{{ID: "list-path", Severity: "high", Match: tc.match}}}
			got := len(EvaluateRules(cfg, &parser.PlanAST{Changes: []parser.ResourceChange{resource}})) == 1
			if got != tc.want {
				t.Errorf("matched = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestRuleCharacterizationLegacyFlatPathSemantics(t *testing.T) {
	resource := parser.ResourceChange{
		Type:   "aws_example",
		Action: parser.ActionUpdate,
		After: map[string]any{
			"settings": map[string]any{"port": float64(22), "label": "production"},
			"engine":   "postgres",
		},
		Before: map[string]any{"settings": map[string]any{"port": float64(5432)}},
	}

	for _, tc := range []struct {
		name  string
		match RuleMatch
		want  bool
	}{
		{"nested value compares YAML integer to JSON number", RuleMatch{AttributePath: "settings.port", Value: 22}, true},
		{"contains is substring", RuleMatch{AttributePath: "settings.label", Contains: "prod"}, true},
		{"secondary path is AND", RuleMatch{AttributePath: "settings.port", Value: 22, AttributePath2: "engine", Value2: "mysql"}, false},
		{"after takes precedence over before", RuleMatch{AttributePath: "settings.port", Value: 5432}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchRule(Rule{Match: tc.match}, resource, ""); got != tc.want {
				t.Errorf("matchRule = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestRuleCharacterizationLegacyFallsBackToBefore(t *testing.T) {
	resource := parser.ResourceChange{
		Action: parser.ActionUpdate,
		After:  map[string]any{},
		Before: map[string]any{"settings": map[string]any{"port": float64(5432)}},
	}
	if !matchRule(Rule{Match: RuleMatch{AttributePath: "settings.port", Value: 5432}}, resource, "") {
		t.Fatal("expected before value to match when after path is absent")
	}
}

func TestValuesEqual(t *testing.T) {
	tests := []struct {
		a, b any
		want bool
	}{
		{"hello", "hello", true},
		{"hello", "world", false},
		{true, true, true},
		{true, false, false},
		{42, 42, true},
		{42.0, 42, true},
		{float64(42), 42, true},
		{"42", 42, true},
	}
	for _, tt := range tests {
		got := valuesEqual(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("valuesEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

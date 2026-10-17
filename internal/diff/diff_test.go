package diff

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/terraspin/terraspin/internal/ai"
	"github.com/terraspin/terraspin/internal/parser"
)

func TestCompareRedactsSensitiveValues(t *testing.T) {
	tests := []struct {
		name             string
		beforeA, beforeB map[string]any
		sensitiveA       map[string]any
		sensitiveB       map[string]any
		path             string
		redacted         bool
	}{
		{
			name: "changed attribute", beforeA: map[string]any{"token": "synthetic-a"}, beforeB: map[string]any{"token": "synthetic-b"},
			sensitiveA: map[string]any{"token": true}, sensitiveB: map[string]any{"token": true}, path: "token", redacted: true,
		},
		{
			name: "unchanged attribute", beforeA: map[string]any{"token": "synthetic-same", "visible": "a"}, beforeB: map[string]any{"token": "synthetic-same", "visible": "b"},
			sensitiveA: map[string]any{"token": true}, sensitiveB: map[string]any{"token": true}, path: "visible", redacted: false,
		},
		{
			name: "partially sensitive list", beforeA: map[string]any{"items": []any{"visible", "synthetic-a"}}, beforeB: map[string]any{"items": []any{"visible", "synthetic-b"}},
			sensitiveA: map[string]any{"items": []any{false, true}}, sensitiveB: map[string]any{"items": []any{false, true}}, path: "items", redacted: true,
		},
		{
			name: "sensitive subtree", beforeA: map[string]any{"secret_tree": map[string]any{"key": "synthetic-a"}}, beforeB: map[string]any{"secret_tree": map[string]any{"key": "synthetic-b"}},
			sensitiveA: map[string]any{"secret_tree": true}, sensitiveB: map[string]any{"secret_tree": true}, path: "secret_tree.key", redacted: true,
		},
		{
			name: "different list lengths", beforeA: map[string]any{"items": []any{"visible", "synthetic-a"}}, beforeB: map[string]any{"items": []any{"visible", "synthetic-b", "synthetic-c"}},
			sensitiveA: map[string]any{"items": []any{false, true}}, sensitiveB: map[string]any{"items": []any{false, true, true}}, path: "items", redacted: true,
		},
		{
			name: "marker only on one side", beforeA: map[string]any{"credential": "synthetic-a"}, beforeB: map[string]any{"credential": "synthetic-b"},
			sensitiveA: map[string]any{"credential": true}, sensitiveB: nil, path: "credential", redacted: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Compare(
				sensitivePlan(t, tc.beforeA, tc.sensitiveA),
				sensitivePlan(t, tc.beforeB, tc.sensitiveB),
				"a", "b",
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.Summary.Modified != 1 {
				t.Fatalf("modified = %d, want 1", result.Summary.Modified)
			}
			change := findChange(t, result.Diffs[0].Changes, tc.path)
			if tc.redacted && (change.OldValue != ai.SensitiveRedacted || change.NewValue != ai.SensitiveRedacted) {
				t.Fatal("sensitive attribute change was not redacted")
			}
			if !tc.redacted && (change.OldValue == ai.SensitiveRedacted || change.NewValue == ai.SensitiveRedacted) {
				t.Fatal("non-sensitive attribute change was redacted")
			}
			if _, ok := tc.beforeA["token"]; ok && result.Diffs[0].Old.Before["token"] != ai.SensitiveRedacted {
				t.Fatal("unchanged sensitive value was not redacted in old resource")
			}
		})
	}
}

func sensitivePlan(t *testing.T, before, sensitive map[string]any) []byte {
	t.Helper()
	plan := map[string]any{
		"format_version":    "1.2",
		"terraform_version": "1.7.2",
		"resource_changes": []any{map[string]any{
			"address": "example.sensitive", "mode": "managed", "type": "example", "name": "sensitive",
			"change": map[string]any{
				"actions": []string{"update"}, "before": before, "after": before,
				"before_sensitive": sensitive, "after_sensitive": sensitive,
			},
		}},
	}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func findChange(t *testing.T, changes []AttributeDiff, path string) AttributeDiff {
	t.Helper()
	for _, change := range changes {
		if change.Path == path {
			return change
		}
	}
	t.Fatalf("missing change at %q", path)
	return AttributeDiff{}
}

func TestCompareASTsOrdersAttributeChangesDeterministically(t *testing.T) {
	left := &parser.PlanAST{Changes: []parser.ResourceChange{{
		Address: "example.ordered", Type: "example", Action: parser.ActionUpdate,
		Before: map[string]any{"delta": 1, "alpha": 1, "charlie": 1, "bravo": 1},
		After:  map[string]any{"delta": 2, "alpha": 2, "charlie": 2, "bravo": 2},
	}}}

	right := &parser.PlanAST{Changes: []parser.ResourceChange{{
		Address: "example.ordered", Type: "example", Action: parser.ActionUpdate,
		Before: map[string]any{"delta": 2, "alpha": 2, "charlie": 2, "bravo": 2},
		After:  map[string]any{"delta": 2, "alpha": 2, "charlie": 2, "bravo": 2},
	}}}
	want := []string{"alpha", "bravo", "charlie", "delta"}
	for range 100 {
		gotResult := CompareASTs(left, right, "left", "right")
		got := make([]string, len(gotResult.Diffs[0].Changes))
		for i, change := range gotResult.Diffs[0].Changes {
			got[i] = change.Path
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("attribute paths = %v, want sorted %v", got, want)
		}
	}
}

func TestCompareASTs_detectsRemovedResources(t *testing.T) {
	planA := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{Address: "aws_s3_bucket.logs", Type: "aws_s3_bucket", Action: parser.ActionNoOp},
		},
	}
	planB := &parser.PlanAST{Changes: nil}

	result := CompareASTs(planA, planB, "a", "b")
	if result.Summary.Removed != 1 {
		t.Errorf("Removed = %d, want 1", result.Summary.Removed)
	}
	if result.Summary.Total != 1 {
		t.Errorf("Total = %d, want 1", result.Summary.Total)
	}
	if result.Diffs[0].Status != StatusRemoved {
		t.Errorf("status = %s, want removed", result.Diffs[0].Status)
	}
}

func TestCompareASTs_detectsAddedResources(t *testing.T) {
	planA := &parser.PlanAST{Changes: nil}
	planB := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{Address: "aws_s3_bucket.logs", Type: "aws_s3_bucket", Action: parser.ActionCreate},
		},
	}

	result := CompareASTs(planA, planB, "a", "b")
	if result.Summary.Added != 1 {
		t.Errorf("Added = %d, want 1", result.Summary.Added)
	}
	if result.Diffs[0].Status != StatusAdded {
		t.Errorf("status = %s, want added", result.Diffs[0].Status)
	}
}

func TestCompareASTs_detectsModifiedAttributes(t *testing.T) {
	planA := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{
				Address: "aws_instance.web",
				Type:    "aws_instance",
				Action:  parser.ActionUpdate,
				Before:  map[string]any{"instance_type": "t3.micro"},
				After:   map[string]any{"instance_type": "t3.micro"},
			},
		},
	}
	planB := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{
				Address: "aws_instance.web",
				Type:    "aws_instance",
				Action:  parser.ActionUpdate,
				Before:  map[string]any{"instance_type": "t3.large"},
				After:   map[string]any{"instance_type": "t3.large"},
			},
		},
	}

	result := CompareASTs(planA, planB, "a", "b")
	if result.Summary.Modified != 1 {
		t.Errorf("Modified = %d, want 1", result.Summary.Modified)
	}
	if len(result.Diffs[0].Changes) == 0 {
		t.Error("expected attribute changes, got none")
	}
}

func TestCompareASTs_detectsActionChange(t *testing.T) {
	planA := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{Address: "r", Type: "aws_s3_bucket", Action: parser.ActionUpdate, Before: map[string]any{"x": "a"}, After: map[string]any{"x": "a"}},
		},
	}
	planB := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{Address: "r", Type: "aws_s3_bucket", Action: parser.ActionDelete, Before: map[string]any{"x": "a"}, After: map[string]any{}},
		},
	}

	result := CompareASTs(planA, planB, "a", "b")
	if result.Summary.Modified != 1 {
		t.Errorf("action change should count as Modified, got Modified=%d", result.Summary.Modified)
	}
}

func TestBuildDiffPromptDoesNotContainAttributeValues(t *testing.T) {
	result := &DiffResult{
		LabelA: "a", LabelB: "b", VersionA: "1.2", VersionB: "1.2",
		Summary: DriftSummary{Modified: 1, Total: 1},
		Diffs:   []ResourceDiff{{Address: "example.internal", Type: "example", Status: StatusModified, Changes: []AttributeDiff{{Path: "token", OldValue: "SYNTHETIC_SENSITIVE", NewValue: "SYNTHETIC_UNMARKED"}}}},
	}
	prompt := BuildDiffPrompt(result)
	if strings.Contains(prompt, "SYNTHETIC_") || !strings.Contains(prompt, "example.internal") {
		t.Fatal("diff prompt has wrong payload")
	}
}

func TestDiffSummaryText(t *testing.T) {
	r := &DiffResult{
		Summary: DriftSummary{Added: 2, Removed: 1, Modified: 3, Unchanged: 5, Total: 11},
	}
	text := r.DiffSummaryText()
	if !strings.Contains(text, "2 added") || !strings.Contains(text, "1 removed") || !strings.Contains(text, "3 modified") {
		t.Errorf("summary = %q, missing expected counts", text)
	}
}

func TestCompareASTs_identicalPlansHaveNoDiff(t *testing.T) {
	changes := []parser.ResourceChange{
		{Address: "aws_s3_bucket.logs", Type: "aws_s3_bucket", Action: parser.ActionNoOp},
	}
	ast := &parser.PlanAST{
		TerraformVersion: "1.7.2",
		Changes:          changes,
	}
	result := CompareASTs(ast, ast, "plan-a", "plan-b")
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Summary.Modified != 0 || result.Summary.Added != 0 || result.Summary.Removed != 0 {
		t.Errorf("unexpected diffs: %+v", result.Summary)
	}
	if result.Summary.Unchanged != 1 {
		t.Errorf("Unchanged = %d, want 1", result.Summary.Unchanged)
	}
	if result.LabelA != "plan-a" || result.LabelB != "plan-b" {
		t.Errorf("labels: %q %q", result.LabelA, result.LabelB)
	}
}

package diff

import (
	"strings"
	"testing"

	"github.com/terraspin/terraspin/internal/parser"
)

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
				Before: map[string]any{"instance_type": "t3.micro"},
				After:  map[string]any{"instance_type": "t3.micro"},
			},
		},
	}
	planB := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{
				Address: "aws_instance.web",
				Type:    "aws_instance",
				Action:  parser.ActionUpdate,
				Before: map[string]any{"instance_type": "t3.large"},
				After:  map[string]any{"instance_type": "t3.large"},
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

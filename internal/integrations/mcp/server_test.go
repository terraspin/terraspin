package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/terraspin/terraspin/internal/analyzer"
)

func TestPlanFromArgs_returnsErrorOnMissingPlanJSON(t *testing.T) {
	_, _, _, err := planFromArgs(map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing plan_json")
	}
	if !strings.Contains(err.Error(), "plan_json") {
		t.Errorf("error = %v, should mention plan_json", err)
	}
}

func TestPlanFromArgs_returnsErrorOnInvalidJSON(t *testing.T) {
	_, _, _, err := planFromArgs(map[string]any{"plan_json": "not json"})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestPlanFromArgs_parsesValidPlan(t *testing.T) {
	planJSON := `{
		"format_version": "1.2",
		"terraform_version": "1.7.2",
		"resource_changes": [
			{
				"address": "aws_s3_bucket.logs",
				"mode": "managed",
				"type": "aws_s3_bucket",
				"name": "logs",
				"provider_name": "aws",
				"change": {
					"actions": ["create"],
					"before": null,
					"after": {"bucket": "my-logs"}
				}
			}
		]
	}`
	ast, score, blast, err := planFromArgs(map[string]any{"plan_json": planJSON})
	if err != nil {
		t.Fatal(err)
	}
	if ast.TerraformVersion != "1.7.2" {
		t.Errorf("version = %q", ast.TerraformVersion)
	}
	if len(ast.Changes) != 1 {
		t.Errorf("changes = %d, want 1", len(ast.Changes))
	}
	if score == nil || score.Overall.Tier != analyzer.TierLow {
		t.Errorf("score = %+v", score)
	}
	if blast == nil {
		t.Error("blast is nil")
	}
}

func TestHandleAnalyzePlan_returnsRiskOutput(t *testing.T) {
	planJSON := `{
		"format_version": "1.2",
		"terraform_version": "1.7.2",
		"resource_changes": [
			{
				"address": "aws_db_instance.primary",
				"mode": "managed",
				"type": "aws_db_instance",
				"name": "primary",
				"provider_name": "aws",
				"change": {
					"actions": ["delete"],
					"before": {"engine": "postgres"},
					"after": null
				}
			}
		]
	}`

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"plan_json": planJSON,
	}

	result, err := handleAnalyzePlan(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
	if result.IsError {
		t.Fatal("result has error flag")
	}
	if len(result.Content) == 0 {
		t.Fatal("no content")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "1.7.2") {
		t.Error("missing terraform version")
	}
	if !strings.Contains(text, "CRITICAL") {
		t.Error("missing critical risk tier")
	}
	if !strings.Contains(text, "aws_db_instance.primary") {
		t.Error("missing resource address")
	}
}

func TestHandleGetBlastRadius_returnsForKnownResource(t *testing.T) {
	planJSON := `{
		"format_version": "1.2",
		"terraform_version": "1.0",
		"resource_changes": [
			{
				"address": "aws_db_instance.p",
				"mode": "managed",
				"type": "aws_db_instance",
				"name": "p",
				"provider_name": "aws",
				"change": {
					"actions": ["delete"],
					"before": {"x": "y"},
					"after": null
				}
			}
		]
	}`

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"plan_json":         planJSON,
		"resource_address":  "aws_db_instance.p",
	}

	result, err := handleGetBlastRadius(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "aws_db_instance.p") {
		t.Error("missing resource address in output")
	}
}

func TestHandleGetBlastRadius_notFound(t *testing.T) {
	planJSON := `{"format_version": "1.2", "resource_changes": []}`

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"plan_json":        planJSON,
		"resource_address": "nonexistent.resource",
	}

	result, err := handleGetBlastRadius(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "not found") {
		t.Errorf("should say not found, got: %s", text)
	}
}

func TestHandleGetBlastRadius_missingAddress(t *testing.T) {
	planJSON := `{"format_version": "1.2"}`

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"plan_json": planJSON,
	}

	result, err := handleGetBlastRadius(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Error("expected error for missing resource_address")
	}
}

func TestHandleExplainChange_returnsDetails(t *testing.T) {
	planJSON := `{
		"format_version": "1.2",
		"terraform_version": "1.0",
		"resource_changes": [
			{
				"address": "aws_s3_bucket.logs",
				"mode": "managed",
				"type": "aws_s3_bucket",
				"name": "logs",
				"provider_name": "aws",
				"change": {
					"actions": ["create"],
					"before": null,
					"after": {"bucket": "my-logs"}
				}
			}
		]
	}`

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"plan_json":         planJSON,
		"resource_address":  "aws_s3_bucket.logs",
	}

	result, err := handleExplainChange(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "aws_s3_bucket.logs") {
		t.Error("missing address")
	}
	if !strings.Contains(text, "CREATED") && !strings.Contains(text, "CREATE") {
		t.Error("missing action")
	}
	if !strings.Contains(text, "created") {
		t.Error("missing create description")
	}
}

func TestHandleExplainChange_notFound(t *testing.T) {
	planJSON := `{"format_version": "1.2", "resource_changes": []}`

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"plan_json":        planJSON,
		"resource_address": "does.not.exist",
	}

	result, err := handleExplainChange(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "not found") {
		t.Errorf("should say not found, got: %s", text)
	}
}

func TestHandleGetRiskSummary_returnsJSON(t *testing.T) {
	planJSON := `{
		"format_version": "1.2",
		"terraform_version": "1.7.2",
		"resource_changes": [
			{
				"address": "a",
				"mode": "managed",
				"type": "aws_s3_bucket",
				"name": "a",
				"provider_name": "aws",
				"change": {
					"actions": ["delete"],
					"before": {"x": "y"},
					"after": null
				}
			}
		]
	}`

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"plan_json": planJSON,
	}

	result, err := handleGetRiskSummary(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "overall_risk") {
		t.Error("missing overall_risk in JSON summary")
	}
	if !strings.Contains(text, "overall_score") {
		t.Error("missing overall_score")
	}
	if !strings.Contains(text, "critical") {
		t.Error("missing risk tier")
	}
}

func TestHandleSuggestRollback_hasRecoverySteps(t *testing.T) {
	planJSON := `{
		"format_version": "1.2",
		"terraform_version": "1.0",
		"resource_changes": [
			{
				"address": "aws_db_instance.p",
				"mode": "managed",
				"type": "aws_db_instance",
				"name": "p",
				"provider_name": "aws",
				"change": {
					"actions": ["delete"],
					"before": {"engine": "postgres"},
					"after": null
				}
			},
			{
				"address": "aws_s3_bucket.logs",
				"mode": "managed",
				"type": "aws_s3_bucket",
				"name": "logs",
				"provider_name": "aws",
				"change": {
					"actions": ["create"],
					"before": null,
					"after": {"bucket": "logs"}
				}
			}
		]
	}`

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"plan_json": planJSON,
	}

	result, err := handleSuggestRollback(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Rollback Strategy") {
		t.Error("missing rollback header")
	}
	if !strings.Contains(text, "Restore deleted") {
		t.Error("missing delete recovery section")
	}
	if !strings.Contains(text, "aws_s3_bucket.logs") {
		t.Error("missing resource in create section")
	}
	if !strings.Contains(text, "Database Recovery") {
		t.Error("missing database recovery section")
	}
}

func TestHandleSuggestRollback_includesFailurePoint(t *testing.T) {
	planJSON := `{
		"format_version": "1.2",
		"resource_changes": [
			{
				"address": "aws_db_instance.p",
				"mode": "managed",
				"type": "aws_db_instance",
				"name": "p",
				"provider_name": "aws",
				"change": {
					"actions": ["delete"],
					"before": {"engine": "postgres"},
					"after": null
				}
			}
		]
	}`

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"plan_json":      planJSON,
		"failure_point":  "aws_db_instance.p",
	}

	result, err := handleSuggestRollback(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Failure point") {
		t.Error("missing failure point section")
	}
	if !strings.Contains(text, "aws_db_instance.p") {
		t.Error("missing failure point resource")
	}
}

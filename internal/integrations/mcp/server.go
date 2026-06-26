// Package mcp implements the MCP (Model Context Protocol) server for Terraspin.
// Exposes plan intelligence tools consumable by AI coding assistants.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	server "github.com/mark3labs/mcp-go/server"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/terraspin/terraspin/internal/ai"
	"github.com/terraspin/terraspin/internal/analyzer"
	"github.com/terraspin/terraspin/internal/parser"
)

// NewServer creates a new MCPServer with all Terraspin tools registered.
func NewServer() *server.MCPServer {
	s := server.NewMCPServer(
		"terraspin",
		"0.3.0",
		server.WithInstructions(`Terraspin MCP server — Terraform plan intelligence.

Tools accept a raw terraform plan JSON (output of 'terraform show -json plan.tfplan')
and return risk analysis, blast radius, explanations, and rollback strategies.

Pass the full plan JSON as a string parameter for each tool call.`),
	)

	s.AddTool(newAnalyzePlanTool())
	s.AddTool(newGetBlastRadiusTool())
	s.AddTool(newExplainChangeTool())
	s.AddTool(newGetRiskSummaryTool())
	s.AddTool(newSuggestRollbackTool())

	return s
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// planFromArgs parses plan JSON from the arguments map and runs the full analysis pipeline.
// Returns the parsed AST, risk scores, blast radius, and any error.
// Config is nil by default for MCP mode — rules are not applied unless explicitly loaded.
func planFromArgs(args map[string]any) (*parser.PlanAST, *analyzer.PlanScore, map[string]*analyzer.BlastRadius, error) {
	planJSON, ok := args["plan_json"].(string)
	if !ok || planJSON == "" {
		return nil, nil, nil, fmt.Errorf("missing required argument: plan_json")
	}

	ast, err := parser.ParsePlan([]byte(planJSON))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse plan: %w", err)
	}

	ai.RedactSensitive(ast)
	score := analyzer.ScorePlan(ast)

	refs := analyzer.ParseDependencyRefs([]byte(planJSON))
	blast := analyzer.AnalyzeBlastRadius(ast.Changes, refs)

	return ast, score, blast, nil
}

// textResult builds a CallToolResult with text content.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: text},
		},
	}
}

// errorResult builds a CallToolResult with an error message.
func errorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
		},
		IsError: true,
	}
}

// ---------------------------------------------------------------------------
// Tool: analyze_plan
// ---------------------------------------------------------------------------

func newAnalyzePlanTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("analyze_plan",
		mcp.WithDescription("Analyze a full terraform plan and return risk scores, blast radii, and an AI narrative."),
		mcp.WithString("plan_json",
			mcp.Required(),
			mcp.Description("Raw JSON from `terraform show -json plan.tfplan`"),
		),
	)
	return tool, handleAnalyzePlan
}

func handleAnalyzePlan(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	ast, score, blast, err := planFromArgs(args)
	if err != nil {
		return errorResult(err), nil
	}

	// Build a compact rule-based narrative (no LLM call — we're in MCP mode,
	// the calling LLM will interpret the structured data).
	var b strings.Builder

	// Summary header
	b.WriteString(fmt.Sprintf("## Terraspin Plan Analysis\n\n"))
	b.WriteString(fmt.Sprintf("**Overall Risk:** %s (score: %.0f)\n", string(score.Overall.Tier), score.Overall.Score))
	b.WriteString(fmt.Sprintf("**Terraform:** %s\n\n", ast.TerraformVersion))

	// Resource counts
	var creates, updates, deletes, replaces int
	for _, c := range ast.Changes {
		switch c.Action {
		case parser.ActionCreate:
			creates++
		case parser.ActionUpdate:
			updates++
		case parser.ActionDelete:
			deletes++
		case parser.ActionReplace:
			replaces++
		}
	}
	b.WriteString(fmt.Sprintf("**Changes:** %d create, %d update, %d delete, %d replace\n\n", creates, updates, deletes, replaces))

	// Tier counts
	b.WriteString(fmt.Sprintf("**Risk breakdown:** critical=%d, high=%d, medium=%d, low=%d\n\n",
		score.Counts[analyzer.TierCritical], score.Counts[analyzer.TierHigh],
		score.Counts[analyzer.TierMedium], score.Counts[analyzer.TierLow]))

	// Critical & high changes with blast radius
	b.WriteString("### Critical & High Risk Changes\n\n")
	for _, rs := range score.ResourceScores {
		if rs.Tier == analyzer.TierCritical || rs.Tier == analyzer.TierHigh {
			b.WriteString(fmt.Sprintf("- **%s** `%s` → *%s* (score: %.1f)\n", strings.ToUpper(string(rs.Tier)), rs.Address, strings.ToUpper(string(rs.Action)), rs.Score))
			if br, ok := blast[rs.Address]; ok && br.TotalAffected > 0 {
				b.WriteString(fmt.Sprintf("  - Blast radius: %d affected\n", br.TotalAffected))
				for _, d := range br.DirectDeps {
					b.WriteString(fmt.Sprintf("    - %s\n", d.Address))
				}
			}
		}
	}

	// Rollback strategy
	b.WriteString("\n### Rollback Strategy\n\n")
	b.WriteString(rollbackOneLiner)

	// JSON summary for structured consumption
	type riskEntry struct {
		Address     string  `json:"address"`
		Action      string  `json:"action"`
		RiskTier    string  `json:"risk_tier"`
		RiskScore   float64 `json:"risk_score"`
		BlastRadius int     `json:"blast_radius,omitempty"`
	}

	var entries []riskEntry
	for _, rs := range score.ResourceScores {
		e := riskEntry{
			Address:   rs.Address,
			Action:    string(rs.Action),
			RiskTier:  string(rs.Tier),
			RiskScore: rs.Score,
		}
		if br, ok := blast[rs.Address]; ok {
			e.BlastRadius = br.TotalAffected
		}
		entries = append(entries, e)
	}

	summaryJSON, _ := json.Marshal(map[string]any{
		"overall_risk":     string(score.Overall.Tier),
		"overall_score":    score.Overall.Score,
		"terraform_version": ast.TerraformVersion,
		"tier_counts": map[string]int{
			"critical": score.Counts[analyzer.TierCritical],
			"high":     score.Counts[analyzer.TierHigh],
			"medium":   score.Counts[analyzer.TierMedium],
			"low":      score.Counts[analyzer.TierLow],
		},
		"resource_changes_count": len(ast.Changes),
		"resources":              entries,
	})

	b.WriteString(fmt.Sprintf("\n```json\n%s\n```\n", string(summaryJSON)))

	return textResult(b.String()), nil
}

// ---------------------------------------------------------------------------
// Tool: get_blast_radius
// ---------------------------------------------------------------------------

func newGetBlastRadiusTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("get_blast_radius",
		mcp.WithDescription("Get the blast radius for a specific resource in a terraform plan."),
		mcp.WithString("plan_json",
			mcp.Required(),
			mcp.Description("Raw JSON from `terraform show -json plan.tfplan`"),
		),
		mcp.WithString("resource_address",
			mcp.Required(),
			mcp.Description("Full resource address, e.g. `aws_db_instance.primary`"),
		),
	)
	return tool, handleGetBlastRadius
}

func handleGetBlastRadius(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	_, _, blast, err := planFromArgs(args)
	if err != nil {
		return errorResult(err), nil
	}

	addr, _ := args["resource_address"].(string)
	if addr == "" {
		return errorResult(fmt.Errorf("missing required argument: resource_address")), nil
	}

	br, ok := blast[addr]
	if !ok {
		return textResult(fmt.Sprintf("Resource `%s` not found in the plan or has no blast radius.", addr)), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Blast Radius: `%s`\n\n", br.RootAddress))
	b.WriteString(fmt.Sprintf("**Total affected:** %d resources\n\n", br.TotalAffected))

	if len(br.DirectDeps) > 0 {
		b.WriteString("### Direct dependents (1 hop)\n")
		for _, d := range br.DirectDeps {
			b.WriteString(fmt.Sprintf("- `%s` (type: %s)\n", d.Address, d.Type))
		}
		b.WriteString("\n")
	}

	if len(br.TransitiveDeps) > 0 {
		b.WriteString("### Transitive dependents (2+ hops)\n")
		for _, d := range br.TransitiveDeps {
			b.WriteString(fmt.Sprintf("- `%s` (type: %s)\n", d.Address, d.Type))
		}
		b.WriteString("\n")
	}

	if br.TotalAffected == 0 {
		b.WriteString("No downstream resources depend on this resource.\n")
	}

	return textResult(b.String()), nil
}

// ---------------------------------------------------------------------------
// Tool: explain_change
// ---------------------------------------------------------------------------

func newExplainChangeTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("explain_change",
		mcp.WithDescription("Get a plain-English explanation of a single resource change in a terraform plan."),
		mcp.WithString("plan_json",
			mcp.Required(),
			mcp.Description("Raw JSON from `terraform show -json plan.tfplan`"),
		),
		mcp.WithString("resource_address",
			mcp.Required(),
			mcp.Description("Full resource address, e.g. `aws_security_group.web`"),
		),
	)
	return tool, handleExplainChange
}

func handleExplainChange(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	ast, score, blast, err := planFromArgs(args)
	if err != nil {
		return errorResult(err), nil
	}

	addr, _ := args["resource_address"].(string)
	if addr == "" {
		return errorResult(fmt.Errorf("missing required argument: resource_address")), nil
	}

	// Find the resource change
	var rc *parser.ResourceChange
	for i := range ast.Changes {
		if ast.Changes[i].Address == addr {
			rc = &ast.Changes[i]
			break
		}
	}
	if rc == nil {
		return textResult(fmt.Sprintf("Resource `%s` not found in the plan.", addr)), nil
	}

	// Find its risk score
	var rs *analyzer.ResourceRiskScore
	for i := range score.ResourceScores {
		if score.ResourceScores[i].Address == addr {
			rs = &score.ResourceScores[i]
			break
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Change: `%s`\n\n", rc.Address))
	b.WriteString(fmt.Sprintf("**Action:** %s\n", strings.ToUpper(string(rc.Action))))
	b.WriteString(fmt.Sprintf("**Type:** %s\n", rc.Type))
	b.WriteString(fmt.Sprintf("**Provider:** %s\n", rc.ProviderName))

	if rs != nil {
		b.WriteString(fmt.Sprintf("**Risk:** %s (score: %.1f)\n", strings.ToUpper(string(rs.Tier)), rs.Score))
	}

	if rc.ActionReason != "" {
		b.WriteString(fmt.Sprintf("**Reason:** %s\n", rc.ActionReason))
	}

	// Describe what changed (non-sensitive keys only)
	if rc.Action == parser.ActionUpdate || rc.Action == parser.ActionReplace {
		b.WriteString("\n### Attribute Changes\n\n")
		seen := map[string]struct{}{}
		for k := range rc.Before {
			seen[k] = struct{}{}
		}
		for k := range rc.After {
			seen[k] = struct{}{}
		}
		for _, k := range slices.Sorted(maps.Keys(seen)) {
			bv := fmt.Sprintf("%v", rc.Before[k])
			av := fmt.Sprintf("%v", rc.After[k])
			if bv == av {
				continue
			}
			if strings.Contains(bv, ai.SensitiveRedacted) || strings.Contains(av, ai.SensitiveRedacted) {
				b.WriteString(fmt.Sprintf("- **%s:** `[SENSITIVE REDACTED]` → `[SENSITIVE REDACTED]`\n", k))
			} else {
				if len(bv) > 80 {
					bv = bv[:80] + "..."
				}
				if len(av) > 80 {
					av = av[:80] + "..."
				}
				b.WriteString(fmt.Sprintf("- **%s:** `%s` → `%s`\n", k, bv, av))
			}
		}
	} else if rc.Action == parser.ActionCreate {
		b.WriteString("\nThis resource will be **created** — no existing state to compare.\n")
	} else if rc.Action == parser.ActionDelete {
		b.WriteString("\nThis resource will be **permanently deleted**.\n")
	}

	// Blast radius
	if br, ok := blast[addr]; ok && br.TotalAffected > 0 {
		b.WriteString(fmt.Sprintf("\n### Impact\n\nBlast radius: %d resources affected.\n", br.TotalAffected))
	}

	return textResult(b.String()), nil
}

// ---------------------------------------------------------------------------
// Tool: get_risk_summary
// ---------------------------------------------------------------------------

func newGetRiskSummaryTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("get_risk_summary",
		mcp.WithDescription("Get a lightweight risk scorecard for a terraform plan (fast, no AI narrative)."),
		mcp.WithString("plan_json",
			mcp.Required(),
			mcp.Description("Raw JSON from `terraform show -json plan.tfplan`"),
		),
	)
	return tool, handleGetRiskSummary
}

func handleGetRiskSummary(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	_, score, _, err := planFromArgs(args)
	if err != nil {
		return errorResult(err), nil
	}

	summary := struct {
		OverallRisk  string         `json:"overall_risk"`
		OverallScore float64        `json:"overall_score"`
		TierCounts   map[string]int `json:"tier_counts"`
		Resources    []struct {
			Address   string  `json:"address"`
			Action    string  `json:"action"`
			RiskTier  string  `json:"risk_tier"`
			RiskScore float64 `json:"risk_score"`
		} `json:"resources"`
	}{
		OverallRisk:  string(score.Overall.Tier),
		OverallScore: score.Overall.Score,
		TierCounts: map[string]int{
			"critical": score.Counts[analyzer.TierCritical],
			"high":     score.Counts[analyzer.TierHigh],
			"medium":   score.Counts[analyzer.TierMedium],
			"low":      score.Counts[analyzer.TierLow],
		},
	}

	for _, rs := range score.ResourceScores {
		summary.Resources = append(summary.Resources, struct {
			Address   string  `json:"address"`
			Action    string  `json:"action"`
			RiskTier  string  `json:"risk_tier"`
			RiskScore float64 `json:"risk_score"`
		}{
			Address:   rs.Address,
			Action:    string(rs.Action),
			RiskTier:  string(rs.Tier),
			RiskScore: rs.Score,
		})
	}

	raw, _ := json.MarshalIndent(summary, "", "  ")
	return textResult(fmt.Sprintf("```json\n%s\n```", string(raw))), nil
}

// ---------------------------------------------------------------------------
// Tool: suggest_rollback
// ---------------------------------------------------------------------------

func newSuggestRollbackTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("suggest_rollback",
		mcp.WithDescription("Generate a structured rollback strategy for a terraform plan, optionally after a specific failure point."),
		mcp.WithString("plan_json",
			mcp.Required(),
			mcp.Description("Raw JSON from `terraform show -json plan.tfplan`"),
		),
		mcp.WithString("failure_point",
			mcp.Description("Optional resource address where apply failed, e.g. `aws_rds_instance.primary`"),
		),
	)
	return tool, handleSuggestRollback
}

func handleSuggestRollback(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	ast, _, blast, err := planFromArgs(args)
	if err != nil {
		return errorResult(err), nil
	}

	failurePoint, _ := args["failure_point"].(string)

	var b strings.Builder
	b.WriteString("## Rollback Strategy\n\n")
	b.WriteString(fmt.Sprintf("**Generated:** %s\n\n", time.Now().UTC().Format(time.RFC3339)))

	if failurePoint != "" {
		b.WriteString(fmt.Sprintf("**Failure point:** `%s`\n\n", failurePoint))
	} else {
		b.WriteString("**Failure point:** not specified (general strategy)\n\n")
	}

	// Categorize changes by action
	var deletes, creates, replaces, updates []string
	for _, c := range ast.Changes {
		switch c.Action {
		case parser.ActionDelete:
			deletes = append(deletes, c.Address)
		case parser.ActionCreate:
			creates = append(creates, c.Address)
		case parser.ActionReplace:
			replaces = append(replaces, c.Address)
		case parser.ActionUpdate:
			updates = append(updates, c.Address)
		}
	}

	// General recovery steps
	b.WriteString("### Recovery Steps\n\n")

	if len(deletes) > 0 || len(replaces) > 0 {
		b.WriteString("1. **Restore deleted/destroyed resources** — roll back to the previous Terraform state:\n")
		b.WriteString("   ```\n   terraform state push <previous-state-backup>\n   ```\n")
		b.WriteString("   Or re-apply the previous known-good configuration.\n\n")
	}

	if len(creates) > 0 {
		b.WriteString("2. **Remove orphaned created resources** — if creation succeeded but subsequent steps failed, remove:\n")
		for _, addr := range creates {
			b.WriteString(fmt.Sprintf("   - `%s`\n", addr))
		}
		b.WriteString("\n")
	}

	if len(updates) > 0 {
		b.WriteString("3. **Revert in-place updates** — re-run terraform apply with the previous version of the configuration.\n\n")
	}

	if failurePoint != "" {
		b.WriteString(fmt.Sprintf("4. **Failure point recovery** — `%s` requires manual intervention:\n", failurePoint))
		if br, ok := blast[failurePoint]; ok && br.TotalAffected > 0 {
			b.WriteString(fmt.Sprintf("   - %d resources depended on this. Verify they are in a consistent state.\n", br.TotalAffected))
		}
		b.WriteString("   - Check Terraform state and AWS/GCP/Azure console for partial state.\n")
		b.WriteString("   - Restore from backup if the resource is a database or stateful service.\n")
	}

	// ponytail: always show DB recovery, avoids duplicating prefix list from analyzer/risk.go
	b.WriteString("\n### Database Recovery\n\n")
	b.WriteString("If a database resource (RDS, DynamoDB, etc.) was affected:\n")
	b.WriteString("- Verify the most recent automated snapshot exists\n")
	b.WriteString("- Restore from snapshot: `aws rds restore-db-instance-from-db-snapshot`\n")
	b.WriteString("- Update connection strings in dependent applications\n")

	// General recommendation
	b.WriteString("\n### Recommended Checks Before Recovery\n\n")
	b.WriteString("- Verify the plan's `--fail-on` threshold was appropriate\n")
	b.WriteString("- Check that the S3 backend (or remote state store) has versioning enabled\n")
	b.WriteString("- Ensure the state file itself was not corrupted during the failed apply\n")
	b.WriteString("- Review recent changes in version control for the affected resource configurations\n")

	return textResult(b.String()), nil
}

// ponytail: shared one-liner for rollback, reused across analyze_plan and suggest_rollback
const rollbackOneLiner = "Apply the previous known-good Terraform state version. For destroyed resources, restore from the most recent backup or snapshot.\n"

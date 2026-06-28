package formatter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/terraspin/terraspin/internal/ai"
	"github.com/terraspin/terraspin/internal/analyzer"
	"github.com/terraspin/terraspin/internal/parser"
)

// ---------------------------------------------------------------------------
// Text format
// ---------------------------------------------------------------------------

// FormatText renders the full analysis as colored terminal text.
func FormatText(ast *parser.PlanAST, score *analyzer.PlanScore, blast map[string]*analyzer.BlastRadius, narr *ai.Narrative, verbose bool) string {
	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "┌─ Terraspin Plan Analysis ──────────────────────────────────┐\n")
	changed := len(ast.Changes)
	fmt.Fprintf(&b, "│  %d resources  ·  terraform %s\n", changed, ast.TerraformVersion)
	fmt.Fprintf(&b, "└────────────────────────────────────────────────────────────┘\n\n")

	// Overall risk
	fmt.Fprintf(&b, "  Overall Risk  %s (score: %.0f)\n\n", strings.ToUpper(string(score.Overall.Tier)), score.Overall.Score)

	// Counts by action
	counts := ast.CountByAction()
	fmt.Fprintf(&b, "  %d to create  ·  %d to update  ·  %d to delete  ·  %d to replace\n\n",
		counts["create"], counts["update"], counts["delete"], counts["replace"])

	// Critical/high changes
	hasCritical := false
	for _, rs := range score.ResourceScores {
		if rs.Tier == analyzer.TierCritical || rs.Tier == analyzer.TierHigh {
			if !hasCritical {
				fmt.Fprintf(&b, "──── Critical & High Risk Changes ────────────────────────────\n\n")
				hasCritical = true
			}
			fmt.Fprintf(&b, "  [%s %5.1f]  %s  →  %s\n", strings.ToUpper(string(rs.Tier)), rs.Score, rs.Address, strings.ToUpper(string(rs.Action)))
			if br, ok := blast[rs.Address]; ok && br.TotalAffected > 0 {
				fmt.Fprintf(&b, "  Blast radius: %d resources affected\n", br.TotalAffected)
				for _, d := range br.DirectDeps {
					fmt.Fprintf(&b, "    ├── %s\n", d.Address)
				}
			}
			fmt.Fprintln(&b)
		}
	}

	// Narrative
	if narr != nil && narr.Summary != "" {
		fmt.Fprintf(&b, "──── Analysis ────────────────────────────────────────────────\n\n")

		// Infrastructure summary
		if narr.InfraSummary != "" {
			fmt.Fprintf(&b, "  Infrastructure summary\n  %s\n\n", narr.InfraSummary)
		}

		// Risk score & level
		if narr.RiskScore != "" && narr.RiskLevel != "" {
			fmt.Fprintf(&b, "  Risk score: %s  ·  Risk level: %s\n\n", narr.RiskScore, narr.RiskLevel)
		}

		// Resource change summary
		if narr.ResourceChangeSummary != "" {
			fmt.Fprintf(&b, "  Resource change summary\n  %s\n\n", narr.ResourceChangeSummary)
		}

		// Blast radius summary
		if narr.BlastRadiusSummary != "" {
			fmt.Fprintf(&b, "  Blast radius summary\n  %s\n\n", narr.BlastRadiusSummary)
		}

		// Critical findings
		if len(narr.CriticalFindings) > 0 {
			fmt.Fprintf(&b, "  Critical findings\n")
			for _, f := range narr.CriticalFindings {
				fmt.Fprintf(&b, "    ⚠  %s\n", f)
			}
			fmt.Fprintln(&b)
		}

		// Affected resources grouped by tier
		if len(narr.AffectedByTier) > 0 {
			fmt.Fprintf(&b, "  Affected resources by severity\n")
			for _, g := range narr.AffectedByTier {
				fmt.Fprintf(&b, "    [%s] %s\n", strings.ToUpper(string(g.Tier)), strings.Join(g.Resources, ", "))
			}
			fmt.Fprintln(&b)
		}

		// Summary
		fmt.Fprintf(&b, "  Summary\n  %s\n\n", narr.Summary)

		// Risk assessment
		if narr.RiskAssessment != "" {
			fmt.Fprintf(&b, "  Risk assessment\n  %s\n\n", narr.RiskAssessment)
		}

		// Actionable recommendations
		if len(narr.Recommendations) > 0 {
			fmt.Fprintf(&b, "  Actionable recommendations\n")
			for _, r := range narr.Recommendations {
				fmt.Fprintf(&b, "    □  %s\n", r)
			}
			fmt.Fprintln(&b)
		}

		// Rollback strategy
		if narr.RollbackStrategy != "" {
			fmt.Fprintf(&b, "  Rollback strategy\n  %s\n\n", narr.RollbackStrategy)
		}

		// Next steps
		if len(narr.NextSteps) > 0 {
			fmt.Fprintf(&b, "  Next steps\n")
			for _, s := range narr.NextSteps {
				fmt.Fprintf(&b, "    →  %s\n", s)
			}
			fmt.Fprintln(&b)
		}
	}

	// Verbose: show medium/low
	if verbose {
		fmt.Fprintf(&b, "\n──── Medium & Low Risk ──────────────────────────────────────\n\n")
		for _, rs := range score.ResourceScores {
			if rs.Tier == analyzer.TierMedium || rs.Tier == analyzer.TierLow {
				fmt.Fprintf(&b, "  [%-8s %5.1f]  %s  →  %s\n", rs.Tier, rs.Score, rs.Address, strings.ToUpper(string(rs.Action)))
			}
		}
	}

	// Footer
	fmt.Fprintf(&b, "\n──── %d critical  ·  %d high  ·  %d medium  ·  %d low ──────────\n",
		score.Counts[analyzer.TierCritical], score.Counts[analyzer.TierHigh],
		score.Counts[analyzer.TierMedium], score.Counts[analyzer.TierLow])

	return b.String()
}



// ---------------------------------------------------------------------------
// JSON format
// ---------------------------------------------------------------------------

// FormatJSON renders the analysis as structured JSON.
func FormatJSON(ast *parser.PlanAST, score *analyzer.PlanScore, blast map[string]*analyzer.BlastRadius, narr *ai.Narrative) (string, error) {
	out := struct {
		TerraformVersion string                     `json:"terraform_version"`
		ResourceCounts   map[string]int             `json:"resource_counts"`
		OverallRisk      analyzer.RiskScore         `json:"overall_risk"`
		ResourceRisks    []analyzer.ResourceRiskScore `json:"resource_risks"`
		BlastRadii       map[string]*analyzer.BlastRadius `json:"blast_radii,omitempty"`
		Narrative        *ai.Narrative              `json:"narrative,omitempty"`
	}{
		TerraformVersion: ast.TerraformVersion,
		ResourceCounts:   ast.CountByAction(),
		OverallRisk:      score.Overall,
		ResourceRisks:    score.ResourceScores,
		BlastRadii:       blast,
		Narrative:        narr,
	}

	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ---------------------------------------------------------------------------
// Markdown format
// ---------------------------------------------------------------------------

// FormatMarkdown renders the analysis as GitHub-flavored markdown.
func FormatMarkdown(ast *parser.PlanAST, score *analyzer.PlanScore, blast map[string]*analyzer.BlastRadius, narr *ai.Narrative) string {
	var b strings.Builder

	fmt.Fprintf(&b, "## 🌀 Terraspin Plan Analysis\n\n")
	fmt.Fprintf(&b, "**Terraform:** %s  ·  **Risk:** %s (%.0f)\n\n",
		ast.TerraformVersion, string(score.Overall.Tier), score.Overall.Score)

	counts := ast.CountByAction()
	fmt.Fprintf(&b, "| Action | Count |\n|--------|-------|\n")
	for _, action := range []string{"create", "update", "delete", "replace"} {
		if c := counts[action]; c > 0 {
			fmt.Fprintf(&b, "| %s | %d |\n", action, c)
		}
	}

	if narr != nil {
		// Infrastructure summary
		if narr.InfraSummary != "" {
			fmt.Fprintf(&b, "\n### Infrastructure Summary\n%s\n\n", narr.InfraSummary)
		}

		// Risk score & level
		if narr.RiskScore != "" && narr.RiskLevel != "" {
			fmt.Fprintf(&b, "### Risk\n**Score:** %s  ·  **Level:** %s\n\n", narr.RiskScore, narr.RiskLevel)
		}

		// Resource change summary
		if narr.ResourceChangeSummary != "" {
			fmt.Fprintf(&b, "### Resource Changes\n%s\n\n", narr.ResourceChangeSummary)
		}

		// Blast radius summary
		if narr.BlastRadiusSummary != "" {
			fmt.Fprintf(&b, "### Blast Radius\n%s\n\n", narr.BlastRadiusSummary)
		}

		// Critical findings
		if len(narr.CriticalFindings) > 0 {
			fmt.Fprintf(&b, "### ⚠ Critical Findings\n")
			for _, f := range narr.CriticalFindings {
				fmt.Fprintf(&b, "- %s\n", f)
			}
			fmt.Fprintln(&b)
		}

		// Affected resources grouped by tier
		if len(narr.AffectedByTier) > 0 {
			fmt.Fprintf(&b, "### Affected Resources by Severity\n")
			for _, g := range narr.AffectedByTier {
				fmt.Fprintf(&b, "- **[%s]** %s\n", strings.ToUpper(string(g.Tier)), strings.Join(g.Resources, ", "))
			}
			fmt.Fprintln(&b)
		}

		// Summary
		fmt.Fprintf(&b, "\n### Summary\n%s\n\n", narr.Summary)

		if len(narr.CriticalChanges) > 0 {
			fmt.Fprintf(&b, "### Critical Changes\n")
			for _, c := range narr.CriticalChanges {
				fmt.Fprintf(&b, "- %s\n", c)
			}
			fmt.Fprintln(&b)
		}
		if narr.RiskAssessment != "" {
			fmt.Fprintf(&b, "### Risk Assessment\n%s\n\n", narr.RiskAssessment)
		}
		if len(narr.Recommendations) > 0 {
			fmt.Fprintf(&b, "### Actionable Recommendations\n")
			for _, r := range narr.Recommendations {
				fmt.Fprintf(&b, "- [ ] %s\n", r)
			}
			fmt.Fprintln(&b)
		}
		if narr.RollbackStrategy != "" {
			fmt.Fprintf(&b, "### Rollback Strategy\n%s\n\n", narr.RollbackStrategy)
		}
		if len(narr.NextSteps) > 0 {
			fmt.Fprintf(&b, "### Next Steps\n")
			for _, s := range narr.NextSteps {
				fmt.Fprintf(&b, "%s\n", s)
			}
			fmt.Fprintln(&b)
		}
	}

	// Risk table
	fmt.Fprintf(&b, "\n### Risk Breakdown\n\n")
	fmt.Fprintf(&b, "| Tier | Count |\n|------|-------|\n")
	for _, t := range []analyzer.RiskTier{analyzer.TierCritical, analyzer.TierHigh, analyzer.TierMedium, analyzer.TierLow} {
		fmt.Fprintf(&b, "| %s | %d |\n", t, score.Counts[t])
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------



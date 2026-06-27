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
		if narr.Summary != "" {
			fmt.Fprintf(&b, "  Summary\n  %s\n\n", narr.Summary)
		}
		if narr.RiskAssessment != "" {
			fmt.Fprintf(&b, "  Risk assessment\n  %s\n\n", narr.RiskAssessment)
		}
		if len(narr.Recommendations) > 0 {
			fmt.Fprintf(&b, "  Recommended checks\n")
			for _, r := range narr.Recommendations {
				fmt.Fprintf(&b, "    □  %s\n", r)
			}
			fmt.Fprintln(&b)
		}
		if narr.RollbackStrategy != "" {
			fmt.Fprintf(&b, "  Rollback strategy\n  %s\n", narr.RollbackStrategy)
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
			fmt.Fprintf(&b, "### Recommended Checks\n")
			for _, r := range narr.Recommendations {
				fmt.Fprintf(&b, "- [ ] %s\n", r)
			}
			fmt.Fprintln(&b)
		}
		if narr.RollbackStrategy != "" {
			fmt.Fprintf(&b, "### Rollback Strategy\n%s\n", narr.RollbackStrategy)
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



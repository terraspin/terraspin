package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/terraspin/terraspin/internal/safetext"
)

// Narrative holds the analysis briefing.
type Narrative struct {
	Provider              string      `json:"provider"`
	Summary               string      `json:"summary"`
	InfraSummary          string      `json:"infra_summary,omitempty"`
	RiskScore             string      `json:"risk_score,omitempty"`
	RiskLevel             string      `json:"risk_level,omitempty"`
	ResourceChangeSummary string      `json:"resource_change_summary,omitempty"`
	BlastRadiusSummary    string      `json:"blast_radius_summary,omitempty"`
	CriticalFindings      []string    `json:"critical_findings,omitempty"`
	AffectedByTier        []TierGroup `json:"affected_by_tier,omitempty"`
	Recommendations       []string    `json:"recommendations"`
	NextSteps             []string    `json:"next_steps,omitempty"`
	CriticalChanges       []string    `json:"critical_changes"`
	RiskAssessment        string      `json:"risk_assessment"`
	RollbackStrategy      string      `json:"rollback_strategy"`
}

// TierGroup groups affected resources by risk tier.
type TierGroup struct {
	Tier      string   `json:"tier"`
	Resources []string `json:"resources"`
}

// PlanChangeSummary is a compact representation of one resource for LLM prompts.
type PlanChangeSummary struct {
	Address   string
	Action    string
	Tier      string
	BlastDesc string
}

// BuildAnalysisPrompt creates a compact prompt for the LLM from plan data.
// ponytail: structured prompt with exact JSON schema to maximise parse rate.
func BuildAnalysisPrompt(changes []PlanChangeSummary, riskTier string, riskScore float64, counts map[string]int) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Overall plan risk: %s (score: %.0f)\n", riskTier, riskScore))
	b.WriteString(fmt.Sprintf("Resource counts by tier: critical=%d high=%d medium=%d low=%d\n\n",
		counts["critical"], counts["high"], counts["medium"], counts["low"]))
	b.WriteString("Changed resources:\n")
	for _, c := range changes {
		b.WriteString(fmt.Sprintf("- %s [%s] → %s", safetext.SafeAddress(c.Address), safetext.SafeSegment(c.Tier), strings.ToUpper(safetext.SafeSegment(c.Action))))
		if c.BlastDesc != "" {
			b.WriteString(fmt.Sprintf(" | blast: %s", c.BlastDesc))
		}
		b.WriteString("\n")
	}
	b.WriteString(`
Analyze this plan and return ONLY valid JSON with these exact keys:
{
  "summary": "2-3 sentence plain-English summary for a non-technical manager",
  "critical_changes": ["one string per critical/high change explaining business impact"],
  "risk_assessment": "1-2 sentences on why this plan is risky or safe",
  "recommendations": ["specific actionable check before applying — not 'review changes'"],
  "rollback_strategy": "numbered steps to recover if apply fails — specific commands where possible"
}`)
	return b.String()
}

// BuildRuleBasedNarrative creates a thorough narrative from plan data (no LLM call).
// Produces a genuinely useful report without any AI dependency.
func BuildRuleBasedNarrative(riskTier string, riskScore float64, criticalChanges []string, infraSummary, resourceChangeSummary, blastRadiusSummary string, criticalFindings []string, affectedByTier []TierGroup, recs, nextSteps []string) *Narrative {
	n := &Narrative{
		Provider:              "rule-based",
		Summary:               fmt.Sprintf("This plan affects %s infrastructure with overall risk %s (%.0f/100). %d changes need immediate review.", infraSummary, strings.ToUpper(riskTier), riskScore, len(criticalChanges)),
		InfraSummary:          infraSummary,
		RiskScore:             fmt.Sprintf("%.0f/100", riskScore),
		RiskLevel:             strings.ToUpper(riskTier),
		ResourceChangeSummary: resourceChangeSummary,
		BlastRadiusSummary:    blastRadiusSummary,
		CriticalFindings:      criticalFindings,
		AffectedByTier:        affectedByTier,
		CriticalChanges:       criticalChanges,
		RiskAssessment:        fmt.Sprintf("Plan scored %s (%.0f/100) with %d critical or high-risk changes. Changes to databases, networking, DNS, or IAM can cause cascading outages. Verify dependencies, ensure rollback plans are in place, and apply during a maintenance window if critical resources are affected.", riskTier, riskScore, len(criticalChanges)),
		RollbackStrategy:      "1. terraform state pull > backup.tfstate (save current state)\n2. terraform destroy -target=<failed resource> (revert specific resource)\n3. terraform apply -var-file=previous.tfvars (re-apply last known good config)\n4. For stateful resources: restore from most recent backup/snapshot (RDS snapshot, EBS snapshot, S3 versioning)",
	}
	if recs == nil {
		recs = []string{"Review all changes before applying", "Ensure recent state backup exists", "Verify dependent resources are healthy"}
	}
	n.Recommendations = recs
	if nextSteps == nil {
		nextSteps = []string{"1. Review this report with your team", "2. Apply in a staging/development environment first if possible", "3. Schedule the apply during a maintenance window for critical/high changes", "4. Verify health of affected resources after apply"}
	}
	n.NextSteps = nextSteps
	return n
}

// stripMarkdownFences removes surrounding ```json/``` or ~~~json/~~~ fences
// and trims whitespace. Returns the inner text.
func stripMarkdownFences(text string) string {
	t := strings.TrimSpace(text)
	for _, pair := range [][2]string{
		{"```json", "```"},
		{"```", "```"},
		{"~~~json", "~~~"},
		{"~~~", "~~~"},
	} {
		open, close := pair[0], pair[1]
		if strings.HasPrefix(t, open) {
			t = strings.TrimPrefix(t, open)
			if idx := strings.LastIndex(t, close); idx >= 0 {
				t = t[:idx]
			}
			return strings.TrimSpace(t)
		}
	}
	return t
}

// extractJSON pulls the first outermost JSON object from text, handling
// markdown fences, whitespace, and surrounding prose.
func extractJSON(text string) (string, bool) {
	t := stripMarkdownFences(text)

	start := strings.IndexByte(t, '{')
	if start < 0 {
		return "", false
	}

	depth := 0
	for i := start; i < len(t); i++ {
		switch t[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return t[start : i+1], true
			}
		}
	}
	return "", false
}

// SanitizeNarrative bounds LLM-controlled narrative text before it reaches output.
func SanitizeNarrative(n *Narrative) *Narrative {
	if n == nil {
		return nil
	}

	remaining := 6000
	n.Provider = boundedNarrativeString(n.Provider, 1500, &remaining)
	n.Summary = boundedNarrativeString(n.Summary, 1500, &remaining)
	n.Recommendations = boundedNarrativeList(n.Recommendations, &remaining)
	n.CriticalChanges = boundedNarrativeList(n.CriticalChanges, &remaining)
	n.CriticalFindings = boundedNarrativeList(n.CriticalFindings, &remaining)
	n.NextSteps = boundedNarrativeList(n.NextSteps, &remaining)
	n.InfraSummary = boundedNarrativeString(n.InfraSummary, 1500, &remaining)
	n.RiskScore = boundedNarrativeString(n.RiskScore, 1500, &remaining)
	n.RiskLevel = boundedNarrativeString(n.RiskLevel, 1500, &remaining)
	n.ResourceChangeSummary = boundedNarrativeString(n.ResourceChangeSummary, 1500, &remaining)
	n.BlastRadiusSummary = boundedNarrativeString(n.BlastRadiusSummary, 1500, &remaining)
	n.RiskAssessment = boundedNarrativeString(n.RiskAssessment, 1500, &remaining)
	n.RollbackStrategy = boundedNarrativeString(n.RollbackStrategy, 1500, &remaining)
	if len(n.AffectedByTier) > 10 {
		n.AffectedByTier = n.AffectedByTier[:10]
	}
	for i := range n.AffectedByTier {
		n.AffectedByTier[i].Tier = boundedNarrativeString(n.AffectedByTier[i].Tier, 1500, &remaining)
		n.AffectedByTier[i].Resources = boundedNarrativeList(n.AffectedByTier[i].Resources, &remaining)
	}
	return n
}

func boundedNarrativeList(values []string, remaining *int) []string {
	if len(values) > 10 {
		values = values[:10]
	}
	for i := range values {
		values[i] = boundedNarrativeString(values[i], 400, remaining)
	}
	return values
}

func boundedNarrativeString(value string, maximum int, remaining *int) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, value)
	runes := []rune(value)
	limit := min(maximum, *remaining)
	if len(runes) <= limit {
		*remaining -= len(runes)
		return value
	}
	if limit == 0 {
		return ""
	}
	*remaining -= limit
	if limit == 1 {
		return "…"
	}
	return string(runes[:limit-1]) + "…"
}

// ParseNarrativeFromLLM tries to parse the LLM response as JSON narrative.
// Handles markdown fences, surrounding prose, and whitespace.
// Falls back to a generic narrative only on genuine parse failure.
func ParseNarrativeFromLLM(text string) *Narrative {
	n := &Narrative{Provider: "llm"}

	if cleaned, ok := extractJSON(text); ok {
		if err := json.Unmarshal([]byte(cleaned), n); err == nil && n.Summary != "" {
			return n
		}
	}

	return &Narrative{
		Provider:         "llm",
		Summary:          "LLM response was not parseable as JSON. Review raw output.",
		RiskAssessment:   "Could not parse LLM output. Check the raw response.",
		Recommendations:  []string{"Ensure the LLM returns valid JSON", "Review the plan manually"},
		RollbackStrategy: "Apply the previous known-good Terraform state version.",
	}
}

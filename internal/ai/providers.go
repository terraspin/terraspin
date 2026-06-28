package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ---------------------------------------------------------------------------
// OpenAI provider
// ---------------------------------------------------------------------------

type openAIRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

// QueryOpenAI sends a prompt to the OpenAI Chat Completions API.
func QueryOpenAI(ctx context.Context, apiKey, model, prompt string) (string, int, error) {
	if model == "" {
		model = "gpt-4o"
	}
	body := openAIRequest{
		Model:     model,
		MaxTokens: 2048,
		Messages: []openAIMessage{
			{Role: "system", Content: SystemPrompt},
			{Role: "user", Content: prompt},
		},
	}
	raw, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", 0, fmt.Errorf("openai: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("openai: send request: %w", err)
	}
	defer resp.Body.Close()

	respRaw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("openai: HTTP %d: %s", resp.StatusCode, string(respRaw))
	}

	var cresp openAIResponse
	if err := json.Unmarshal(respRaw, &cresp); err != nil {
		return "", 0, fmt.Errorf("openai: parse response: %w", err)
	}
	if len(cresp.Choices) == 0 {
		return "", 0, fmt.Errorf("openai: no choices in response")
	}
	tokens := 0
	if cresp.Usage != nil {
		tokens = cresp.Usage.TotalTokens
	}
	return cresp.Choices[0].Message.Content, tokens, nil
}

// ---------------------------------------------------------------------------
// Ollama provider
// ---------------------------------------------------------------------------

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Tokens   int    `json:"eval_count"`
}

// QueryOllama sends a prompt to a local Ollama instance.
func QueryOllama(ctx context.Context, host, model, prompt string) (string, int, error) {
	if host == "" {
		host = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3.2"
	}
	body := ollamaRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	}
	raw, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(host, "/")+"/api/generate", bytes.NewReader(raw))
	if err != nil {
		return "", 0, fmt.Errorf("ollama: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("ollama: send request: %w", err)
	}
	defer resp.Body.Close()

	respRaw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("ollama: HTTP %d: %s", resp.StatusCode, string(respRaw))
	}

	var oresp ollamaResponse
	if err := json.Unmarshal(respRaw, &oresp); err != nil {
		return "", 0, fmt.Errorf("ollama: parse response: %w", err)
	}
	return oresp.Response, oresp.Tokens, nil
}

// ---------------------------------------------------------------------------
// Provider router
// ---------------------------------------------------------------------------

// QueryLLM routes to the right provider and returns the text response.
func QueryLLM(ctx context.Context, provider, apiKey, model, host, prompt string) (string, int, error) {
	switch provider {
	case "claude":
		return QueryClaude(ctx, apiKey, model, prompt)
	case "openai":
		return QueryOpenAI(ctx, apiKey, model, prompt)
	case "ollama":
		return QueryOllama(ctx, host, model, prompt)
	default:
		return "", 0, fmt.Errorf("unknown LLM provider: %s (use claude|openai|ollama)", provider)
	}
}

// ---------------------------------------------------------------------------
// Narrative types
// ---------------------------------------------------------------------------

// Narrative holds the analysis briefing.
type Narrative struct {
	Provider            string              `json:"provider"`
	Summary             string              `json:"summary"`
	InfraSummary        string              `json:"infra_summary,omitempty"`
	RiskScore           string              `json:"risk_score,omitempty"`
	RiskLevel           string              `json:"risk_level,omitempty"`
	ResourceChangeSummary string            `json:"resource_change_summary,omitempty"`
	BlastRadiusSummary  string              `json:"blast_radius_summary,omitempty"`
	CriticalFindings    []string            `json:"critical_findings,omitempty"`
	AffectedByTier      []TierGroup         `json:"affected_by_tier,omitempty"`
	Recommendations     []string            `json:"recommendations"`
	NextSteps           []string            `json:"next_steps,omitempty"`
	CriticalChanges     []string            `json:"critical_changes"`
	RiskAssessment      string              `json:"risk_assessment"`
	RollbackStrategy    string              `json:"rollback_strategy"`
}

// TierGroup groups affected resources by risk tier.
type TierGroup struct {
	Tier      string   `json:"tier"`
	Resources []string `json:"resources"`
}

// ---------------------------------------------------------------------------
// Prompt builder
// ---------------------------------------------------------------------------

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
		b.WriteString(fmt.Sprintf("- %s [%s] → %s", c.Address, c.Tier, strings.ToUpper(string(c.Action))))
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

	// Find first '{'
	start := strings.IndexByte(t, '{')
	if start < 0 {
		return "", false
	}

	// Match braces
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

// ParseNarrativeFromLLM tries to parse the LLM response as JSON narrative.
// Handles markdown fences, surrounding prose, and whitespace.
// Falls back to a generic narrative only on genuine parse failure.
func ParseNarrativeFromLLM(text string) *Narrative {
	n := &Narrative{Provider: "llm"}

	// Try extracting clean JSON from the raw text first.
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

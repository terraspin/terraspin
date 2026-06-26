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
		Messages:  []openAIMessage{{Role: "user", Content: prompt}},
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
		text, err := QueryClaude(ctx, apiKey, prompt)
		if err != nil {
			return "", 0, err
		}
		return text, 0, nil
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
	Provider         string   `json:"provider"`
	Summary          string   `json:"summary"`
	CriticalChanges  []string `json:"critical_changes"`
	RiskAssessment   string   `json:"risk_assessment"`
	Recommendations  []string `json:"recommendations"`
	RollbackStrategy string   `json:"rollback_strategy"`
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
// ponytail: minimal prompt, instructs JSON output for easy parsing.
func BuildAnalysisPrompt(changes []PlanChangeSummary, riskTier string, riskScore float64, counts map[string]int) string {
	var b strings.Builder
	b.WriteString("You are Terraspin, a Terraform plan analysis assistant. ")
	b.WriteString("Analyze the following infrastructure plan changes and output a JSON object with these keys:\n")
	b.WriteString("- summary: 2-3 sentence plain-English summary\n")
	b.WriteString("- critical_changes: array of strings, one per critical/high risk change\n")
	b.WriteString("- risk_assessment: 1-2 sentence assessment\n")
	b.WriteString("- recommendations: array of actionable check strings\n")
	b.WriteString("- rollback_strategy: step-by-step recovery plan\n\n")

	b.WriteString(fmt.Sprintf("Overall risk: %s (score: %.0f)\n", riskTier, riskScore))
	b.WriteString(fmt.Sprintf("Resource counts by tier: %v\n\n", counts))
	b.WriteString("Changes:\n")

	for _, c := range changes {
		b.WriteString(fmt.Sprintf("- %s [%s] → %s", c.Address, c.Tier, strings.ToUpper(string(c.Action))))
		if c.BlastDesc != "" {
			b.WriteString(fmt.Sprintf(" | blast: %s", c.BlastDesc))
		}
		b.WriteString("\n")
	}

	b.WriteString("\nOutput ONLY valid JSON, no markdown fences, no commentary.")
	return b.String()
}

// BuildRuleBasedNarrative creates a narrative from rules (no LLM call).
func BuildRuleBasedNarrative(riskTier string, riskScore float64, criticalChanges []string, recs []string) *Narrative {
	n := &Narrative{
		Provider:         "rule-based",
		Summary:          fmt.Sprintf("Terraform plan scored %s (%.0f) with %d changes requiring attention.", riskTier, riskScore, len(criticalChanges)),
		CriticalChanges:  criticalChanges,
		RiskAssessment:   fmt.Sprintf("Plan scored %s (%.0f). %d critical/high changes.", riskTier, riskScore, len(criticalChanges)),
		RollbackStrategy: "Apply the previous known-good Terraform state version. For destroyed resources, restore from the most recent backup/snapshot.",
	}
	if recs == nil {
		recs = []string{"Review all changes before applying", "Ensure recent state backup exists"}
	}
	n.Recommendations = recs
	return n
}

// ParseNarrativeFromLLM tries to parse the LLM response as JSON narrative.
// Falls back to a generic narrative on parse failure.
func ParseNarrativeFromLLM(text string) *Narrative {
	n := &Narrative{Provider: "llm"}
	if err := json.Unmarshal([]byte(text), n); err == nil && n.Summary != "" {
		return n
	}
	return &Narrative{
		Provider:         "llm",
		Summary:          "LLM response was not parseable as JSON. Review raw output.",
		RiskAssessment:   "Could not parse LLM output. Check the raw response.",
		Recommendations:  []string{"Ensure the LLM returns valid JSON", "Review the plan manually"},
		RollbackStrategy: "Apply the previous known-good Terraform state version.",
	}
}

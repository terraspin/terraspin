package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/terraspin/terraspin/internal/ai"
	"github.com/terraspin/terraspin/internal/analyzer"
	"github.com/terraspin/terraspin/internal/formatter"
	"github.com/terraspin/terraspin/internal/parser"
)

func main() {
	var (
		format     = "text"
		failOn     = ""
		noAI       = false
		llmProvider = "claude"
		llmModel   = ""
		ollamaHost = "http://localhost:11434"
		verbose    = false
	)
	flag.StringVar(&format, "format", "text", "output format: text|json|markdown")
	flag.StringVar(&failOn, "fail-on", "", "exit 1 if risk >= tier: critical|high|medium|low")
	flag.BoolVar(&noAI, "no-ai", false, "rule-based analysis only, skip LLM call")
	flag.StringVar(&llmProvider, "llm", "claude", "LLM provider: claude|openai|ollama")
	flag.StringVar(&llmModel, "model", "", "LLM model name (provider default if empty)")
	flag.StringVar(&ollamaHost, "ollama-host", "http://localhost:11434", "Ollama host URL")
	flag.BoolVar(&verbose, "v", false, "show all risk tiers including medium and low")
	flag.Usage = printUsage
	flag.Parse()

	planFile := flag.Arg(0)

	// Read plan file or stdin
	var data []byte
	if planFile != "" {
		var err error
		data, err = os.ReadFile(planFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}
	} else {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			var err error
			data, err = io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
				os.Exit(2)
			}
		} else {
			printUsage()
			os.Exit(2)
		}
	}

	// Parse plan
	ast, err := parser.ParsePlan(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(2)
	}

	// Redact sensitive values
	ai.RedactSensitive(ast)

	// Score risk
	score := analyzer.ScorePlan(ast)

	// Analyze blast radius
	refs := analyzer.ParseDependencyRefs(data)
	blast := analyzer.AnalyzeBlastRadius(ast.Changes, refs)

	// Build narrative
	var narr *ai.Narrative
	if noAI {
		narr = buildRuleNarrative(ast, score, blast)
	} else {
		narr = buildLLMNarrative(ast, score, blast, llmProvider, llmModel, ollamaHost)
	}

	// Format output
	switch format {
	case "json":
		out, err := formatter.FormatJSON(ast, score, blast, narr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "format error: %v\n", err)
			os.Exit(2)
		}
		fmt.Println(out)
	case "markdown":
		fmt.Println(formatter.FormatMarkdown(ast, score, blast, narr))
	default:
		fmt.Print(formatter.FormatText(ast, score, blast, narr, verbose))
	}

	// --fail-on gate
	if failOn != "" {
		exitCode := checkFailOn(score.Overall.Tier, failOn)
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `usage: terraspin [flags] <plan.json>
       terraform plan -json | terraspin [flags]

Flags:
  --format string      output format: text|json|markdown (default "text")
  --fail-on string     exit 1 if risk >= tier: critical|high|medium|low
  --no-ai              rule-based analysis only, skip LLM call
  --llm string         LLM provider: claude|openai|ollama (default "claude")
  --model string       LLM model name (provider default if empty)
  --ollama-host string Ollama host URL (default "http://localhost:11434")
  -v                   show all risk tiers including medium and low
`)
}

func buildRuleNarrative(ast *parser.PlanAST, score *analyzer.PlanScore, blast map[string]*analyzer.BlastRadius) *ai.Narrative {
	var criticalChanges []string
	var recs []string
	for _, rs := range score.ResourceScores {
		if rs.Tier == analyzer.TierCritical || rs.Tier == analyzer.TierHigh {
			criticalChanges = append(criticalChanges, fmt.Sprintf("%s → %s (score: %.1f)", rs.Address, rs.Action, rs.Score))
			if br, ok := blast[rs.Address]; ok && br.TotalAffected > 0 {
				recs = append(recs, fmt.Sprintf("Review blast radius of %s (%d resources affected)", rs.Address, br.TotalAffected))
			}
		}
	}
	return ai.BuildRuleBasedNarrative(string(score.Overall.Tier), score.Overall.Score, criticalChanges, recs)
}

func buildLLMNarrative(ast *parser.PlanAST, score *analyzer.PlanScore, blast map[string]*analyzer.BlastRadius, provider, model, ollamaHost string) *ai.Narrative {
	changes := make([]ai.PlanChangeSummary, 0, len(score.ResourceScores))
	for _, rs := range score.ResourceScores {
		s := ai.PlanChangeSummary{
			Address: rs.Address,
			Action:  string(rs.Action),
			Tier:    string(rs.Tier),
		}
		if br, ok := blast[rs.Address]; ok && br.TotalAffected > 0 {
			s.BlastDesc = fmt.Sprintf("%d affected", br.TotalAffected)
		}
		changes = append(changes, s)
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if provider == "openai" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	counts := make(map[string]int)
	for k, v := range score.Counts {
		counts[string(k)] = v
	}

	prompt := ai.BuildAnalysisPrompt(changes, string(score.Overall.Tier), score.Overall.Score, counts)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	text, _, err := ai.QueryLLM(ctx, provider, apiKey, model, ollamaHost, prompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: LLM error (%v), falling back to rule-based analysis\n", err)
		return buildRuleNarrative(ast, score, blast)
	}

	return ai.ParseNarrativeFromLLM(text)
}

func checkFailOn(tier analyzer.RiskTier, threshold string) int {
	levels := map[analyzer.RiskTier]int{
		analyzer.TierLow: 1, analyzer.TierMedium: 2,
		analyzer.TierHigh: 3, analyzer.TierCritical: 4,
	}
	if levels[tier] >= levels[analyzer.RiskTier(strings.ToLower(threshold))] {
		fmt.Fprintf(os.Stderr, "FAIL: plan risk %s meets --fail-on=%s threshold\n", tier, threshold)
		return 1
	}
	return 0
}

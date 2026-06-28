package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/terraspin/terraspin/internal/ai"
	"github.com/terraspin/terraspin/internal/diff"
)

func diffCmd(args []string) {
	// ponytail: pull boolean flags and --key=value flags from the front
	// so stdlib flag.Parse doesn't stop at the first positional arg.
	var flags, posArgs []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			flags = append(flags, args[i])
			posArgs = append(posArgs, args[i+1:]...)
			break
		}
		if strings.HasPrefix(args[i], "--") {
			flags = append(flags, args[i])
			if !strings.Contains(args[i], "=") && !isBoolFlag(args[i]) {
				i++
				if i < len(args) {
					flags = append(flags, args[i])
				}
			}
		} else {
			posArgs = append(posArgs, args[i])
		}
	}

	fs := flag.NewFlagSet("diff", flag.ExitOnError)

	format := fs.String("format", "text", "output format: text|json|markdown")
	noAI := fs.Bool("no-ai", false, "rule-based analysis only, skip LLM call")
	llmProvider := fs.String("llm", "claude", "LLM provider: claude|openai|ollama")
	llmModel := fs.String("model", "", "LLM model name (provider default if empty)")
	ollamaHost := fs.String("ollama-host", "http://localhost:11434", "Ollama host URL")
	labelA := fs.String("label-a", "plan-a", "label for the first plan")
	labelB := fs.String("label-b", "plan-b", "label for the second plan")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `usage: terraspin diff <plan_a.json> <plan_b.json> [flags]

Compare two terraform plan JSON files and produce a drift analysis.

Flags:
  --format string      output format: text|json|markdown (default "text")
  --no-ai              rule-based analysis only, skip LLM call
  --llm string         LLM provider: claude|openai|ollama (default "claude")
  --model string       LLM model name (provider default if empty)
  --ollama-host string Ollama host URL (default "http://localhost:11434")
  --label-a string     label for the first plan (default "plan-a")
  --label-b string     label for the second plan (default "plan-b")
`)
	}

	if err := fs.Parse(flags); err != nil {
		os.Exit(2)
	}

	if len(posArgs) != 2 {
		fs.Usage()
		os.Exit(2)
	}

	dataA, err := os.ReadFile(posArgs[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", posArgs[0], err)
		os.Exit(2)
	}
	dataB, err := os.ReadFile(posArgs[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", posArgs[1], err)
		os.Exit(2)
	}

	result, err := diff.Compare(dataA, dataB, *labelA, *labelB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "diff error: %v\n", err)
		os.Exit(2)
	}

	// Generate AI analysis unless --no-ai
	var narr *ai.Narrative
	if !*noAI {
		narr = diffAI(result, *llmProvider, *llmModel, *ollamaHost)
	}

	// Format and print output
	switch *format {
	case "json":
		printDiffJSON(result, narr)
	case "markdown":
		fmt.Println(formatDiffMD(result, narr))
	default:
		fmt.Println(formatDiffText(result, narr))
	}
}

// diffAI sends the diff to the LLM for analysis.
func diffAI(result *diff.DiffResult, provider, model, ollamaHost string) *ai.Narrative {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if provider == "openai" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "warning: no API key set for %s, skipping AI analysis\n", provider)
		return nil
	}

	prompt := diff.BuildDiffPrompt(result)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	text, _, err := ai.QueryLLM(ctx, provider, apiKey, model, ollamaHost, prompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: LLM error (%v), skipping AI analysis\n", err)
		return nil
	}

	return ai.ParseNarrativeFromLLM(text)
}

func printDiffJSON(result *diff.DiffResult, narr *ai.Narrative) {
	out := struct {
		Diff     *diff.DiffResult `json:"diff"`
		Analysis *ai.Narrative    `json:"analysis,omitempty"`
	}{Diff: result, Analysis: narr}

	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "json error: %v\n", err)
		os.Exit(2)
	}
	fmt.Println(string(raw))
}

func formatDiffText(result *diff.DiffResult, narr *ai.Narrative) string {
	var b strings.Builder

	fmt.Fprintf(&b, "┌─ Terraspin Drift Analysis ─────────────────────────────────┐\n")
	fmt.Fprintf(&b, "│  %s  →  %s\n", result.LabelA, result.LabelB)
	fmt.Fprintf(&b, "│  Terraform %s  vs  %s\n", result.VersionA, result.VersionB)
	fmt.Fprintf(&b, "└────────────────────────────────────────────────────────────┘\n\n")

	fmt.Fprintf(&b, "  %s\n\n", result.DiffSummaryText())

	if narr != nil && narr.Summary != "" {
		fmt.Fprintf(&b, "──── AI Analysis ──────────────────────────────────────────────\n\n")
		fmt.Fprintf(&b, "  %s\n\n", narr.Summary)
		if len(narr.Recommendations) > 0 {
			fmt.Fprintf(&b, "  Recommendations:\n")
			for _, r := range narr.Recommendations {
				fmt.Fprintf(&b, "    □  %s\n", r)
			}
			fmt.Fprintln(&b)
		}
	}

	// List diffs by status
	for _, status := range []diff.DiffStatus{diff.StatusModified, diff.StatusAdded, diff.StatusRemoved} {
		var entries []diff.ResourceDiff
		for _, d := range result.Diffs {
			if d.Status == status {
				entries = append(entries, d)
			}
		}
		if len(entries) == 0 {
			continue
		}
		statusLabel := strings.ToUpper(string(status))
		fmt.Fprintf(&b, "──── %s ────────────────────────────────────────────────────────\n\n", statusLabel)
		for _, d := range entries {
			fmt.Fprintf(&b, "  %s [%s]\n", d.Address, d.Type)
			for _, c := range d.Changes {
				ov := fmt.Sprintf("%v", c.OldValue)
				nv := fmt.Sprintf("%v", c.NewValue)
				if len(ov) > 60 {
					ov = ov[:60] + "..."
				}
				if len(nv) > 60 {
					nv = nv[:60] + "..."
				}
				if ov != "<nil>" || nv != "<nil>" {
					fmt.Fprintf(&b, "    %s: %s → %s\n", c.Path, ov, nv)
				}
			}
			fmt.Fprintln(&b)
		}
	}

	return b.String()
}

// isBoolFlag returns true for flags that don't take a value argument.
// ponytail: exact list of bool flags, keeps arg splitting simple.
func isBoolFlag(name string) bool {
	switch name {
	case "--no-ai", "--verbose", "-v":
		return true
	}
	return false
}

func formatDiffMD(result *diff.DiffResult, narr *ai.Narrative) string {
	var b strings.Builder

	fmt.Fprintf(&b, "## 🌀 Terraspin Drift Analysis\n\n")
	fmt.Fprintf(&b, "**%s** → **%s**  ·  Terraform %s vs %s\n\n",
		result.LabelA, result.LabelB, result.VersionA, result.VersionB)

	fmt.Fprintf(&b, "**Summary:** %s\n\n", result.DiffSummaryText())

	if narr != nil && narr.Summary != "" {
		fmt.Fprintf(&b, "### AI Analysis\n\n%s\n\n", narr.Summary)
		if len(narr.Recommendations) > 0 {
			fmt.Fprintf(&b, "**Recommendations:**\n")
			for _, r := range narr.Recommendations {
				fmt.Fprintf(&b, "- [ ] %s\n", r)
			}
			fmt.Fprintln(&b)
		}
	}

	for _, status := range []diff.DiffStatus{diff.StatusModified, diff.StatusAdded, diff.StatusRemoved} {
		var entries []diff.ResourceDiff
		for _, d := range result.Diffs {
			if d.Status == status {
				entries = append(entries, d)
			}
		}
		if len(entries) == 0 {
			continue
		}
		statusLabel := strings.ToUpper(string(status))
		fmt.Fprintf(&b, "### %s Resources\n\n", statusLabel)
		fmt.Fprintf(&b, "| Resource | Type | Details |\n|----------|------|--------|\n")
		for _, d := range entries {
			details := ""
			for _, c := range d.Changes {
				ov := fmt.Sprintf("%v", c.OldValue)
				nv := fmt.Sprintf("%v", c.NewValue)
				if len(ov) > 40 {
					ov = ov[:40] + "..."
				}
				if len(nv) > 40 {
					nv = nv[:40] + "..."
				}
				details += fmt.Sprintf("`%s`: %s → %s<br>", c.Path, ov, nv)
			}
			if details == "" {
				details = "-"
			}
			fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", d.Address, d.Type, details)
		}
		fmt.Fprintln(&b)
	}

	return b.String()
}

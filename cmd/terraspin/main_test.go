package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terraspin/terraspin/internal/ai"
	"github.com/terraspin/terraspin/internal/analyzer"
	"github.com/terraspin/terraspin/internal/config"
	"github.com/terraspin/terraspin/internal/parser"
)

func TestBuildLLMNarrativeFallsBackWhenLLMDisabled(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, "testdata", "plan_empty.json"))
	if err != nil {
		t.Fatal(err)
	}
	ast, err := parser.ParsePlan(data)
	if err != nil {
		t.Fatal(err)
	}
	ai.RedactSensitive(ast)
	score := analyzer.ScorePlan(ast)
	blast := analyzer.AnalyzeBlastRadius(ast.Changes, analyzer.ParseDependencyRefs(data))

	narr := buildLLMNarrative(ast, score, blast, &config.Config{Version: 1}, "", "")
	if narr == nil || narr.Summary == "" {
		t.Fatal("expected a rule-based narrative")
	}
}

func TestCLIWithoutConfigUsesRuleBasedNarrative(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "terraspin")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = filepath.Join(repoRoot, "cmd", "terraspin")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	for _, fixture := range []string{"plan_empty.json", "plan.json", "plan_with_modules.json", "plan_with_sensitive.json"} {
		for _, format := range []string{"text", "json", "markdown"} {
			t.Run(fixture+"/"+format, func(t *testing.T) {
				cmd := exec.Command(binary, "--format", format, filepath.Join(repoRoot, "testdata", fixture))
				cmd.Dir = t.TempDir()
				cmd.Env = append(os.Environ(), "ANTHROPIC_API_KEY=", "OPENAI_API_KEY=")
				output, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("run CLI: %v\n%s", err, output)
				}
				if len(output) == 0 {
					t.Fatal("CLI output is empty")
				}
			})
		}
	}
}

func TestCLIExplicitLLMUsesAPIKeyEnvironment(t *testing.T) {
	apiKeys := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKeys <- r.Header.Get("x-api-key")
		_, _ = w.Write([]byte(`{"content":[{"text":"{\"summary\":\"LLM narrative\",\"risk_assessment\":\"low\",\"recommendations\":[],\"rollback_strategy\":\"none\"}"}]}`))
	}))
	defer server.Close()

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "terraspin")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = filepath.Join(repoRoot, "cmd", "terraspin")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	cmd := exec.Command(binary, "--llm", "claude", filepath.Join(repoRoot, "testdata", "plan_empty.json"))
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "ANTHROPIC_API_KEY=test-api-key", "ANTHROPIC_BASE_URL="+server.URL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run CLI: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "LLM narrative") {
		t.Fatalf("CLI output does not contain LLM narrative:\n%s", output)
	}
	if got := <-apiKeys; got != "test-api-key" {
		t.Errorf("x-api-key = %q, want test API key", got)
	}
}

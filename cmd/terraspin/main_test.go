package main

import (
	"encoding/json"
	"fmt"
	"io"
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

func TestBuildRuleNarrativeFormatsIncludedDependents(t *testing.T) {
	for _, count := range []int{1, 2, 5, 6, 20} {
		t.Run(fmt.Sprintf("%d", count), func(t *testing.T) {
			deps := make([]analyzer.DependentResource, count)
			for i := range deps {
				deps[i] = analyzer.DependentResource{Address: fmt.Sprintf("dep-%02d", i)}
			}
			ast := &parser.PlanAST{Changes: []parser.ResourceChange{{Address: "root", Action: parser.ActionDelete}}}
			score := &analyzer.PlanScore{Overall: analyzer.RiskScore{Tier: analyzer.TierCritical}, Counts: map[analyzer.RiskTier]int{analyzer.TierCritical: 1}, ResourceScores: []analyzer.ResourceRiskScore{{Address: "root", Action: parser.ActionDelete, Tier: analyzer.TierCritical}}}
			narr := buildRuleNarrative(ast, score, map[string]*analyzer.BlastRadius{"root": {RootAddress: "root", DirectDeps: deps, TotalAffected: count}})
			got := narr.CriticalFindings[0]
			if strings.Count(got, "(") != strings.Count(got, ")") {
				t.Fatalf("unbalanced: %s", got)
			}
			if count <= 5 && !strings.Contains(got, "(incl. "+strings.Join(func() []string {
				names := make([]string, count)
				for i := range names {
					names[i] = fmt.Sprintf("dep-%02d", i)
				}
				return names
			}(), ", ")+")") {
				t.Fatalf("got %s", got)
			}
			if count > 5 && !strings.Contains(got, "dep-04 and "+fmt.Sprint(count-5)+" more)") {
				t.Fatalf("got %s", got)
			}
		})
	}
}

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

	narr, err := buildLLMNarrative(ast, score, blast, &config.Config{Version: 1}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if narr == nil || narr.Summary == "" {
		t.Fatal("expected a rule-based narrative")
	}
}

func TestBuildLLMNarrativeHonorsFallbackSetting(t *testing.T) {
	ast := &parser.PlanAST{}
	score := &analyzer.PlanScore{Counts: map[analyzer.RiskTier]int{}}
	cfg := &config.Config{LLM: &config.LLMConfig{Provider: "claude", Timeout: "30s", FallbackToRules: false}}
	if _, err := buildLLMNarrative(ast, score, nil, cfg, "", ""); err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Fatalf("err=%v, want missing key", err)
	}
	cfg.LLM.FallbackToRules = true
	narr, err := buildLLMNarrative(ast, score, nil, cfg, "", "")
	if err != nil || narr == nil {
		t.Fatalf("narr=%v err=%v", narr, err)
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

	for _, fixture := range []string{"plan_empty.json", "plan.json", "plan_with_modules.json", "plan_with_sensitive.json", "plan_with_sensitive_collections_a.json", "plan_with_sensitive_collections_b.json"} {
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

func TestCLISensitiveValuesNeverReachOutput(t *testing.T) {
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

	planA := filepath.Join(repoRoot, "testdata", "plan_with_sensitive_collections_a.json")
	planB := filepath.Join(repoRoot, "testdata", "plan_with_sensitive_collections_b.json")
	secrets := []string{
		"synthetic-token-before-a", "synthetic-token-after-a", "synthetic-token-before-b", "synthetic-token-after-b",
		"synthetic-unchanged-token", "synthetic-list-before-a", "synthetic-list-after-a", "synthetic-list-before-b", "synthetic-list-after-b",
		"synthetic-list-unchanged", "synthetic-nested-before-a", "synthetic-nested-after-a", "synthetic-nested-before-b", "synthetic-nested-after-b",
		"synthetic-tree-before-a", "synthetic-tree-after-a", "synthetic-tree-before-b", "synthetic-tree-after-b",
		"synthetic-child-before-a", "synthetic-child-after-a", "synthetic-child-before-b", "synthetic-child-after-b",
	}

	for _, format := range []string{"text", "json", "markdown"} {
		t.Run(format, func(t *testing.T) {
			analyze := exec.Command(binary, "--no-ai", "--format", format, planA)
			analyze.Dir = t.TempDir()
			analyze.Env = append(os.Environ(), "ANTHROPIC_API_KEY=", "OPENAI_API_KEY=")
			analyzeOutput, err := analyze.CombinedOutput()
			if err != nil {
				t.Fatalf("analyze: %v\n%s", err, analyzeOutput)
			}
			for _, secret := range secrets {
				if strings.Contains(string(analyzeOutput), secret) {
					t.Fatal("analyze output contains a synthetic sensitive value")
				}
			}

			diffCmd := exec.Command(binary, "diff", "--no-ai", "--format", format, planA, planB)
			diffCmd.Dir = t.TempDir()
			diffOutput, err := diffCmd.CombinedOutput()
			if err != nil {
				t.Fatalf("diff: %v\n%s", err, diffOutput)
			}
			for _, secret := range secrets {
				if strings.Contains(string(diffOutput), secret) {
					t.Fatal("diff output contains a synthetic sensitive value")
				}
			}
			if !strings.Contains(string(diffOutput), "api_token") || !strings.Contains(string(diffOutput), ai.SensitiveRedacted) {
				t.Fatal("diff output must retain the changed sensitive attribute with a redaction marker")
			}
		})
	}
}

func TestCLIDiffAddedAndRemovedSensitiveResourcesAreRedacted(t *testing.T) {
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

	plans := t.TempDir()
	removed := syntheticPlan(t, []any{syntheticResource("example.removed", "delete", "synthetic-removed-value", true)})
	added := syntheticPlan(t, []any{syntheticResource("example.added", "create", "synthetic-added-value", true)})
	planA := filepath.Join(plans, "plan-a.json")
	planB := filepath.Join(plans, "plan-b.json")
	if err := os.WriteFile(planA, removed, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planB, added, 0644); err != nil {
		t.Fatal(err)
	}

	for _, format := range []string{"text", "json", "markdown"} {
		t.Run(format, func(t *testing.T) {
			cmd := exec.Command(binary, "diff", "--no-ai", "--format", format, planA, planB)
			cmd.Dir = t.TempDir()
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("diff: %v\n%s", err, output)
			}
			if strings.Contains(string(output), "synthetic-removed-value") || strings.Contains(string(output), "synthetic-added-value") {
				t.Fatal("diff output contains a synthetic sensitive value")
			}
		})
	}
}

func syntheticPlan(t *testing.T, changes []any) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"format_version": "1.2", "terraform_version": "1.7.2", "resource_changes": changes,
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func syntheticResource(address, action, value string, sensitive bool) map[string]any {
	change := map[string]any{"actions": []string{action}}
	if action == "create" {
		change["before"] = nil
		change["after"] = map[string]any{"credential": value}
		change["before_sensitive"] = map[string]any{}
		change["after_sensitive"] = map[string]any{"credential": sensitive}
	} else {
		change["before"] = map[string]any{"credential": value}
		change["after"] = nil
		change["before_sensitive"] = map[string]any{"credential": sensitive}
		change["after_sensitive"] = map[string]any{}
	}
	return map[string]any{
		"address": address, "mode": "managed", "type": "example", "name": "test", "change": change,
	}
}

func TestCLIUsesExplicitConfigModel(t *testing.T) {
	models := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		models <- body.Model
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
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, ".terraspin.yml"), []byte("version: 1\nllm:\n  provider: claude\n  model: configured-gateway-model\n  timeout: 1s\n  max_retries: 0\n  fallback_to_rules: false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, filepath.Join(repoRoot, "testdata", "plan_empty.json"))
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), "ANTHROPIC_API_KEY=test-api-key", "ANTHROPIC_BASE_URL="+server.URL)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run CLI: %v\n%s", err, output)
	}
	if got := <-models; got != "configured-gateway-model" {
		t.Errorf("model=%q, want configured-gateway-model", got)
	}
}

func TestCLIPayloadsExcludeAttributeValuesIncludingSystem(t *testing.T) {
	bodies := make(chan []byte, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		bodies <- raw
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
	workdir := t.TempDir()
	planA := filepath.Join(workdir, "a.json")
	planB := filepath.Join(workdir, "b.json")
	for path, values := range map[string][2]string{
		planA: {"SYNTHETIC_SENSITIVE_A", "SYNTHETIC_UNMARKED_A"},
		planB: {"SYNTHETIC_SENSITIVE_B", "SYNTHETIC_UNMARKED_B"},
	} {
		plan := syntheticPayloadPlan(t, values[0], values[1])
		if err := os.WriteFile(path, plan, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(workdir, ".terraspin.yml"), []byte("version: 1\nllm:\n  provider: claude\n  timeout: 1s\n  max_retries: 0\n  fallback_to_rules: false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{planA}, {"diff", planA, planB}} {
		cmd := exec.Command(binary, args...)
		cmd.Dir = workdir
		cmd.Env = append(os.Environ(), "ANTHROPIC_API_KEY=payload-placeholder", "ANTHROPIC_BASE_URL="+server.URL)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("run %v: %v\n%s", args, err, output)
		}
	}
	for _, command := range []string{"analyze", "diff"} {
		raw := <-bodies
		if strings.Contains(string(raw), "SYNTHETIC_") {
			t.Fatalf("%s payload contains an attribute value: %s", command, raw)
		}
		var payload struct {
			System string `json:"system"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.System != ai.SystemPrompt {
			t.Fatalf("%s system prompt changed or missing", command)
		}
	}
}

func syntheticPayloadPlan(t *testing.T, secret, plain string) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"format_version": "1.2", "terraform_version": "1.7.2",
		"resource_changes": []any{map[string]any{
			"address": "example_payload.test", "mode": "managed", "type": "example_payload", "name": "test",
			"change": map[string]any{
				"actions":          []string{"update"},
				"before":           map[string]any{"secret": secret + "-before", "label": plain + "-before"},
				"after":            map[string]any{"secret": secret + "-after", "label": plain + "-after"},
				"before_sensitive": map[string]any{"secret": true}, "after_sensitive": map[string]any{"secret": true},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCLIDiffUsesLLMConfigForFallback(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"content":[{"text":"{\"summary\":\"LLM narrative\",\"recommendations\":[]}"}]}`))
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
	planA := filepath.Join(repoRoot, "testdata", "plan_with_sensitive_collections_a.json")
	planB := filepath.Join(repoRoot, "testdata", "plan_with_sensitive_collections_b.json")

	for _, tc := range []struct {
		name        string
		config      string
		key         string
		flags       []string
		wantExit    bool
		wantRequest int
		wantLLM     bool
	}{
		{"config without llm with key", "version: 1\n", "test-key", nil, false, 1, true},
		{"config without llm without key", "version: 1\n", "", nil, false, 0, false},
		{"flag with fallback disabled", "version: 1\nllm:\n  fallback_to_rules: false\n", "", []string{"--llm", "claude"}, true, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests = 0
			workdir := t.TempDir()
			if err := os.WriteFile(filepath.Join(workdir, ".terraspin.yml"), []byte(tc.config), 0644); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"diff", "--format", "json"}, tc.flags...)
			args = append(args, planA, planB)
			cmd := exec.Command(binary, args...)
			cmd.Dir = workdir
			cmd.Env = append(os.Environ(), "ANTHROPIC_API_KEY="+tc.key, "ANTHROPIC_BASE_URL="+server.URL)
			output, err := cmd.CombinedOutput()
			if tc.wantExit {
				if err == nil || !strings.Contains(string(output), "ANTHROPIC_API_KEY") {
					t.Fatalf("err=%v output=%s, want key error", err, output)
				}
			} else if err != nil {
				t.Fatalf("err=%v output=%s", err, output)
			}
			if requests != tc.wantRequest {
				t.Fatalf("requests=%d, want %d", requests, tc.wantRequest)
			}
			if got := strings.Contains(string(output), "LLM narrative"); got != tc.wantLLM {
				t.Fatalf("LLM narrative=%t, want %t; output=%s", got, tc.wantLLM, output)
			}
		})
	}
}

func TestCLIFlagLLMMissingKeyUsesFallbackDefaults(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"content":[{"text":"{\"summary\":\"LLM narrative\"}"}]}`))
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

	for _, tc := range []struct {
		name     string
		config   string
		wantExit bool
	}{
		{"no config", "", false},
		{"config without llm", "version: 1\n", false},
		{"fallback disabled", "version: 1\nllm:\n  fallback_to_rules: false\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests = 0
			workdir := t.TempDir()
			if tc.config != "" {
				if err := os.WriteFile(filepath.Join(workdir, ".terraspin.yml"), []byte(tc.config), 0644); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(binary, "--llm", "claude", "--format", "json", filepath.Join(repoRoot, "testdata", "plan_empty.json"))
			cmd.Dir = workdir
			cmd.Env = append(os.Environ(), "ANTHROPIC_API_KEY=", "ANTHROPIC_BASE_URL="+server.URL)
			output, err := cmd.CombinedOutput()
			if tc.wantExit {
				if err == nil || !strings.Contains(string(output), "ANTHROPIC_API_KEY") {
					t.Fatalf("err=%v output=%s, want key error", err, output)
				}
			} else if err != nil || !strings.Contains(string(output), "rule-based") || !strings.Contains(string(output), "falling back to rule-based") {
				t.Fatalf("err=%v output=%s, want warning and rule-based fallback", err, output)
			}
			if requests != 0 {
				t.Fatalf("requests=%d, want no request without key", requests)
			}
		})
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

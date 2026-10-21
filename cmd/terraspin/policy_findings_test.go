package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terraspin/terraspin/internal/analyzer"
)

func TestPolicyFindingsCLI(t *testing.T) {
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
	openSSH := filepath.Join(repoRoot, "testdata", "aws", "sg_open_ssh_world", "plan.json")
	internalSSH := filepath.Join(repoRoot, "testdata", "aws", "sg_ssh_internal", "plan.json")

	run := func(dir, format, plan string) string {
		t.Helper()
		cmd := exec.Command(binary, "--no-ai", "--format", format, plan)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("run %s: %v\n%s", format, err, output)
		}
		return string(output)
	}

	for _, format := range []string{"text", "markdown"} {
		if output := run(t.TempDir(), format, openSSH); !strings.Contains(output, "AWS-SG-001") {
			t.Fatalf("%s output missing built-in finding: %s", format, output)
		}
	}
	var openResult struct {
		OverallRisk analyzer.RiskScore `json:"overall_risk"`
		Findings    []analyzer.Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(run(t.TempDir(), "json", openSSH)), &openResult); err != nil {
		t.Fatal(err)
	}
	if openResult.OverallRisk.Tier != analyzer.TierCritical || len(openResult.Findings) != 1 || openResult.Findings[0].Source != "builtin" {
		t.Fatalf("open SSH result = %#v", openResult)
	}

	var internalResult struct {
		OverallRisk analyzer.RiskScore `json:"overall_risk"`
		Findings    []analyzer.Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(run(t.TempDir(), "json", internalSSH)), &internalResult); err != nil {
		t.Fatal(err)
	}
	if internalResult.OverallRisk.Tier != analyzer.TierLow || len(internalResult.Findings) != 0 {
		t.Fatalf("internal SSH result = %#v", internalResult)
	}

	for _, config := range []string{"builtin_rules: false\n", "disable_rules: [AWS-SG-001]\n"} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".terraspin.yml"), []byte("version: 1\n"+config), 0644); err != nil {
			t.Fatal(err)
		}
		var result struct {
			Findings []analyzer.Finding `json:"findings"`
		}
		if err := json.Unmarshal([]byte(run(dir, "json", openSSH)), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Findings) != 0 {
			t.Fatalf("config %q did not disable built-in finding: %#v", config, result.Findings)
		}
	}

	dir := t.TempDir()
	custom := "version: 1\nbuiltin_rules: false\nrules:\n  - id: CUSTOM-SG-001\n    severity: high\n    description: Custom finding\n    remediation: Custom remediation\n    match:\n      resource_type_pattern: aws_security_group\n      action: create\n"
	if err := os.WriteFile(filepath.Join(dir, ".terraspin.yml"), []byte(custom), 0644); err != nil {
		t.Fatal(err)
	}
	var customResult struct {
		Findings []analyzer.Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(run(dir, "json", openSSH)), &customResult); err != nil {
		t.Fatal(err)
	}
	if len(customResult.Findings) != 1 || customResult.Findings[0].Source != "custom" || customResult.Findings[0].Remediation != "Custom remediation" {
		t.Fatalf("custom result = %#v", customResult.Findings)
	}
}

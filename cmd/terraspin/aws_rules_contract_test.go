package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestAWSExpectedFindings(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, "testdata", "aws", "expected_findings.json"))
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string][]string{}
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}

	binary := filepath.Join(t.TempDir(), "terraspin")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = filepath.Join(repoRoot, "cmd", "terraspin")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	plans, err := filepath.Glob(filepath.Join(repoRoot, "testdata", "aws", "*", "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != len(expected) {
		t.Fatalf("plans=%d expected scenarios=%d", len(plans), len(expected))
	}

	for _, plan := range plans {
		scenario := filepath.Base(filepath.Dir(plan))
		want, ok := expected[scenario]
		if !ok {
			t.Fatalf("%s is missing from expected_findings.json", scenario)
		}
		t.Run(scenario, func(t *testing.T) {
			cmd := exec.Command(binary, "--no-ai", "--format", "json", plan)
			cmd.Dir = t.TempDir()
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("run CLI: %v\n%s", err, output)
			}
			var result struct {
				Findings []struct {
					RuleID string `json:"rule_id"`
				} `json:"findings"`
			}
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatalf("decode output: %v\n%s", err, output)
			}
			got := make([]string, 0, len(result.Findings))
			for _, finding := range result.Findings {
				got = append(got, finding.RuleID)
			}
			sort.Strings(got)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("findings=%v, want %v", got, want)
			}
		})
	}
}

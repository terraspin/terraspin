package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

const sshOpenRuleConfig = `
version: 1
rules:
  - id: sg-ssh-open-to-world
    severity: critical
    description: Public SSH ingress needs review
    match:
      resource_type_pattern: aws_security_group
      attribute_path: ingress[*].cidr_blocks
      contains: 0.0.0.0/0
      attribute_path_2: ingress[*].from_port
      value_2: 22
`

func TestCLIRuleListWildcardPaths(t *testing.T) {
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

	for _, fixture := range []string{"sg_open_ssh_world", "sg_ssh_internal"} {
		t.Run(fixture, func(t *testing.T) {
			plan := filepath.Join(repoRoot, "testdata", "aws", fixture, "plan.json")
			baseline := ruleListPathTier(t, binary, t.TempDir(), plan)
			workdir := t.TempDir()
			if err := os.WriteFile(filepath.Join(workdir, ".terraspin.yml"), []byte(sshOpenRuleConfig), 0644); err != nil {
				t.Fatal(err)
			}
			got := ruleListPathTier(t, binary, workdir, plan)
			if fixture == "sg_open_ssh_world" && got != "critical" {
				t.Fatalf("tier = %q, want critical", got)
			}
			if fixture == "sg_ssh_internal" && got != baseline {
				t.Fatalf("tier = %q, want unchanged baseline %q", got, baseline)
			}
		})
	}
}

func TestCLIRuleCorrelationLeavesOtherAWSFixturesUnchanged(t *testing.T) {
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
	fixtures, err := filepath.Glob(filepath.Join(repoRoot, "testdata", "aws", "*", "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range fixtures {
		name := filepath.Base(filepath.Dir(plan))
		if name == "sg_open_ssh_world" || name == "sg_ssh_internal" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			baseline := ruleListPathTiers(t, binary, t.TempDir(), plan)
			workdir := t.TempDir()
			if err := os.WriteFile(filepath.Join(workdir, ".terraspin.yml"), []byte(sshOpenRuleConfig), 0644); err != nil {
				t.Fatal(err)
			}
			if got := ruleListPathTiers(t, binary, workdir, plan); !reflect.DeepEqual(got, baseline) {
				t.Errorf("tiers = %v, want unchanged %v", got, baseline)
			}
		})
	}
}

func ruleListPathTier(t *testing.T, binary, workdir, plan string) string {
	t.Helper()
	tiers := ruleListPathTiers(t, binary, workdir, plan)
	if len(tiers) != 1 {
		t.Fatalf("tiers = %#v, want one", tiers)
	}
	return tiers[0]
}

func ruleListPathTiers(t *testing.T, binary, workdir, plan string) []string {
	t.Helper()
	cmd := exec.Command(binary, "--no-ai", "--format", "json", plan)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run CLI: %v\n%s", err, output)
	}
	var result struct {
		ResourceRisks []struct {
			Tier string `json:"tier"`
		} `json:"resource_risks"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	tiers := make([]string, len(result.ResourceRisks))
	for i, risk := range result.ResourceRisks {
		tiers[i] = risk.Tier
	}
	return tiers
}

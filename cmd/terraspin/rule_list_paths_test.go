package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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
			workdir := t.TempDir()
			config := `
version: 1
rules:
  - id: public-ingress
    severity: critical
    description: Public ingress needs review
    match:
      resource_type_pattern: aws_security_group
      attribute_path: ingress[*].cidr_blocks
      contains: 0.0.0.0/0
`
			if err := os.WriteFile(filepath.Join(workdir, ".terraspin.yml"), []byte(config), 0644); err != nil {
				t.Fatal(err)
			}
			plan := filepath.Join(repoRoot, "testdata", "aws", fixture, "plan.json")
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
			if len(result.ResourceRisks) != 1 || result.ResourceRisks[0].Tier != "critical" {
				t.Fatalf("resource risks = %#v, want one critical risk", result.ResourceRisks)
			}
		})
	}
	// sg_ssh_internal matches because M4a evaluates wildcard results independently;
	// M4b will correlate the CIDR and port within one ingress element.
}

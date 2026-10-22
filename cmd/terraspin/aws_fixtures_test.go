package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terraspin/terraspin/internal/parser"
)

func TestAWSFixturesParse(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixtures, err := filepath.Glob(filepath.Join(repoRoot, "testdata", "aws", "*", "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	wantChanges := map[string]int{
		"ebs_encrypted": 1, "ebs_unencrypted": 1,
		"eks_private": 1, "eks_public_endpoint": 1, "eks_public_restricted": 1,
		"iam_wildcard": 2, "kms_rotation_off": 1, "kms_rotation_on": 1,
		"rds_defaults": 1, "rds_insecure": 1, "rds_replace": 1, "rds_secure": 1,
		"s3_destroy": 3, "s3_public": 3,
		"sg_all_traffic_world": 1, "sg_clean": 1, "sg_open_rdp_world": 1,
		"sg_open_ssh_ipv6": 1, "sg_open_ssh_world": 1, "sg_ssh_internal": 1,
	}
	if len(fixtures) != len(wantChanges) {
		t.Fatalf("fixtures = %d, want %d", len(fixtures), len(wantChanges))
	}

	plans := make(map[string]*parser.PlanAST, len(fixtures))
	for _, fixture := range fixtures {
		name := filepath.Base(filepath.Dir(fixture))
		data, err := os.ReadFile(fixture)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := parser.ParsePlan(data)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if got, want := len(plan.Changes), wantChanges[name]; got != want {
			t.Errorf("%s changes = %d, want %d", name, got, want)
		}
		plans[name] = plan
	}

	openSSH := plans["sg_open_ssh_world"].Changes[0].After["ingress"].([]any)
	if !hasIngress(openSSH, 22, "0.0.0.0/0") {
		t.Fatal("sg_open_ssh_world lacks public TCP/22 ingress")
	}
	if got := plans["rds_insecure"].Changes[0].AfterSensitive["password"]; got != true {
		t.Fatalf("rds_insecure password sensitivity = %v, want true", got)
	}
	if policy, ok := plans["iam_wildcard"].Changes[0].After["policy"].(string); !ok || !strings.Contains(policy, `"Action":"*"`) {
		t.Fatalf("iam wildcard policy = %q, want Action wildcard", policy)
	}

	binary := filepath.Join(t.TempDir(), "terraspin")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = filepath.Join(repoRoot, "cmd", "terraspin")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	secret := "SYNTH_DB_PASSWORD_DO_NOT_USE"
	for _, args := range [][]string{
		{"--no-ai", "--format", "json", filepath.Join(repoRoot, "testdata", "aws", "rds_insecure", "plan.json")},
		{"diff", "--no-ai", "--format", "json", filepath.Join(repoRoot, "testdata", "aws", "rds_insecure", "plan.json"), filepath.Join(repoRoot, "testdata", "aws", "rds_replace", "plan.json")},
	} {
		cmd := exec.Command(binary, args...)
		cmd.Env = append(os.Environ(), "ANTHROPIC_API_KEY=", "OPENAI_API_KEY=")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("run %q: %v\n%s", args, err, output)
		}
		if strings.Contains(string(output), secret) {
			t.Fatalf("output for %q contains sensitive value", args)
		}
	}
}

func hasIngress(ingress []any, port float64, cidr string) bool {
	for _, rule := range ingress {
		values, ok := rule.(map[string]any)
		if !ok || values["from_port"] != port {
			continue
		}
		for _, value := range values["cidr_blocks"].([]any) {
			if value == cidr {
				return true
			}
		}
	}
	return false
}

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/terraspin/terraspin/internal/parser"
)

func TestBuiltinAWSRuleContract(t *testing.T) {
	wantIDs := []string{
		"AWS-EBS-001", "AWS-EKS-001", "AWS-KMS-001",
		"AWS-RDS-001", "AWS-RDS-002", "AWS-RDS-003",
		"AWS-SG-001", "AWS-SG-002", "AWS-SG-003", "AWS-SG-004", "AWS-SG-005", "AWS-SG-006", "AWS-SG-007", "AWS-SG-008",
		"AWS-S3-001", "AWS-S3-002", "AWS-IAM-001", "AWS-IAM-002", "AWS-IAM-003",
	}
	rules, err := builtinRules()
	if err != nil {
		t.Fatal(err)
	}
	gotIDs := make([]string, 0, len(rules))
	seen := map[string]bool{}
	for _, rule := range rules {
		if seen[rule.ID] {
			t.Errorf("duplicate rule ID %q", rule.ID)
		}
		seen[rule.ID] = true
		gotIDs = append(gotIDs, rule.ID)
		if rule.Description == "" || rule.Remediation == "" {
			t.Errorf("%s must have description and remediation", rule.ID)
		}
		switch rule.Severity {
		case "critical", "high", "medium":
		default:
			t.Errorf("%s severity=%q", rule.ID, rule.Severity)
		}
		if rule.Match.ResourceTypePattern == "" {
			t.Errorf("%s has no resource_type_pattern", rule.ID)
		} else if _, err := filepath.Match(rule.Match.ResourceTypePattern, "aws_example"); err != nil {
			t.Errorf("%s resource_type_pattern=%q: %v", rule.ID, rule.Match.ResourceTypePattern, err)
		}
	}
	sort.Strings(gotIDs)
	sort.Strings(wantIDs)
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("built-in IDs=%v, want %v", gotIDs, wantIDs)
	}
}

func TestBuiltinAWSRulesHavePositiveAndNegativeFixtures(t *testing.T) {
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

	rules, err := builtinRules()
	if err != nil {
		t.Fatal(err)
	}
	hasPositive := map[string]bool{}
	hasNegative := map[string]bool{}
	for scenario, ids := range expected {
		planData, err := os.ReadFile(filepath.Join(repoRoot, "testdata", "aws", scenario, "plan.json"))
		if err != nil {
			t.Fatal(err)
		}
		ast, err := parser.ParsePlan(planData)
		if err != nil {
			t.Fatal(err)
		}
		for _, rule := range rules {
			for _, change := range ast.Changes {
				matchesType, err := filepath.Match(rule.Match.ResourceTypePattern, change.Type)
				if err != nil || !matchesType {
					continue
				}
				if containsRuleID(ids, rule.ID) {
					hasPositive[rule.ID] = true
				} else {
					hasNegative[rule.ID] = true
				}
			}
		}
	}
	for _, rule := range rules {
		if !hasPositive[rule.ID] || !hasNegative[rule.ID] {
			t.Errorf("%s positive=%t negative=%t", rule.ID, hasPositive[rule.ID], hasNegative[rule.ID])
		}
	}
}

func TestRDSDefaultsAreNotExpectedFindings(t *testing.T) {
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
	if got := expected["rds_defaults"]; len(got) != 0 {
		t.Fatalf("rds_defaults expected findings=%v, want none", got)
	}
	planData, err := os.ReadFile(filepath.Join(repoRoot, "testdata", "aws", "rds_defaults", "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	ast, err := parser.ParsePlan(planData)
	if err != nil {
		t.Fatal(err)
	}
	after := ast.Changes[0].After
	if after["storage_encrypted"] != nil || after["deletion_protection"] != nil {
		t.Fatalf("rds_defaults security values=%v,%v, want null", after["storage_encrypted"], after["deletion_protection"])
	}
}

func containsRuleID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

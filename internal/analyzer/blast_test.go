package analyzer

import (
	"testing"

	"github.com/terraspin/terraspin/internal/parser"
)

func TestParseDependencyRefs_nilOnNoConfiguration(t *testing.T) {
	data := []byte(`{"format_version": "1.2"}`)
	refs := ParseDependencyRefs(data)
	if refs != nil {
		t.Errorf("expected nil, got %v", refs)
	}
}

func TestParseDependencyRefs_nilOnInvalidJSON(t *testing.T) {
	refs := ParseDependencyRefs([]byte("not json"))
	if refs != nil {
		t.Errorf("expected nil for invalid JSON, got %v", refs)
	}
}

func TestAnalyzeBlastRadius_noopOrReadGetsEmptyRadius(t *testing.T) {
	changes := []parser.ResourceChange{
		{Address: "a", Action: parser.ActionNoOp},
		{Address: "b", Action: parser.ActionRead},
	}
	refs := map[string][]string{"a": {"b"}}
	result := AnalyzeBlastRadius(changes, refs)

	for _, addr := range []string{"a", "b"} {
		br := result[addr]
		if br == nil {
			t.Fatalf("expected result for %q", addr)
		}
		if br.TotalAffected != 0 {
			t.Errorf("%q.TotalAffected = %d, want 0", addr, br.TotalAffected)
		}
	}
}

func TestAnalyzeBlastRadius_directDependentsFound(t *testing.T) {
	changes := []parser.ResourceChange{
		{Address: "aws_db_instance.primary", Action: parser.ActionDelete},
	}
	// db is referenced by lambda and ssm parameter
	refs := map[string][]string{
		"aws_lambda_function.api":          {"aws_db_instance.primary"},
		"aws_ssm_parameter.db_url":        {"aws_db_instance.primary"},
	}
	result := AnalyzeBlastRadius(changes, refs)

	br := result["aws_db_instance.primary"]
	if br == nil {
		t.Fatal("expected blast radius for aws_db_instance.primary")
	}
	if br.TotalAffected != 2 {
		t.Errorf("TotalAffected = %d, want 2", br.TotalAffected)
	}
	if len(br.DirectDeps) != 2 {
		t.Errorf("DirectDeps = %d, want 2", len(br.DirectDeps))
	}
}

func TestParseDependencyRefs_filtersVariablesAndDataRefs(t *testing.T) {
	data := []byte(`{
		"format_version": "1.2",
		"configuration": {
			"root_module": {
				"resources": [
					{
						"address": "aws_instance.web",
						"mode": "managed",
						"type": "aws_instance",
						"name": "web",
						"provider_name": "aws",
						"expressions": {
							"ami": {
								"references": ["var.ami_id", "data.aws_ami.ubuntu.id", "aws_security_group.web.id"]
							},
							"subnet_id": {
								"references": ["module.network.subnet_id"]
							}
						}
					}
				]
			}
		}
	}`)

	refs := ParseDependencyRefs(data)
	if len(refs) == 0 {
		t.Fatal("expected refs")
	}

	deps := refs["aws_instance.web"]
	// var.*, data.*, and module.* should all be filtered
	for _, d := range deps {
		if d == "var.ami_id" {
			t.Error("var.ami_id should have been filtered")
		}
		if d == "data.aws_ami.ubuntu" {
			t.Error("data.aws_ami.ubuntu should have been filtered")
		}
		if d == "module.network.subnet_id" {
			t.Error("module.* refs should have been filtered")
		}
	}
	// Only actual resource ref should remain
	found := false
	for _, d := range deps {
		if d == "aws_security_group.web" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("deps = %v, should include aws_security_group.web", deps)
	}
}

func TestAnalyzeBlastRadius_transitiveDepsFound(t *testing.T) {
	changes := []parser.ResourceChange{
		{Address: "aws_vpc.main", Action: parser.ActionDelete},
	}
	// vpc → subnet → instance (2 hops)
	refs := map[string][]string{
		"aws_subnet.private": {"aws_vpc.main"},
		"aws_instance.app":   {"aws_subnet.private"},
	}
	result := AnalyzeBlastRadius(changes, refs)

	br := result["aws_vpc.main"]
	if br == nil {
		t.Fatal("expected blast radius")
	}
	if br.TotalAffected != 2 {
		t.Errorf("TotalAffected = %d, want 2", br.TotalAffected)
	}
	if len(br.DirectDeps) != 1 {
		t.Errorf("DirectDeps = %d, want 1 (aws_subnet.private)", len(br.DirectDeps))
	}
	if len(br.TransitiveDeps) != 1 {
		t.Errorf("TransitiveDeps = %d, want 1 (aws_instance.app)", len(br.TransitiveDeps))
	}
	if br.TransitiveDeps[0].Hops != 2 {
		t.Errorf("transitive hop count = %d, want 2", br.TransitiveDeps[0].Hops)
	}
}

func TestAnalyzeBlastRadius_circularDepsDontHang(t *testing.T) {
	changes := []parser.ResourceChange{
		{Address: "a", Action: parser.ActionDelete},
	}
	// a → b → c → a (circle)
	refs := map[string][]string{
		"a": {"c"},
		"b": {"a"},
		"c": {"b"},
	}
	result := AnalyzeBlastRadius(changes, refs)
	br := result["a"]
	if br == nil {
		t.Fatal("expected blast radius")
	}
	// Should not loop forever; b and c discovered once each
	if br.TotalAffected != 2 {
		t.Errorf("TotalAffected = %d, want 2 (b and c, not looping back to a)", br.TotalAffected)
	}
}

func TestAnalyzeBlastRadius_emptyChanges(t *testing.T) {
	result := AnalyzeBlastRadius(nil, nil)
	if result == nil {
		t.Fatal("expected non-nil map")
	}
	if len(result) != 0 {
		t.Errorf("got %d entries, want 0", len(result))
	}
}

func TestParseDependencyRefs_extractsResourceReferences(t *testing.T) {
	data := []byte(`{
		"format_version": "1.2",
		"terraform_version": "1.0.0",
		"configuration": {
			"root_module": {
				"resources": [
					{
						"address": "aws_security_group.web",
						"mode": "managed",
						"type": "aws_security_group",
						"name": "web",
						"provider_name": "aws",
						"expressions": {
							"ingress": {
								"references": ["aws_security_group_rule.ssh.id"]
							},
							"name": {
								"references": []
							}
						}
					}
				]
			}
		}
	}`)

	refs := ParseDependencyRefs(data)
	if len(refs) == 0 {
		t.Fatal("expected at least one ref, got none")
	}

	src := "aws_security_group.web"
	deps, ok := refs[src]
	if !ok {
		t.Fatalf("no refs found for %q", src)
	}
	if len(deps) != 1 || deps[0] != "aws_security_group_rule.ssh" {
		t.Errorf("refs[%q] = %v, want [aws_security_group_rule.ssh]", src, deps)
	}
}

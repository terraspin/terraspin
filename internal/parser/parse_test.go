package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParsePlan(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "plan.json"))
	if err != nil {
		t.Fatal(err)
	}

	ast, err := ParsePlan(data)
	if err != nil {
		t.Fatalf("ParsePlan() error = %v", err)
	}

	if ast.TerraformVersion != "1.7.2" {
		t.Errorf("terraform_version = %q, want %q", ast.TerraformVersion, "1.7.2")
	}

	want := 5
	if got := len(ast.Changes); got != want {
		t.Errorf("len(changes) = %d, want %d", got, want)
	}

	tests := []struct {
		addr   string
		action ChangeAction
	}{
		{"aws_instance.web", ActionCreate},
		{"aws_db_instance.primary", ActionDelete},
		{"module.network.aws_vpc.main", ActionUpdate},
		{"aws_security_group.web", ActionReplace},
		{"aws_s3_bucket.logs", ActionNoOp},
	}

	for _, tt := range tests {
		var found bool
		for _, c := range ast.Changes {
			if c.Address == tt.addr {
				found = true
				if c.Action != tt.action {
					t.Errorf("change[%q].action = %q, want %q", tt.addr, c.Action, tt.action)
				}
				if tt.addr == "module.network.aws_vpc.main" && c.ModulePath != "module.network" {
					t.Errorf("change[%q].module_path = %q, want %q", tt.addr, c.ModulePath, "module.network")
				}
				break
			}
		}
		if !found {
			t.Errorf("change %q not found in parsed AST", tt.addr)
		}
	}

	if _, ok := ast.OutputChanges["web_public_ip"]; !ok {
		t.Error("output change 'web_public_ip' not found")
	}

	// Verify data sources are excluded
	for _, c := range ast.Changes {
		if c.Type == "aws_availability_zones" {
			t.Errorf("data source should be excluded from changes, got %q", c.Address)
		}
	}
}

func TestParsePlan_InvalidJSON(t *testing.T) {
	_, err := ParsePlan([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParsePlan_Empty(t *testing.T) {
	_, err := ParsePlan([]byte(`{}`))
	if err == nil {
		t.Fatal("expected error for missing format_version")
	}
}

func TestParsePlan_Minimal(t *testing.T) {
	input := `{
		"format_version": "1.2",
		"terraform_version": "1.0.0",
		"resource_changes": [
			{
				"address": "aws_instance.x",
				"mode": "managed",
				"type": "aws_instance",
				"name": "x",
				"provider_name": "aws",
				"change": {
					"actions": ["create"],
					"before": null,
					"after": {"ami": "ami-1"}
				}
			}
		]
	}`
	ast, err := ParsePlan([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(ast.Changes) != 1 {
		t.Errorf("got %d changes, want 1", len(ast.Changes))
	}
}

func TestRoundTrip(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	ast, err := ParsePlan(data)
	if err != nil {
		t.Fatal(err)
	}
	// Marshal back to JSON — just verify it's valid JSON
	out, err := json.Marshal(ast)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(out) {
		t.Fatal("round-tripped AST is not valid JSON")
	}
}

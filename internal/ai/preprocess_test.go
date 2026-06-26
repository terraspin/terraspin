package ai

import (
	"testing"

	"github.com/terraspin/terraspin/internal/parser"
)

func TestRedactSensitive_basic(t *testing.T) {
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{
				Address: "aws_db_instance.primary",
				Before: map[string]any{
					"engine":   "postgres",
					"password": "super-secret-123",
					"port":     float64(5432),
				},
				BeforeSensitive: map[string]any{
					"password": true,
				},
			},
		},
	}

	RedactSensitive(ast)

	got := ast.Changes[0].Before["password"]
	if got != redacted {
		t.Errorf("password = %v (%T), want %q", got, got, redacted)
	}
	if ast.Changes[0].Before["engine"] != "postgres" {
		t.Errorf("engine changed to %v", ast.Changes[0].Before["engine"])
	}
}

func TestRedactSensitive_nonSensitiveUnchanged(t *testing.T) {
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{
				Address: "aws_instance.web",
				After: map[string]any{
					"ami":           "ami-abc123",
					"instance_type": "t3.micro",
				},
				AfterSensitive: map[string]any{},
			},
		},
	}

	RedactSensitive(ast)

	if ast.Changes[0].After["ami"] != "ami-abc123" {
		t.Error("non-sensitive field was modified")
	}
}

func TestRedactSensitive_nestedPath(t *testing.T) {
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{
				Address: "aws_instance.app",
				After: map[string]any{
					"tags": map[string]any{
						"Name":   "my-app",
						"Secret": "hidden-value",
					},
				},
				AfterSensitive: map[string]any{
					"tags": map[string]any{
						"Secret": true,
					},
				},
			},
		},
	}

	RedactSensitive(ast)

	tags := ast.Changes[0].After["tags"].(map[string]any)
	if tags["Secret"] != redacted {
		t.Errorf("nested Secret = %v, want %q", tags["Secret"], redacted)
	}
	if tags["Name"] != "my-app" {
		t.Error("nested non-sensitive field was modified")
	}
}

func TestRedactSensitive_bothBeforeAndAfter(t *testing.T) {
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{
				Address: "aws_db_instance.db",
				Before: map[string]any{
					"password": "old-secret",
				},
				After: map[string]any{
					"password": "new-secret",
				},
				BeforeSensitive: map[string]any{
					"password": true,
				},
				AfterSensitive: map[string]any{
					"password": true,
				},
			},
		},
	}

	RedactSensitive(ast)

	if ast.Changes[0].Before["password"] != redacted {
		t.Errorf("before password = %v", ast.Changes[0].Before["password"])
	}
	if ast.Changes[0].After["password"] != redacted {
		t.Errorf("after password = %v", ast.Changes[0].After["password"])
	}
}

func TestRedactSensitive_nilSensitiveMaps(t *testing.T) {
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{
				Address: "aws_s3_bucket.logs",
				Before: map[string]any{
					"bucket": "logs",
				},
				// BeforeSensitive/AfterSensitive nil
			},
		},
	}

	RedactSensitive(ast)

	if ast.Changes[0].Before["bucket"] != "logs" {
		t.Error("nil sensitive map caused modification")
	}
}

func TestRedactSensitive_multipleResources(t *testing.T) {
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{
				Address: "safe",
				Before:  map[string]any{"x": "keep"},
				BeforeSensitive: map[string]any{},
			},
			{
				Address: "leaky",
				Before: map[string]any{"token": "abc"},
				BeforeSensitive: map[string]any{"token": true},
			},
		},
	}

	RedactSensitive(ast)

	if ast.Changes[0].Before["x"] != "keep" {
		t.Error("safe resource modified")
	}
	if ast.Changes[1].Before["token"] != redacted {
		t.Error("leaky resource not redacted")
	}
}

func TestRedactSensitive_integration(t *testing.T) {
	// Parse the real fixture, redact, verify the known sensitive value
	data := []byte(`{
		"format_version": "1.2",
		"terraform_version": "1.0.0",
		"resource_changes": [
			{
				"address": "aws_db_instance.primary",
				"mode": "managed",
				"type": "aws_db_instance",
				"name": "primary",
				"provider_name": "aws",
				"change": {
					"actions": ["delete"],
					"before": {
						"engine": "postgres",
						"password": "super-secret-db-pass-123"
					},
					"after": null,
					"before_sensitive": {
						"password": true
					},
					"after_sensitive": {}
				}
			}
		]
	}`)

	ast, err := parser.ParsePlan(data)
	if err != nil {
		t.Fatal(err)
	}

	RedactSensitive(ast)

	if ast.Changes[0].Before["password"] != redacted {
		t.Errorf("integration: password = %v, want %q", ast.Changes[0].Before["password"], redacted)
	}
	if ast.Changes[0].Before["engine"] != "postgres" {
		t.Error("integration: non-sensitive field modified")
	}
}

func TestRedactSensitive_deepNested(t *testing.T) {
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{
				Address: "deep",
				After: map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"secret": "bury-me",
							"open":   "visible",
						},
					},
				},
				AfterSensitive: map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"secret": true,
						},
					},
				},
			},
		},
	}

	RedactSensitive(ast)

	inner := ast.Changes[0].After["a"].(map[string]any)["b"].(map[string]any)
	if inner["secret"] != redacted {
		t.Errorf("deep nested secret = %v", inner["secret"])
	}
	if inner["open"] != "visible" {
		t.Error("deep nested non-sensitive field modified")
	}
}

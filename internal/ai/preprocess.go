package ai

import (
	"github.com/terraspin/terraspin/internal/parser"
)

const redacted = "[SENSITIVE REDACTED]"

// SensitiveRedacted is the replacement string for sensitive values.
const SensitiveRedacted = redacted

// RedactSensitive replaces all sensitive values in Before/After maps
// with "[SENSITIVE REDACTED]" based on BeforeSensitive/AfterSensitive
// path markers from the terraform plan JSON.
//
// Must be called before any serialization or LLM transmission.
func RedactSensitive(ast *parser.PlanAST) {
	for i := range ast.Changes {
		rc := &ast.Changes[i]
		redactMap(rc.Before, rc.BeforeSensitive)
		redactMap(rc.After, rc.AfterSensitive)
	}
}

// redactMap walks the sensitive mirror and replaces marked values in data.
func redactMap(data, sensitive map[string]any) {
	if data == nil || sensitive == nil {
		return
	}
	for k, marker := range sensitive {
		if value, ok := data[k]; ok {
			data[k] = redactValue(value, marker)
		}
	}
}

func redactValue(value, marker any) any {
	switch m := marker.(type) {
	case bool:
		if m {
			return redacted
		}
	case map[string]any:
		if data, ok := value.(map[string]any); ok {
			redactMap(data, m)
		}
	case []any:
		if data, ok := value.([]any); ok {
			for i, child := range m {
				if i < len(data) {
					data[i] = redactValue(data[i], child)
				}
			}
		}
	}
	return value
}

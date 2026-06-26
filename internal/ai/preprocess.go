package ai

import (
	"github.com/terraspin/terraspin/internal/parser"
)

const redacted = "[SENSITIVE REDACTED]"

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

// redactMap walks the sensitive mirror map and replaces values in data
// at every path where the sensitive leaf is true.
func redactMap(data, sensitive map[string]any) {
	if data == nil || sensitive == nil {
		return
	}
	for k, sv := range sensitive {
		dv, ok := data[k]
		if !ok {
			continue
		}
		switch s := sv.(type) {
		case map[string]any:
			if dm, ok := dv.(map[string]any); ok {
				redactMap(dm, s)
			}
		case bool:
			if s {
				data[k] = redacted
			}
		}
	}
}

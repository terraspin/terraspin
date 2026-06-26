package parser

// ChangeAction is the type of change for a resource.
type ChangeAction string

const (
	ActionCreate  ChangeAction = "create"
	ActionUpdate  ChangeAction = "update"
	ActionDelete  ChangeAction = "delete"
	ActionReplace ChangeAction = "replace"
	ActionNoOp    ChangeAction = "no-op"
	ActionRead    ChangeAction = "read"
)

// ResourceChange represents a single resource change in a terraform plan.
type ResourceChange struct {
	Address      string         `json:"address"`
	ModulePath   string         `json:"module_path"`
	Type         string         `json:"type"`
	Name         string         `json:"name"`
	ProviderName string         `json:"provider_name"`
	Action       ChangeAction   `json:"action"`
	ActionReason string         `json:"action_reason,omitempty"`
	Before       map[string]any `json:"before"`
	After        map[string]any `json:"after"`
	Sensitive    bool           `json:"sensitive,omitempty"`
	ForceReplace bool           `json:"force_replace,omitempty"`
}

// PlanAST is the internal representation of a terraform plan JSON.
type PlanAST struct {
	TerraformVersion string                    `json:"terraform_version"`
	FormatVersion    string                    `json:"format_version"`
	Variables        map[string]Variable       `json:"variables,omitempty"`
	Changes          []ResourceChange          `json:"changes"`
	OutputChanges    map[string]OutputChange   `json:"output_changes,omitempty"`
	
}

// Variable holds a plan variable value.
type Variable struct {
	Value any `json:"value"`
}

// OutputChange represents a change to a terraform output.
type OutputChange struct {
	Actions []string   `json:"actions"`
	Before  any        `json:"before"`
	After   any        `json:"after"`
}



package parser

import (
	"encoding/json"
	"fmt"
	"strings"
)

// terraformPlanJSON maps the raw terraform show -json output.
type terraformPlanJSON struct {
	FormatVersion    string            `json:"format_version"`
	TerraformVersion string            `json:"terraform_version"`
	Workspace        string            `json:"workspace,omitempty"`
	Variables        map[string]struct {
		Value any `json:"value"`
	} `json:"variables,omitempty"`
	ResourceChanges []terraformResourceChange `json:"resource_changes,omitempty"`
	OutputChanges   map[string]terraformOutputChange `json:"output_changes,omitempty"`
	Configuration   *terraformConfiguration  `json:"configuration,omitempty"`
}

type terraformResourceChange struct {
	Address         string `json:"address"`
	ModuleAddress   string `json:"module_address,omitempty"`
	Mode            string `json:"mode"`
	Type            string `json:"type"`
	Name            string `json:"name"`
	ProviderName    string `json:"provider_name"`
	ActionReason    string `json:"action_reason,omitempty"`
	Change          struct {
		Actions         []string     `json:"actions"`
		Before          any          `json:"before"`
		After           any          `json:"after"`
		BeforeSensitive any          `json:"before_sensitive"`
		AfterSensitive  any          `json:"after_sensitive"`
	} `json:"change"`
}

type terraformOutputChange struct {
	Actions []string `json:"actions"`
	Before  any      `json:"before"`
	After   any      `json:"after"`
}

type terraformConfiguration struct {
	ProviderConfig map[string]any `json:"provider_config,omitempty"`
}

// ParsePlan parses a raw terraform show -json byte slice into a PlanAST.
func ParsePlan(data []byte) (*PlanAST, error) {
	var raw terraformPlanJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse terraform plan: %w", err)
	}

	if raw.FormatVersion == "" {
		return nil, fmt.Errorf("parse terraform plan: missing format_version — input is not a valid terraform plan JSON")
	}

	ast := &PlanAST{
		TerraformVersion: raw.TerraformVersion,
		FormatVersion:    raw.FormatVersion,
		Workspace:        raw.Workspace,
		Variables:        make(map[string]Variable, len(raw.Variables)),
		Changes:          make([]ResourceChange, 0, len(raw.ResourceChanges)),
		OutputChanges:    make(map[string]OutputChange, len(raw.OutputChanges)),
	}

	// Fill variables
	for k, v := range raw.Variables {
		ast.Variables[k] = Variable{Value: v.Value}
	}

	// Fill resource changes
	for _, rc := range raw.ResourceChanges {
		if rc.Mode != "managed" {
			continue // skip data sources
		}

		action := collapseActions(rc.Change.Actions)
		before, _ := rc.Change.Before.(map[string]any)
		after, _ := rc.Change.After.(map[string]any)

		// Determine module path from address
		modulePath := moduleFromAddress(rc.Address)

		forceReplace := action == ActionReplace

		sensitive := false
		if m, ok := rc.Change.AfterSensitive.(map[string]any); ok {
			sensitive = len(m) > 0
		}
		beforeSensitive, _ := rc.Change.BeforeSensitive.(map[string]any)
		afterSensitive, _ := rc.Change.AfterSensitive.(map[string]any)

		ast.Changes = append(ast.Changes, ResourceChange{
			Address:         rc.Address,
			ModulePath:      modulePath,
			Type:            rc.Type,
			Name:            rc.Name,
			ProviderName:    rc.ProviderName,
			Action:          action,
			ActionReason:    rc.ActionReason,
			Before:          before,
			After:           after,
			BeforeSensitive: beforeSensitive,
			AfterSensitive:  afterSensitive,
			Sensitive:       sensitive,
			ForceReplace:    forceReplace,
		})
	}

	// Fill output changes
	for k, oc := range raw.OutputChanges {
		ast.OutputChanges[k] = OutputChange{
			Actions: oc.Actions,
			Before:  oc.Before,
			After:   oc.After,
		}
	}

	return ast, nil
}

// collapseActions converts a terraform action list to a single ChangeAction.
//   ["create"]          → create
//   ["update"]          → update
//   ["delete"]          → delete
//   ["delete","create"] → replace
//   ["no-op"]           → no-op
//   ["read"]            → read
func collapseActions(actions []string) ChangeAction {
	if len(actions) == 0 {
		return ActionNoOp
	}
	if len(actions) == 1 {
		switch actions[0] {
		case "create":
			return ActionCreate
		case "update":
			return ActionUpdate
		case "delete":
			return ActionDelete
		case "no-op":
			return ActionNoOp
		case "read":
			return ActionRead
		}
	}
	if len(actions) == 2 && actions[0] == "delete" && actions[1] == "create" {
		return ActionReplace
	}
	return ActionReplace
}

func moduleFromAddress(addr string) string {
	if !strings.HasPrefix(addr, "module.") {
		return ""
	}
	parts := strings.Split(addr, ".")
	return strings.Join(parts[:len(parts)-2], ".")
}

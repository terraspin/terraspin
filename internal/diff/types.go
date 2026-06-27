// Package diff implements environment drift analysis between two
// terraform plan outputs. Supports the `terraspin diff` command.
package diff

import "github.com/terraspin/terraspin/internal/parser"

// DiffStatus classifies a resource difference between two plans.
type DiffStatus string

const (
	StatusAdded     DiffStatus = "added"
	StatusRemoved   DiffStatus = "removed"
	StatusModified  DiffStatus = "modified"
	StatusUnchanged DiffStatus = "unchanged"
)

// ResourceDiff describes one resource's difference between two plans.
type ResourceDiff struct {
	Address  string              `json:"address"`
	Type     string              `json:"type"`
	Status   DiffStatus          `json:"status"`
	Old      *parser.ResourceChange `json:"old,omitempty"`
	New      *parser.ResourceChange `json:"new,omitempty"`
	Changes  []AttributeDiff     `json:"changes,omitempty"`
}

// AttributeDiff describes a single attribute value change.
type AttributeDiff struct {
	Path     string `json:"path"`
	OldValue any    `json:"old_value,omitempty"`
	NewValue any    `json:"new_value,omitempty"`
}

// DriftSummary holds aggregate counts for the diff.
type DriftSummary struct {
	Added     int `json:"added"`
	Removed   int `json:"removed"`
	Modified  int `json:"modified"`
	Unchanged int `json:"unchanged"`
	Total     int `json:"total"`
}

// DiffResult is the complete comparison of two terraform plans.
type DiffResult struct {
	LabelA  string          `json:"label_a"`
	LabelB  string          `json:"label_b"`
	VersionA string         `json:"terraform_version_a"`
	VersionB string         `json:"terraform_version_b"`
	Summary DriftSummary    `json:"summary"`
	Diffs   []ResourceDiff  `json:"diffs"`
}

package diff

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/terraspin/terraspin/internal/parser"
)

// Compare parses two plan JSON byte slices and returns a structured
// resource-by-resource comparison.
func Compare(dataA, dataB []byte, labelA, labelB string) (*DiffResult, error) {
	astA, err := parser.ParsePlan(dataA)
	if err != nil {
		return nil, fmt.Errorf("parse plan A: %w", err)
	}
	astB, err := parser.ParsePlan(dataB)
	if err != nil {
		return nil, fmt.Errorf("parse plan B: %w", err)
	}

	return CompareASTs(astA, astB, labelA, labelB), nil
}

// CompareASTs compares two parsed plans and returns a structured diff.
func CompareASTs(astA, astB *parser.PlanAST, labelA, labelB string) *DiffResult {
	idxA := indexByAddress(astA.Changes)
	idxB := indexByAddress(astB.Changes)

	allAddrs := collectAddrs(idxA, idxB)

	result := &DiffResult{
		LabelA:    labelA,
		LabelB:    labelB,
		VersionA:  astA.TerraformVersion,
		VersionB:  astB.TerraformVersion,
	}

	for _, addr := range allAddrs {
		rcA, inA := idxA[addr]
		rcB, inB := idxB[addr]

		switch {
		case inA && !inB:
			result.Diffs = append(result.Diffs, ResourceDiff{
				Address: addr,
				Type:    rcA.Type,
				Status:  StatusRemoved,
				Old:     &rcA,
			})
		case !inA && inB:
			result.Diffs = append(result.Diffs, ResourceDiff{
				Address: addr,
				Type:    rcB.Type,
				Status:  StatusAdded,
				New:     &rcB,
			})
		case inA && inB:
			changes := diffAttrs(rcA.Before, rcB.Before, "")
			if rcA.Action != rcB.Action || len(changes) > 0 || valuesDiffer(rcA.After, rcB.After) {
				// Also diff after states for a fuller picture
				afterChanges := diffAttrs(rcA.After, rcB.After, "")
				changes = append(changes, afterChanges...)
				result.Diffs = append(result.Diffs, ResourceDiff{
					Address: addr,
					Type:    rcA.Type,
					Status:  StatusModified,
					Old:     &rcA,
					New:     &rcB,
					Changes: dedupChanges(changes),
				})
			} else {
				result.Diffs = append(result.Diffs, ResourceDiff{
					Address: addr,
					Type:    rcA.Type,
					Status:  StatusUnchanged,
				})
			}
		}
	}

	// Tally summary
	for _, d := range result.Diffs {
		switch d.Status {
		case StatusAdded:
			result.Summary.Added++
		case StatusRemoved:
			result.Summary.Removed++
		case StatusModified:
			result.Summary.Modified++
		case StatusUnchanged:
			result.Summary.Unchanged++
		}
	}
	result.Summary.Total = len(result.Diffs)

	return result
}

// indexByAddress builds a map of address → ResourceChange.
func indexByAddress(changes []parser.ResourceChange) map[string]parser.ResourceChange {
	idx := make(map[string]parser.ResourceChange, len(changes))
	for _, c := range changes {
		idx[c.Address] = c
	}
	return idx
}

// collectAddrs returns all unique addresses across both indexes, sorted.
func collectAddrs(idxA, idxB map[string]parser.ResourceChange) []string {
	seen := make(map[string]struct{}, len(idxA)+len(idxB))
	for addr := range idxA {
		seen[addr] = struct{}{}
	}
	for addr := range idxB {
		seen[addr] = struct{}{}
	}
	return slices.Sorted(maps.Keys(seen))
}

// diffAttrs compares two maps and returns a list of changed paths.
// prefix is used for nested keys (e.g. "tags.Name").
func diffAttrs(a, b map[string]any, prefix string) []AttributeDiff {
	var changes []AttributeDiff
	for _, k := range collectKeys(a, b) {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		va, hasA := a[k]
		vb, hasB := b[k]

		if !hasA {
			changes = append(changes, AttributeDiff{Path: path, NewValue: vb})
			continue
		}
		if !hasB {
			changes = append(changes, AttributeDiff{Path: path, OldValue: va})
			continue
		}

		// Recurse into nested maps
		ma, okA := va.(map[string]any)
		mb, okB := vb.(map[string]any)
		if okA && okB {
			changes = append(changes, diffAttrs(ma, mb, path)...)
			continue
		}

		if !valuesEqual(va, vb) {
			changes = append(changes, AttributeDiff{
				Path:     path,
				OldValue: va,
				NewValue: vb,
			})
		}
	}
	return changes
}

// collectKeys returns all unique keys from both maps.
func collectKeys(a, b map[string]any) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	return keys
}

// valuesEqual compares two values using reflect.DeepEqual as fallback.
// ponytail: fmt.Sprintf for cross-type comparison (int vs float64).
func valuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// valuesDiffer is the inverse of valuesEqual for the After check.
func valuesDiffer(a, b map[string]any) bool {
	if len(a) != len(b) {
		return true
	}
	for k := range a {
		va, okA := a[k]
		vb, okB := b[k]
		if !okA || !okB || !valuesEqual(va, vb) {
			return true
		}
	}
	return false
}

// dedupChanges removes duplicate entries with the same path (prefers first).
func dedupChanges(changes []AttributeDiff) []AttributeDiff {
	seen := make(map[string]bool, len(changes))
	out := make([]AttributeDiff, 0, len(changes))
	for _, c := range changes {
		if !seen[c.Path] {
			seen[c.Path] = true
			out = append(out, c)
		}
	}
	return out
}



// DiffSummaryText returns a one-line summary string for the diff.
func (r *DiffResult) DiffSummaryText() string {
	t := r.Summary
	var parts []string
	if t.Added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", t.Added))
	}
	if t.Removed > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", t.Removed))
	}
	if t.Modified > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", t.Modified))
	}
	if t.Unchanged > 0 {
		parts = append(parts, fmt.Sprintf("%d unchanged", t.Unchanged))
	}
	if len(parts) == 0 {
		return "no resource differences"
	}
	return strings.Join(parts, ", ")
}

// BuildDiffPrompt builds a prompt for AI analysis of the diff.
func BuildDiffPrompt(result *DiffResult) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Environment drift analysis between %q and %q:\n\n", result.LabelA, result.LabelB)
	fmt.Fprintf(&b, "Terraform versions: %s vs %s\n\n", result.VersionA, result.VersionB)
	fmt.Fprintf(&b, "Summary: %s\n\n", result.DiffSummaryText())
	fmt.Fprintf(&b, "Resource differences:\n")

	for _, d := range result.Diffs {
		if d.Status == StatusUnchanged {
			continue
		}
		fmt.Fprintf(&b, "- %s [%s] %s", d.Address, d.Type, string(d.Status))
		if len(d.Changes) > 0 {
			fmt.Fprintf(&b, " (%d attributes changed)", len(d.Changes))
		}
		fmt.Fprintln(&b)
	}

	b.WriteString(`
Analyze these environment drifts and return ONLY valid JSON with these exact keys:
{
  "summary": "2-3 sentence explanation of what differs and why it matters",
  "critical_changes": ["any drift that could cause instability or data loss"],
  "risk_assessment": "whether the drift is expected (intentional config change) or unexpected (configuration drift)",
  "recommendations": ["specific steps to align or remediate each drift"],
  "rollback_strategy": "how to revert the drifted environment to match the source"
}`)
	return b.String()
}

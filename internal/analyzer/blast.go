// Package analyzer provides risk scoring and blast radius analysis for terraform plan changes.
package analyzer

import (
	"encoding/json"
	"strings"

	"github.com/terraspin/terraspin/internal/parser"
)

// DependentResource holds one dependent resource.
type DependentResource struct {
	Address string `json:"address"`
	Type    string `json:"type"`
	Hops    int    `json:"hops"`
}

// BlastRadius holds full blast radius for one changed resource.
type BlastRadius struct {
	RootAddress    string              `json:"root_address"`
	DirectDeps     []DependentResource `json:"direct_deps,omitempty"`
	TransitiveDeps []DependentResource `json:"transitive_deps,omitempty"`
	TotalAffected  int                 `json:"total_affected"`
}

// AnalyzeBlastRadius computes blast radius for each changed resource using
// the dependency graph. refs maps resource address → addresses it references.
func AnalyzeBlastRadius(changes []parser.ResourceChange, refs map[string][]string) map[string]*BlastRadius {
	// Build reverse deps: resource → set of resources that reference it
	rev := make(map[string]map[string]bool)
	for src, targets := range refs {
		for _, t := range targets {
			if rev[t] == nil {
				rev[t] = make(map[string]bool)
			}
			rev[t][src] = true
		}
	}

	result := make(map[string]*BlastRadius, len(changes))
	for _, rc := range changes {
		if rc.Action == parser.ActionNoOp || rc.Action == parser.ActionRead {
			result[rc.Address] = &BlastRadius{RootAddress: rc.Address}
			continue
		}

		br := &BlastRadius{RootAddress: rc.Address}
		visited := map[string]int{rc.Address: 0}
		queue := []string{rc.Address}

		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			curDist := visited[cur]

			for dep := range rev[cur] {
				if _, seen := visited[dep]; seen {
					continue
				}
				visited[dep] = curDist + 1
				queue = append(queue, dep)

				dr := DependentResource{
					Address: dep,
					Type:    extractType(dep),
					Hops:    curDist + 1,
				}
				if curDist+1 == 1 {
					br.DirectDeps = append(br.DirectDeps, dr)
				} else {
					br.TransitiveDeps = append(br.TransitiveDeps, dr)
				}
			}
		}

		br.TotalAffected = len(br.DirectDeps) + len(br.TransitiveDeps)
		result[rc.Address] = br
	}
	return result
}

// ParseDependencyRefs reads the raw plan JSON and extracts resource-level
// expression references from the configuration section.
// Returns map of resource address → addresses it references.
func ParseDependencyRefs(data []byte) map[string][]string {
	var raw struct {
		Configuration *struct {
			RootModule *struct {
				Resources []struct {
					Address     string `json:"address"`
					Expressions map[string]struct {
						References []string `json:"references,omitempty"`
					} `json:"expressions,omitempty"`
				} `json:"resources,omitempty"`
			} `json:"root_module,omitempty"`
		} `json:"configuration,omitempty"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	if raw.Configuration == nil || raw.Configuration.RootModule == nil {
		return nil
	}

	out := make(map[string][]string)
	for _, r := range raw.Configuration.RootModule.Resources {
		for _, expr := range r.Expressions {
			for _, ref := range expr.References {
				if isResourceRef(ref) {
					out[r.Address] = append(out[r.Address], extractResourceAddr(ref))
				}
			}
		}
	}
	return out
}

// isResourceRef returns true if the ref looks like a resource reference.
func isResourceRef(ref string) bool {
	if ref == "" || !strings.Contains(ref, ".") {
		return false
	}
	prefix := strings.SplitN(ref, ".", 2)[0]
	switch prefix {
	case "var", "data", "path", "terraform", "local", "each", "count":
		return false
	}
	return !strings.HasPrefix(ref, "module.")
}

// extractResourceAddr drops the trailing attribute from a reference.
// "aws_security_group.web.id" → "aws_security_group.web"
func extractResourceAddr(ref string) string {
	parts := strings.Split(ref, ".")
	if len(parts) < 2 {
		return ref
	}
	return strings.Join(parts[:len(parts)-1], ".")
}

// extractType pulls the resource type from a full address.
func extractType(addr string) string {
	if strings.HasPrefix(addr, "module.") {
		parts := strings.Split(addr, ".")
		if len(parts) >= 4 {
			return parts[len(parts)-2]
		}
		return addr
	}
	parts := strings.Split(addr, ".")
	if len(parts) >= 2 {
		return parts[0]
	}
	return addr
}

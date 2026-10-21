package config

import (
	"fmt"
	"strconv"
)

// Path is a parsed custom-rule attribute path.
type Path struct {
	segments       []pathSegment
	wildcardPrefix string
}

// IndexedValue pairs a wildcard-expanded list index with its resolved value.
type IndexedValue struct {
	Index int
	Value any
}

type pathSegment struct {
	name     string
	index    int
	hasIndex bool
	wildcard bool
}

// ParsePath parses dot-separated attribute paths with an optional index or
// wildcard on each segment.
func ParsePath(value string) (Path, error) {
	if value == "" {
		return Path{}, fmt.Errorf("path is empty")
	}

	var path Path
	wildcards := 0
	for pos := 0; pos < len(value); {
		start := pos
		for pos < len(value) && isPathChar(value[pos]) {
			pos++
		}
		if start == pos {
			return Path{}, fmt.Errorf("invalid segment at byte %d", pos)
		}
		segment := pathSegment{name: value[start:pos]}

		if pos < len(value) && value[pos] == '[' {
			wildcardPrefix := value[:pos]
			pos++
			if pos < len(value) && value[pos] == '*' {
				segment.wildcard = true
				path.wildcardPrefix = wildcardPrefix
				wildcards++
				pos++
			} else {
				indexStart := pos
				for pos < len(value) && value[pos] >= '0' && value[pos] <= '9' {
					pos++
				}
				if indexStart == pos {
					return Path{}, fmt.Errorf("invalid index at byte %d", indexStart)
				}
				index, err := strconv.Atoi(value[indexStart:pos])
				if err != nil {
					return Path{}, fmt.Errorf("invalid index %q", value[indexStart:pos])
				}
				segment.index, segment.hasIndex = index, true
			}
			if pos >= len(value) || value[pos] != ']' {
				return Path{}, fmt.Errorf("unclosed index at byte %d", start)
			}
			pos++
		}
		if segment.wildcard && wildcards > 1 {
			return Path{}, fmt.Errorf("path has more than one wildcard")
		}
		path.segments = append(path.segments, segment)

		if pos == len(value) {
			return path, nil
		}
		if value[pos] != '.' {
			return Path{}, fmt.Errorf("expected dot at byte %d", pos)
		}
		pos++
	}
	return Path{}, fmt.Errorf("path is empty")
}

// Resolve returns every value addressed by the path, or an empty slice if no
// value matches. Paths without indexes preserve legacy flat-path behavior.
func (p Path) Resolve(root any) []any {
	if !p.hasSelector() {
		return p.resolveLegacy(root)
	}

	values := []any{root}
	for _, segment := range p.segments {
		var next []any
		for _, value := range values {
			object, ok := value.(map[string]any)
			if !ok {
				continue
			}
			value, ok = object[segment.name]
			if !ok {
				continue
			}
			switch {
			case segment.wildcard:
				if list, ok := value.([]any); ok {
					next = append(next, list...)
				}
			case segment.hasIndex:
				if list, ok := value.([]any); ok && segment.index < len(list) {
					next = append(next, list[segment.index])
				}
			default:
				next = append(next, value)
			}
		}
		values = next
		if len(values) == 0 {
			return nil
		}
	}
	return values
}

// ResolveIndexed returns wildcard-expanded values paired with the index of the
// element selected by [*]. It returns an empty slice for paths without [*].
func (p Path) ResolveIndexed(root any) []IndexedValue {
	if p.wildcardPrefix == "" {
		return nil
	}

	values := []IndexedValue{{Index: -1, Value: root}}
	for _, segment := range p.segments {
		var next []IndexedValue
		for _, indexed := range values {
			object, ok := indexed.Value.(map[string]any)
			if !ok {
				continue
			}
			value, ok := object[segment.name]
			if !ok {
				continue
			}
			switch {
			case segment.wildcard:
				if list, ok := value.([]any); ok {
					for index, item := range list {
						next = append(next, IndexedValue{Index: index, Value: item})
					}
				}
			case segment.hasIndex:
				if list, ok := value.([]any); ok && segment.index < len(list) {
					next = append(next, IndexedValue{Index: indexed.Index, Value: list[segment.index]})
				}
			default:
				next = append(next, IndexedValue{Index: indexed.Index, Value: value})
			}
		}
		values = next
		if len(values) == 0 {
			return nil
		}
	}
	return values
}

func (p Path) hasSelector() bool {
	for _, segment := range p.segments {
		if segment.hasIndex || segment.wildcard {
			return true
		}
	}
	return false
}

func (p Path) resolveLegacy(root any) []any {
	data, ok := root.(map[string]any)
	if !ok {
		return nil
	}
	for _, segment := range p.segments {
		value, ok := data[segment.name]
		if !ok {
			return nil
		}
		if nested, ok := value.(map[string]any); ok {
			data = nested
			continue
		}
		return []any{value}
	}
	return nil
}

func isPathChar(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '_' || value == '-'
}

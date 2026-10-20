// Package safetext validates untrusted text before it becomes prompt content.
package safetext

import "strings"

const invalidAddress = "<invalid-address>"

// SafeAddress returns a Terraform resource address only when it uses the
// supported address grammar. Invalid string keys are replaced independently.
func SafeAddress(addr string) string {
	if addr == "" || len(addr) > 256 {
		return invalidAddress
	}

	var out strings.Builder
	for pos := 0; pos < len(addr); {
		end := pos
		for end < len(addr) && isIdentifierChar(addr[end], end == pos) {
			end++
		}
		if end == pos {
			return invalidAddress
		}
		out.WriteString(addr[pos:end])
		pos = end

		if pos < len(addr) && addr[pos] == '[' {
			index, end, ok := safeIndex(addr[pos:])
			if !ok {
				return invalidAddress
			}
			out.WriteString(index)
			pos += end
		}
		if pos == len(addr) {
			return out.String()
		}
		if addr[pos] != '.' {
			return invalidAddress
		}
		out.WriteByte('.')
		pos++
	}
	return invalidAddress
}

// SafeSegment validates Terraform resource types and statuses used in prompts.
func SafeSegment(value string) string {
	if value == "" {
		return "<invalid>"
	}
	for i := range len(value) {
		if !isIdentifierChar(value[i], i == 0) {
			return "<invalid>"
		}
	}
	return value
}

func isIdentifierChar(b byte, first bool) bool {
	if b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b == '_' {
		return true
	}
	return !first && (b >= '0' && b <= '9' || b == '-')
}

func safeIndex(value string) (string, int, bool) {
	if len(value) < 3 || value[0] != '[' {
		return "", 0, false
	}
	if value[1] != '"' {
		end := strings.IndexByte(value, ']')
		if end < 2 {
			return "", 0, false
		}
		for _, b := range value[1:end] {
			if b < '0' || b > '9' {
				return "", 0, false
			}
		}
		return value[:end+1], end + 1, true
	}

	keyStart := 2
	i := keyStart
	valid := true
	for i < len(value) {
		if value[i] == '\\' {
			valid = false
			i += 2
			continue
		}
		if value[i] == '"' {
			break
		}
		if !isKeyChar(value[i]) {
			valid = false
		}
		i++
	}
	if i >= len(value) || i+1 >= len(value) || value[i] != '"' || value[i+1] != ']' {
		return "", 0, false
	}
	if i-keyStart > 64 {
		valid = false
	}
	if !valid {
		return `["<redacted-key>"]`, i + 2, true
	}
	return value[:i+2], i + 2, true
}

func isKeyChar(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || strings.ContainsRune("_./:@-", rune(b))
}

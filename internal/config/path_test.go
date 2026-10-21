package config

import "testing"

func TestParsePath(t *testing.T) {
	for _, tc := range []struct {
		path  string
		valid bool
	}{
		{"ingress[*].cidr_blocks", true},
		{"ingress[0].from_port", true},
		{"tags.environment", true},
		{"a-b_1[12].c", true},
		{"", false},
		{".ingress", false},
		{"ingress.", false},
		{"ingress..cidr_blocks", false},
		{"ingress[-1]", false},
		{"ingress[", false},
		{"ingress[0][1]", false},
		{"ingress[*].cidr_blocks[*]", false},
		{"ingress[nope]", false},
		{"ingress!.cidr_blocks", false},
	} {
		t.Run(tc.path, func(t *testing.T) {
			_, err := ParsePath(tc.path)
			if (err == nil) != tc.valid {
				t.Errorf("ParsePath(%q) error = %v, valid = %t", tc.path, err, tc.valid)
			}
		})
	}
}

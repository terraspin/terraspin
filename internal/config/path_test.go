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

func TestPathResolveIndexed(t *testing.T) {
	root := map[string]any{"ingress": []any{
		map[string]any{"from_port": float64(443)},
		map[string]any{"from_port": float64(22)},
	}}
	path, err := ParsePath("ingress[*].from_port")
	if err != nil {
		t.Fatal(err)
	}
	got := path.ResolveIndexed(root)
	if len(got) != 2 || got[0].Index != 0 || got[0].Value != float64(443) || got[1].Index != 1 || got[1].Value != float64(22) {
		t.Fatalf("ResolveIndexed() = %#v", got)
	}
}

func TestPathResolve(t *testing.T) {
	root := map[string]any{
		"ingress": []any{
			map[string]any{"cidr_blocks": []any{"10.0.0.0/8", "0.0.0.0/0"}, "from_port": float64(22)},
			map[string]any{"cidr_blocks": []any{"192.168.0.0/16"}, "from_port": float64(80)},
		},
		"protocol": "tcp",
	}
	for _, tc := range []struct {
		path string
		want []any
	}{
		{"ingress[0].from_port", []any{float64(22)}},
		{"ingress[*].cidr_blocks", []any{[]any{"10.0.0.0/8", "0.0.0.0/0"}, []any{"192.168.0.0/16"}}},
		{"ingress[2].from_port", nil},
		{"protocol[*]", nil},
	} {
		t.Run(tc.path, func(t *testing.T) {
			path, err := ParsePath(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			got := path.Resolve(root)
			if len(got) != len(tc.want) {
				t.Fatalf("Resolve() = %#v, want %#v", got, tc.want)
			}
			for i := range got {
				if !valuesEqual(got[i], tc.want[i]) {
					t.Errorf("Resolve()[%d] = %#v, want %#v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

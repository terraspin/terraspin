package main

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update CLI golden files")

func TestCLIGolden(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "terraspin")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = filepath.Join(repoRoot, "cmd", "terraspin")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	fixtures, err := filepath.Glob(filepath.Join(repoRoot, "testdata", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		name := filepath.Base(fixture)
		for _, format := range []string{"text", "json", "markdown"} {
			t.Run("analyze/"+name+"/"+format, func(t *testing.T) {
				goldenOutput(t, repoRoot, filepath.Join("analyze", name+"."+format), runGoldenCLI(t, binary, "--no-ai", "--format", format, fixture))
			})
		}
	}

	for _, format := range []string{"text", "json", "markdown"} {
		t.Run("diff/sensitive-collections/"+format, func(t *testing.T) {
			goldenOutput(t, repoRoot, filepath.Join("diff", "sensitive-collections."+format), runGoldenCLI(t, binary,
				"diff", "--no-ai", "--format", format,
				filepath.Join(repoRoot, "testdata", "plan_with_sensitive_collections_a.json"),
				filepath.Join(repoRoot, "testdata", "plan_with_sensitive_collections_b.json"),
			))
		})
	}
}

func runGoldenCLI(t *testing.T, binary string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "ANTHROPIC_API_KEY=", "OPENAI_API_KEY=")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run CLI: %v\n%s", err, output)
	}
	return output
}

func goldenOutput(t *testing.T, repoRoot, name string, got []byte) {
	t.Helper()
	got = []byte(strings.TrimRight(string(got), "\n") + "\n")
	path := filepath.Join(repoRoot, "testdata", "golden", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run go test ./cmd/terraspin -run TestCLIGolden -update)", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden %s differs\nwant:\n%s\ngot:\n%s", name, strings.TrimSpace(string(want)), strings.TrimSpace(string(got)))
	}
}

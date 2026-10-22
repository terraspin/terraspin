package config

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"testing"
)

func TestAWSRulesDocumentationMatchesBuiltinRules(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, "docs", "aws-rules.md"))
	if err != nil {
		t.Fatal(err)
	}
	matches := regexp.MustCompile(`(?m)^\| (AWS-[A-Z]+-\d{3}) \|`).FindAllStringSubmatch(string(data), -1)
	documented := make([]string, 0, len(matches))
	for _, match := range matches {
		documented = append(documented, match[1])
	}
	sort.Strings(documented)

	rules, err := builtinRules()
	if err != nil {
		t.Fatal(err)
	}
	builtins := make([]string, 0, len(rules))
	for _, rule := range rules {
		builtins = append(builtins, rule.ID)
	}
	sort.Strings(builtins)
	if !reflect.DeepEqual(documented, builtins) {
		t.Fatalf("documented IDs=%v, built-in IDs=%v", documented, builtins)
	}
}

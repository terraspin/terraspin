package formatter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/terraspin/terraspin/internal/ai"
	"github.com/terraspin/terraspin/internal/analyzer"
	"github.com/terraspin/terraspin/internal/parser"
)

func TestFormatText_showsNarrative(t *testing.T) {
	ast := &parser.PlanAST{TerraformVersion: "1.0"}
	score := &analyzer.PlanScore{
		Overall: analyzer.RiskScore{Score: 50, Tier: analyzer.TierMedium},
		Counts:  map[analyzer.RiskTier]int{analyzer.TierMedium: 1},
		ResourceScores: []analyzer.ResourceRiskScore{
			{Address: "x", Score: 50, Tier: analyzer.TierMedium},
		},
	}
	narr := &ai.Narrative{
		Summary:          "Plan summary text",
		RiskAssessment:   "This is risky because...",
		Recommendations:  []string{"Check RDS snapshot", "Verify backup"},
		RollbackStrategy: "Restore from state backup",
	}
	output := FormatText(ast, score, nil, narr, false)

	if !strings.Contains(output, narr.Summary) {
		t.Error("missing summary")
	}
	if !strings.Contains(output, narr.RiskAssessment) {
		t.Error("missing risk assessment")
	}
	if !strings.Contains(output, narr.Recommendations[0]) || !strings.Contains(output, narr.Recommendations[1]) {
		t.Error("missing recommendations")
	}
	if !strings.Contains(output, narr.RollbackStrategy) {
		t.Error("missing rollback strategy")
	}
}

func TestFormatText_verboseShowsMediumLow(t *testing.T) {
	ast := &parser.PlanAST{TerraformVersion: "1.0", Changes: []parser.ResourceChange{
		{Address: "low-resource", Type: "a", Action: parser.ActionUpdate},
		{Address: "med-resource", Type: "b", Action: parser.ActionUpdate},
	}}
	score := &analyzer.PlanScore{
		Overall: analyzer.RiskScore{Score: 40, Tier: analyzer.TierMedium},
		Counts:  map[analyzer.RiskTier]int{analyzer.TierMedium: 1, analyzer.TierLow: 1},
		ResourceScores: []analyzer.ResourceRiskScore{
			{Address: "med-resource", Score: 40, Tier: analyzer.TierMedium},
			{Address: "low-resource", Score: 20, Tier: analyzer.TierLow},
		},
	}

	normal := FormatText(ast, score, nil, nil, false)
	verbose := FormatText(ast, score, nil, nil, true)

	if strings.Contains(normal, "Medium & Low") {
		t.Error("medium/low section visible without -v")
	}
	if !strings.Contains(verbose, "Medium & Low") {
		t.Error("medium/low section missing with -v")
	}
	if !strings.Contains(verbose, "low-resource") || !strings.Contains(verbose, "med-resource") {
		t.Error("verbose output missing resource addresses")
	}
}

func TestFormatText_showsBlastRadiusForCritical(t *testing.T) {
	ast := &parser.PlanAST{TerraformVersion: "1.0", Changes: []parser.ResourceChange{
		{Address: "aws_db_instance.p", Type: "aws_db_instance", Action: parser.ActionDelete},
	}}
	score := &analyzer.PlanScore{
		Overall: analyzer.RiskScore{Score: 90, Tier: analyzer.TierCritical},
		Counts:  map[analyzer.RiskTier]int{analyzer.TierCritical: 1},
		ResourceScores: []analyzer.ResourceRiskScore{
			{Address: "aws_db_instance.p", Score: 90, Tier: analyzer.TierCritical},
		},
	}
	blast := map[string]*analyzer.BlastRadius{
		"aws_db_instance.p": {
			RootAddress:   "aws_db_instance.p",
			DirectDeps:    []analyzer.DependentResource{{Address: "aws_lambda.api", Type: "aws_lambda_function"}},
			TotalAffected: 1,
		},
	}
	output := FormatText(ast, score, blast, nil, false)

	if !strings.Contains(output, "Blast radius") {
		t.Error("missing blast radius section")
	}
	if !strings.Contains(output, "aws_lambda.api") {
		t.Error("missing dependent in blast radius")
	}
}

func TestFormatJSON_validStructure(t *testing.T) {
	ast := &parser.PlanAST{TerraformVersion: "1.7.2", Changes: []parser.ResourceChange{
		{Address: "a", Type: "aws_s3_bucket", Action: parser.ActionCreate},
	}}
	score := &analyzer.PlanScore{
		Overall: analyzer.RiskScore{Score: 10, Tier: analyzer.TierLow},
		Counts:  map[analyzer.RiskTier]int{analyzer.TierLow: 1},
		ResourceScores: []analyzer.ResourceRiskScore{
			{Address: "a", Score: 10, Tier: analyzer.TierLow},
		},
	}
	output, err := FormatJSON(ast, score, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid([]byte(output)) {
		t.Fatal("output is not valid JSON")
	}
	var parsed struct {
		TerraformVersion string           `json:"terraform_version"`
		OverallRisk      analyzer.RiskScore `json:"overall_risk"`
		ResourceRisks    []json.RawMessage `json:"resource_risks"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.TerraformVersion != "1.7.2" {
		t.Errorf("version = %q", parsed.TerraformVersion)
	}
	if parsed.OverallRisk.Score != 10 {
		t.Errorf("score = %.0f", parsed.OverallRisk.Score)
	}
	if len(parsed.ResourceRisks) != 1 {
		t.Errorf("resource_risks = %d", len(parsed.ResourceRisks))
	}
}

func TestFormatJSON_includesNarrative(t *testing.T) {
	ast := &parser.PlanAST{TerraformVersion: "1.0"}
	score := &analyzer.PlanScore{
		Overall: analyzer.RiskScore{Score: 10, Tier: analyzer.TierLow},
		Counts:  map[analyzer.RiskTier]int{},
	}
	narr := &ai.Narrative{Summary: "test narrative"}
	output, err := FormatJSON(ast, score, nil, narr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, `test narrative`) {
		t.Error("narrative not in JSON output")
	}
}

func TestFormatMarkdown_showsActionTable(t *testing.T) {
	ast := &parser.PlanAST{
		TerraformVersion: "1.7.2",
		Changes: []parser.ResourceChange{
			{Address: "a", Type: "aws_s3_bucket", Action: parser.ActionCreate},
			{Address: "b", Type: "aws_s3_bucket", Action: parser.ActionDelete},
		},
	}
	score := &analyzer.PlanScore{
		Overall: analyzer.RiskScore{Score: 75, Tier: analyzer.TierHigh},
		Counts:  map[analyzer.RiskTier]int{analyzer.TierHigh: 1},
		ResourceScores: []analyzer.ResourceRiskScore{
			{Address: "b", Score: 75, Tier: analyzer.TierHigh},
		},
	}
	narr := &ai.Narrative{
		Summary:          "markdown summary",
		RiskAssessment:   "markdown assessment",
		Recommendations:  []string{"action one"},
		CriticalChanges:  []string{"delete of b is critical"},
		RollbackStrategy: "restore from snapshot",
	}
	output := FormatMarkdown(ast, score, nil, narr)

	if !strings.HasPrefix(output, "## 🌀") {
		t.Error("missing header")
	}
	if !strings.Contains(output, "1.7.2") {
		t.Error("missing terraform version")
	}
	if !strings.Contains(output, "| create") || !strings.Contains(output, "| delete") {
		t.Error("missing action table")
	}
	if !strings.Contains(output, "markdown summary") {
		t.Error("missing narrative text")
	}
	if !strings.Contains(output, "Critical Changes") {
		t.Error("missing critical changes section")
	}
	if !strings.Contains(output, "| critical") {
		t.Error("missing risk breakdown table")
	}
}

func TestFormatText_showsHeaderAndRisk(t *testing.T) {
	ast := &parser.PlanAST{
		TerraformVersion: "1.7.2",
		Changes: []parser.ResourceChange{
			{Address: "a", Type: "aws_s3_bucket", Action: parser.ActionCreate},
			{Address: "b", Type: "aws_s3_bucket", Action: parser.ActionDelete},
		},
	}
	score := &analyzer.PlanScore{
		Overall: analyzer.RiskScore{Score: 85, Tier: analyzer.TierCritical},
		Counts:  map[analyzer.RiskTier]int{analyzer.TierCritical: 1},
		ResourceScores: []analyzer.ResourceRiskScore{
			{Address: "b", Action: parser.ActionDelete, Score: 85, Tier: analyzer.TierCritical},
		},
	}
	output := FormatText(ast, score, nil, nil, false)

	if !strings.Contains(output, "Terraspin Plan Analysis") {
		t.Error("missing header")
	}
	if !strings.Contains(output, "1.7.2") {
		t.Error("missing terraform version")
	}
	if !strings.Contains(output, "CRITICAL") {
		t.Error("missing risk badge")
	}
	if !strings.Contains(output, "1 to create") || !strings.Contains(output, "1 to delete") {
		t.Error("missing resource action counts")
	}
}

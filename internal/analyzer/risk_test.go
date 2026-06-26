package analyzer

import (
	"testing"

	"github.com/terraspin/terraspin/internal/parser"
)

func TestScorePlan_basic(t *testing.T) {
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{Address: "aws_s3_bucket.logs", Type: "aws_s3_bucket", Action: parser.ActionCreate},
			{Address: "aws_db_instance.primary", Type: "aws_db_instance", Action: parser.ActionDelete},
			{Address: "aws_security_group.web", Type: "aws_security_group", Action: parser.ActionUpdate},
		},
	}
	ps := ScorePlan(ast)
	if ps.Overall.Tier != TierCritical {
		t.Errorf("overall tier = %s, want critical", ps.Overall.Tier)
	}
	if len(ps.ResourceScores) != 3 {
		t.Errorf("got %d resource scores, want 3", len(ps.ResourceScores))
	}
	// db delete should be highest
	if ps.ResourceScores[1].Address != "aws_db_instance.primary" || ps.ResourceScores[1].Tier != TierCritical {
		t.Errorf("db score = %.1f / %s", ps.ResourceScores[1].Score, ps.ResourceScores[1].Tier)
	}
}

func TestScorePlan_skipsNoop(t *testing.T) {
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{Address: "a", Type: "aws_s3_bucket", Action: parser.ActionNoOp},
			{Address: "b", Type: "aws_s3_bucket", Action: parser.ActionRead},
		},
	}
	ps := ScorePlan(ast)
	if len(ps.ResourceScores) != 0 {
		t.Errorf("got %d scores, want 0", len(ps.ResourceScores))
	}
}

func TestScorePlan_forceDestroy(t *testing.T) {
	ast := &parser.PlanAST{
		Changes: []parser.ResourceChange{
			{
				Address:      "aws_s3_bucket.data",
				Type:         "aws_s3_bucket",
				Action:       parser.ActionUpdate,
				ForceReplace: true,
			},
		},
	}
	ps := ScorePlan(ast)
	// update with force_destroy → 2.5× not 1.8×
	if ps.ResourceScores[0].Score != 50 {
		t.Errorf("score = %.1f, want 50.0", ps.ResourceScores[0].Score)
	}
}

func TestTierFromName(t *testing.T) {
	tests := []struct {
		name string
		want RiskTier
	}{
		{"critical", TierCritical},
		{"high", TierHigh},
		{"medium", TierMedium},
		{"low", TierLow},
		{"unknown", ""},
	}
	for _, tt := range tests {
		got := TierFromName(tt.name)
		if got != tt.want {
			t.Errorf("TierFromName(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestTierWeight(t *testing.T) {
	if TierWeight(TierCritical) != 4 {
		t.Error("critical weight != 4")
	}
	if TierWeight(TierLow) != 1 {
		t.Error("low weight != 1")
	}
	if TierWeight("") != 0 {
		t.Error("empty tier weight != 0")
	}
}

func TestApplyCustomRules_noMatches(t *testing.T) {
	ps := &PlanScore{
		ResourceScores: []ResourceRiskScore{
			{Address: "a", Tier: TierMedium, Score: 40},
		},
		Overall: RiskScore{Score: 40, Tier: TierMedium},
		Counts:  map[RiskTier]int{TierMedium: 1},
	}
	ApplyCustomRules(ps, nil)
	if ps.Overall.Tier != TierMedium {
		t.Errorf("tier changed to %s", ps.Overall.Tier)
	}
}

func TestApplyCustomRules_escalates(t *testing.T) {
	ps := &PlanScore{
		ResourceScores: []ResourceRiskScore{
			{Address: "aws_s3_bucket.logs", Tier: TierLow, Score: 20, Action: parser.ActionUpdate},
			{Address: "aws_db_instance.db", Tier: TierMedium, Score: 40, Action: parser.ActionUpdate},
		},
		Overall: RiskScore{Score: 40, Tier: TierMedium},
		Counts:  map[RiskTier]int{TierLow: 1, TierMedium: 1},
	}
	matches := []ConfigRuleMatch{
		{Address: "aws_s3_bucket.logs", Severity: "critical"},
	}
	ApplyCustomRules(ps, matches)
	if ps.ResourceScores[0].Tier != TierCritical {
		t.Errorf("expected critical, got %s", ps.ResourceScores[0].Tier)
	}
	if ps.ResourceScores[0].Score < 85 {
		t.Errorf("score not clamped: %.1f", ps.ResourceScores[0].Score)
	}
	// Overall should be critical now
	if ps.Overall.Tier != TierCritical {
		t.Errorf("overall = %s, want critical", ps.Overall.Tier)
	}
	if ps.Counts[TierCritical] != 1 {
		t.Errorf("critical count = %d, want 1", ps.Counts[TierCritical])
	}
}

func TestApplyCustomRules_noDowngrade(t *testing.T) {
	ps := &PlanScore{
		ResourceScores: []ResourceRiskScore{
			{Address: "critical-resource", Tier: TierCritical, Score: 92, Action: parser.ActionDelete},
		},
		Overall: RiskScore{Score: 92, Tier: TierCritical},
		Counts:  map[RiskTier]int{TierCritical: 1},
	}
	// Rule says "low" — should not downgrade
	matches := []ConfigRuleMatch{
		{Address: "critical-resource", Severity: "low"},
	}
	ApplyCustomRules(ps, matches)
	if ps.ResourceScores[0].Tier != TierCritical {
		t.Errorf("downgraded to %s", ps.ResourceScores[0].Tier)
	}
}

func TestApplyCustomRules_highestSeverityWins(t *testing.T) {
	ps := &PlanScore{
		ResourceScores: []ResourceRiskScore{
			{Address: "x", Tier: TierLow, Score: 10, Action: parser.ActionUpdate},
		},
		Overall: RiskScore{Score: 10, Tier: TierLow},
		Counts:  map[RiskTier]int{TierLow: 1},
	}
	matches := []ConfigRuleMatch{
		{Address: "x", Severity: "high"},
		{Address: "x", Severity: "critical"},
	}
	ApplyCustomRules(ps, matches)
	if ps.ResourceScores[0].Tier != TierCritical {
		t.Errorf("expected critical, got %s", ps.ResourceScores[0].Tier)
	}
}

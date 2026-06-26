package analyzer

import (
	"math"
	"strings"

	"github.com/terraspin/terraspin/internal/parser"
)

// RiskTier classifies a risk score.
type RiskTier string

const (
	TierCritical RiskTier = "critical"
	TierHigh     RiskTier = "high"
	TierMedium   RiskTier = "medium"
	TierLow      RiskTier = "low"
)

// RiskScore holds a scored risk assessment.
type RiskScore struct {
	Score float64  `json:"score"`
	Tier  RiskTier `json:"tier"`
}

// ResourceRiskScore holds per-resource risk.
type ResourceRiskScore struct {
	Address string     `json:"address"`
	Action  parser.ChangeAction `json:"action"`
	Score   float64    `json:"score"`
	Tier    RiskTier   `json:"tier"`
}

// PlanScore is the full scoring result for a plan.
type PlanScore struct {
	Overall        RiskScore           `json:"overall"`
	ResourceScores []ResourceRiskScore `json:"resource_scores"`
	Counts         map[RiskTier]int    `json:"counts"`
}

// baseScore returns the base risk score for a change action.
func baseScore(action parser.ChangeAction) float64 {
	switch action {
	case parser.ActionCreate:
		return 10
	case parser.ActionUpdate:
		return 20
	case parser.ActionDelete:
		return 75
	case parser.ActionReplace:
		return 90
	default:
		return 0
	}
}

type multiplierEntry struct {
	patterns   []string
	multiplier float64
}

// resourceMultipliers maps resource type prefixes to risk multipliers.
// ponytail: flat list, sorted by priority. Add patterns as new providers appear.
var resourceMultipliers = []multiplierEntry{
	{[]string{"aws_db_instance", "aws_rds_cluster", "aws_dynamodb_table", "google_sql_database_instance", "azurerm_mssql_database", "azurerm_cosmosdb_account"}, 3.0},
	{[]string{"aws_route53_record", "google_dns_record_set", "azurerm_dns_"}, 2.5},
	{[]string{"aws_iam_role", "aws_iam_policy", "google_project_iam_", "azurerm_role_assignment"}, 2.5},
	{[]string{"aws_vpc", "aws_subnet", "aws_network_acl", "aws_route_table", "google_compute_network", "azurerm_virtual_network"}, 2.5},
	{[]string{"aws_security_group", "aws_security_group_rule", "google_compute_firewall", "azurerm_network_security_group"}, 2.5},
	{[]string{"aws_lb", "aws_alb", "aws_elb", "google_compute_forwarding_rule", "azurerm_lb"}, 2.0},
	{[]string{"aws_ebs_volume", "google_compute_disk", "azurerm_managed_disk"}, 2.0},
	{[]string{"aws_s3_bucket", "google_storage_bucket", "azurerm_storage_account"}, 1.8},
	{[]string{"aws_cloudfront_distribution", "google_compute_backend_service", "azurerm_cdn_endpoint"}, 1.5},
	{[]string{"aws_instance", "google_compute_instance", "azurerm_virtual_machine"}, 1.5},
	{[]string{"aws_ecs_service", "aws_eks_cluster", "google_container_cluster", "azurerm_kubernetes_cluster"}, 1.4},
	{[]string{"kubernetes_"}, 1.4},
	{[]string{"aws_lambda_function", "google_cloudfunctions_function", "azurerm_function_app"}, 1.2},
}

// resourceMultiplier returns the risk multiplier for a resource type.
func resourceMultiplier(resourceType string) float64 {
	for _, e := range resourceMultipliers {
		for _, p := range e.patterns {
			if strings.HasPrefix(resourceType, p) {
				return e.multiplier
			}
		}
	}
	return 1.0
}

// tierFromScore maps a score to its risk tier.
// tierFromScore maps a score to its risk tier.
func tierFromScore(score float64) RiskTier {
	switch {
	case score >= 85:
		return TierCritical
	case score >= 60:
		return TierHigh
	case score >= 30:
		return TierMedium
	default:
		return TierLow
	}
}

// TierFromName converts a severity string to a RiskTier.
func TierFromName(name string) RiskTier {
	switch name {
	case "critical":
		return TierCritical
	case "high":
		return TierHigh
	case "medium":
		return TierMedium
	case "low":
		return TierLow
	default:
		return ""
	}
}

// TierWeight returns a numeric weight for comparison (higher = more severe).
func TierWeight(t RiskTier) int {
	switch t {
	case TierCritical:
		return 4
	case TierHigh:
		return 3
	case TierMedium:
		return 2
	case TierLow:
		return 1
	default:
		return 0
	}
}

// ScorePlan scores every resource change in a plan and returns the overall score.
func ScorePlan(ast *parser.PlanAST) *PlanScore {
	scores := make([]ResourceRiskScore, 0, len(ast.Changes))
	counts := make(map[RiskTier]int)
	var topScore float64
	for _, rc := range ast.Changes {
		base := baseScore(rc.Action)
		if base == 0 {
			continue
		}
		mult := resourceMultiplier(rc.Type)
		// ponytail: aws_s3_bucket with force_destroy=true → 2.5× not 1.8×
		if rc.Type == "aws_s3_bucket" && rc.ForceReplace {
			mult = 2.5
		}
		score := math.Min(100, base*mult)
		s := ResourceRiskScore{
			Address: rc.Address,
			Action:  rc.Action,
			Score:   math.Round(score*10) / 10,
			Tier:    tierFromScore(score),
		}
		scores = append(scores, s)
		counts[s.Tier]++
		if s.Score > topScore {
			topScore = s.Score
		}
	}
	return &PlanScore{
		Overall:        RiskScore{Score: topScore, Tier: tierFromScore(topScore)},
		ResourceScores: scores,
		Counts:         counts,
	}
}

// ConfigRuleMatch is a minimal match result from custom rules.
type ConfigRuleMatch struct {
	Address  string
	Severity string
}

// ApplyCustomRules escalates resource tiers where a matched rule severity exceeds
// the computed risk tier. Modifies ps in-place.
func ApplyCustomRules(ps *PlanScore, ruleMatches []ConfigRuleMatch) {
	if len(ruleMatches) == 0 {
		return
	}
	// Build map: address → highest severity from matched rules
	addrEscalation := make(map[string]RiskTier)
	for _, m := range ruleMatches {
		rt := TierFromName(m.Severity)
		if rt == "" {
			continue
		}
		if existing, ok := addrEscalation[m.Address]; !ok || TierWeight(rt) > TierWeight(existing) {
			addrEscalation[m.Address] = rt
		}
	}

	// Apply escalations
	for i := range ps.ResourceScores {
		rs := &ps.ResourceScores[i]
		if override, ok := addrEscalation[rs.Address]; ok && TierWeight(override) > TierWeight(rs.Tier) {
			rs.Tier = override
			switch override {
			case TierCritical:
				if rs.Score < 85 {
					rs.Score = 85
				}
			case TierHigh:
				if rs.Score < 60 {
					rs.Score = 60
				}
			case TierMedium:
				if rs.Score < 30 {
					rs.Score = 30
				}
			}
		}
	}

	// Recalculate counts and overall
	counts := make(map[RiskTier]int)
	var topScore float64
	var topTier RiskTier
	for _, rs := range ps.ResourceScores {
		counts[rs.Tier]++
		if rs.Score > topScore || (rs.Score == topScore && TierWeight(rs.Tier) > TierWeight(topTier)) {
			topScore = rs.Score
			topTier = rs.Tier
		}
	}
	ps.Counts = counts
	ps.Overall = RiskScore{Score: topScore, Tier: topTier}
}

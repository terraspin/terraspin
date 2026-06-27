# Architecture

## Pipeline

```
plan.json → ParsePlan() → RedactSensitive() → ScorePlan() → EvaluateRules()
                                                      ↓
                                          AnalyzeBlastRadius()
                                                      ↓
                                          BuildAnalysisPrompt() → QueryLLM()
                                                      ↓
                                          FormatText/JSON/Markdown → output
```

Each step is a self-contained package with zero cross-package coupling. `main()` orchestrates the pipeline linearly — no middleware, no DI container, no factory. Read the order of imports and function calls in `cmd/terraspin/main.go` to understand the full flow.

## Risk scoring

**Deterministic 0–100 score** per resource change. Computed from:

1. **Base score** by action type: read/nop=0, create=10, update=20, delete=75, replace=90
2. **Resource multiplier** by type prefix — databases 3×, DNS/IAM 2.5×, compute 1.5×, etc.
3. **Special cases** — e.g., `aws_s3_bucket` with `force_destroy=true` escalates from 1.8× to 2.5×

Final score is `min(100, base × multiplier)`. Tiers: critical (≥85), high (≥60), medium (≥30), low (<30).

Custom rules in `.terraspin.yml` can escalate a resource's tier if the rule severity exceeds the computed tier. They never downgrade.

## Blast radius

Builds a dependency graph from terraform plan JSON references. For each resource, traverses direct dependencies and reports the total affected count and the dependency tree. Used to highlight cascading impact: "deleting the RDS instance → Lambda function loses DB connectivity".

## AI narrative

A structured prompt containing the plan summary (resource counts, tiers, actions) is sent to the LLM. The response is expected as JSON with these keys:

- `summary` — 2–3 sentence plain-English summary
- `critical_changes` — business-impact descriptions for each critical/high change
- `risk_assessment` — why this plan is risky or safe
- `recommendations` — specific actionable checks (not "review changes")
- `rollback_strategy` — numbered recovery steps

If the LLM call fails or `--no-ai` is set, a rule-based narrative is generated from the scored data.

## Sensitive value redaction

Values marked `sensitive` in the terraform plan are redacted from the data **before** any LLM call. The LLM never sees sensitive values.

## Internal packages

| Package           | Responsibility                                               |
| ----------------- | ------------------------------------------------------------ |
| `parser`          | Terraform plan JSON → `PlanAST` struct tree                  |
| `ai`              | LLM providers (Claude, OpenAI, Ollama), prompt building, narrative types, sensitive redaction |
| `analyzer`        | Risk scoring algorithm, blast radius dependency graph        |
| `config`          | `.terraspin.yml` loading, validation, rule evaluation        |
| `formatter`       | Output formatting: terminal (with color), JSON, markdown     |
| `integrations`    | GitHub PR comments, GitLab MR notes, Slack webhooks, MCP server |
| `diff`            | Two-plan comparison: resource diffs (added/modified/removed) |

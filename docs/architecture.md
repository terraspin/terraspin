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

`main()` orchestrates the analyze pipeline linearly. `diff` has its own two-plan comparison path in `cmd/terraspin/diff.go`.

## Risk scoring

**Deterministic 0–100 score** per resource change. Computed from:

1. **Base score** by action type: read/nop=0, create=10, update=20, delete=75, replace=90
2. **Resource multiplier** by type prefix — databases 3×, DNS/IAM 2.5×, compute 1.5×, etc.
3. **Special cases** — e.g., `aws_s3_bucket` with `force_destroy=true` escalates from 1.8× to 2.5×

Final score is `min(100, base × multiplier)`. Tiers: critical (≥85), high (≥60), medium (≥30), low (<30).

Custom rules in `.terraspin.yml` can escalate a resource's tier if the rule severity exceeds the computed tier. They never downgrade.

## Blast radius

Builds a dependency graph from expression references in the Terraform root module. For each resource, it traverses reverse dependencies and reports affected-resource counts. It excludes `module.*` references, data sources, and `depends_on` relationships.

## AI narrative

A structured prompt containing the plan summary (resource counts, tiers, actions) is sent to the LLM. The response is expected as JSON with these keys:

- `summary` — 2–3 sentence plain-English summary
- `critical_changes` — business-impact descriptions for each critical/high change
- `risk_assessment` — why this plan is risky or safe
- `recommendations` — specific actionable checks (not "review changes")
- `rollback_strategy` — numbered recovery steps

Analyze uses a rule-based narrative when no LLM is enabled, `--no-ai` is set, or its LLM call fails. Diff prints its structural result without an AI narrative when its LLM call fails.

## Sensitive value redaction

Values Terraform marks sensitive are redacted before LLM calls. The current prompts contain summary metadata rather than raw `Before`/`After` values; unmarked values can still appear in local `diff` output.

## Internal packages

| Package           | Responsibility                                               |
| ----------------- | ------------------------------------------------------------ |
| `parser`          | Terraform plan JSON → `PlanAST` struct tree                  |
| `ai`              | LLM providers (Claude, OpenAI, Ollama), prompt building, narrative types, sensitive redaction |
| `analyzer`        | Risk scoring algorithm, blast radius dependency graph        |
| `config`          | `.terraspin.yml` loading, validation, rule evaluation        |
| `formatter`       | Output formatting: terminal text, JSON, markdown             |
| `integrations`    | GitHub PR comments, GitLab MR notes, Slack webhooks, MCP server |
| `diff`            | Two-plan comparison: resource diffs (added/modified/removed) |

# 🌀 Terraspin

**Understand your infrastructure changes before you apply them.**

Terraspin is an open-source CLI tool and AI agent that turns `terraform plan` output into risk-scored, human-readable analysis. It tells you not just _what_ will change, but _what it means_, _what could go wrong_, and _how to recover_ if apply fails.

```
$ terraform show -json plan.tfplan | terraspin

┌─ Terraspin Plan Analysis ──────────────────────────────────┐
│  47 resources  ·  terraform 1.7.2                          │
└────────────────────────────────────────────────────────────┘

  Overall Risk  CRITICAL (score: 92.5)

  8 to create  ·  12 to update  ·  3 to delete  ·  0 to replace

──── Critical & High Risk Changes ────────────────────────────

  [CRITICAL 92.5]  aws_db_instance.primary  →  DELETE
  Blast radius: 6 resources affected
    ├── aws_lambda_function.api  connection string will break
    ├── aws_security_group.db    will be deleted as dependent
    └── module.app.aws_ssm_parameter.db_url  will be deleted

  Summary
  This plan permanently deletes the primary production RDS
  instance. There is no replacement instance — this is a
  permanent deletion. The Lambda functions serving production
  traffic will immediately lose database connectivity.
```

## Features

- **Risk scoring** — deterministic 0–100 score per resource and overall tier (critical/high/medium/low)
- **Blast radius analysis** — dependency graph traversal to find cascading impact
- **AI narrative** — plain-English risk briefing via Claude, OpenAI, or local Ollama
- **Custom rules** — organization-specific risk policies in `.terraspin.yml`
- **CI/CD integration** — GitHub Action, PR comments, `--fail-on` gate
- **MCP server** — run as a Model Context Protocol server for AI coding assistants (Claude Code, Copilot, Cursor)
- **Multiple formats** — colored terminal, JSON, markdown, compact one-line
- **Sensitive value redaction** — values marked `sensitive` in plan are redacted before any LLM call
- **Slack notifications** — webhook alerts on critical/high risk

## Install

```bash
# Homebrew
brew install terraspin/tap/terraspin

# Go install
go install github.com/terraspin/terraspin/cmd/terraspin@latest

# Binary releases
# Download from https://github.com/terraspin/terraspin/releases
```

## Quick start

```bash
# Analyze a plan file
terraspin plan.json

# Pipe from terraform
terraform plan -json | terraspin

# Fail CI if risk is high or above
terraspin plan.json --fail-on high

# Rule-based only (no LLM API call)
terraspin plan.json --no-ai

# JSON output for scripting
terraspin plan.json --format json
```

## Usage

```
terraspin [flags] <plan.json>
       terraform plan -json | terraspin [flags]
terraspin serve [flags]
terraspin diff <state-a> <state-b> [flags]
terraspin version
```

| Flag             | Description                                                 |
| ---------------- | ----------------------------------------------------------- |
| `--format`       | Output format: `text` (default), `json`, `markdown`         |
| `--fail-on`      | Exit 1 if risk >= tier: `critical`, `high`, `medium`, `low` |
| `--no-ai`        | Rule-based only, skip LLM call                              |
| `--llm`          | LLM provider: `claude` (default), `openai`, `ollama`        |
| `--model`        | Model name (provider default if empty)                      |
| `--ollama-host`  | Ollama host URL (default `http://localhost:11434`)          |
| `-v`             | Show all risk tiers including medium and low                |
| `--config`       | Config file path (default `.terraspin.yml`)                 |
| `--post-comment` | Post analysis as GitHub PR comment / GitLab MR note         |

### MCP server

```bash
terraspin serve                         # stdio transport
terraspin serve --transport sse --port 8080  # SSE transport
```

Tools exposed: `analyze_plan`, `get_blast_radius`, `explain_change`, `get_risk_summary`, `suggest_rollback`.

Claude Code config (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "terraspin": {
      "command": "terraspin",
      "args": ["serve"]
    }
  }
}
```

### Diff environments

```bash
terraspin diff s3://bucket/prod.tfstate s3://bucket/staging.tfstate
```

## Configuration

Create `.terraspin.yml` in your Terraform workspace:

```yaml
version: 1

llm:
  provider: claude
  model: claude-sonnet-4-6
  fallback_to_rules: true

risk:
  fail_on: high

rules:
  - id: no-public-rds
    severity: critical
    description: "Database must not be publicly accessible"
    match:
      resource_type_pattern: "*_db_instance"
      attribute_path: "publicly_accessible"
      value: true

  - id: prod-deletion-gate
    severity: critical
    description: "Deletions in production require manual approval"
    match:
      action: delete
      workspace_pattern: "prod-*"

slack:
  webhook_url_env: SLACK_WEBHOOK_URL
  notify_on: [critical, high]
```

## Environment variables

| Variable            | Purpose                       |
| ------------------- | ----------------------------- |
| `ANTHROPIC_API_KEY` | Claude LLM provider           |
| `OPENAI_API_KEY`    | OpenAI LLM provider           |
| `GITHUB_TOKEN`      | GitHub PR comment integration |
| `SLACK_WEBHOOK_URL` | Slack notifications           |

## GitHub Action

```yaml
- name: Terraspin plan analysis
  uses: terraspin/terraspin-action@v1
  with:
    plan-file: plan.json
    fail-on: high
    post-comment: true
  env:
    ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## How it works

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

## Project layout

```
cmd/terraspin/     CLI entry points (main, serve, diff)
internal/
  ai/              LLM providers (Claude, OpenAI, Ollama) + preprocessing
  analyzer/        Risk scorer + blast radius dependency graph
  config/          .terraspin.yml loading + rule evaluation
  diff/            Environment diff comparison
  formatter/       Output formatting (text, JSON, markdown)
  integrations/    GitHub PR comments, GitLab MR notes, Slack webhooks, MCP server
  parser/          Terraform plan JSON → PlanAST
testdata/          Terraform plan fixtures for testing
```

## License

MIT — see [LICENSE](LICENSE).

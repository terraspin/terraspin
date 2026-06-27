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
brew install terraspin/tap/terraspin
```

→ [Full installation guide](docs/installation.md)

## Quick start

```bash
terraspin plan.json
export ANTHROPIC_API_KEY="sk-ant-..."
terraform plan -json | terraspin
```

→ [Getting started](docs/getting-started.md)

## Documentation

| Document                                       | Content                                            |
| ---------------------------------------------- | -------------------------------------------------- |
| [Getting Started](docs/getting-started.md)     | First analysis, common workflows                   |
| [Installation](docs/installation.md)           | Homebrew, Go install, binary releases              |
| [Configuration](docs/configuration.md)         | `.terraspin.yml` reference, rules, LLM, Slack      |
| [CLI Reference](docs/cli.md)                   | Commands, flags, exit codes, MCP server            |
| [Architecture](docs/architecture.md)           | Pipeline, risk scoring, AI narrative, blast radius |
| [Project Structure](docs/project-structure.md) | Directory layout, package responsibilities         |
| [Development](docs/development.md)             | Build, test, lint, conventions                     |
| [Contributing](docs/contributing.md)           | Setup, PR process, design principles               |

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

→ [Architecture overview](docs/architecture.md)

## License

MIT — see [LICENSE](LICENSE).

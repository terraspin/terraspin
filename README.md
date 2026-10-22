# 🌀 Terraspin

**Understand your infrastructure changes before you apply them.**

Terraspin is a CLI that reads `terraform show -json` output and produces a deterministic risk analysis. It can optionally request an LLM narrative when LLM use is explicitly enabled.

```text
$ terraspin --no-ai testdata/plan.json

  Overall Risk  CRITICAL (score: 100)

  1 to create  ·  1 to update  ·  1 to delete  ·  1 to replace

──── Critical & High Risk Changes ────────────────────────────

  [CRITICAL 100.0]  aws_db_instance.primary  →  DELETE
  [CRITICAL 100.0]  aws_security_group.web  →  REPLACE

  Blast radius summary
  No cascading dependencies detected
```

This example is the current text output for `testdata/plan.json` with `--no-ai`.

## Features

- **Risk scoring** — deterministic 0–100 score per resource and overall tier (`critical`, `high`, `medium`, `low`)
- **Blast radius analysis** — traverses Terraform root-module expression references
- **Optional AI narrative** — Claude, OpenAI, or local Ollama when configured or selected with `--llm`
- **Policy rules** — embedded AWS checks and organization-specific policies in `.terraspin.yml`
- **CI integration** — `--fail-on`, PR/MR comments, and Slack notifications
- **MCP server** — run as a Model Context Protocol server for AI coding assistants
- **Formats** — text, JSON, and Markdown
- **Sensitive value redaction** — values Terraform marks sensitive are redacted before LLM calls and in `diff` output

## Limitations

- Redaction covers only values Terraform marks sensitive. Unmarked `user_data`, tags, ARNs, and account IDs remain visible; a list containing a sensitive element is redacted as one attribute.
- Blast radius uses expression references from the Terraform root module. It excludes `module.*`, data-source, and `depends_on` relationships.
- LLM calls use `llm.timeout` per attempt and retry network, timeout, HTTP 429, and 5xx failures. With default settings, the worst-case wait is about 110 seconds; successful requests stop sooner.
- Custom rules support literal list indexes and one `[*]` wildcard, but not regular-expression matching.
- Built-in AWS rules inspect plan values only; missing, `null`, and unknown values do not match an explicit expected value.

Terraspin includes built-in AWS rules for security groups, RDS, EBS, KMS, and EKS. They are active by default and can be disabled per rule in configuration. See [Built-in AWS rules](docs/aws-rules.md) for checks, remediations, and limits.

## Install

### Homebrew

```bash
brew install terraspin/tap/terraspin
```

### Go

```bash
go install github.com/terraspin/terraspin/cmd/terraspin@latest
```

→ [Full installation guide](docs/installation.md)

## Quick start

```bash
# Deterministic analysis; no LLM request.
terraspin --no-ai plan.json

# Explicitly enable Claude for this run.
ANTHROPIC_API_KEY="..." terraspin --llm claude plan.json
```

→ [Getting started](docs/getting-started.md)

## Documentation

| Document                                       | Content                                            |
| ---------------------------------------------- | -------------------------------------------------- |
| [Getting Started](docs/getting-started.md)     | First analysis, common workflows                   |
| [Installation](docs/installation.md)           | Go install and binary releases                     |
| [Configuration](docs/configuration.md)         | `.terraspin.yml` reference, rules, LLM, Slack      |
| [CLI Reference](docs/cli.md)                   | Commands, flags, exit codes, MCP server            |
| [Architecture](docs/architecture.md)           | Pipeline, risk scoring, AI narrative, blast radius |
| [Project Structure](docs/project-structure.md) | Directory layout, package responsibilities         |
| [Development](docs/development.md)             | Build, test, lint, conventions                     |
| [Security](SECURITY.md) and [Security Model](docs/security.md) | Reporting policy and current security boundaries |
| [Contributing](docs/contributing.md)           | Setup, PR process, design principles               |

## How it works

```text
plan.json → ParsePlan() → RedactSensitive() → ScorePlan() → EvaluateRules()
                                                      ↓
                                          AnalyzeBlastRadius()
                                                      ↓
                           optional BuildAnalysisPrompt() → QueryLLM()
                                                      ↓
                                          FormatText/JSON/Markdown → output
```

→ [Architecture overview](docs/architecture.md)

## License

MIT — see [LICENSE](LICENSE).

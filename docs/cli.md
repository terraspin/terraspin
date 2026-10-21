# CLI Reference

## Synopsis

```
terraspin [flags] <plan.json>
terraspin serve [flags]
terraspin diff <state-a> <state-b> [flags]
terraspin version
```

## Analyze command (default)

Reads a terraform plan JSON from a file or stdin and produces risk analysis.

```
terraspin [flags] <plan.json>
terraform plan -json | terraspin [flags]
```

### Flags

| Flag             | Default  | Description                                                 |
| ---------------- | -------- | ----------------------------------------------------------- |
| `--format`       | `text`   | Output format: `text`, `json`, `markdown`                   |
| `--fail-on`      | _(none)_ | Exit 1 if risk >= tier: `critical`, `high`, `medium`, `low` |
| `--no-ai`        | `false`  | Rule-based only, skip LLM call                              |
| `--llm`          | `claude` | LLM provider: `claude`, `openai`, `ollama`                  |
| `--model`        | _(none)_ | Model name (provider default if empty)                      |
| `--ollama-host`  | `http://localhost:11434` | Ollama host URL                                |
| `-v`             | `false`  | Show all risk tiers including medium and low                |
| `--config`       | `.terraspin.yml` | Path to config file                                  |
| `--post-comment` | `false`  | Post analysis as GitHub PR / GitLab MR comment              |

### Policy findings

Matching policy rules are shown after the risk summary in text output, in a `Policy Findings` table in Markdown, and in the JSON `findings` array. Each finding includes its severity, rule ID, resource address, message, optional remediation, and source (`builtin` or `custom`). JSON always includes `findings`, using `[]` when nothing matches. The `analyze_plan` and `get_risk_summary` MCP tools include all findings; `explain_change` includes findings for its selected resource.

### Exit codes

| Code | Meaning                             |
| ---- | ----------------------------------- |
| 0    | Success                             |
| 1    | `--fail-on` threshold exceeded      |
| 2    | Input or parsing error              |
| 3    | MCP server error                    |

## Serve command (MCP server)

Runs Terraspin as a [Model Context Protocol](https://modelcontextprotocol.io) server for AI coding assistants (Claude Code, Copilot, Cursor).

```
terraspin serve [flags]
```

### Flags

| Flag          | Default     | Description                       |
| ------------- | ----------- | --------------------------------- |
| `--transport` | `stdio`     | MCP transport: `stdio` or `sse`  |
| `--port`      | `8080`      | SSE port                          |
| `--host`      | `localhost` | SSE host                          |

### Tools exposed

| Tool                 | Description                            |
| -------------------- | -------------------------------------- |
| `analyze_plan`       | Full plan analysis with risk scoring   |
| `get_blast_radius`   | Dependency graph for a resource        |
| `explain_change`     | Plain-English explanation of a change  |
| `get_risk_summary`   | Compact risk tier summary              |
| `suggest_rollback`   | Rollback steps for the current plan    |

### Claude Code config

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

## Diff command

Compares two terraform plan JSON files (different environments, states, or runs).

```
terraspin diff <plan_a.json> <plan_b.json> [flags]
```

### Flags

| Flag             | Default     | Description                                        |
| ---------------- | ----------- | -------------------------------------------------- |
| `--format`       | `text`      | Output format: `text`, `json`, `markdown`          |
| `--no-ai`        | `false`     | Rule-based only, skip LLM call                     |
| `--llm`          | `claude`    | LLM provider for drift narrative                   |
| `--model`        | _(none)_    | Model name                                         |
| `--ollama-host`  | `http://localhost:11434` | Ollama host URL                       |
| `--label-a`      | `plan-a`    | Label for the first plan                           |
| `--label-b`      | `plan-b`    | Label for the second plan                          |

### Example

```bash
terraspin diff prod-plan.json staging-plan.json --label-a prod --label-b staging
```

> A standalone GitHub Action is not currently published. In GitHub Actions, install or build the CLI and invoke `terraspin` directly.

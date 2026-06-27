# Project Structure

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

## Entry points (`cmd/terraspin/`)

- `main.go` — analyze command: plan parsing, scoring, AI narrative, formatting, `--fail-on` gate, CI posting, Slack notifications
- `serve.go` — MCP server (stdio and SSE transports)
- `diff.go` — two-plan drift comparison with AI narrative

## Internal packages

### `parser`
- `types.go` — `PlanAST`, `ResourceChange`, `ChangeAction` (create/update/delete/replace/no-op/read)
- `parse.go` — JSON unmarshal + structural validation

### `ai`
- `claude.go` — Anthropic Claude API client
- `providers.go` — OpenAI + Ollama clients, `QueryLLM()` router, `Narrative` type, prompt builder, rule-based fallback
- `preprocess.go` — sensitive value redaction

### `analyzer`
- `risk.go` — `ScorePlan()`: base scores × resource multipliers, tier mapping, `ApplyCustomRules()` for escalation from config rules
- `blast.go` — `ParseDependencyRefs()`, `AnalyzeBlastRadius()`: dependency graph and cascading impact

### `config`
- `config.go` — `Config` struct, `Load()`, `DefaultConfig()`, validation
- `rules.go` — `EvaluateRules()`, glob + attribute matching

### `formatter`
- `formatter.go` — `FormatText()`, `FormatJSON()`, `FormatMarkdown()`: all three output formats

### `integrations`
- `github.go` — GitHub PR comment client (post + update with tag)
- `gitlab.go` — GitLab MR note client
- `slack.go` — Slack webhook notifications
- `http.go` — Shared HTTP helpers (retry, auth headers)
- `mcp/server.go` — MCP tool registration and server setup

### `diff`
- `types.go` — `DiffResult`, `ResourceDiff`, `DiffStatus`
- `diff.go` — `Compare()`: two-plan side-by-side analysis

## Test fixtures (`testdata/`)

- `plan.json` — typical plan with creates, updates, deletes
- `plan_empty.json` — no changes
- `plan_with_modules.json` — module-based resources
- `plan_with_sensitive.json` — sensitive value redaction scenarios

## Config files

| File            | Purpose                          |
| --------------- | -------------------------------- |
| `go.mod`        | Go module (1.24). Deps: mcp-go, yaml.v3 |
| `shell.nix`     | Nix dev shell with full tooling  |
| `.env.example`  | Environment variable template    |
| `.gitignore`    | Go build artifacts, .env         |

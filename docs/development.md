# Development

## Prerequisites

- Go 1.25.5+
- Optional: [Nix](https://nixos.org) for the provided `shell.nix` development environment

## Dev environment

```bash
# With Nix (recommended — all tools included)
nix-shell

# Without Nix: install manually
# Go 1.25.5+, terraform/opentofu, golangci-lint, gotestsum, gofumpt
```

The shell includes: Go (1.25.5+ required), golangci-lint, goreleaser, gofumpt, gotestsum, terraform, opentofu, ollama, gh, jq, yq-go, shellcheck, cosign.

Go 1.25.5+ is the floor in `go.mod`, required by `mcp-go v0.58.0`; CI tests that floor and stable, and releases build with stable.

## Build

```bash
go build ./cmd/terraspin/
# → ./terraspin binary
```

## Run tests

```bash
# All tests
go test ./...

# With test summaries
gotestsum --format testname ./...
```

Test files are co-located with their packages (e.g., `internal/parser/parse_test.go`).

## Lint

```bash
golangci-lint run ./...
```

## Format

```bash
gofumpt -w .
```

## Dependencies

```
github.com/mark3labs/mcp-go  — MCP server implementation
gopkg.in/yaml.v3             — .terraspin.yml parsing
```

No framework, no ORM, no DI container. Stdlib HTTP for all API clients.

## Project conventions

- All internal packages are under `internal/` — not importable by external Go modules
- Test fixtures in `testdata/` — used across multiple test files
- `main()` is the only orchestrator — reads like a script
- Config file validation happens at `Load()` time — invalid configs fail fast
- LLM timeout, retries, model selection, and fallback behavior are configured by the `llm` block; analyze and diff share the provider execution path
- Sensitive values are redacted before any external API call

# Development

## Prerequisites

- Go 1.24+
- Optional: [Nix](https://nixos.org) for the provided `shell.nix` development environment

## Dev environment

```bash
# With Nix (recommended — all tools included)
nix-shell

# Without Nix: install manually
# Go 1.24, terraform/opentofu, golangci-lint, gotestsum, gofumpt
```

The shell includes: Go 1.24, golangci-lint, goreleaser, gofumpt, gotestsum, terraform, opentofu, ollama, gh, jq, yq-go, shellcheck, cosign.

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
- Analyze and diff use a fixed 30-second LLM timeout; analyze falls back to rules, while diff omits the AI narrative on error
- Sensitive values are redacted before any external API call

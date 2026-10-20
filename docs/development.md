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

## Deterministic output checks

CLI snapshots live in `testdata/golden/`. Review output changes normally; regenerate approved snapshots with `go test ./cmd/terraspin -run TestCLIGolden -update`. Parser fuzzing is seeded from every `testdata/*.json` fixture and can be run with `go test ./internal/parser -fuzz=FuzzParsePlan -fuzztime=60s`.

## AWS fixture regeneration

AWS plan fixtures live in `testdata/aws/`. Regenerate them with:

```bash
scripts/gen-aws-fixtures.sh
```

The script requires Terraform, `jq`, and network access to the Terraform Registry to install the pinned AWS provider. It uses fake credentials and provider skip flags, so it does not need an AWS account and never applies a plan. `rds_replace` and `s3_destroy` are explicitly labeled `DERIVED`; they are transformed from generated plans rather than Terraform configurations.

Release note draft: Added AWS plan fixtures generated offline from the AWS provider.

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

# Contributing

Contributions welcome — bug reports, feature requests, documentation improvements, and code.

## Setup

See [Development](development.md) for environment setup and tooling.

## Before submitting

1. Run tests: `go test ./...`
2. Run linter: `golangci-lint run ./...`
3. Format code: `gofumpt -w .`
4. Make sure new code has test coverage for non-trivial logic

## Project design principles

- **No unnecessary abstractions** — one implementation per concept, no interfaces with one concrete type, no factories
- **Stdlib first** — `net/http` for API calls, no HTTP client libraries
- **Self-contained packages** — each internal package has zero imports from other internal packages (except shared types)
- **Test co-location** — `*_test.go` files live next to the code they test

## Pull request process

1. Fork the repo, create a feature branch
2. Make your changes, add tests
3. Ensure CI passes (tests + lint)
4. Open a PR with a clear description

## License

MIT — see [LICENSE](../LICENSE).

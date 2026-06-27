# Installation

## Homebrew

```bash
brew install terraspin/tap/terraspin
```

## Go install

```bash
go install github.com/terraspin/terraspin/cmd/terraspin@latest
```

Requires Go 1.24+.

## Binary releases

Download pre-built binaries from [GitHub Releases](https://github.com/terraspin/terraspin/releases).

## Verify

```bash
terraspin version
# terraspin v0.3.0
```

## Environment variables

| Variable            | Purpose                       |
| ------------------- | ----------------------------- |
| `ANTHROPIC_API_KEY` | Claude LLM provider           |
| `OPENAI_API_KEY`    | OpenAI LLM provider           |
| `GITHUB_TOKEN`      | GitHub PR comment integration |
| `SLACK_WEBHOOK_URL` | Slack notifications           |

Copy `.env.example` to `.env` and fill in the keys you need. The `.envrc` file (direnv) will source it automatically if you use `direnv allow`.

## Development shell

A `shell.nix` is provided with all dev tooling (Go 1.24, goreleaser, golangci-lint, terraform, opentofu, ollama, etc.):

```bash
nix-shell
```

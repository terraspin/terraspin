# Installation

## Homebrew

```bash
brew install terraspin/tap/terraspin
```

The public `terraspin/homebrew-tap` repository contains `terraspin.rb` for release `v0.1.0`.

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
```

The command prints the version and, for release builds, commit/build metadata. A binary installed with `go install ...@latest` may report a `dev` version because it is not built with release linker metadata.

## Environment variables

| Variable | Purpose |
| --- | --- |
| `ANTHROPIC_API_KEY` | API key for Claude when Claude is enabled |
| `ANTHROPIC_BASE_URL` | Optional Anthropic-compatible endpoint URL |
| `CLAUDE_MODEL` | Claude model when `--model` is not set |
| `OPENAI_API_KEY` | API key for OpenAI when OpenAI is selected |
| `GITHUB_TOKEN` | GitHub PR comment integration token |
| `GITHUB_REPOSITORY` | GitHub repository for PR comments |
| `GITHUB_REF` / `GITHUB_REF_NAME` | GitHub PR reference used to find the PR number |
| `CI_SERVER_URL` | GitLab base URL (defaults to `https://gitlab.com`) |
| `GITLAB_TOKEN` / `CI_JOB_TOKEN` | GitLab MR note token |
| `CI_PROJECT_ID` | GitLab project ID for MR notes |
| `CI_MERGE_REQUEST_IID` | GitLab merge-request IID for notes |
| `SLACK_WEBHOOK_URL` | Slack notification webhook |

Only the variables above are read directly by the executable. `.env.example` contains only these runtime variables; use `--llm`, `--model`, and `--ollama-host` to select providers and models. If you use `direnv`, `.envrc` can source a local `.env` after `direnv allow`.

## Development shell

A `shell.nix` provides development tooling:

```bash
nix-shell
```

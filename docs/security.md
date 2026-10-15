# Security Model

This is a code-derived model of Terraspin v0.1.x, not a guarantee. **[VERIFIED]** statements cite the implementation or tests; **[NOT VERIFIED]** statements identify limits not covered by a dedicated security test.

## Trust Boundaries

- **[VERIFIED]** `terraform show -json` output is untrusted input. The parser checks JSON syntax and `format_version`, then preserves managed-resource addresses, metadata, and before/after maps (`internal/parser/parse.go:45-106`).
- **[VERIFIED]** Analyze redacts Terraform-marked sensitive resource values before scoring, formatting, or an LLM call (`cmd/terraspin/main.go:121-169`; `internal/ai/preprocess.go:13-23`). `diff` redacts the source ASTs and sensitive changed attributes (`internal/diff/diff.go:15-43`). CLI tests cover marked scalar, collection, nested, added, and removed values in text, JSON, and Markdown output (`cmd/terraspin/main_test.go:90-180`).
- **[VERIFIED]** Redaction follows Terraform's `before_sensitive` and `after_sensitive` markers; it is not attribute-name-based (`internal/ai/preprocess.go:25-52`). Unmarked values can therefore remain in local `diff` output, which formats old/new attribute values (`cmd/terraspin/diff.go:194-226`, `239-293`). Plan variables and output changes are retained by the parser, but this redactor only traverses resource changes (`internal/parser/parse.go:56-68`; `internal/ai/preprocess.go:15-21`).

## Optional LLM Requests

- **[VERIFIED]** Analyze sends only risk tier/score, tier counts, resource address, action, tier, and optional affected-resource count to its prompt (`cmd/terraspin/main.go:349-383`; `internal/ai/providers.go:201-232`). Diff sends labels, Terraform versions, resource address/type/status, and changed-attribute counts—not attribute values (`internal/diff/diff.go:305-334`). Payload tests assert that synthetic sensitive and unmarked attribute values are absent for analyze and diff (`cmd/terraspin/main_test.go:252-313`; `cmd/terraspin/payload_providers_test.go:100-129`).
- **[VERIFIED]** Claude sends `SystemPrompt` in Anthropic's `system` field and an API key in `x-api-key` (`internal/ai/claude.go:41-51`, `78-96`). OpenAI sends the same system prompt as a system message and uses `Authorization: Bearer` (`internal/ai/providers.go:34-62`). Ollama sends only its user prompt—no system field and no API-key header (`internal/ai/providers.go:93-133`); provider tests assert this difference (`cmd/terraspin/payload_providers_test.go:112-124`).
- **[VERIFIED]** Resource addresses are interpolated into analyze and diff prompts (`cmd/terraspin/main.go:350-360`; `internal/diff/diff.go:314-322`). **[NOT VERIFIED]** No address-validation or prompt-injection mitigation test was found. Treat plan JSON as hostile and do not rely on an LLM to follow instructions embedded in addresses.

## Local and CI Outputs

- **[VERIFIED]** `--post-comment` embeds the complete analyze output in a GitHub PR comment or GitLab MR note (`cmd/terraspin/main.go:407-438`). These outputs contain resource addresses, risk data, blast-radius information, and any narrative. GitHub uses `GITHUB_TOKEN`; GitLab uses `GITLAB_TOKEN` or `CI_JOB_TOKEN` (`internal/integrations/github.go:21-29`, `82-98`; `internal/integrations/gitlab.go:18-49`).
- **[VERIFIED]** Slack receives the risk tier, score, action counts, and narrative summary when configured (`cmd/terraspin/main.go:441-479`; `internal/integrations/slack.go:40-60`).
- **[NOT VERIFIED]** LLM output is not independently sanitized before being printed or included in comments/Slack. Use a trusted provider and review automated comments.

## Keys, Retries, and Limits

- **[VERIFIED]** Claude and OpenAI fail before any request when their key is empty; a gateway without authentication still needs a non-secret placeholder (`internal/ai/executor.go:54-66`; `internal/ai/executor_test.go:153-184`). Ollama does not require a key (`internal/ai/executor_test.go:186-197`). Keys are read from environment variables and placed in request headers; this code does not write them to output (`cmd/terraspin/main.go:363-383`; `internal/ai/claude.go:90-96`).
- **[VERIFIED]** Each LLM attempt has its own timeout; retryable failures are network errors, timeouts, HTTP 429, and 5xx. Backoff and `Retry-After` are capped at 10 seconds (`internal/ai/executor.go:54-86`, `108-132`).
- **[NOT VERIFIED]** Analyze and diff start retries with `context.Background()`, so they do not impose a separate total deadline across attempts and waits (`cmd/terraspin/main.go:378-383`; `cmd/terraspin/diff.go:147-151`).

## Limitations

- **[VERIFIED]** Values that Terraform does not mark sensitive remain visible in local diff output (`internal/ai/preprocess.go:25-52`; `cmd/terraspin/diff.go:194-226`).
- **[VERIFIED]** Resource addresses enter analyze and diff prompts (`cmd/terraspin/main.go:350-360`; `internal/diff/diff.go:314-322`).
- **[VERIFIED]** Release artifacts are currently unsigned: the configured checksum remains active while the only `signs` block is commented out (`.goreleaser.yml:33-36`, `.goreleaser.yml:71-81`).

## Blast-Radius Limits

- **[VERIFIED]** Blast radius reads root-module expression references and deliberately excludes `module.*`, data-source, and `depends_on` relationships (`internal/analyzer/blast.go:82-139`; `internal/analyzer/blast_test.go:65-111`). It is an aid to review, not a complete dependency graph.

# Security Model

This is a code-derived model of Terraspin v0.1.x, not a guarantee. **[VERIFIED]** statements cite the implementation or tests; **[NOT VERIFIED]** statements identify limits not covered by a dedicated security test. Code references use stable file and symbol names rather than line numbers.

## Trust Boundaries

- **[VERIFIED]** `terraform show -json` output is untrusted input. `internal/parser.ParsePlan` checks JSON syntax and `format_version`, then preserves managed-resource addresses, metadata, and before/after maps.
- **[VERIFIED]** Analyze calls `internal/ai.RedactSensitive` before scoring, formatting, or an LLM call; `internal/diff.Compare` redacts source ASTs and sensitive changed attributes. `cmd/terraspin.TestCLISensitiveValuesNeverReachOutput` covers marked scalar, collection, nested, added, and removed values in text, JSON, and Markdown output.
- **[VERIFIED]** `internal/ai.RedactSensitive` follows Terraform's `before_sensitive` and `after_sensitive` markers; it is not attribute-name-based. Unmarked values can therefore remain in local diff output, which `cmd/terraspin.formatDiffText` and `formatDiffMD` render. `internal/parser.ParsePlan` retains plan variables and output changes, but the redactor traverses only resource changes.

## Optional LLM Requests

- **[VERIFIED]** `cmd/terraspin.buildLLMNarrative` and `internal/ai.BuildAnalysisPrompt` send analyze risk data, resource addresses, actions, tiers, and affected-resource counts—not attribute maps. `internal/diff.BuildDiffPrompt` sends labels, Terraform versions, resource address/type/status, and changed-attribute counts—not attribute values. `cmd/terraspin.TestCLIPayloadsExcludeAttributeValuesIncludingSystem` and `TestCLIOpenAIAndOllamaPayloadsExcludeAttributeValues` cover these payloads.
- **[VERIFIED]** `internal/ai.QueryClaude` sends `SystemPrompt` in Anthropic's `system` field and an API key in `x-api-key`. `internal/ai.QueryOpenAI` sends the same system prompt as a system message and uses `Authorization: Bearer`. `internal/ai.QueryOllama` sends only its user prompt—no system field and no API-key header; `cmd/terraspin.assertProviderPayloads` tests this difference.
- **[VERIFIED]** `cmd/terraspin.buildLLMNarrative` and `internal/diff.BuildDiffPrompt` interpolate resource addresses into prompts. **[NOT VERIFIED]** No address-validation or prompt-injection mitigation test was found. Treat plan JSON as hostile and do not rely on an LLM to follow instructions embedded in addresses.

## Local and CI Outputs

- **[VERIFIED]** `cmd/terraspin.postToCI` embeds complete analyze output in a GitHub PR comment or GitLab MR note. These outputs contain resource addresses, risk data, blast-radius information, and any narrative. `internal/integrations.NewGitHubClientFromEnv` uses `GITHUB_TOKEN`; `NewGitLabClientFromEnv` uses `GITLAB_TOKEN` or `CI_JOB_TOKEN`.
- **[VERIFIED]** `cmd/terraspin.notifySlack` and `internal/integrations.SlackWebhook.SendRiskNotification` send the risk tier, score, action counts, and narrative summary when configured.
- **[NOT VERIFIED]** LLM output is not independently sanitized before being printed or included in comments/Slack. Use a trusted provider and review automated comments.

## Keys, Retries, and Limits

- **[VERIFIED]** `internal/ai.Execute` rejects empty Claude and OpenAI keys before a request; a gateway without authentication still needs a non-secret placeholder. `TestExecuteGatewayStillRequiresClaudeKey` covers that path. `TestExecuteAllowsOllamaWithoutKey` covers Ollama without a key. Keys are read from environment variables and passed in provider headers; the reviewed code does not write them to output.
- **[VERIFIED]** `internal/ai.Execute`, `isRetryable`, and `retryDelay` apply a timeout per attempt; retryable failures are network errors, timeouts, HTTP 429, and 5xx. Backoff and `Retry-After` are capped at 10 seconds.
- **[NOT VERIFIED]** Analyze and diff call `internal/ai.Execute` with `context.Background()`, so they do not impose a separate total deadline across attempts and waits.

## Limitations

- **[VERIFIED]** Values that Terraform does not mark sensitive remain visible in local diff output through `internal/ai.RedactSensitive` and `cmd/terraspin.formatDiffText` / `formatDiffMD`.
- **[VERIFIED]** Resource addresses enter analyze and diff prompts through `cmd/terraspin.buildLLMNarrative` and `internal/diff.BuildDiffPrompt`.
- **[VERIFIED]** Release artifacts are currently unsigned: the `checksum` section remains active while the `signs` section in `.goreleaser.yml` is commented out.

## Blast-Radius Limits

- **[VERIFIED]** `internal/analyzer.ParseDependencyRefs` reads root-module expression references and excludes `module.*`, data-source, and `depends_on` relationships. `TestParseDependencyRefs_filtersVariablesAndDataRefs` covers the filtering. Blast radius is an aid to review, not a complete dependency graph.

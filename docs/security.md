# Security Model

This is a code-derived model of Terraspin v0.2.x, not a guarantee. **[VERIFIED]** statements cite the implementation or tests; **[NOT VERIFIED]** statements identify limits not covered by a dedicated security test. Code references use stable file and symbol names rather than line numbers. Earlier releases (v0.1.0) behave differently; see [SECURITY.md](../SECURITY.md).

## Trust Boundaries

- **[VERIFIED]** `terraform show -json` output is untrusted input. `internal/parser.ParsePlan` checks JSON syntax and `format_version`, then preserves managed-resource addresses, metadata, variables, output changes, and before/after maps.
- **[VERIFIED]** Analyze calls `internal/ai.RedactSensitive` before scoring, formatting, or an LLM call; `internal/diff.Compare` redacts source ASTs and sensitive changed attributes. `cmd/terraspin.TestCLISensitiveValuesNeverReachOutput` covers marked scalar, collection, nested, added, and removed values in text, JSON, and Markdown output.
- **[VERIFIED manually; no dedicated test]** `PlanAST` retains plan variables and output changes, while `internal/ai.RedactSensitive` traverses resource changes only. A Terraform `terraform_data` fixture with sensitive variable/output values in both plans produced no synthetic values in analyze, diff, LLM payload, or MCP output. The current formatters and MCP handlers do not serialize those fields. Re-run this check with two `terraform show -json` plans when changing parser, formatter, diff, or MCP output code.
- **[VERIFIED]** Redaction follows Terraform's `before_sensitive` and `after_sensitive` markers; it is not attribute-name-based. Unmarked values can therefore remain in local diff output, which `cmd/terraspin.formatDiffText` and `formatDiffMD` render.

## Optional LLM Requests

- **[VERIFIED]** `cmd/terraspin.buildLLMNarrative` and `internal/ai.BuildAnalysisPrompt` send analyze risk data, resource addresses, actions, tiers, and affected-resource counts—not attribute maps. `internal/diff.BuildDiffPrompt` sends labels, resource address/type/status, and changed-attribute counts—not attribute values or Terraform versions. `cmd/terraspin.TestCLIPayloadsExcludeAttributeValuesIncludingSystem`, `TestCLIOpenAIAndOllamaPayloadsExcludeAttributeValues`, and `internal/diff.TestBuildDiffPromptExcludesPlanVersions` cover these payloads.
- **[VERIFIED]** `internal/ai.bedrockProvider.Query` sends `SystemPrompt` as a Bedrock system text block and the user prompt as one user message; its AWS SDK request is SigV4-signed. `internal/ai.QueryClaude` sends `SystemPrompt` in Anthropic's `system` field and an API key in `x-api-key`. `internal/ai.QueryOpenAI` sends the same system prompt as a system message and uses `Authorization: Bearer`. `internal/ai.QueryOllama` sends only its user prompt—no system field and no API-key header. `cmd/terraspin.assertProviderPayloads` tests these differences.
- **[VERIFIED]** `internal/ai.BuildAnalysisPrompt` and `internal/diff.BuildDiffPrompt` pass resource addresses through `internal/safetext.SafeAddress`; invalid addresses become `<invalid-address>`, while invalid indexed keys become `["<redacted-key>"]`. `internal/safetext.TestSafeAddress` and `cmd/terraspin.TestCLIPayloadsSanitizeHostileAddresses` cover direct validation and recorded analyze/diff request payloads.

## Local and CI Outputs

- **[VERIFIED]** `cmd/terraspin.postToCI` embeds complete analyze output in a GitHub PR comment or GitLab MR note. These outputs contain resource addresses, risk data, blast-radius information, and any narrative. `internal/integrations.NewGitHubClientFromEnv` uses `GITHUB_TOKEN`; `NewGitLabClientFromEnv` uses `GITLAB_TOKEN` or `CI_JOB_TOKEN`.
- **[VERIFIED]** Rule `description` and `remediation` values from `.terraspin.yml` appear in local output and PR/MR comments, so configuration from an untrusted PR can insert displayed text.
- **[VERIFIED]** `cmd/terraspin.notifySlack` and `internal/integrations.SlackWebhook.SendRiskNotification` send the risk tier, score, action counts, and narrative summary when configured.
- **[VERIFIED]** Claude, OpenAI, Ollama, and Bedrock provider failures are returned to Analyze and diff, which print errors or fallback warnings to stderr. Existing HTTP-provider implementations include non-success response bodies; Bedrock maps Smithy HTTP response status to `HTTPError`. Treat provider errors as potentially sensitive.
- **[VERIFIED]** Analyze and diff call `internal/ai.SanitizeNarrative` immediately after `ParseNarrativeFromLLM`; it removes control characters except newline/tab, bounds strings, list items, lists, and the total narrative before formatter, PR/MR, or Slack use. `internal/ai.TestSanitizeNarrative` covers those limits.

## Configuration and CI

`cmd/terraspin.main` loads `.terraspin.yml` from the working directory by default (or the `--config` path). `cmd/terraspin.diffCmd` and `mcp.planFromArgs` load `.terraspin.yml` from their current working directory. The file can select the LLM provider/model, timeout, retries, fallback behavior, risk threshold, custom rules, and Slack environment-variable name/notification tiers. LLM endpoint selection is not a YAML field: Claude uses `ANTHROPIC_BASE_URL`, OpenAI uses its built-in endpoint, Ollama uses `--ollama-host`, and the AWS SDK honors `AWS_ENDPOINT_URL_BEDROCK_RUNTIME` for Bedrock Runtime endpoint testing.

**[VERIFIED]** A CI job that runs untrusted PR code can consume a PR-controlled `.terraspin.yml`. Treat its provider selection, LLM behavior, custom rules, and Slack settings as untrusted configuration; restrict credentials and do not run it with secrets unless that exposure is intended.

**[NOT VERIFIED]** The precedence of `--fail-on` over `risk.fail_on` in `.terraspin.yml` has not been measured. In CI, pass `--fail-on` explicitly and load configuration from the base branch.

## Keys, Retries, and Limits

- **[VERIFIED]** `internal/ai.Execute` rejects empty Claude and OpenAI keys before a request; a gateway without authentication still needs a non-secret placeholder. `TestExecuteGatewayStillRequiresClaudeKey` covers that path. `TestExecuteAllowsOllamaWithoutKey` covers Ollama without a key. `bedrockProvider.Query` requires an explicit model, AWS Region, and credentials before its invoke request; `TestBedrockRejectsMissingAWSConfigurationWithoutRequest` covers missing region and credentials. Terraspin does not store or print AWS credentials.
- **[VERIFIED: AWS documentation]** Converse requires the minimum IAM action `bedrock:InvokeModel`; streaming is not used. Scope this permission to only the model or inference-profile resources needed. Source: <https://docs.aws.amazon.com/bedrock/latest/userguide/conversation-inference.html> and <https://docs.aws.amazon.com/bedrock/latest/userguide/security_iam_id-based-policy-examples.html>.
- **[VERIFIED]** Bedrock request charges are billed to the user's AWS account. Source: <https://docs.aws.amazon.com/bedrock/latest/userguide/bedrock-pricing.html>.
- **[VERIFIED]** `internal/ai.Execute`, `isRetryable`, and `retryDelay` apply a timeout per attempt; retryable failures are network errors, timeouts, HTTP 429, and 5xx. Bedrock disables SDK retries (`RetryMaxAttempts=1`) and maps Smithy HTTP status to `HTTPError`, so the executor remains the retry owner. Backoff and `Retry-After` are capped at 10 seconds.
- **[NOT VERIFIED]** Analyze and diff call `internal/ai.Execute` with `context.Background()`, so they do not impose a separate total deadline across attempts and waits.

## MCP Server

- **[VERIFIED]** `cmd/terraspin.serveCmd` defaults to stdio, which opens no Terraspin TCP listener. Its SSE transport binds the configured `--host` and `--port` (defaults `localhost:8080`); the server exposes no authentication option or authentication middleware in the reviewed code.
- **[VERIFIED]** Terraspin pins `github.com/mark3labs/mcp-go` v0.58.0 in `go.mod` and constructs its SSE server with `NewSSEServer(s)` without `WithSSEDisableLocalhostProtection` or `WithSSECORS`. In that version, `SSEServer.ServeHTTP` calls `rejectDNSRebinding`: a loopback request with `Host: evil.example` received `403 Forbidden` in this measurement. The source shows that disabling this protection is opt-in.
- **[VERIFIED]** On a server bound to `127.0.0.1`, `GET /sse` returned `200 OK`, `Access-Control-Allow-Origin: *`, and an `endpoint` event containing a `sessionId`. The same request with `Origin: http://evil.example` also returned `200 OK` and `Access-Control-Allow-Origin: *`. This matches v0.58.0 `handleSSE`, which sets that header when explicit `WithSSECORS` configuration is absent; the measurement found no Origin-header rejection.
- **[VERIFIED]** A `POST` of JSON-RPC `initialize` to the measured endpoint with `Content-Type: text/plain` and `Origin: http://evil.example` returned `202 Accepted`; the connected SSE stream received the JSON-RPC result. v0.58.0 `handleMessage` requires the `sessionId` query parameter and decodes the request body as JSON. These are direct HTTP results, not a claim that a browser can exploit them: browser Private/Local Network Access protections can affect browser behavior.
- **[RECOMMENDATION]** Prefer stdio. Do not run the SSE transport while visiting untrusted sites. Origin rejection and a request-body size limit remain follow-up work.
- **[VERIFIED]** `mcp.NewServer` exposes `analyze_plan` (`plan_json`), `get_blast_radius` (`plan_json`, `resource_address`), `explain_change` (`plan_json`, `resource_address`), `get_risk_summary` (`plan_json`), and `suggest_rollback` (`plan_json`, optional `failure_point`). All accept raw plan JSON as a string and return text content containing risk summaries, resource addresses, blast data, explanations, or rollback guidance.
- **[VERIFIED]** `mcp.planFromArgs` redacts resource changes before handlers format them. `TestHandleExplainChange_redactsSensitiveLists` covers marked sensitive values. The same manual Terraform fixture returned no synthetic variable/output value from all five MCP tools.
- **[NOT VERIFIED]** There is no repository-level input-size limit in `mcp.planFromArgs`; upstream transport limits were not evaluated. `explain_change` can return unmarked changed resource attribute values, just as local diff output can.

## Limitations

- **[VERIFIED]** Values that Terraform does not mark sensitive remain visible in local diff and MCP `explain_change` output through `internal/ai.RedactSensitive`, `cmd/terraspin.formatDiffText` / `formatDiffMD`, and `mcp.handleExplainChange`.
- **[VERIFIED]** Analyze and diff prompt resource addresses pass through `internal/safetext.SafeAddress` in `internal/ai.BuildAnalysisPrompt` and `internal/diff.BuildDiffPrompt`.
- **[LIMIT]** Valid Terraform index keys still enter LLM prompts. Local stdout retains resource text and can display terminal escapes; LLM narrative has no Markdown/HTML escaping, so `@user` mentions remain possible in automated comments.
- **[VERIFIED]** Release artifacts are currently unsigned: the `checksum` section remains active while the `signs` section in `.goreleaser.yml` is commented out.

## Blast-Radius Limits

- **[VERIFIED]** `internal/analyzer.ParseDependencyRefs` reads root-module expression references and excludes `module.*`, data-source, and `depends_on` relationships. `TestParseDependencyRefs_filtersVariablesAndDataRefs` covers the filtering. Blast radius is an aid to review, not a complete dependency graph.

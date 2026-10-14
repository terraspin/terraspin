# Configuration

Create `.terraspin.yml` in your Terraform workspace. All sections are optional.

## Full reference

```yaml
version: 1

llm:
  provider: claude          # claude | openai | ollama
  model: claude-sonnet-4-6  # accepted, but not currently applied at runtime
  timeout: 30s              # accepted, but not currently applied at runtime
  max_retries: 2            # accepted, but not currently applied at runtime
  fallback_to_rules: true   # accepted, but not currently applied at runtime

risk:
  fail_on: high             # exit code 1 if risk >= critical|high|medium|low

rules:
  - id: no-public-rds
    severity: critical
    description: "Database must not be publicly accessible"
    match:
      resource_type_pattern: "*_db_instance"
      attribute_path: "publicly_accessible"
      value: true

  - id: prod-deletion-gate
    severity: critical
    description: "Deletions in production require manual approval"
    match:
      action: delete
      workspace_pattern: "prod-*"

slack:
  webhook_url_env: SLACK_WEBHOOK_URL
  notify_on: [critical, high]
```

## Analyze LLM behavior

| Situation | LLM request | Result |
| --- | --- | --- |
| No `.terraspin.yml` file | No | Rule-based narrative |
| Config file without `llm` | Yes, using documented defaults | LLM narrative when the provider succeeds; rule-based fallback on error |
| Config file with `llm` | Yes | Uses its `provider` value |
| `--llm` flag | Yes | Enables and selects the flag's provider, even without a config file |
| API key without config file or `--llm` | No | Rule-based narrative |
| `--llm` without an API key | Attempts the provider request with an empty key | Provider error falls back to rule-based analysis |

The default config-file provider is `claude`. The `--model` flag is applied to the provider request; the `llm.model` config field is not yet applied.

### LLM config fields

| Field | Config default | Runtime status |
| --- | --- | --- |
| `provider` | `claude` | Applied when an `llm` block exists. |
| `model` | `claude-sonnet-4-6` | Not yet applied from config; use `--model` (or `CLAUDE_MODEL` for Claude). |
| `timeout` | `30s` | Not yet applied from config; callers use a fixed 30-second timeout. |
| `max_retries` | `2` | Not yet applied; requests are not retried. |
| `fallback_to_rules` | `true` | Not yet applied; analyze falls back to rules after an LLM error. |

These unused config fields are retained for compatibility and should be treated as **not yet effective**.

`terraspin diff` does not load `.terraspin.yml`; it uses its own LLM flags and only requests its provider when the matching API key is present.

## Data sent to LLM providers

Terraspin sends a static system prompt plus the following plan-derived fields. It does not send raw `Before` or `After` attribute values in either prompt.

| Command | Plan-derived fields in the user prompt |
| --- | --- |
| Analyze | Overall risk tier and score; counts for critical/high/medium/low; per changed resource: address, tier, action, and affected-resource count when blast radius is nonzero. |
| Diff | Input labels; Terraform versions; added/removed/modified/unchanged summary; for each non-unchanged resource: address, type, status, and number of changed attributes. |

Values Terraform marks sensitive are redacted before analysis. Values not marked sensitive are not included in the current LLM prompt, but may still appear in local `diff` output; see [Limitations](#limitations).

## Rules

Each rule has:

| Field | Required | Description |
| --- | --- | --- |
| `id` | yes | Unique rule identifier |
| `severity` | yes | `critical`, `high`, `medium`, or `low` |
| `description` | yes | Human-readable explanation |
| `match` | yes | Conditions (at least one required) |

Match conditions:

| Condition | Example | Description |
| --- | --- | --- |
| `resource_type_pattern` | `*_db_instance` | Glob match against resource type |
| `attribute_path` | `publicly_accessible` | Dot-separated attribute path to inspect |
| `value` | `true` | Value to match at `attribute_path` |
| `attribute_path_2` | `engine` | Optional second dot-separated path; evaluated with the first condition |
| `value_2` | `postgres` | Value to match at `attribute_path_2` |
| `contains` | `prod` | Substring required in `attribute_path` value |
| `action` | `delete` | Match by change action |
| `workspace_pattern` | `prod-*` | Glob match against terraform workspace name |

If a rule matches, the resource's risk tier escalates to the rule's severity; it never downgrades. List indexes, list wildcards, and regex matching are not supported.

## Risk config

| Field | Description |
| --- | --- |
| `fail_on` | `critical`, `high`, `medium`, or `low`. Overridden by `--fail-on`. |

## Slack config

| Field | Description |
| --- | --- |
| `webhook_url_env` | Environment variable containing the webhook URL |
| `notify_on` | Risk tiers that trigger a notification |
| `channel` | Reserved; not used at runtime |

Notifications fire when `SLACK_WEBHOOK_URL` (or the custom environment variable) is set and the overall plan risk matches one of `notify_on`.

## Limitations

- Redaction covers only Terraform-marked sensitive values. Unmarked `user_data`, tags, ARNs, and account IDs remain visible in `diff`; a list containing a sensitive element is redacted as one attribute.
- Analyze and diff use fixed 30-second LLM timeouts. They do not retry or rate-limit requests.
- The rule engine does not support list indexes, wildcards, or regex matching.
- The built-in AWS behavior is limited to risk-score multipliers; no AWS rule pack is included.

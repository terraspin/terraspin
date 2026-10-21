# Configuration

Create `.terraspin.yml` in your Terraform workspace. All sections are optional.

## Full reference

```yaml
version: 1

llm:
  provider: claude          # claude | openai | ollama
  model: claude-sonnet-4-6  # provider model (unless --model is set)
  timeout: 30s              # timeout per provider attempt
  max_retries: 2            # additional retry attempts
  fallback_to_rules: true   # use rules after an LLM error

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
| `--llm` without an API key | No | Fails fast; fallback follows `fallback_to_rules`. |

The default config-file provider is `claude`. The `--model` flag is applied to the provider request; `llm.model` is used only when explicitly written in the config file. Claude and OpenAI require a non-empty API-key environment variable even when a gateway/proxy endpoint is configured. For an unauthenticated gateway/proxy, set the required variable to a non-secret placeholder value.

### LLM config fields

| Field | Config default | Runtime status |
| --- | --- | --- |
| `provider` | `claude` | Applied when an `llm` block exists. |
| `model` | `claude-sonnet-4-6` | Applied only when explicitly written; `--model` takes precedence. Provider defaults apply otherwise. |
| `timeout` | `30s` | Applied per attempt; invalid Go durations fail config loading. |
| `max_retries` | `2` | Applied as additional attempts for network, timeout, HTTP 429, and 5xx errors. |
| `fallback_to_rules` | `true` | Applied: true falls back after warning; false exits with code 2. |

Retries use capped exponential backoff with jitter and honor `Retry-After` up to 10 seconds. The conservative maximum request time is `(max_retries + 1) × timeout + total retry waits`; using the 10-second wait cap per retry, the defaults are `(2 + 1) × 30s + 2 × 10s = 110s`. A request that succeeds earlier stops retrying sooner.

`terraspin diff` loads `.terraspin.yml` and uses its `llm` settings; `--llm` and `--model` override the configured provider and model. It only requests Claude or OpenAI when the matching API key is non-empty.

## Data sent to LLM providers

For v0.1.x, Claude and OpenAI requests include a static system prompt plus the following plan-derived fields in their user prompt. Ollama sends only the user prompt (no system field). They do not include raw `Before` or `After` attribute values today. This is observed behavior, not a long-term payload contract.

| Command | Plan-derived fields in the user prompt |
| --- | --- |
| Analyze | Overall risk tier and score; counts for critical/high/medium/low; per changed resource: address, tier, action, and affected-resource count when blast radius is nonzero. |
| Diff | Input labels; Terraform versions; added/removed/modified/unchanged summary; for each non-unchanged resource: address, type, status, and number of changed attributes. |

Values Terraform marks sensitive are redacted before analysis. Values not marked sensitive are not included in the current v0.1.x LLM prompt, but may still appear in local `diff` output. Resource addresses are sent and can contain internal naming information; treat them as untrusted input because address-based prompt injection is not mitigated yet. See [Limitations](#limitations).

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
| `attribute_path` | `publicly_accessible` | Attribute path to inspect |
| `value` | `true` | Value to match at `attribute_path` |
| `attribute_path_2` | `engine` | Optional second dot-separated path; evaluated with the first condition |
| `value_2` | `postgres` | Value to match at `attribute_path_2` |
| `contains` | `prod` | Required substring or list member at `attribute_path` |
| `action` | `delete` | Match by change action |
| `workspace_pattern` | `prod-*` | Glob match against terraform workspace name |

If a rule matches, the resource's risk tier escalates to the rule's severity; it never downgrades.

### Attribute paths

Paths use dot-separated segments containing letters, digits, `_`, or `-`. A segment may end in a zero-based list index (`[0]`) or a list wildcard (`[*]`). A path can contain at most one wildcard.

A condition matches when any resolved value matches. `contains` is a substring check for strings and an exact string-membership check for lists.

```yaml
rules:
  - id: public-ingress
    severity: critical
    description: Public ingress needs review
    match:
      resource_type_pattern: aws_security_group
      attribute_path: ingress[*].cidr_blocks
      contains: 0.0.0.0/0
```

Regex paths are not supported.

### Wildcard correlation

When both `attribute_path` and `attribute_path_2` contain `[*]` with the same prefix before the wildcard, their conditions must match the same list element. Paths with different prefixes, or with a wildcard on only one path, remain independent.

```yaml
rules:
  - id: sg-ssh-open-to-world
    severity: critical
    description: Public SSH ingress needs review
    match:
      resource_type_pattern: aws_security_group
      attribute_path: ingress[*].cidr_blocks
      contains: 0.0.0.0/0
      attribute_path_2: ingress[*].from_port
      value_2: 22
```

This rule does not match an ingress list where port 22 and `0.0.0.0/0` occur in different elements.

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
- LLM retries cover network, timeout, HTTP 429, and 5xx errors only; there is no rate-limit scheduler beyond retry backoff.
- Resource addresses are included in LLM prompts and may reveal internal names. Address-based prompt injection is not mitigated.
- Automated tests lock Claude CLI `analyze` and `diff` payloads against synthetic attribute values, including the system field; treat the documented v0.1.x field list as version-specific.
- Claude and OpenAI fail before sending a request when their required API key is absent. Ollama does not require a key.
- Rule paths allow one `[*]` wildcard but no regex. Only paths with the same wildcard prefix correlate; other wildcard conditions remain independent.
- The built-in AWS behavior is limited to risk-score multipliers; no AWS rule pack is included.

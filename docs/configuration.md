# Configuration

Create `.terraspin.yml` in your Terraform workspace. All sections are optional.

## Full reference

```yaml
version: 1

llm:
  provider: claude          # claude | openai | ollama
  model: claude-sonnet-4-6  # model name (provider default if omitted)
  fallback_to_rules: true   # fall back to rule-based if LLM call fails

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

## LLM config

| Field              | Default                    | Description                              |
| ------------------ | -------------------------- | ---------------------------------------- |
| `provider`         | `claude`                   | LLM provider                             |
| `model`            | `claude-sonnet-4-20250514` | Model name                               |
| `timeout`          | `30s`                      | API call timeout                         |
| `max_retries`      | `2`                        | Retries on transient failure             |
| `fallback_to_rules` | `true`                   | Use rule-based narrative if LLM fails    |

## Rules

Each rule has:

| Field         | Required | Description                                |
| ------------- | -------- | ------------------------------------------ |
| `id`          | yes      | Unique rule identifier                     |
| `severity`    | yes      | `critical` / `high` / `medium` / `low`     |
| `description` | yes      | Human-readable explanation                 |
| `match`       | yes      | Conditions (at least one required)         |

Match conditions:

| Condition               | Example                | Description                                   |
| ----------------------- | ---------------------- | --------------------------------------------- |
| `resource_type_pattern` | `*_db_instance`        | Glob match against resource type              |
| `attribute_path`        | `publicly_accessible`  | Check a resource attribute value              |
| `value`                 | `true`                 | Value to match against `attribute_path`       |
| `contains`              | `prod`                 | Substring match on `attribute_path` value     |
| `action`                | `delete`               | Match by change action                        |
| `workspace_pattern`     | `prod-*`               | Glob match against terraform workspace name   |

If a rule matches, the resource's risk tier escalates to the rule's severity (never downgrades).

## Risk config

| Field     | Description                                        |
| --------- | -------------------------------------------------- |
| `fail_on` | `critical` / `high` / `medium` / `low`. Overridden by `--fail-on` flag. |

## Slack config

| Field             | Description                                    |
| ----------------- | ---------------------------------------------- |
| `webhook_url_env` | Env var containing the webhook URL             |
| `notify_on`       | Risk tiers that trigger a notification         |
| `channel`         | (reserved)                                     |

Notifications fire when `SLACK_WEBHOOK_URL` (or the custom env var) is set and the overall plan risk matches one of `notify_on` tiers.

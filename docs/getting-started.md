# Getting Started

## First analysis

```bash
# Generate a plan file
terraform plan -out=plan.tfplan
terraform show -json plan.tfplan > plan.json

# Analyze it without an LLM request
terraspin --no-ai plan.json
```

## Pipe a saved plan

`terraform plan -json` emits Terraform UI events, not a `terraform show -json` plan document. Pipe the saved plan JSON instead:

```bash
terraform show -json plan.tfplan | terraspin --no-ai
```

## Common workflows

```bash
# Fail CI if risk is high or above
terraspin --fail-on high plan.json

# Rule-based only — no LLM API call
terraspin --no-ai plan.json

# JSON output for scripting
terraspin --format json plan.json

# Show all risk tiers (verbose)
terraspin -v plan.json

# Post analysis as PR comment (auto-detects GitHub/GitLab env)
terraspin --post-comment plan.json
```

## With a config file

Create `.terraspin.yml` in your Terraform workspace — see [Configuration](configuration.md). A config file without an `llm` block still enables the documented Claude defaults; no config file uses rule-based analysis.

## AI providers

The analyze command makes an LLM request only when an `llm` config block exists or the `--llm` flag is set. The diff command uses its own flags and requests an LLM only when its matching API key is present.

```bash
ANTHROPIC_API_KEY="..." terraspin --llm claude plan.json
terraspin --llm ollama --ollama-host http://localhost:11434 plan.json
```

For OpenAI, set `OPENAI_API_KEY` and select `--llm openai`. Analyze falls back to a rule-based narrative when an LLM call fails. `diff` instead prints the diff without an AI narrative when its LLM call fails.

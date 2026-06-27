# Getting Started

## First analysis

```bash
# Generate a plan file
terraform plan -out=plan.tfplan
terraform show -json plan.tfplan > plan.json

# Analyze it
terraspin plan.json
```

## Pipe from terraform

```bash
terraform plan -json | terraspin
```

## Common workflows

```bash
# Fail CI if risk is high or above
terraspin plan.json --fail-on high

# Rule-based only — no LLM API call
terraspin plan.json --no-ai

# JSON output for scripting
terraspin plan.json --format json

# Show all risk tiers (verbose)
terraspin plan.json -v

# Post analysis as PR comment (auto-detects GitHub/GitLab env)
terraspin plan.json --post-comment
```

## With a config file

Create `.terraspin.yml` in your Terraform workspace — see [Configuration](configuration.md).

## AI providers

Terraspin uses Claude by default. Set `ANTHROPIC_API_KEY` and you're done.

To use OpenAI or Ollama:

```bash
terraspin plan.json --llm openai
terraspin plan.json --llm ollama --ollama-host http://localhost:11434
```

If the LLM call fails, Terraspin falls back to rule-based analysis automatically.

# AWS quickstart

Use Terraspin to review a Terraform plan before applying it. The built-in AWS rules inspect plan values locally; they do not query AWS.

## Prerequisites

- Terraform CLI and an AWS Terraform configuration.
- A Terraspin binary. The commands below use Linux x86_64; select the matching release archive for another platform.

## Install a release binary

This example uses the older `v0.1.0` release so the CI example can pin a release that is more than 14 days old. Check [GitHub Releases](https://github.com/terraspin/terraspin/releases) for available versions.

```bash
VERSION=v0.1.0
ASSET="terraspin_${VERSION#v}_linux_amd64.tar.gz"
curl -fLO "https://github.com/terraspin/terraspin/releases/download/${VERSION}/${ASSET}"
curl -fLO "https://github.com/terraspin/terraspin/releases/download/${VERSION}/checksums.txt"
grep " ${ASSET}$" checksums.txt | sha256sum --check -
tar -xzf "$ASSET"
install -m 0755 terraspin "$HOME/.local/bin/terraspin"
```

The release checksums detect corruption, but they are not signed. See [Security Model](security.md) and [Security Policy](../SECURITY.md) before using release artifacts or plan output in CI.

## Analyze a plan

```bash
terraform init
terraform plan -out=plan.tfplan
terraform show -json plan.tfplan > plan.json
terraspin --no-ai plan.json
```

`Policy findings` identify the matching rule, severity, resource address, message, and suggested fix. The overall score and tier summarize the plan risk; use the finding details to decide what to change or explicitly accept.

Use a nonzero exit code to block high-risk plans in CI:

```bash
terraspin --no-ai --fail-on high plan.json
```

The command exits with code 1 when the overall risk is `high` or `critical`.

## Configure rules

Create `.terraspin.yml` in the Terraform workspace to disable a specific built-in rule and add an organization rule. The `[*]` path evaluates every ingress element.

```yaml
version: 1

disable_rules:
  - AWS-SG-002

rules:
  - id: no-public-ingress
    severity: critical
    description: Public ingress needs review
    remediation: Restrict ingress to trusted CIDR ranges
    match:
      resource_type_pattern: aws_security_group
      attribute_path: "ingress[*].cidr_blocks"
      contains: 0.0.0.0/0
```

See [Configuration](configuration.md#built-in-rules) for every rule setting and [Built-in AWS rules](aws-rules.md) for checks and known limitations.

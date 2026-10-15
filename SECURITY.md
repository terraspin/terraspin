# Security Policy

## Supported Versions

Only the latest release receives security fixes.

| Version | Supported |
| --- | --- |
| The latest release | Yes |
| Earlier releases | No |

v0.1.0 has a sensitive-value handling issue that is corrected in the next release.

## Reporting a Vulnerability

Please use [GitHub private vulnerability reporting](https://github.com/terraspin/terraspin/security/advisories/new) rather than opening a public issue. Include a minimal reproduction, affected version, impact, and any suggested mitigation. Do not include credentials or real sensitive plan values.

The maintainer will acknowledge reports and share status updates on a best-effort basis. Response times and remediation timelines are not guaranteed.

## Scope

This policy covers Terraspin source code and official release artifacts from this repository. It does not cover Terraform providers, Terraform/OpenTofu, third-party LLM providers, or infrastructure described by a plan.

Official release artifacts are not currently signed. Each release includes checksums; verify them before use.

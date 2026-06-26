# Terraspin — Product Requirements Document

**Version:** 0.1-draft  
**Date:** June 2026  
**Status:** Draft for Review  
**Authors:** TBD  
**License:** Apache 2.0

---

## Table of Contents

1. [Overview](#1-overview)
2. [Problem Statement](#2-problem-statement)
3. [Goals & Success Metrics](#3-goals--success-metrics)
4. [Target Users](#4-target-users)
5. [Competitive Analysis](#5-competitive-analysis)
6. [Features](#6-features)
7. [Technical Architecture](#7-technical-architecture)
8. [CLI Specification](#8-cli-specification)
9. [Risk Scoring Model](#9-risk-scoring-model)
10. [Configuration Schema](#10-configuration-schema)
11. [Integration Specifications](#11-integration-specifications)
12. [Release Milestones](#12-release-milestones)
13. [Open Questions & Risks](#13-open-questions--risks)
14. [Appendix — Sample Output](#14-appendix--sample-output)

---

## 1. Overview

Terraspin is an open-source CLI tool and AI agent that transforms `terraform plan` output into actionable intelligence. Rather than leaving engineers staring at a wall of resource diffs, Terraspin produces a risk-scored, human-readable analysis that answers not just _what_ will change, but _what it means_, _what could go wrong_, and _how to recover_ if an apply fails midway.

**Tagline:** _"Understand your infrastructure changes before you apply them."_

**One-line pitch:** Terraspin is the AI intelligence layer between `terraform plan` and `terraform apply` — it tells you whether your plan is safe, what will break if it isn't, and how to recover.

**What Terraspin is not:** Terraspin does not run `terraform apply`, manage state, or replace security policy tools. It is a pure analysis layer designed to sit alongside existing tools like Atlantis, Infracost, and Checkov — not compete with them.

---

## 2. Problem Statement

### 2.1 The Core Pain

Every team running Terraform at scale faces a recurring crisis: staring at a `terraform plan` output containing hundreds of resource changes and trying to decide whether it's safe to apply. The native plan output is:

- **Verbose and technically dense** — hundreds of lines of resource attribute diffs with no prioritization
- **Risk-unaware** — treats `delete aws_rds_instance.primary` identically to `update aws_s3_bucket.logs add tag`
- **Context-free** — no understanding of which resources depend on what is changing
- **Inaccessible** — only engineers with deep Terraform expertise can meaningfully interpret it
- **Rollback-silent** — provides zero guidance on recovery if apply fails halfway through

### 2.2 The Real-World Impact

- Senior engineers spend 30–60 minutes reviewing complex plans before high-stakes applies
- Junior engineers and on-call SREs regularly miss critical destructive changes buried in long diffs
- Security and compliance teams cannot audit plans without dedicated Terraform knowledge, creating bottlenecks
- Post-incident reviews repeatedly cite "didn't realize the full impact of that plan" as a contributing root cause
- Teams in regulated industries build manual review checklists as a workaround — checklists that don't scale

### 2.3 The Gap in the Ecosystem

No existing open-source tool answers the core question: _"Is this specific plan safe to apply right now, and what is my recovery path if it isn't?"_

| Tool             | Core question answered                     | What it doesn't answer                                   |
| ---------------- | ------------------------------------------ | -------------------------------------------------------- |
| Infracost        | How much will this cost?                   | Risk, semantic meaning, blast radius, rollback           |
| Atlantis         | How do I automate plan/apply on PRs?       | Intelligence, risk assessment, impact analysis           |
| Checkov / tfsec  | Does this config violate security policy?  | Plan-time blast radius, downstream impact, LLM narrative |
| terraform-docs   | How is this module documented?             | Plan analysis, runtime risk                              |
| Spacelift / env0 | How do I manage Terraform at scale (SaaS)? | Not open source; requires platform migration             |

---

## 3. Goals & Success Metrics

### 3.1 Primary Goal

Become the de facto open-source standard for Terraform plan intelligence — the tool every DevOps team installs alongside Atlantis and Infracost.

### 3.2 Secondary Goal

Establish Terraspin as the canonical Terraform intelligence MCP server, making it native to AI coding assistant workflows (Claude Code, GitHub Copilot, Cursor).

### 3.3 Quantitative Targets

| Metric                                   | 3 months | 6 months | 12 months |
| ---------------------------------------- | -------- | -------- | --------- |
| GitHub stars                             | 500      | 2,000    | 5,000+    |
| GitHub Actions installs (unique repos)   | 100      | 500      | 2,000     |
| CLI downloads (all channels)             | 1,000    | 5,000    | 20,000    |
| External contributors                    | 5        | 20       | 50+       |
| Packages/projects depending on Terraspin | 0        | 5        | 20+       |
| MCP server adopters                      | —        | 100      | 1,000+    |

### 3.4 Qualitative Goals

- Terraspin is mentioned in Terraform community discussions as the natural complement to Atlantis
- At least one major DevOps content creator features Terraspin in a tutorial or demo
- The project receives contributions from engineers at cloud infrastructure companies (HashiCorp, AWS, GCP)

---

## 4. Target Users

### 4.1 Primary — DevOps / Platform Engineer

**Profile:** Runs `terraform plan`/`apply` multiple times per day. Manages 10–200+ Terraform workspaces. Has deep Terraform knowledge but needs to review plans faster and catch things they might miss under time pressure.

**Job to be done:** "I need to review this 200-resource plan change in under 5 minutes and be confident it's safe to apply."

**Current workarounds:** Manual reading, personal mental checklists, asking a colleague to review.

**Pain intensity:** High. This is a daily workflow friction point.

### 4.2 Secondary — SRE / On-Call Engineer

**Profile:** Responsible for production stability. Reviews infrastructure changes before deployment windows. High fear of production incidents from mis-applied plans, especially during incidents when cognitive load is already high.

**Job to be done:** "Is applying this plan going to cause a production outage? What's my rollback if it does?"

**Current workarounds:** Full plan re-read, escalate to the author, delay until safer time.

**Pain intensity:** Very high. Production risk has existential stakes for SREs.

### 4.3 Tertiary — Security / Compliance Engineer

**Profile:** Reviews infrastructure changes for security and compliance requirements. Not a Terraform expert. Currently relies on DevOps teams to translate plan output into something they can audit against controls.

**Job to be done:** "Which security controls are changing in this plan, in plain English, so I can approve or flag it?"

**Current workarounds:** Spreadsheet of questions sent to the DevOps team; manual walkthrough calls.

**Pain intensity:** Medium. Blocked by expertise gap.

### 4.4 Influencer — Engineering Manager / Tech Lead

**Profile:** Approves high-risk infrastructure changes. Needs risk communication in plain business terms. Does not read HCL or care about resource attribute diffs.

**Job to be done:** "Give me a one-paragraph risk summary I can use to approve or escalate this change."

**Usage pattern:** Reads Terraspin's PR comment output; does not run the CLI directly.

---

## 5. Competitive Analysis

### 5.1 Direct Adjacent Tools

**Infracost** (infracost.io, ~11K stars)

- Core value: Cloud cost estimation per plan change
- Strengths: Mature, excellent GitHub integration, widely adopted
- Weaknesses: Cost-only perspective; no risk intelligence, no blast radius, no narrative
- Terraspin positioning: Complementary. Infracost answers "how much?"; Terraspin answers "how risky?" Both belong in the same PR comment thread.

**Atlantis** (runatlantis.io, ~7.5K stars)

- Core value: Automates `terraform plan` and `apply` via PR comments
- Strengths: Mature automation, locked workspace management, broad adoption
- Weaknesses: No plan intelligence — it runs the plan but tells you nothing about what the plan means
- Terraspin positioning: Complementary. Atlantis is the runner; Terraspin is the analyst. Terraspin posts its intelligence comment immediately after Atlantis posts the raw plan.

**Checkov / tfsec / Trivy**

- Core value: Static analysis of Terraform configuration against policy rules
- Strengths: Fast, policy-driven, good CI integration
- Weaknesses: Pre-plan static analysis only; not plan-time; no AI narrative; no blast radius
- Terraspin positioning: Complementary. They catch config problems before plan; Terraspin provides runtime intelligence on the plan itself.

**Spacelift / env0 / Scalr**

- Core value: Full Terraform management platforms with UI, RBAC, policy engine
- Strengths: Comprehensive, enterprise-ready
- Weaknesses: SaaS/paid, requires platform migration, not open-source
- Terraspin positioning: OSS alternative intelligence layer. Teams on these platforms still benefit from Terraspin's CLI and CI integration.

### 5.2 Indirect Competitors

**GitHub Copilot / Claude Code** — Can analyze plan output when pasted manually, but no native Terraform plan awareness, no persistent rules, no blast radius calculation, no CI integration. Terraspin as MCP server bridges this gap and makes Terraspin the intelligence layer that powers these assistants.

### 5.3 Competitive Moat

Terraspin's defensibility is built on four pillars:

1. **Community rules library** — organization-contributed risk rules for industry-specific resources become a shared knowledge base that improves over time
2. **MCP first-mover** — first canonical Terraform intelligence MCP server creates deep integration with AI coding tools
3. **Pipeline switching cost** — once embedded in CI/CD across an org, replacement friction is high
4. **Data flywheel (opt-in)** — anonymized risk pattern analysis can continuously improve the scoring model

---

## 6. Features

### 6.1 MVP — v0.1 (Month 1–2)

---

#### F1 — Plan Parser

**What it does:** Parses `terraform show -json <planfile>` output into an internal `PlanAST` representation that all downstream components consume.

**Input:** JSON from `terraform show -json plan.tfplan` or `terraform plan -json` (streaming).

**Internal output — `PlanAST` struct:**

- All resource changes with `before` / `after` state and change action
- Provider and workspace metadata
- Resource dependency graph (parsed from `references` fields)
- Sensitive value paths (for redaction before LLM transmission)
- Plan-level variables and output changes

**Terraform version support:** 1.0+ and OpenTofu 1.6+.

**Acceptance criteria:**

- Processes a 10MB plan file in under 2 seconds on standard hardware
- Handles plans with 0 to 5,000+ resource changes without memory issues
- Surfaces parsing failures with clear, actionable error messages (not stack traces)
- Correctly identifies `force-replace` annotations and marks affected resources accordingly
- Handles multi-module plans by flattening resource addresses to `module.path.resource_type.name`

---

#### F2 — Risk Scorer

**What it does:** Assigns a deterministic risk score (0–100) and tier (critical / high / medium / low) to each resource change and to the plan as a whole.

**Scoring model:** See Section 9 (Risk Scoring Model) for the full specification.

**Acceptance criteria:**

- Scoring is deterministic: identical input always produces identical output
- Supports AWS, GCP, Azure, Kubernetes, and Datadog providers in v0.1
- Unknown resource types receive a `1.0×` multiplier with a logged warning so operators can submit rules
- Custom multipliers are configurable via `.terraspin.yml` (see F7)
- Plan-level risk is the highest individual resource tier with count metadata per tier

---

#### F3 — AI Narrative Engine

**What it does:** Generates a structured plain-English risk briefing using an LLM. The briefing is designed to be readable by engineers, security teams, and engineering managers without Terraform expertise.

**Preprocessing — what is sent to the LLM:**
Terraspin does not send raw plan JSON to the LLM. It sends a preprocessed plan summary that:

- Contains only changed resources (no-ops excluded)
- Replaces all sensitive values with `[SENSITIVE REDACTED]`
- Includes computed risk scores and blast radius data from F2/F4
- Is structured as a prompt-optimized text document (not JSON)

This preprocessing reduces token usage by 80–95% vs raw JSON and ensures sensitive data never leaves the machine unredacted.

**Output structure:**

```
## Summary
[2–3 sentences: what this plan does at a business level]

## Critical changes
[Bulleted list of critical/high risk changes with plain-English explanation of impact]

## Risk assessment
[Why this plan is risky or safe; what specific aspects warrant attention]

## Recommended checks before applying
[Specific, actionable steps to take before running terraform apply]

## Rollback strategy
[Step-by-step recovery plan if apply fails or causes unexpected behavior]
```

**LLM provider support (v0.1):**

- Anthropic Claude claude-sonnet-4-6 (default)
- OpenAI GPT-4o
- Ollama local inference (llama3.2, mistral, codellama) via `--llm ollama`

**Acceptance criteria:**

- Narrative generated in under 15 seconds for typical plans (< 100 changed resources)
- `--no-ai` flag produces a rule-based summary without any LLM call (free, offline-safe)
- LLM API failures (rate limits, auth errors, timeouts) fall back to rule-based summary with an explicit warning
- Sensitive values are verified as redacted via unit tests against sensitive plan fixtures before each release

---

#### F4 — Blast Radius Analyzer

**What it does:** For each changed resource, identifies which other resources in the plan depend on it and surfaces the cascading impact.

**Method:**

1. Parse `resource_changes` and `planned_values.root_module.resources` from the plan AST to build a directed dependency graph
2. Traverse the graph from each changed resource to identify direct dependents (1 hop) and transitive dependents (2+ hops)
3. Surface resources that will be destroyed or forced-replace as a side effect of the primary change
4. Include module-level blast radius for module-level changes

**Example output:**

```
aws_security_group.db [UPDATE — inbound rule added]
  └── Blast radius: 3 resources directly affected
       ├── aws_db_instance.primary — inbound access rule changed [CRITICAL]
       ├── aws_lambda_function.api[0..3] — SG association, will reconnect
       └── aws_security_group_rule.db_egress — dependent rule
```

**Acceptance criteria:**

- Correctly follows both explicit `depends_on` and implicit resource reference dependencies
- Detects circular dependencies and surfaces them as warnings rather than crashing
- Handles module references by flattening to full resource addresses
- Blast radius included in both terminal output and LLM narrative context

---

#### F5 — CLI Output (TUI)

**What it does:** Renders analysis results as a rich, colored terminal UI using charmbracelet/lipgloss. Degrades gracefully in CI environments without color support.

**Terminal output sections:**

1. Plan header — workspace, resource counts by action (create/update/delete)
2. Overall risk scorecard — tier badge + per-tier resource counts
3. Critical changes — expanded view with blast radius and plain-English impact
4. High changes — collapsed by default, expanded with `-v`
5. AI narrative — full risk briefing
6. Recommended actions — checklist format

**Output format flags:**

- `--format text` — default colored TUI (CI-safe: colors stripped when no TTY)
- `--format json` — machine-readable structured JSON (see schema in Section 8)
- `--format markdown` — GitHub-flavored markdown for PR comments and Slack
- `--format compact` — single-line risk summary for CI log lines

**Acceptance criteria:**

- Single binary with no runtime dependencies
- Compiles and runs on macOS (arm64/amd64), Linux (amd64/arm64), Windows (amd64)
- Color codes stripped automatically when stdout is not a TTY (CI log safety)
- JSON output validates against published JSON schema

---

#### F6 — GitHub Action

**What it does:** Official `terraspin/terraspin-action` GitHub Action that integrates Terraspin into CI/CD pipelines and posts analysis as a PR comment.

**Usage:**

```yaml
- name: Terraspin plan analysis
  uses: terraspin/terraspin-action@v1
  with:
    plan-file: plan.json # path to terraform show -json output
    fail-on: high # exit 1 if risk >= this tier
    post-comment: true # post analysis as PR comment
    llm-provider: claude # claude | openai | ollama
    format: markdown # output format for PR comment
  env:
    ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**PR comment structure:**

- Terraspin header with badge and risk tier
- Collapsible risk scorecard table
- Critical changes section (always expanded)
- Full AI narrative inside `<details>` block (expanded for critical risk)
- "View full report" footer with run link

**Comment behavior:**

- On first run: creates new comment tagged with Terraspin identifier
- On subsequent pushes to same PR: edits existing comment (does not create duplicates)
- On PR merge or close: comment is preserved for audit

**Acceptance criteria:**

- Action installs and runs in under 45 seconds including binary download
- Works with GitHub Enterprise Server (configurable API endpoint)
- Works with self-hosted runners including arm64
- No secrets logged in any circumstance
- `fail-on` correctly causes the step to exit 1, failing the CI check

---

### 6.2 Post-MVP — v0.2 (Month 3)

#### F7 — Custom Rule Engine

Configuration-driven rules for organization-specific risk policies, defined in `.terraspin.yml`.

```yaml
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
    description: "Deletions in prod require manual review"
    match:
      action: delete
      workspace_pattern: "prod-*"

  - id: open-ssh-warning
    severity: high
    description: "SSH open to 0.0.0.0/0 detected"
    match:
      resource_type: "aws_security_group_rule"
      attribute_path: "cidr_blocks"
      contains: "0.0.0.0/0"
      attribute_path_2: "from_port"
      value_2: 22
```

#### F8 — GitLab CI + Merge Request Integration

Equivalent of the GitHub Action for GitLab CI/CD pipelines. Posts analysis as a Merge Request note via GitLab API.

#### F9 — Slack / Teams Webhook

Push plan analysis to a Slack channel or Teams channel on plan generation. Configurable notification threshold (only notify on high or critical). Uses Block Kit for Slack formatting.

---

### 6.3 v0.3 — AI Agent Layer (Month 4–5)

#### F10 — MCP Server Mode

Run Terraspin as a Model Context Protocol server, exposing plan intelligence as tools consumable by AI coding assistants.

```bash
terraspin serve                    # default: stdio transport
terraspin serve --transport sse --port 8080   # SSE transport
```

**MCP tools exposed:**

`analyze_plan`

- Input: `plan_json: string` (raw terraform show JSON)
- Output: Full `PlanIntelligence` object including risk scores, blast radii, and narrative
- Use case: "Analyze this terraform plan and tell me if it's safe to apply"

`get_blast_radius`

- Input: `resource_address: string`, `plan_json: string`
- Output: Blast radius for a specific resource
- Use case: "What will break if I delete aws_rds_instance.primary?"

`explain_change`

- Input: `resource_address: string`, `plan_json: string`
- Output: Plain-English explanation of a single resource change
- Use case: "What exactly is changing in the aws_security_group.web resource?"

`get_risk_summary`

- Input: `plan_json: string`
- Output: Lightweight risk scorecard (no full narrative — fast)
- Use case: Quick check during development iteration

`suggest_rollback`

- Input: `plan_json: string`, `failure_point: string` (optional)
- Output: Structured rollback strategy
- Use case: "If this apply fails after the RDS deletion, how do I recover?"

---

### 6.4 v0.4 — Drift Intelligence (Month 6)

#### F11 — Environment Drift Analysis

Compare terraform state (or plan outputs) across two environments and produce an AI-powered explanation of the semantic differences.

```bash
terraspin diff \
  --state-a s3://mybucket/prod/terraform.tfstate \
  --state-b s3://mybucket/staging/terraform.tfstate \
  --format markdown
```

**Output:** Side-by-side diff of resource configurations, AI explanation of why environments diverge, classification of each drift as expected vs unexpected, and suggested remediation.

---

## 7. Technical Architecture

### 7.1 Repository Structure

```
terraspin/
├── cmd/
│   ├── analyze.go          (terraspin analyze)
│   ├── diff.go             (terraspin diff)
│   ├── serve.go            (terraspin serve — MCP mode)
│   ├── config.go           (terraspin config init)
│   └── root.go             (global flags, version)
├── internal/
│   ├── parser/             (plan JSON → PlanAST)
│   ├── analyzer/
│   │   ├── risk.go         (risk scorer)
│   │   └── blast.go        (blast radius graph traversal)
│   ├── ai/
│   │   ├── preprocess.go   (plan summarization + redaction)
│   │   ├── prompt.go       (prompt construction + few-shot examples)
│   │   ├── providers/      (claude.go, openai.go, ollama.go)
│   │   └── narrative.go    (narrative struct + parsing)
│   ├── formatter/
│   │   ├── text.go         (lipgloss TUI)
│   │   ├── markdown.go     (GitHub PR comment markdown)
│   │   ├── json.go         (machine-readable output)
│   │   └── compact.go      (single-line CI output)
│   ├── integrations/
│   │   ├── github.go       (PR comment post/update)
│   │   ├── gitlab.go       (MR note post/update)
│   │   ├── slack.go        (webhook delivery)
│   │   └── mcp/            (MCP server implementation)
│   └── config/             (config file loading, validation)
├── pkg/
│   └── terraspin/          (public Go API — for use as a library)
├── testdata/               (real plan JSON fixtures for testing)
│   ├── aws/
│   ├── gcp/
│   ├── azure/
│   └── kubernetes/
├── action/                 (GitHub Action source, Dockerfile)
├── docs/                   (documentation site source)
└── .goreleaser.yml         (binary release config)
```

### 7.2 Technology Stack

| Component           | Technology                    | Rationale                                                        |
| ------------------- | ----------------------------- | ---------------------------------------------------------------- |
| Language            | Go 1.22+                      | DevOps ecosystem standard; single static binary; Terraform is Go |
| CLI framework       | cobra + viper                 | Industry standard for Go CLI tools                               |
| TUI / output        | charmbracelet/lipgloss        | Modern DevOps TUI standard (k9s, lazygit)                        |
| JSON parsing        | encoding/json + tidwall/gjson | gjson for fast path extraction in large plan files               |
| LLM integration     | Raw net/http (no vendor SDK)  | Provider-agnostic; avoids SDK version lock-in                    |
| GitHub integration  | google/go-github              | Official, well-maintained                                        |
| Binary distribution | goreleaser                    | Multi-arch, Homebrew, deb/rpm, checksums, SBOM                   |
| MCP server          | mark3labs/mcp-go              | Idiomatic Go MCP server library                                  |
| Testing             | testify + golden file testing | Readable assertions; golden files for output stability           |
| Linting             | golangci-lint                 | Comprehensive, CI-ready                                          |
| Release signing     | cosign                        | Supply chain security; verifiable binary provenance              |

### 7.3 Why Go over Rust

Rust would provide marginal CPU performance gains that do not apply to Terraspin's actual bottlenecks:

- **Bottleneck is network, not CPU.** The rate-limiting step is LLM API latency (3–15 seconds), not plan JSON parsing (< 200ms). Rust's performance advantage is irrelevant.
- **Ecosystem alignment.** Terraform, kubectl, Helm, ArgoCD, and Crossplane are all Go. DevOps contributors already know Go. Easier onboarding means more external contributions.
- **Distribution simplicity.** `goreleaser` produces multi-arch static binaries, Homebrew formulae, deb/rpm packages, and Docker images in one command. Rust's ecosystem is approaching this but Go's is more mature.
- **Faster iteration.** Early-stage OSS projects need to ship quickly to build community. Go's compilation speed and tooling are optimized for this.

Revisit Rust if Terraspin evolves to process thousands of concurrent plans in a server context (e.g., a hosted SaaS tier) where CPU-bound plan parsing becomes the bottleneck.

### 7.4 Core Data Models

```go
// PlanAST is the internal representation of a terraform plan JSON
type PlanAST struct {
    TerraformVersion string
    FormatVersion    string
    Variables        map[string]Variable
    Changes          []ResourceChange
    OutputChanges    map[string]OutputChange
    DependencyGraph  *DependencyGraph
    ProviderMetadata map[string]ProviderMeta
    PlanHash         string  // SHA256 of raw plan for caching
}

// ResourceChange represents a single resource change in the plan
type ResourceChange struct {
    Address        string
    ModulePath     string
    Type           string        // e.g., "aws_instance"
    Name           string        // e.g., "web_server"
    ProviderName   string
    Action         ChangeAction  // create | update | delete | replace | no-op | read
    ActionReason   string        // e.g., "replacement requested"
    Before         map[string]any
    After          map[string]any
    SensitivePaths []string      // paths to redact before LLM
    ForceReplace   bool
}

// PlanIntelligence is the full analysis output
type PlanIntelligence struct {
    Plan            *PlanAST
    OverallRisk     RiskScore
    ResourceRisks   []ResourceRiskScore
    BlastRadii      map[string]BlastRadius  // keyed by resource address
    Narrative       *Narrative
    Recommendations []string
    RollbackPlan    string
    AnalyzedAt      time.Time
    AnalysisDuration time.Duration
}

// RiskScore holds a scored risk assessment
type RiskScore struct {
    Score     float64   // 0.0 to 100.0
    Tier      RiskTier  // critical | high | medium | low
    Reasoning string
}

// BlastRadius holds the dependency impact of a resource change
type BlastRadius struct {
    RootAddress    string
    DirectDeps     []DependentResource
    TransitiveDeps []DependentResource
    TotalAffected  int
}

// Narrative holds the LLM-generated briefing
type Narrative struct {
    Summary          string
    CriticalChanges  []string
    RiskAssessment   string
    Recommendations  []string
    RollbackStrategy string
    Provider         string    // "claude-sonnet-4-6", "gpt-4o", "llama3", "rule-based"
    GeneratedAt      time.Time
    TokensUsed       int
}
```

### 7.5 LLM Integration Design

Terraspin sends a **preprocessed plan summary** to the LLM, never the raw JSON. The preprocessing pipeline:

1. Extract only changed resources (skip `no-op` and `read`)
2. Redact all sensitive values — any path listed in `SensitivePaths` or matching config-level redaction patterns
3. Attach computed risk scores and blast radius data to each resource entry
4. Serialize to a compact structured text format (not JSON — reduces token count and improves completion quality)
5. Inject into a few-shot prompt with 2–3 example plans and expected outputs
6. Enforce JSON output format for reliable parsing

**Token budget for claude-sonnet-4-6:**

- System prompt + few-shot examples: ~2,000 tokens
- Preprocessed plan summary: 500–8,000 tokens depending on plan size
- Target output: ~1,000 tokens

For very large plans (1,000+ changed resources), batch by risk tier: send critical + high resources in full, summarize medium/low as counts with resource types only.

### 7.6 Security Considerations

**Sensitive value redaction:** All values on paths marked `sensitive: true` in the plan JSON are replaced with `[SENSITIVE REDACTED]` in a pre-processing step that runs before any serialization. This is tested via unit tests against fixtures containing sensitive values; any regression blocks release.

**No plan data persistence:** Terraspin does not log, store, or transmit plan data except to the explicitly configured LLM provider. No telemetry collected without explicit opt-in.

**Local LLM option:** `--llm ollama` mode performs all inference locally. Zero data leaves the machine. This is the recommended configuration for air-gapped environments, financial services, and healthcare.

**Binary supply chain:** All release binaries are signed with cosign. SHA256 checksums and SBOM are published alongside every release. Verify with: `cosign verify-blob --cert terraspin.pem terraspin_linux_amd64`.

**API key handling:** LLM API keys are accepted only via environment variables (never CLI flags or config files). Keys are never logged. A startup check verifies the key is valid before processing begins.

---

## 8. CLI Specification

### 8.1 Commands

```
terraspin analyze <plan-file|--> [flags]

  Analyze a terraform plan file and produce risk intelligence.
  Use '--' to read from stdin: terraform plan -json | terraspin analyze --

  Flags:
    -f, --format string         Output format: text|json|markdown|compact (default "text")
        --fail-on string        Exit 1 if plan risk meets this tier: critical|high|medium|low
        --no-ai                 Rule-based analysis only, skip LLM call
        --llm string            LLM provider: claude|openai|ollama (default "claude")
        --model string          Model name (default "claude-sonnet-4-6")
        --ollama-host string    Ollama host URL (default "http://localhost:11434")
    -v, --verbose               Show all risk tiers including medium and low
    -c, --config string         Config file path (default ".terraspin.yml")
    -o, --output string         Write output to file instead of stdout
        --post-comment          Post analysis as GitHub PR comment
        --pr-number int         GitHub PR number (auto-detected from CI env vars)
        --repo string           GitHub repo "owner/name" (auto-detected from CI)

---

terraspin diff <state-a> <state-b> [flags]

  Compare two terraform state files or plan outputs and explain differences.

  Flags:
    -f, --format string         Output format: text|json|markdown (default "text")
        --no-ai                 Rule-based diff only

---

terraspin serve [flags]

  Run Terraspin as a Model Context Protocol (MCP) server.

  Flags:
        --transport string      MCP transport: stdio|sse (default "stdio")
        --port int              SSE port (default 8080, only used with --transport sse)
        --host string           SSE host (default "localhost")

---

terraspin config init [flags]

  Create a .terraspin.yml config file with defaults in the current directory.

---

terraspin version

  Print version, build date, and Go runtime info.
```

### 8.2 Exit Codes

| Code | Meaning                                                           |
| ---- | ----------------------------------------------------------------- |
| 0    | Analysis complete; no `--fail-on` threshold exceeded              |
| 1    | Analysis complete; `--fail-on` threshold exceeded                 |
| 2    | Input file not found, unreadable, or invalid plan JSON            |
| 3    | LLM API error when `--no-ai` is not set and fallback is disabled  |
| 4    | GitHub PR comment post/update failed                              |
| 5    | Configuration file error (invalid schema, missing required field) |

### 8.3 JSON Output Schema

```json
{
  "$schema": "https://terraspin.dev/schema/plan-intelligence/v1.json",
  "schema_version": "1",
  "analyzed_at": "2026-06-15T14:23:00Z",
  "plan": {
    "terraform_version": "1.7.2",
    "workspace": "production",
    "resource_counts": { "create": 8, "update": 12, "delete": 3 }
  },
  "overall_risk": {
    "tier": "critical",
    "score": 92.0,
    "reasoning": "Plan includes deletion of a production RDS instance"
  },
  "resource_risks": [
    {
      "address": "aws_db_instance.primary",
      "action": "delete",
      "risk_score": 92.5,
      "risk_tier": "critical",
      "blast_radius": {
        "direct_dependents": 4,
        "transitive_dependents": 2,
        "total_affected": 6
      }
    }
  ],
  "narrative": {
    "summary": "This plan deletes the primary production RDS instance...",
    "critical_changes": ["aws_db_instance.primary will be permanently deleted"],
    "risk_assessment": "...",
    "recommendations": ["Verify RDS snapshot exists and is recent"],
    "rollback_strategy": "1. Restore from snapshot...",
    "provider": "claude-sonnet-4-6",
    "tokens_used": 847
  }
}
```

---

## 9. Risk Scoring Model

### 9.1 Base Scores by Change Action

| Action                         | Base Score | Description                                    |
| ------------------------------ | ---------- | ---------------------------------------------- |
| `no-op`                        | 0          | No change                                      |
| `read`                         | 0          | Data source read only                          |
| `create`                       | 10         | New resource provisioned                       |
| `update` in-place              | 20         | Attribute change, resource not replaced        |
| `update` requires replace      | 70         | Attribute change triggers destroy-create cycle |
| `delete`                       | 75         | Resource permanently destroyed                 |
| `replace` (delete then create) | 90         | Force destroy + recreate                       |

### 9.2 Resource Type Risk Multipliers

| Resource Category                   | Multiplier | Examples                                                                 |
| ----------------------------------- | ---------- | ------------------------------------------------------------------------ |
| Databases                           | 3.0×       | `aws_rds_instance`, `aws_dynamodb_table`, `google_sql_database_instance` |
| DNS and routing                     | 2.5×       | `aws_route53_record`, `google_dns_record_set`                            |
| IAM and access control              | 2.5×       | `aws_iam_role`, `aws_iam_policy`, `google_project_iam_binding`           |
| Network and VPC                     | 2.5×       | `aws_vpc`, `aws_subnet`, `google_compute_network`                        |
| Security groups and firewall        | 2.5×       | `aws_security_group`, `google_compute_firewall`                          |
| Object storage with `force_destroy` | 2.5×       | `aws_s3_bucket` (when `force_destroy = true`)                            |
| Load balancers                      | 2.0×       | `aws_lb`, `aws_alb`, `google_compute_forwarding_rule`                    |
| Storage volumes                     | 2.0×       | `aws_ebs_volume`, `google_compute_disk`                                  |
| Object storage (no force_destroy)   | 1.8×       | `aws_s3_bucket`                                                          |
| CDN and caching                     | 1.5×       | `aws_cloudfront_distribution`, `google_compute_backend_service`          |
| Compute instances                   | 1.5×       | `aws_instance`, `google_compute_instance`                                |
| Container services                  | 1.4×       | `aws_ecs_service`, `google_container_cluster`                            |
| Kubernetes resources                | 1.4×       | `kubernetes_deployment`, `kubernetes_service`                            |
| Serverless functions                | 1.2×       | `aws_lambda_function`, `google_cloudfunctions_function`                  |
| Default (unknown type)              | 1.0×       | Any resource not in the above list                                       |

**Final score:** `min(100, base_score × multiplier)`

### 9.3 Risk Tiers

| Tier     | Score Range | Color  | Meaning                                                       |
| -------- | ----------- | ------ | ------------------------------------------------------------- |
| Critical | 85–100      | Red    | Likely causes outage or data loss; requires explicit sign-off |
| High     | 60–84       | Orange | Significant production impact; senior review required         |
| Medium   | 30–59       | Yellow | Moderate risk; standard review process                        |
| Low      | 0–29        | Green  | Routine change; standard diff review sufficient               |

### 9.4 Plan-Level Risk

The plan-level risk tier is the highest tier present among all resource changes. The score includes metadata on the count of resources per tier to give context to the overall tier.

Example: Plan-level `CRITICAL` with `{critical: 1, high: 4, medium: 12, low: 22}` is treated differently (review the one critical) than `CRITICAL` with `{critical: 15}` (review everything).

---

## 10. Configuration Schema

```yaml
# .terraspin.yml
# Full reference configuration

version: 1

# LLM provider configuration
llm:
  provider: claude # claude | openai | ollama
  model: claude-sonnet-4-6 # provider-specific model name
  timeout: 30s # LLM request timeout
  max_retries: 2 # retries on rate limit / transient error
  fallback_to_rules: true # fall back to rule-based if LLM unavailable

# Risk behavior
risk:
  fail_on: high # CI gate: exit 1 if risk >= this tier

# Custom resource type risk multipliers
# These override the built-in multipliers
resource_risk:
  aws_s3_bucket:
    multiplier: 3.0 # Our org treats S3 as critical (financial data)
  some_custom_resource:
    multiplier: 2.5

# Custom rules (evaluated after scoring, can escalate tier)
rules:
  - id: no-public-rds
    severity: critical
    description: "Database must not be publicly accessible per SOC2 CC6.1"
    match:
      resource_type_pattern: "*_db_instance"
      attribute_path: "publicly_accessible"
      value: true

  - id: prod-deletion-gate
    severity: critical
    description: "Any deletion in production requires out-of-band approval"
    match:
      action: delete
      workspace_pattern: "prod-*"

  - id: open-ssh
    severity: high
    description: "SSH must not be open to 0.0.0.0/0"
    match:
      resource_type: "aws_security_group_rule"
      attribute_path: "cidr_blocks[*]"
      contains: "0.0.0.0/0"

# Additional value paths to redact before LLM transmission
# Terraspin already redacts plan-marked sensitive values;
# add extra patterns here for defense in depth
redact_paths:
  - "**password**"
  - "**secret**"
  - "**api_key**"
  - "**private_key**"

# GitHub integration
github:
  token_env: GITHUB_TOKEN # env var name containing the token
  update_existing_comment: true # update vs create new comment on re-run
  comment_tag: "<!-- terraspin -->" # HTML tag to identify Terraspin comments

# Slack integration
slack:
  webhook_url_env: SLACK_WEBHOOK_URL
  notify_on: [critical, high] # tiers that trigger Slack notification
  channel: "#infra-changes"

# Ollama configuration (for --llm ollama)
ollama:
  host: http://localhost:11434
  model: llama3.2
```

---

## 11. Integration Specifications

### 11.1 Atlantis Integration

Terraspin does not replace Atlantis. The recommended pipeline when both tools are used:

1. PR opened → Atlantis runs `terraform plan`, posts raw plan as PR comment
2. Atlantis webhooks (or CI step) triggers `terraspin analyze plan.json --post-comment`
3. Terraspin posts a second PR comment with risk analysis
4. Team reviews both comments before running `atlantis apply`

No Atlantis configuration changes required. Terraspin can be added as a step in `.atlantis.yml` `post_plan_commands` or as a separate CI job.

### 11.2 GitHub Actions Workflow Example

```yaml
name: Terraform plan and analysis
on:
  pull_request:
    paths:
      - "**.tf"
      - "**.tfvars"

jobs:
  plan-and-analyze:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write

    steps:
      - uses: actions/checkout@v4

      - uses: hashicorp/setup-terraform@v3
        with:
          terraform_version: "1.7.2"

      - name: Terraform init and plan
        run: |
          terraform init
          terraform plan -out=plan.tfplan
          terraform show -json plan.tfplan > plan.json
        env:
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}

      - name: Terraspin analysis
        uses: terraspin/terraspin-action@v1
        with:
          plan-file: plan.json
          fail-on: high
          post-comment: true
          llm-provider: claude
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### 11.3 MCP Server Configuration (Claude Code)

```json
{
  "mcpServers": {
    "terraspin": {
      "command": "terraspin",
      "args": ["serve", "--transport", "stdio"],
      "env": {
        "ANTHROPIC_API_KEY": "your-key-here"
      }
    }
  }
}
```

After configuration, engineers can use natural language in Claude Code:

- "Analyze the terraform plan in `plan.json` and tell me if it's safe to apply"
- "What resources depend on `aws_db_instance.primary` according to the plan?"
- "Generate a rollback strategy for this plan"

---

## 12. Release Milestones

### v0.1 — MVP (Month 1–2)

**Milestone goal:** A working CLI that delivers immediate daily value to Terraform engineers.

**Sprint 0 (Weeks 1–2) — Foundations:**

- Project scaffold: cobra CLI, viper config, makefile, goreleaser config
- Terraform plan JSON parser → `PlanAST`
- Basic risk scorer (rule-based, no LLM)
- Terminal output with lipgloss (colors, badges)
- Unit tests for parser and scorer with real fixture files from AWS, GCP, Azure

**Sprint 1 (Weeks 3–4) — Intelligence:**

- LLM integration: Claude (default), OpenAI, Ollama
- Preprocessing pipeline (plan summarization + sensitive value redaction)
- AI narrative generator with few-shot prompting
- Blast radius analyzer (dependency graph traversal)
- `--no-ai` flag for offline/rule-based mode

**Sprint 2 (Weeks 5–6) — Integrations:**

- GitHub Action (`terraspin/terraspin-action`)
- PR comment generator (markdown output)
- `.terraspin.yml` config schema and validation
- `--fail-on` flag for CI gate
- JSON output format

**Sprint 3 (Weeks 7–8) — Launch Readiness:**

- Comprehensive error handling with actionable messages
- 80%+ unit test coverage on core packages
- Integration tests against real Terraform plan fixtures
- README with animated demo GIF (Terraspin #1 viral asset)
- Homebrew tap: `brew install terraspin/tap/terraspin`
- goreleaser: binary releases for linux/darwin/windows × amd64/arm64
- cosign signing and SBOM for all binaries

**Launch channels:** Hacker News Show HN, r/devops, r/terraform, r/kubernetes, Terraform Discord, DevOps Weekly newsletter, X/Twitter DevOps community

---

### v0.2 — Integrations (Month 3)

**Milestone goal:** Production-ready for enterprise CI/CD pipelines.

**Deliverables:**

- Custom rule engine with `.terraspin.yml` (F7)
- GitLab CI + Merge Request integration (F8)
- Slack / Teams webhook (F9)
- Compact output format for CI log lines
- Documentation site at terraspin.dev

---

### v0.3 — AI Agent Layer (Month 4–5)

**Milestone goal:** First-mover MCP Terraform intelligence server.

**Deliverables:**

- MCP server mode with all 5 tools (F10)
- Ollama integration (local LLM, air-gapped environments)
- Published MCP registry entry
- Integration guides for Claude Code, GitHub Copilot, Cursor

---

### v0.4 — Drift Intelligence (Month 6)

**Milestone goal:** Expand addressable use cases beyond plan analysis.

**Deliverables:**

- `terraspin diff` command for environment comparison (F11)
- Cross-environment drift classification (expected vs unexpected)
- Drift summary PR/MR comment format

---

## 13. Open Questions & Risks

### 13.1 Technical Risks

| Risk                                                   | Probability | Impact   | Mitigation                                                                                                                            |
| ------------------------------------------------------ | ----------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| Terraform plan JSON schema changes across versions     | Medium      | High     | Pin to documented format; test against fixtures from 1.0–1.8 and OpenTofu 1.6+ on every CI run                                        |
| LLM hallucination in risk narrative                    | Medium      | Medium   | Rule-based risk score is always the authoritative signal; LLM provides supplementary interpretation only. `--no-ai` always available. |
| Large plans (5,000+ changes) exceed LLM context limits | Medium      | Medium   | Batching: send critical+high in full, summarize medium/low by resource type count                                                     |
| Sensitive value leakage through incomplete redaction   | Low         | Critical | Redaction is a pre-processing step verified by unit tests on sensitive fixtures; any test failure blocks the release pipeline         |
| goreleaser + cosign supply chain attack                | Low         | Critical | Pin all tool versions in CI; verify cosign signatures; SBOM published for each release                                                |

### 13.2 Product Risks

| Risk                                               | Probability | Impact | Mitigation                                                                                    |
| -------------------------------------------------- | ----------- | ------ | --------------------------------------------------------------------------------------------- |
| Infracost launches competing risk analysis feature | Medium      | High   | Accelerate MCP integration (unique moat); build community before this happens                 |
| "Yet another Terraform tool" fatigue               | High        | Medium | Demo-first marketing; README GIF must show the value in 10 seconds                            |
| LLM API costs deter adoption                       | Medium      | Medium | `--no-ai` mode is always free; Ollama support for zero-cost local inference                   |
| Low external contribution after launch             | High        | Medium | Curated `good-first-issue` track; public roadmap in GitHub Projects; contributor office hours |

### 13.3 Open Questions

1. **LLM response caching:** Should Terraspin cache LLM responses keyed by `PlanHash` to reduce cost and latency for repeated analysis of the same plan? Adds complexity; reduces cost for teams that re-analyze without changes.

2. **Opt-in telemetry program:** Should there be an opt-in program for teams to contribute anonymized risk patterns (resource type + risk tier + actual outcome) to improve the scoring model over time? Clear community benefit vs privacy concerns.

3. **MCP server binary:** Should the MCP server be a subcommand (`terraspin serve`) or a separate binary (`terraspin-mcp`)? Subcommand is simpler; separate binary keeps the main binary small.

4. **Provider scope:** Should v0.1 explicitly scope to AWS, GCP, and Azure only, or attempt all providers with unknown-multiplier fallback? Focused scope reduces support surface.

5. **Pulumi / CDK support:** Long-term, should Terraspin support Pulumi preview output and CDK diff output? Significantly broadens TAM but dilutes the Terraform-native positioning.

---

## 14. Appendix — Sample Output

### Terminal Output (text format)

```
┌─ Terraspin Plan Analysis ─────────────────────────────────────────────────────┐
│  plan.json  ·  workspace: production  ·  47 resources  ·  terraform 1.7.2    │
└───────────────────────────────────────────────────────────────────────────────┘

  Overall Risk  ██████████  CRITICAL (score: 92.5)

  8 to create  ·  12 to update  ·  3 to delete  ·  24 unchanged

──── Critical Changes ──────────────────────────────────────────────────────────

  [CRITICAL 92.5]  aws_db_instance.primary  →  DELETE
  Blast radius: 6 resources affected
    ├── aws_lambda_function.api[0..3]  connection string will break
    ├── aws_security_group.db          will be deleted as dependent
    └── module.app.aws_ssm_parameter.db_url  will be deleted

  [CRITICAL 87.5]  aws_security_group.web  →  UPDATE
  Port 0.0.0.0/0 added to inbound rules — all traffic exposed

──── AI Risk Briefing ──────────────────────────────────────────────────────────

  Summary
  This plan permanently deletes the primary production RDS instance and opens
  the web security group to all inbound traffic. Both changes are independently
  critical; together they represent a high-severity production risk.

  Risk assessment
  The RDS deletion will immediately sever the database connection for 4 Lambda
  functions serving production traffic. There is no replacement instance in this
  plan — this is a permanent deletion. The security group change likely introduces
  a security vulnerability by removing port restrictions.

  Recommended checks before applying
    □  Confirm a recent RDS snapshot exists (< 1 hour old)
    □  Get explicit sign-off from security team on SG change
    □  Validate no traffic is hitting production before apply window
    □  Have rollback runbook open and tested

  Rollback strategy
    1.  Restore RDS from most recent snapshot (est. 15–30 min)
    2.  Update Lambda env vars with restored instance endpoint
    3.  Revert security group via previous Terraform state
    4.  Verify Lambda connectivity end-to-end before declaring resolved

──── 4 High  ·  12 Medium  ·  22 Low ──────────────────────────── run -v to show ─
```

---

_Terraspin is open-source under the Apache 2.0 license._  
_Contributions, feedback, and issue reports are welcome at github.com/terraspin/terraspin._

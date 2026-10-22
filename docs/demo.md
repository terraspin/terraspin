# AWS demo

Run the fixture demo with a built binary:

```bash
TERRASPIN_BIN=/tmp/terraspin scripts/demo.sh
```

The following is the complete output captured from that command:

```text

=== sg_open_ssh_world ===

┌─ Terraspin Plan Analysis ──────────────────────────────────┐
│  1 resources  ·  terraform 1.16.5
└────────────────────────────────────────────────────────────┘

  Overall Risk  CRITICAL (score: 85)

  1 to create  ·  0 to update  ·  0 to delete  ·  0 to replace

──── Policy findings ─────────────────────────────────────────

  [CRITICAL] AWS-SG-001 aws_security_group.open_ssh_world: SSH is open to the internet.
  Fix: Restrict SSH access to trusted CIDR ranges.

──── Critical & High Risk Changes ────────────────────────────

  [CRITICAL  85.0]  aws_security_group.open_ssh_world  →  CREATE

──── Analysis ────────────────────────────────────────────────

  Infrastructure summary
  1 Security

  Risk score: 85/100  ·  Risk level: CRITICAL

  Resource change summary
  1 create

  Blast radius summary
  No cascading dependencies detected

  Critical findings
    ⚠  [CRITICAL] aws_security_group.open_ssh_world → CREATE: No direct blast radius detected, but review dependent resources manually

  Affected resources by severity
    [CRITICAL] aws_security_group.open_ssh_world

  Summary
  This plan affects 1 Security infrastructure with overall risk CRITICAL (85/100). 1 changes need immediate review.

  Risk assessment
  Plan scored critical (85/100) with 1 critical or high-risk changes. Changes to databases, networking, DNS, or IAM can cause cascading outages. Verify dependencies, ensure rollback plans are in place, and apply during a maintenance window if critical resources are affected.

  Actionable recommendations
    □  Review aws_security_group.open_ssh_world before applying

  Rollback strategy
  1. terraform state pull > backup.tfstate (save current state)
2. terraform destroy -target=<failed resource> (revert specific resource)
3. terraform apply -var-file=previous.tfvars (re-apply last known good config)
4. For stateful resources: restore from most recent backup/snapshot (RDS snapshot, EBS snapshot, S3 versioning)

  Next steps
    →  1. DO NOT apply this plan to production without explicit approval from your team lead
    →  2. Review each CRITICAL finding above and ensure recovery plans are in place
    →  3. Take a state backup: terraform state pull > backup-$(date +%Y%m%d-%H%M%S).tfstate
    →  4. Apply in staging first if possible, then schedule production apply during a maintenance window


──── 1 critical  ·  0 high  ·  0 medium  ·  0 low ──────────


=== rds_insecure ===

┌─ Terraspin Plan Analysis ──────────────────────────────────┐
│  1 resources  ·  terraform 1.16.5
└────────────────────────────────────────────────────────────┘

  Overall Risk  HIGH (score: 60)

  1 to create  ·  0 to update  ·  0 to delete  ·  0 to replace

──── Policy findings ─────────────────────────────────────────

  [HIGH] AWS-RDS-001 aws_db_instance.insecure: An RDS instance is publicly accessible.
  Fix: Set publicly_accessible to false unless public database access is required.
  [HIGH] AWS-RDS-002 aws_db_instance.insecure: An RDS instance does not enable storage encryption.
  Fix: Set storage_encrypted to true for the RDS instance.
  [MEDIUM] AWS-RDS-003 aws_db_instance.insecure: An RDS instance does not enable deletion protection.
  Fix: Set deletion_protection to true for the RDS instance.

──── Critical & High Risk Changes ────────────────────────────

  [HIGH  60.0]  aws_db_instance.insecure  →  CREATE

──── Analysis ────────────────────────────────────────────────

  Infrastructure summary
  1 Database

  Risk score: 60/100  ·  Risk level: HIGH

  Resource change summary
  1 create

  Blast radius summary
  No cascading dependencies detected

  Critical findings
    ⚠  [HIGH] aws_db_instance.insecure → CREATE: No direct blast radius detected, but review dependent resources manually

  Affected resources by severity
    [HIGH] aws_db_instance.insecure

  Summary
  This plan affects 1 Database infrastructure with overall risk HIGH (60/100). 1 changes need immediate review.

  Risk assessment
  Plan scored high (60/100) with 1 critical or high-risk changes. Changes to databases, networking, DNS, or IAM can cause cascading outages. Verify dependencies, ensure rollback plans are in place, and apply during a maintenance window if critical resources are affected.

  Actionable recommendations
    □  Review aws_db_instance.insecure before applying

  Rollback strategy
  1. terraform state pull > backup.tfstate (save current state)
2. terraform destroy -target=<failed resource> (revert specific resource)
3. terraform apply -var-file=previous.tfvars (re-apply last known good config)
4. For stateful resources: restore from most recent backup/snapshot (RDS snapshot, EBS snapshot, S3 versioning)

  Next steps
    →  1. Review high-risk changes above and verify they are expected
    →  2. Take a state backup before applying
    →  3. Verify health of dependent resources after apply


──── 0 critical  ·  1 high  ·  0 medium  ·  0 low ──────────


=== eks_public_endpoint ===

┌─ Terraspin Plan Analysis ──────────────────────────────────┐
│  1 resources  ·  terraform 1.16.5
└────────────────────────────────────────────────────────────┘

  Overall Risk  HIGH (score: 60)

  1 to create  ·  0 to update  ·  0 to delete  ·  0 to replace

──── Policy findings ─────────────────────────────────────────

  [HIGH] AWS-EKS-001 aws_eks_cluster.public_endpoint: An EKS public endpoint allows internet access.
  Fix: Restrict public_access_cidrs or disable endpoint_public_access.

──── Critical & High Risk Changes ────────────────────────────

  [HIGH  60.0]  aws_eks_cluster.public_endpoint  →  CREATE

──── Analysis ────────────────────────────────────────────────

  Infrastructure summary
  1 Container

  Risk score: 60/100  ·  Risk level: HIGH

  Resource change summary
  1 create

  Blast radius summary
  No cascading dependencies detected

  Critical findings
    ⚠  [HIGH] aws_eks_cluster.public_endpoint → CREATE: No direct blast radius detected, but review dependent resources manually

  Affected resources by severity
    [HIGH] aws_eks_cluster.public_endpoint

  Summary
  This plan affects 1 Container infrastructure with overall risk HIGH (60/100). 1 changes need immediate review.

  Risk assessment
  Plan scored high (60/100) with 1 critical or high-risk changes. Changes to databases, networking, DNS, or IAM can cause cascading outages. Verify dependencies, ensure rollback plans are in place, and apply during a maintenance window if critical resources are affected.

  Actionable recommendations
    □  Review aws_eks_cluster.public_endpoint before applying

  Rollback strategy
  1. terraform state pull > backup.tfstate (save current state)
2. terraform destroy -target=<failed resource> (revert specific resource)
3. terraform apply -var-file=previous.tfvars (re-apply last known good config)
4. For stateful resources: restore from most recent backup/snapshot (RDS snapshot, EBS snapshot, S3 versioning)

  Next steps
    →  1. Review high-risk changes above and verify they are expected
    →  2. Take a state backup before applying
    →  3. Verify health of dependent resources after apply


──── 0 critical  ·  1 high  ·  0 medium  ·  0 low ──────────


=== sg_clean ===

┌─ Terraspin Plan Analysis ──────────────────────────────────┐
│  1 resources  ·  terraform 1.16.5
└────────────────────────────────────────────────────────────┘

  Overall Risk  LOW (score: 25)

  1 to create  ·  0 to update  ·  0 to delete  ·  0 to replace

──── Analysis ────────────────────────────────────────────────

  Infrastructure summary
  1 Security

  Risk score: 25/100  ·  Risk level: LOW

  Resource change summary
  1 create

  Blast radius summary
  No cascading dependencies detected

  Affected resources by severity
    [LOW] aws_security_group.clean

  Summary
  This plan affects 1 Security infrastructure with overall risk LOW (25/100). 0 changes need immediate review.

  Risk assessment
  Plan scored low (25/100) with 0 critical or high-risk changes. Changes to databases, networking, DNS, or IAM can cause cascading outages. Verify dependencies, ensure rollback plans are in place, and apply during a maintenance window if critical resources are affected.

  Actionable recommendations
    □  Review all changes before applying
    □  Ensure recent state backup exists
    □  Verify dependent resources are healthy

  Rollback strategy
  1. terraform state pull > backup.tfstate (save current state)
2. terraform destroy -target=<failed resource> (revert specific resource)
3. terraform apply -var-file=previous.tfvars (re-apply last known good config)
4. For stateful resources: restore from most recent backup/snapshot (RDS snapshot, EBS snapshot, S3 versioning)

  Next steps
    →  1. Review changes above to confirm they match expectations
    →  2. Apply at your convenience; risk is moderate or low


──── 0 critical  ·  0 high  ·  0 medium  ·  1 low ──────────

```

## Optional recording

If the repository owner already has asciinema, record the script with:

```bash
TERRASPIN_BIN=/path/to/terraspin asciinema rec --command ./scripts/demo.sh docs/demo.cast
```

If the owner already has VHS, create a local tape that runs `TERRASPIN_BIN=/path/to/terraspin ./scripts/demo.sh`, then run `vhs demo.tape`. Neither recorder is required for the demo.

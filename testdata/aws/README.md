# AWS plan fixtures

These fixtures are generated offline from the official HashiCorp AWS provider with synthetic configuration and fake credentials. They never require an AWS account or an apply.

## Provenance

- Terraform: 1.16.5 (updated by scripts/gen-aws-fixtures.sh)
- AWS provider: 6.69.1 (pinned exactly in every scenario)
- Generation: `scripts/gen-aws-fixtures.sh` runs `init -backend=false`, `plan -refresh=false`, then removes the volatile `timestamp` and sorts JSON keys with `jq -S`.
- All scenario data uses `SYNTH_` names where identifiers are configurable. No account IDs, real ARNs, access keys, or machine paths are included.

## Scenarios

| Scenario | Provenance | Resources | Resource changes | Observed behavior |
| --- | --- | --- | ---: | --- |
| `sg_open_ssh_world` | GENERATED | security group | 1 | LOW (25) |
| `sg_ssh_internal` | GENERATED | security group | 1 | LOW (25) |
| `s3_public` | GENERATED | S3 bucket, public-access block, bucket policy | 3 | LOW (18) |
| `rds_insecure` | GENERATED | RDS instance | 1 | MEDIUM (30) |
| `iam_wildcard` | GENERATED | IAM policy, IAM role | 2 | LOW (25) |
| `kms_rotation_off` | GENERATED | KMS key | 1 | LOW (10) |
| `eks_public_endpoint` | GENERATED | EKS cluster | 1 | LOW (14) |
| `ebs_unencrypted` | GENERATED | EBS volume | 1 | LOW (20) |
| `rds_replace` | DERIVED | RDS instance | 1 | CRITICAL (100) |
| `s3_destroy` | DERIVED | S3 bucket, public-access block, bucket policy | 3 | CRITICAL (100) |

`Observed behavior` is the tier and score reported by `terraspin --no-ai --format json`; it is not a correctness claim. `rds_replace` is derived from `rds_insecure` by changing its resource action to replacement. `s3_destroy` is derived from `s3_public` by changing each resource action to deletion. Derived plans have no Terraform configuration or state.

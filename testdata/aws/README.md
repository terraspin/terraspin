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
| `sg_open_ssh_world` | GENERATED | security group | pending | pending |
| `sg_ssh_internal` | GENERATED | security group | pending | pending |
| `s3_public` | GENERATED | S3 bucket, public-access block, bucket policy | pending | pending |
| `rds_insecure` | GENERATED | RDS instance | pending | pending |
| `iam_wildcard` | GENERATED | IAM policy, IAM role | pending | pending |
| `kms_rotation_off` | GENERATED | KMS key | pending | pending |
| `eks_public_endpoint` | GENERATED | EKS cluster | pending | pending |
| `ebs_unencrypted` | GENERATED | EBS volume | pending | pending |
| `rds_replace` | DERIVED | RDS instance | pending | pending |
| `s3_destroy` | DERIVED | S3 bucket, public-access block, bucket policy | pending | pending |

`rds_replace` is derived from `rds_insecure` by changing its resource action to replacement. `s3_destroy` is derived from `s3_public` by changing each resource action to deletion. Derived plans have no Terraform configuration or state.

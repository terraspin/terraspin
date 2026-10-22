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
| `ebs_encrypted` | GENERATED | EBS volume | 1 | encrypted is `true` |
| `ebs_unencrypted` | GENERATED | EBS volume | 1 | encrypted is `false` |
| `eks_private` | GENERATED | EKS cluster | 1 | endpoint public access is `false` |
| `eks_public_endpoint` | GENERATED | EKS cluster | 1 | public CIDR is `0.0.0.0/0` |
| `eks_public_restricted` | GENERATED | EKS cluster | 1 | public CIDR is `10.0.0.0/8` |
| `iam_wildcard` | GENERATED | IAM policy, IAM role | 2 | no target AWS rule resource |
| `kms_rotation_off` | GENERATED | KMS key | 1 | key rotation is `false` |
| `kms_rotation_on` | GENERATED | KMS key | 1 | key rotation is `true` |
| `rds_defaults` | GENERATED | RDS instance | 1 | `publicly_accessible=false`; `storage_encrypted` and `deletion_protection` are `null` |
| `rds_insecure` | GENERATED | RDS instance | 1 | public, unencrypted, no deletion protection |
| `rds_secure` | GENERATED | RDS instance | 1 | private, encrypted, deletion protection enabled |
| `rds_replace` | DERIVED | RDS instance | 1 | replacement derived from `rds_insecure` |
| `s3_destroy` | DERIVED | S3 bucket, public-access block, bucket policy | 3 | deletion derived from `s3_public` |
| `s3_public` | GENERATED | S3 bucket, public-access block, bucket policy | 3 | no target AWS rule resource |
| `sg_all_traffic_world` | GENERATED | security group | 1 | public protocol `-1` |
| `sg_clean` | GENERATED | security group | 1 | internal HTTPS only |
| `sg_open_rdp_world` | GENERATED | security group | 1 | public RDP on port 3389 |
| `sg_open_ssh_ipv6` | GENERATED | security group | 1 | public IPv6 SSH on port 22 |
| `sg_open_ssh_world` | GENERATED | security group | 1 | public SSH on port 22 |
| `sg_ssh_internal` | GENERATED | security group | 1 | public port 80 and internal SSH are separate ingress elements |

`Observed behavior` records plan attributes, not an AWS security claim. `rds_defaults` shows the provider values observed when the three security attributes are omitted: only explicit, known plan values can satisfy a built-in rule; `null` does not match `value: false`. `rds_replace` is derived from `rds_insecure` by changing its resource action to replacement. `s3_destroy` is derived from `s3_public` by changing each resource action to deletion. Derived plans have no Terraform configuration or state.

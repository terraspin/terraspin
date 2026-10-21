# Built-in AWS rules

Terraspin evaluates these embedded rules by default. They inspect Terraform plan values only; they do not query AWS. A missing, `null`, or unknown value does not satisfy a rule that expects `false` or `true`.

| ID | Severity | Resource | What it checks | Why it matters | Remediation | Limitations |
| --- | --- | --- | --- | --- | --- | --- |
| AWS-SG-001 | critical | `aws_security_group` | An ingress element permits TCP/22 from `0.0.0.0/0`. | Public SSH increases exposure to brute-force and exploit attempts. | Restrict SSH access to trusted CIDR ranges. | IPv4 CIDRs only; source and port must be in the same ingress element. |
| AWS-SG-002 | critical | `aws_security_group` | An ingress element permits port 3389 from `0.0.0.0/0`. | Public RDP increases exposure to credential attacks and remote compromise. | Restrict RDP access to trusted CIDR ranges. | IPv4 CIDRs only; source and port must be in the same ingress element. |
| AWS-SG-003 | high | `aws_security_group` | An ingress element permits protocol `-1` from `0.0.0.0/0`. | All-protocol ingress exposes every port and protocol. | Limit security group ingress to required protocols and trusted CIDR ranges. | IPv4 CIDRs only; source and protocol must be in the same ingress element. |
| AWS-SG-004 | critical | `aws_security_group` | An ingress element permits port 22 from `::/0`. | Public IPv6 SSH exposes the host to the internet. | Restrict SSH access to trusted IPv6 CIDR ranges. | IPv6 CIDRs only; source and port must be in the same ingress element. |
| AWS-RDS-001 | high | `aws_db_instance` | `publicly_accessible` is `true`. | A public database endpoint increases network exposure. | Set `publicly_accessible` to `false` unless public database access is required. | Requires an explicit, known planned value. |
| AWS-RDS-002 | high | `aws_db_instance` | `storage_encrypted` is `false`. | Unencrypted storage increases data-at-rest risk. | Set `storage_encrypted` to `true` for the RDS instance. | Requires an explicit, known planned value. |
| AWS-RDS-003 | medium | `aws_db_instance` | `deletion_protection` is `false`. | A database can be deleted accidentally. | Set `deletion_protection` to `true` for the RDS instance. | Requires an explicit, known planned value. |
| AWS-EBS-001 | medium | `aws_ebs_volume` | `encrypted` is `false`. | Unencrypted volumes increase data-at-rest risk. | Set `encrypted` to `true` for the EBS volume. | Requires an explicit, known planned value. |
| AWS-KMS-001 | medium | `aws_kms_key` | `enable_key_rotation` is `false`. | Long-lived key material increases rotation risk. | Set `enable_key_rotation` to `true` for the KMS key. | Requires an explicit, known planned value. |
| AWS-EKS-001 | high | `aws_eks_cluster` | `vpc_config[0]` enables public access and permits `0.0.0.0/0`. | An unrestricted Kubernetes API endpoint increases control-plane exposure. | Restrict `public_access_cidrs` or disable `endpoint_public_access`. | Checks the first VPC config block and requires explicit, known values. |

Disable a single rule with `disable_rules`, or disable all embedded rules with `builtin_rules: false`; see [Configuration](configuration.md#built-in-rules).

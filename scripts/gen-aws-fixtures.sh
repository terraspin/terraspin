#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
fixtures_dir="$repo_root/testdata/aws"

generated_scenarios=(
  ebs_encrypted
  ebs_unencrypted
  eks_private
  eks_public_endpoint
  eks_public_restricted
  iam_wildcard
  kms_rotation_off
  kms_rotation_on
  rds_defaults
  rds_insecure
  rds_secure
  s3_public
  sg_all_traffic_world
  sg_clean
  sg_open_rdp_world
  sg_open_ssh_ipv6
  sg_open_ssh_world
  sg_ssh_internal
)

for scenario in "${generated_scenarios[@]}"; do
  scenario_dir="$fixtures_dir/$scenario"
  [ -f "$scenario_dir/main.tf" ] || { echo "missing $scenario_dir/main.tf" >&2; exit 1; }
  (
    cd "$scenario_dir"
    terraform init -backend=false -input=false >/dev/null
    terraform plan -refresh=false -input=false -out=plan.tfplan >/dev/null
    terraform show -json plan.tfplan | jq -S 'del(.timestamp)' > plan.json
    rm -f plan.tfplan
  )
done

mkdir -p "$fixtures_dir/rds_replace" "$fixtures_dir/s3_destroy"
jq -S '
  (.resource_changes[] | select(.address == "aws_db_instance.insecure")) |= (
    .action_reason = "replace_because_cannot_update" |
    .change.actions = ["delete", "create"] |
    .change.before = .change.after |
    .change.before_sensitive = .change.after_sensitive
  )
' "$fixtures_dir/rds_insecure/plan.json" > "$fixtures_dir/rds_replace/plan.json"
jq -S '
  .resource_changes |= map(
    .change.actions = ["delete"] |
    .change.before = .change.after |
    .change.after = null
  )
' "$fixtures_dir/s3_public/plan.json" > "$fixtures_dir/s3_destroy/plan.json"

terraform_version=$(terraform version -json | jq -r '.terraform_version')
provider_version=$(awk '/^[[:space:]]*version[[:space:]]*=/ { gsub(/"/, "", $3); print $3; exit }' "$fixtures_dir/sg_open_ssh_world/.terraform.lock.hcl")
tmp=$(mktemp)
awk -v terraform_version="$terraform_version" -v provider_version="$provider_version" '
  /^- Terraform: / { print "- Terraform: " terraform_version " (updated by scripts/gen-aws-fixtures.sh)"; next }
  /^- AWS provider: / { print "- AWS provider: " provider_version " (pinned exactly in every scenario)"; next }
  { print }
' "$fixtures_dir/README.md" > "$tmp"
mv "$tmp" "$fixtures_dir/README.md"

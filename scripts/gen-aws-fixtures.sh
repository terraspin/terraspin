#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
fixtures_dir="$repo_root/testdata/aws"

for scenario_dir in "$fixtures_dir"/*; do
  [ -f "$scenario_dir/main.tf" ] || continue
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
    .change.before = .change.after
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

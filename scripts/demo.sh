#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
terraspin=${TERRASPIN_BIN:-terraspin}

for fixture in sg_open_ssh_world rds_insecure eks_public_endpoint sg_clean; do
  printf '\n=== %s ===\n\n' "$fixture"
  "$terraspin" --no-ai "$root/testdata/aws/$fixture/plan.json"
done

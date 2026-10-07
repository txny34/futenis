#!/usr/bin/env bash
# sim coverage must stay >= 95% (CLAUDE.md).
set -euo pipefail
cd "$(dirname "$0")/../.."

min=95
out=$(go test -short -cover ./sim/)
echo "$out"
pct=$(echo "$out" | grep -o 'coverage: [0-9.]*' | grep -o '[0-9.]*$')
if awk -v p="$pct" -v m="$min" 'BEGIN { exit !(p < m) }'; then
  echo "::error::sim coverage $pct% < $min%"
  exit 1
fi

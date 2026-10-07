#!/usr/bin/env bash
# File size limits from CLAUDE.md: Go files < 400 lines, TS files < 200 lines.
set -euo pipefail
cd "$(dirname "$0")/../.."

fail=0
check() {
  local limit=$1; shift
  while IFS= read -r f; do
    n=$(wc -l < "$f")
    if [ "$n" -ge "$limit" ]; then
      echo "::error file=$f::$n lines (limit < $limit)"
      fail=1
    fi
  done < <(git ls-files "$@")
}

check 400 '*.go'
check 200 'client/src/*.ts'
exit $fail

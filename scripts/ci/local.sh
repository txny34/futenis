#!/usr/bin/env bash
# Runs every step of .github/workflows/ci.yml locally, in the same order, and
# stops at the first failure. Run before every push: bash scripts/ci/local.sh
# STRESS=N re-runs the -race tests N extra times to shake out flaky tests.
set -uo pipefail
cd "$(dirname "$0")/../.."

step() {
  local name=$1; shift
  printf '== %s\n' "$name"
  if ! "$@"; then
    printf 'FAIL: %s\n' "$name" >&2
    exit 1
  fi
}

gofmt_clean() { local out; out=$(gofmt -l .); [ -z "$out" ] || { echo "$out"; return 1; }; }
tsc_strict() { (cd scripts/ci/typecheck && { [ -d node_modules ] || npm install --no-audit --no-fund; } && npx tsc -p tsconfig.json --noEmit); }

step "gofmt" gofmt_clean
step "go vet" go vet ./...
step "go vet (wasm)" env GOOS=js GOARCH=wasm go vet ./wasm/
step "tests (race)" go test -short -race ./...
if [ "${STRESS:-0}" -gt 0 ]; then
  step "tests (race x${STRESS})" go test -short -race -count="$STRESS" ./...
fi
step "sim coverage >= 95%" bash scripts/ci/coverage-check.sh
step "build wasm + TS bundle" go run ./cmd/dev -check
step "staticcheck" env GOTOOLCHAIN=auto go run honnef.co/go/tools/cmd/staticcheck@v0.8.1 ./...
step "govulncheck" go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
step "tsc (strict)" tsc_strict
step "file size limits" bash scripts/ci/size-check.sh
echo "local CI: all green"

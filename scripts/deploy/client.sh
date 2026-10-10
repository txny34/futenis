#!/usr/bin/env bash
# Builds the client and deploys client/ as a static site to Vercel.
# Usage: FUTENIS_SERVER=wss://futenis-server.onrender.com bash scripts/deploy/client.sh [--prod]
set -euo pipefail
cd "$(dirname "$0")/../.."
: "${FUTENIS_SERVER:?set FUTENIS_SERVER to the online server URL (wss://...)}"
FUTENIS_SERVER="$FUTENIS_SERVER" go run ./cmd/dev -check
vercel deploy client "$@"

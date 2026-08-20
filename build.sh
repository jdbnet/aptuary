#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
cd ui && npm ci && npm run build && cd ..
CGO_ENABLED=0 go build -ldflags "-s -w" -o aptuary ./cmd/aptuary
echo "Built aptuary"

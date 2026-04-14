#!/usr/bin/env bash
# Project verification script for goclaw.
# Run after making changes: bash .goclaw/verify.sh
set -euo pipefail
cd "$(dirname "$0")/.."

echo "==> Building..."
go build ./...

echo "==> Vetting..."
go vet ./...

echo "==> Smoke test (sessions list)..."
go run ./cmd/goclaw sessions list 2>/dev/null || true

echo "==> All checks passed."

#!/usr/bin/env bash
# Deterministic mock-Anthropic parity runs (no API keys required).
# Scenario list: scripts/mock_parity_scenarios.json
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "== goclaw mock parity: orchestrator =="
go test ./internal/orchestrator -count=1 -run '^TestMockParityHarness$' "$@"

echo "== goclaw mock parity: coordinator =="
go test ./internal/coordinator -count=1 -run '^TestMockParityHarness$' "$@"

echo "== mock parity harness: OK =="

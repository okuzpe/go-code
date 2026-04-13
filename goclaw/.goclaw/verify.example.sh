#!/usr/bin/env bash
# Example verification entry point for goclaw (copy to verify.sh and customize).
# See docs/goclaw/verification-recipe.md
set -euo pipefail
cd "$(dirname "$0")/.."
go build ./...

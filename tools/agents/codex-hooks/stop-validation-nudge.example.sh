#!/usr/bin/env bash
set -euo pipefail

if git diff --name-only --cached -- '*.go' | grep -q .; then
  echo "Reminder: run go test ./... before finalizing Go changes." >&2
fi

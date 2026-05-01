#!/usr/bin/env bash
set -euo pipefail

branch="$(git branch --show-current 2>/dev/null || true)"
if [[ "$branch" == "main" || "$branch" == "master" ]]; then
  echo "Refusing tracked-file edits from default branch. Create a feature worktree first." >&2
  exit 1
fi

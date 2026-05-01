---
name: navia-build-engineer
description: Build, test, run, CI, and worktree workflow skill for Navia. Use when running Go commands, debugging test/build failures, configuring GitHub Actions, or creating feature worktrees.
---

# Navia Build Engineer

Use this skill for build, test, run, and worktree operations.

## Standard Commands

- Run app: `go run ./cmd/navia`
- Run app on a path: `go run ./cmd/navia /path/to/root`
- Version: `go run ./cmd/navia --version`
- Test all packages: `go test ./...`
- Build binary: `go build ./cmd/navia`
- Format Go: `go fmt ./...`
- Vet: `go vet ./...`

## Workflow

1. Use targeted package tests first when debugging.
2. Expand to `go test ./...` before publishing.
3. For CI changes, mirror local commands in GitHub Actions.
4. Capture the exact command and first meaningful error before changing code.

## Worktrees

Create feature worktrees with `python3 tools/agents/scripts/pre_worktree.py "<feature>"`. Do not implement from `main`.

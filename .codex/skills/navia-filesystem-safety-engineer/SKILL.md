---
name: navia-filesystem-safety-engineer
description: Filesystem safety skill for Navia. Use for read-only scanning, path validation, search, preview, config loading, editor launching, and any behavior that touches user files.
---

# Navia Filesystem Safety Engineer

Use this skill for `internal/fs`, `internal/config`, and file-affecting app flows.

## Rules

- Treat file mutations as safety-critical.
- Preserve Navia's read-only boundary; do not add filesystem mutation without an explicit product-scope proposal.
- Use `t.TempDir()` and `t.Setenv()` in tests.
- Avoid tests that depend on the developer's home directory or editor.
- Bound previews by configured byte limits.
- Validate subpath and destination behavior before moving or deleting.

## Validation

- Run focused tests such as `go test ./internal/fs`.
- Run `go test ./...` before publishing.

## Review Focus

- accidental mutation or overwrite behavior
- path traversal or wrong-root operations
- unsafe fallback when config parsing fails
- missing test coverage for error paths

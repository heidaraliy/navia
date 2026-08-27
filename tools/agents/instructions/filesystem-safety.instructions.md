# Filesystem Safety Instructions

Use for scanning, search, preview, config, external-editor handoff, and path boundary behavior.

## Rules

- Treat filesystem operations as safety-critical.
- Use temp-dir tests for every behavior that scans or reads paths.
- Preserve the read-only boundary. Do not add filesystem mutation without an explicit product-scope proposal.
- Validate path boundaries with existing helpers before adding new path-sensitive operations.
- Avoid depending on the developer machine's home directory, git state, or editor setup in tests.
- Keep previews bounded by configured byte limits.

## Validation

- Run targeted package tests such as `go test ./internal/fs`.
- Run `go test ./...` before publishing.
- Document residual manual checks when behavior depends on terminal/editor integration.

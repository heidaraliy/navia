# Filesystem Safety Instructions

Use for copy, move, rename, create, delete, search, preview, config, and path boundary behavior.

## Rules

- Treat filesystem operations as safety-critical.
- Use temp-dir tests for every behavior that creates, moves, renames, deletes, or scans paths.
- Preserve safe delete as the default. Deleting should move into the Navia trash location unless explicitly designed otherwise.
- Validate path boundaries with existing helpers before adding new path-sensitive operations.
- Avoid depending on the developer machine's home directory, git state, or editor setup in tests.
- Keep previews bounded by configured byte limits.

## Validation

- Run targeted package tests such as `go test ./internal/fs`.
- Run `go test ./...` before publishing.
- Document residual manual checks when behavior depends on terminal/editor integration.

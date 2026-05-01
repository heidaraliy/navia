# Go TUI Instructions

Use for Bubble Tea, Lip Gloss, keyboard workflows, layout, and terminal behavior.

## Rules

- Keep state transitions in `internal/app` explicit and testable.
- Preserve keyboard-first workflows and keep help text current.
- Prefer small pure helpers for selection, filtering, and rendering decisions.
- Do not let long paths, snippets, or status text break compact terminals.
- Keep terminal UI copy direct and action-oriented.

## Validation

- Run `go test ./...`.
- For visible UI changes, run `go run ./cmd/navia` and inspect common viewport sizes when possible.
- Add tests for non-rendering state transitions and helper behavior when practical.

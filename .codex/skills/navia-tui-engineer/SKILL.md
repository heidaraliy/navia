---
name: navia-tui-engineer
description: Terminal UI engineering skill for Navia. Use when changing Bubble Tea models, update loops, views, keybindings, help text, layout, status messages, previews, or terminal interaction behavior.
---

# Navia TUI Engineer

Use this skill for `internal/app` and `internal/ui` work.

## Workflow

1. Read the model, update path, view path, and styles for the touched behavior.
2. Keep mode transitions explicit.
3. Keep keybindings reflected in help text and footer hints.
4. Ensure long paths, snippets, and status text are truncated or wrapped intentionally.
5. Prefer testable helpers for non-rendering decisions.
6. Load `navia-render-performance-engineer` for render helpers, editor view paths, syntax highlighting, previews, diff panes, tabs, or render caches.

## Validation

- Run `go test ./...`.
- For visual changes, run `go run ./cmd/navia` and inspect normal, search, modal, and help states when possible.

## Review Focus

- stranded modes or unhandled escape paths
- stale help text
- broken small-terminal layout
- status messages that misrepresent filesystem behavior

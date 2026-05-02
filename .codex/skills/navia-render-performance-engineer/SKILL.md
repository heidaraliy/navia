---
name: navia-render-performance-engineer
description: Rendering and allocation discipline for Navia's Bubble Tea TUI and editor. Use when changing `internal/app`, `internal/editor`, `internal/ui`, `internal/syntax`, viewport rendering, syntax highlighting, preview/diff rendering, tabs, row selection, caches, or any code that can run per frame.
---

# Navia Render Performance Engineer

## Overview

Keep Navia's frame rendering bounded, predictable, and reviewable. Treat `View()` and render helpers as hot paths: they should do work proportional to the visible terminal surface unless a measured exception is isolated and documented.

## Context Pass

Before changing render or editor code:

- Read `tools/agents/instructions/render-performance.instructions.md`.
- Search the touched hot path with `rg -n "View\\(|render|visible|Highlight|SetContent|Viewport|strings\\.Builder|lipgloss\\.NewStyle" internal/app internal/editor internal/ui internal/syntax`.
- Identify whether the change runs on every frame, every keypress, every filesystem refresh, or only during explicit commands.
- State the expected input size: rows, tabs, buffer lines, viewport cells, preview bytes, syntax-highlighted lines, or search results.

## Render Rules

- Keep `View()` and render helpers free of filesystem, git, LSP, config, and full-buffer work.
- Render only the visible window for editor buffers, previews, diff panes, trees, and tab bars.
- Move full scans, sorting, search, syntax tokenization, and preview generation into update-time state, async commands, or explicitly invalidated caches.
- Avoid nested scans over independent unbounded collections in frame paths. If a lookup is repeated, build a map, set, sparse index, or cached projection once per state change.
- Prefer `map[T]struct{}` or `map[T]bool` for membership, and keep ordered slices only for display order.
- Bound caches by file, viewport, query, line range, or explicit invalidation. Do not add global or model-owned caches that grow for the lifetime of the process without a cap or reset.
- Reuse styles from `internal/ui.Styles` or local variables outside row loops. Avoid constructing `lipgloss.NewStyle()` for every visible row unless the row count is tiny and the cost is measured as harmless.
- Build strings with `strings.Builder` for multi-line output and call `Grow` when a reasonable bound is known. Avoid repeated `+` concatenation in loops.
- Treat ANSI-aware clipping and width measurement as expensive enough to keep near the final visible surface.

## Complexity Targets

- `View()` target: `O(visible rows * visible columns)` plus small fixed panel overhead.
- Editor render target: `O(visible editor lines * visible columns)`, not `O(total buffer lines)`.
- Tree/list render target: `O(visible rows)`, with selection, expansion, and git state membership through maps or cached indexes.
- Search/filter update target: `O(total candidates)` per query update is acceptable; `O(total candidates^2)` is not acceptable without a measured, documented reason.
- Preview/syntax render target: cache or precompute expensive work by file, query, and line range; invalidate on content, query, or style changes.

## Review Checklist

- Name the hot path and whether the new code runs per frame.
- Explain the complexity in terms of visible rows, total rows, buffer lines, tabs, or preview bytes.
- Check for unbounded allocation growth across repeated render cycles.
- Check for nested loops over total rows, tabs, lines, tokens, or matches.
- Check that caches have ownership, invalidation, and a practical bound.
- Add a large-input regression test when the change affects editor buffers, tab ranges, tree rows, preview rendering, syntax highlighting, or diff rendering.
- Add a benchmark with `-benchmem` when the change is explicitly performance-motivated or when complexity is hard to reason about from code alone.

## Validation

Run the normal validation for the touched domain:

- Go render/editor changes: `go test ./...`.
- Formatting-sensitive Go changes: `gofmt` or `go fmt ./...`, then inspect churn.
- Agent guidance changes: `python3 tools/agents/scripts/validate_agent_config.py`, `bash -n tools/agents/git-hooks/* tools/agents/codex-hooks/*`, and `git diff --check`.
- Performance-sensitive changes: run a targeted `go test -run <TestName>` for large-input coverage, and run `go test -bench <BenchmarkName> -benchmem ./<package>` when a benchmark exists or is added.

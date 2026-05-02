# Render Performance Instructions

Use for per-frame TUI rendering, editor rendering, syntax highlighting, previews, diff panes, tab bars, and any cache or data structure that supports those paths.

## Rules

- Treat `View()` and render helpers as hot paths.
- Keep render work bounded by visible terminal rows and columns wherever possible.
- Do not scan full buffers, full trees, all tabs, all search results, or entire preview payloads from a frame path unless the collection is already bounded and documented.
- Move sorting, filtering, filesystem reads, git calls, LSP calls, syntax tokenization, and preview generation into update-time state, async commands, or invalidated caches.
- Avoid `O(n^2)` joins, membership checks, or pairwise comparisons over rows, tabs, lines, matches, or tokens. Use maps, sets, sparse indexes, or cached projections for repeated lookups.
- Prefer a small memory increase for stable `O(n)` behavior when the ownership and invalidation are clear.
- Bound every new cache by file, query, viewport, line range, explicit capacity, or lifecycle reset.
- Reuse `internal/ui.Styles` or local style variables outside row loops. Avoid constructing styles per visible row unless measured and intentionally accepted.
- Use `strings.Builder` for multi-line output and grow it when a reasonable size estimate is available.
- Keep ANSI-aware width measurement and clipping close to the final visible output; do not repeatedly measure the same large styled string inside nested loops.

## Red Flags

- `View()` or render helpers performing filesystem, git, LSP, config, or full-buffer work.
- Nested loops over total rows, total tabs, total lines, syntax tokens, or search matches.
- Rebuilding lowercased labels, path sets, git-state sets, or syntax-highlighted lines every frame.
- Unbounded maps keyed by paths, lines, queries, or viewport widths without invalidation.
- `strings.Split` or `strings.Join` over large backing content inside a frame path when only visible lines are needed.
- Per-row `lipgloss.NewStyle()` or repeated width calculation for values that can be styled or measured once.

## Validation

- Add or update tests with large buffers, many rows, many tabs, long lines, or large previews when the touched behavior can scale with user data.
- Assert rendered output stays bounded to the viewport or documented render limit.
- Use `go test ./...` for Go changes.
- Use `go test -bench <BenchmarkName> -benchmem ./<package>` for explicit performance work or changes where allocation behavior is the point of the change.

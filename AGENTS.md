---
scope: Project
alwaysApply: true
description: Navia agent entrypoint. Keep this file sparse; detailed workflows live in tools/agents and .codex/skills.
---

# Navia Agent Guide

## Goal

Build Navia into a professional, safe, open-source terminal file navigator. Good work is small, codebase-shaped, validated, and explicit about filesystem risk.

## Load Order

1. Read this file first.
2. Before repo-tracked implementation, commit, push, worktree setup, or PR work, read `tools/agents/instructions/pre-worktree-pr.instructions.md`.
3. Read `tools/agents/instructions/index.md` and only the instruction files that match the touched paths or task domain.
4. For non-trivial features, multi-agent work, safety-sensitive filesystem behavior, or PR publishing, read `tools/agents/instructions/accuracy-pipeline.instructions.md`.
5. Load every relevant Navia skill before planning or editing.
6. Search local repo context with `rg` before designing or changing behavior.

## Skill Routing

- `navia-agent`: full feature pipeline from context bundle through audited plan, worktree implementation, validation, review, and draft PR.
- `navia-feature-architect`: feature planning and architecture.
- `navia-plan-auditor`: plan review before implementation.
- `navia-code-reviewer`: final diff review before merge.
- `navia-build-engineer`: Go build, test, run, CI, and worktree workflow.
- `navia-tui-engineer`: Bubble Tea model/update/view work, terminal UX, keybindings, and layout.
- `navia-render-performance-engineer`: per-frame TUI/editor rendering, allocation discipline, cache bounds, and space-time complexity review.
- `navia-filesystem-safety-engineer`: read-only scanning, path validation, preview, search, config, and editor-handoff safety.
- `navia-git-release-engineer`: git helpers, repository automation, release, install, and PR hygiene.
- `navia-docs-engineer`: README, contributor docs, security docs, examples, and public-facing copy.

Use the smallest skill set that covers the task.

## Hard Invariants

- Never implement, commit, or push feature work from `main`.
- Use feature worktrees under `~/programs/wt` and branches named `agent/<slug>`.
- Filesystem operations must be predictable, path-bounded, and covered by temp-dir tests.
- Preserve Navia's read-only boundary; filesystem or Git mutation requires an explicit product-scope proposal.
- Terminal UI changes must keep keyboard workflows visible, responsive, and usable in small terminals.
- Per-frame rendering must stay bounded by visible output where possible, with explicit review for allocation growth, cache bounds, and avoidable `O(n^2)` work.
- Prefer existing package boundaries: `internal/app`, `internal/fs`, `internal/config`, `internal/gitview`, `internal/syntax`, and `internal/textsafe`.
- Keep root guidance compact; put detailed agent rules in `tools/agents/**` or skills.

## Required Validation

- Go changes: `go test ./...`.
- Formatting-sensitive Go changes: `gofmt` or `go fmt ./...`, then verify no unintended churn.
- Agent config/docs changes: `python3 tools/agents/scripts/validate_agent_config.py`, `bash -n tools/agents/git-hooks/* tools/agents/codex-hooks/*`, and `git diff --check`.
- CI or release changes: verify GitHub Actions YAML shape and document any unrun hosted checks.

## Failure Handling

Read the first meaningful error and fix the root cause. If the same validation failure persists after three focused attempts, stop and report what was tried, what failed, and the most likely next fix.

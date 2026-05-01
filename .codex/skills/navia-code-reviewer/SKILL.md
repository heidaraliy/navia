---
name: navia-code-reviewer
description: Code review skill for Navia diffs, branches, and PRs. Use before merging changes, especially filesystem operations, terminal UI behavior, config parsing, git helpers, CI, release automation, and public documentation.
---

# Navia Code Reviewer

Use this skill for final-gate review.

## Load Context

1. Determine review scope.
2. Read every changed file completely.
3. Read nearby tests and owning package files.
4. Read relevant instruction files for safety, TUI, or automation changes.

## Review Focus

- destructive or surprising filesystem behavior
- missing temp-dir tests
- path traversal or unsafe boundary assumptions
- TUI state transitions that strand the user
- stale help text or misleading docs
- CI or release automation that cannot run from a clean checkout

## Output

Findings first, ordered by severity, with `file:line`. If no findings remain, say `No findings` and list residual risk or unverified areas.

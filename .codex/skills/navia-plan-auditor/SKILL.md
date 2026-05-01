---
name: navia-plan-auditor
description: Audit Navia implementation plans before code changes. Use for non-trivial features, filesystem safety changes, TUI workflows, CI/release automation, or multi-agent plans.
---

# Navia Plan Auditor

Use this skill before implementation.

## Audit Focus

- Does the plan identify the real owning package?
- Are filesystem risks and path boundaries covered?
- Are TUI state, keybindings, and help text consistent?
- Are tests targeted to the behavior at risk?
- Are worktree, commit, push, and draft PR steps off `main`?
- Is the scope small enough for one reviewable PR?

## Output

List findings by severity:

- `Critical`: blocks implementation.
- `Major`: likely bug, safety gap, or validation gap.
- `Minor`: clarity or maintainability issue.

If no blockers remain, say the plan is implementation-ready and mention residual risks.

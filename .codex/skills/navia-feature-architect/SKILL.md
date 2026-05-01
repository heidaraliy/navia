---
name: navia-feature-architect
description: Feature planning and architecture skill for Navia. Use before implementing non-trivial CLI, TUI, filesystem, config, git, automation, or documentation changes that need clear package ownership and validation.
---

# Navia Feature Architect

Use this skill to turn a request and context bundle into a decision-complete plan.

## Plan Requirements

- State the user-facing promise.
- Identify owning packages or docs.
- Route relevant skills and instruction files.
- Split work into dependency-ordered slices with write ownership.
- Define validation proof.
- List assumptions and non-goals.

## Design Defaults

- Preserve existing package boundaries.
- Prefer small helpers over broad abstractions.
- Keep filesystem behavior explicit and testable.
- Keep TUI state transitions easy to reason about.
- Avoid public docs that promise unreleased behavior.

## Output

Return a compact plan with summary, key changes, tests, and assumptions.

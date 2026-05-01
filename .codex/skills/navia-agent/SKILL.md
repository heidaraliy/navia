---
name: navia-agent
description: Generic autonomous implementation pipeline for Navia. Use when a feature or fix should move from request to context bundle, audited plan, worktree implementation, validation, code review, and draft PR with dependency-aware parallel agents.
---

# Navia Agent Pipeline

Use this skill for full feature-to-PR work or when the user asks for agent-team orchestration.

## Pipeline Contract

1. Preflight branch and worktree state.
2. Build a context bundle from local repo search.
3. Produce an architecture plan.
4. Audit the plan.
5. Implement from a feature worktree.
6. Run targeted validation, then `go test ./...`.
7. Review the diff.
8. Push and open a draft PR when requested.

Never implement, commit, push, or open a PR from `main`.

## Preflight

- Derive branch slug `agent/<short-feature-slug>`.
- Run `git branch --show-current` and `git status -sb`.
- If on `main`, create a worktree with `tools/agents/scripts/pre_worktree.py`.
- Load `tools/agents/instructions/index.md`, matching instruction files, and relevant Navia skills.

## Context Bundle

Use `rg` before designing. Include owning packages, nearby tests, user-facing behavior, safety risks, and validation gates. When subagents are available, use independent explorers for code context, risk review, and test discovery.

## Implementation

Assign disjoint write sets to parallel workers. Tell workers they are not alone in the codebase and must not revert unrelated changes. Keep urgent blocking work local.

## Review And PR

Run `navia-code-reviewer` on the final diff. Fix correctness, safety, and test gaps before publishing. Draft PR descriptions must cover summary, validation, and residual risk.

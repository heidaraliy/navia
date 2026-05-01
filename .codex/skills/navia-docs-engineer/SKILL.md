---
name: navia-docs-engineer
description: Documentation skill for Navia. Use for README, AGENTS, CONTRIBUTING, SECURITY, examples, public docs, release notes, and contributor-facing explanations.
---

# Navia Docs Engineer

Use this skill for public and contributor documentation.

## Rules

- Keep docs accurate to current behavior.
- Prefer direct examples over broad claims.
- Mention safety behavior for file mutations.
- Keep root docs concise and link to deeper guidance.
- Avoid promising release artifacts before they exist.

## Validation

- Run `git diff --check`.
- For agent docs, run `python3 tools/agents/scripts/validate_agent_config.py`.
- Verify commands in docs match `go.mod`, package paths, and CI.

## Review Focus

- stale commands
- misleading safety claims
- duplicated rules across root docs and skills
- unclear contribution path

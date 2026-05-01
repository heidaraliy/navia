---
name: navia-git-release-engineer
description: GitHub, release, install, repository automation, git helper, and PR hygiene skill for Navia. Use for GitHub Actions, branch protection, release packaging, install docs, git integration, and publishing workflows.
---

# Navia Git Release Engineer

Use this skill for repository automation and publishing.

## Defaults

- Draft PRs are the default.
- `main` is protected by PR and CI.
- CI stays fast: format check, vet, tests.
- Public docs should distinguish source installs from tagged releases.
- Use GitHub noreply author emails for public commits.

## Validation

- Run local Go checks before push.
- Confirm hosted CI after push when workflows change.
- Use `tools/agents/templates/pr-body.md` for PR descriptions.

## Review Focus

- direct pushes to `main`
- CI that rewrites files instead of checking them
- install docs that assume unreleased tags
- release scripts that are not reproducible from a clean checkout

# Repo Automation Instructions

Use for GitHub Actions, release, install, PR templates, branch protection, and repository automation.

## Rules

- Keep CI fast and boring: format check, vet, and tests.
- Default publishing flow is a draft PR from a feature branch.
- Do not push directly to `main`.
- Public docs must avoid overstating stability before releases exist.
- Prefer GitHub noreply author email for public commits.

## Initial CI Contract

CI should run on pushes and pull requests to `main`:

- `go mod download`
- formatting check with `gofmt -w` forbidden in CI
- `go vet ./...`
- `go test ./...`

## Branch Protection

After CI exists, protect `main` with required PRs, required CI, blocked force pushes, and blocked deletions.

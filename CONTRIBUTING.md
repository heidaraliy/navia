# Contributing

Thanks for helping improve Navia. Keep contributions small, tested, and explicit about filesystem safety.

## Workflow

1. Create a feature branch from `main`.
2. Make the smallest change that solves the issue.
3. Run validation:

```bash
go test ./...
```

4. Open a pull request with a short summary, validation results, and any residual risk.

## Development

```bash
go run ./cmd/navia
go run ./cmd/navia /path/to/project
go build ./cmd/navia
go test ./...
```

Run `go fmt ./...` before submitting Go changes.

## Safety Expectations

Changes that scan, search, or preview user files need focused tests. Use `t.TempDir()` and avoid relying on local machine state. Navia is read-only; filesystem or Git mutation requires an explicit product-scope proposal before implementation.

## Agent Workflow

Agents should read `AGENTS.md` first. Feature work belongs in worktrees under `~/programs/wt` on branches named `agent/<slug>`, with draft PRs after validation passes.

# Navia Agent Configuration

This directory keeps detailed agent guidance out of always-loaded root files so agents can spend context on the current task.

## Load Model

1. `AGENTS.md` establishes the project contract.
2. `tools/agents/instructions/pre-worktree-pr.instructions.md` establishes the no-`main` implementation gate.
3. `tools/agents/instructions/index.md` routes agents to detailed instruction files.
4. `tools/agents/instructions/accuracy-pipeline.instructions.md` adds context retrieval, plan audit, validation, review, and draft PR flow for non-trivial work.
5. `tools/agents/prompting/` and `tools/agents/checklists/` provide reusable task prompts and review checklists loaded only when useful.
6. `.codex/skills/*/SKILL.md` provides domain workflows.

## Validation

For agent config and docs-only changes:

```bash
python3 tools/agents/scripts/validate_agent_config.py
python3 tools/agents/scripts/assert_worktree_ready.py --allow-dirty-agent-config
bash -n tools/agents/git-hooks/* tools/agents/codex-hooks/*
git diff --check
```

For Go implementation changes:

```bash
go test ./...
```

Run targeted package tests first when debugging, then broaden to the full suite.

## Hooks

Git hooks are opt-in:

```bash
git config core.hooksPath tools/agents/git-hooks
```

The hooks block commits and pushes from `main`. Codex hook examples under `tools/agents/codex-hooks/` are examples, not enabled project policy.

## Worktree And PR Flow

Create feature worktrees with:

```bash
python3 tools/agents/scripts/pre_worktree.py "some-new-feature"
```

The helper fetches `origin/main` and creates `agent/<slug>` under `~/programs/wt/<slug>`. Draft PRs are the default after validation passes.

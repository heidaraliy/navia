# Agent Config Instructions

Use for `AGENTS.md`, `.codex/skills/**`, `tools/agents/**`, hooks, scripts, checklists, and templates.

## Rules

- Keep root `AGENTS.md` sparse and stable.
- Put task-specific rules in instruction files and skills.
- Keep skill descriptions trigger-rich; they are the routing surface.
- Avoid copying non-Navia rules from other repositories.
- Prefer direct references over deeply nested docs.
- Validate all referenced `tools/agents/**` paths.

## Required Checks

```bash
python3 tools/agents/scripts/validate_agent_config.py
bash -n tools/agents/git-hooks/* tools/agents/codex-hooks/*
git diff --check
```

If a check is not runnable, report why and what was inspected instead.

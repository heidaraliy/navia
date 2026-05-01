---
scope: Path
paths:
  - .codex/skills/**
description: Navia skill maintenance guidance.
---

# Skill Maintenance

Read `tools/agents/instructions/agent-config.instructions.md` before editing skills.

Load `skill-creator` for skill changes. Keep `SKILL.md` files compact, trigger-rich, and procedural. Put examples or source maps in direct reference files only when they are too large for the skill body.

Run `python3 tools/agents/scripts/validate_agent_config.py` after changes.

#!/usr/bin/env python3
"""Validate Navia agent config files."""

from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]

REQUIRED_DOCS = [
    "AGENTS.md",
    ".codex/skills/AGENTS.md",
    ".codex/skills/navia-agent/SKILL.md",
    "tools/agents/instructions/index.md",
    "tools/agents/instructions/pre-worktree-pr.instructions.md",
    "tools/agents/instructions/accuracy-pipeline.instructions.md",
    "tools/agents/templates/pr-body.md",
]


def read(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def frontmatter(text: str) -> dict[str, str]:
    if not text.startswith("---\n"):
        return {}
    end = text.find("\n---", 4)
    if end == -1:
        return {}
    data: dict[str, str] = {}
    for line in text[4:end].splitlines():
        if ":" not in line or line.startswith("  "):
            continue
        key, value = line.split(":", 1)
        data[key.strip()] = value.strip().strip('"')
    return data


def fail(errors: list[str], message: str) -> None:
    errors.append(message)


def validate_required(errors: list[str]) -> None:
    for rel in REQUIRED_DOCS:
        if not (ROOT / rel).exists():
            fail(errors, f"required file is missing: {rel}")


def validate_instruction_index(errors: list[str]) -> None:
    index = ROOT / "tools/agents/instructions/index.md"
    if not index.exists():
        return
    text = read(index)
    names = set(re.findall(r"`([^`]+\.instructions\.md)`", text))
    for name in sorted(names):
        if not (index.parent / name).exists():
            fail(errors, f"instruction index references missing file: {name}")


def validate_references(errors: list[str]) -> None:
    candidates = [
        ROOT / "AGENTS.md",
        ROOT / ".codex/skills/AGENTS.md",
        *ROOT.glob("tools/agents/**/*.md"),
        *ROOT.glob("tools/agents/**/*.sh"),
        *ROOT.glob(".codex/skills/**/*.md"),
    ]
    for path in candidates:
        if not path.exists():
            continue
        text = read(path)
        for match in re.findall(r"`((?:tools/agents|\.codex/skills|\.github)/[^`]+)`", text):
            if "*" in match or match.endswith("/"):
                continue
            if not (ROOT / match).exists():
                fail(errors, f"{path.relative_to(ROOT)} references missing {match}")


def validate_skills(errors: list[str]) -> None:
    skill_root = ROOT / ".codex/skills"
    for skill in sorted(skill_root.glob("*/SKILL.md")):
        text = read(skill)
        meta = frontmatter(text)
        if not meta.get("name"):
            fail(errors, f"{skill.relative_to(ROOT)} missing frontmatter name")
        description = meta.get("description", "")
        if not description:
            fail(errors, f"{skill.relative_to(ROOT)} missing description")
        elif len(description.split()) < 8:
            fail(errors, f"{skill.relative_to(ROOT)} description is too terse")


def main() -> int:
    errors: list[str] = []
    validate_required(errors)
    validate_instruction_index(errors)
    validate_references(errors)
    validate_skills(errors)
    if errors:
        for error in errors:
            print(f"error: {error}", file=sys.stderr)
        return 1
    print("agent config validation passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

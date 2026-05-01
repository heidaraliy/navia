#!/usr/bin/env python3
"""Create a Navia feature worktree without relying on shell aliases."""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path


def git(args: list[str], *, cwd: Path | None = None) -> str:
    return subprocess.check_output(["git", *args], cwd=cwd, text=True).strip()


def run(args: list[str], *, cwd: Path | None = None) -> None:
    subprocess.check_call(args, cwd=cwd)


def slugify(value: str) -> str:
    value = value.lower()
    value = re.sub(r"[^a-z0-9]+", "-", value)
    value = value.strip("-")
    return value[:64].strip("-") or "feature"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("name", help="Feature name, e.g. 'safe delete audit'")
    parser.add_argument("--prefix", default="agent", help="Branch prefix, default: agent")
    parser.add_argument("--base", default="main", help="Base branch, default: main")
    parser.add_argument("--no-fetch", action="store_true", help="Skip fetching origin first")
    parser.add_argument(
        "--worktree-root",
        default=str(Path.home() / "programs/wt"),
        help="Directory where worktrees are created",
    )
    args = parser.parse_args()

    repo = Path(git(["rev-parse", "--show-toplevel"]))
    slug = slugify(args.name)
    branch = f"{args.prefix.strip('/')}/{slug}"
    worktree = Path(args.worktree_root).expanduser() / slug

    if worktree.exists():
        print(f"error: worktree path already exists: {worktree}", file=sys.stderr)
        return 1

    existing = set(git(["branch", "--format=%(refname:short)"], cwd=repo).splitlines())
    if branch in existing:
        print(f"error: branch already exists: {branch}", file=sys.stderr)
        return 1

    base = args.base
    if not args.no_fetch:
        run(["git", "fetch", "origin", args.base], cwd=repo)
        remote_base = f"origin/{args.base}"
        remotes = set(git(["branch", "-r", "--format=%(refname:short)"], cwd=repo).splitlines())
        if remote_base in remotes:
            base = remote_base

    run(["git", "worktree", "add", str(worktree), "-b", branch, base], cwd=repo)
    print(f"created worktree: {worktree}")
    print(f"branch: {branch}")
    print(f"base: {base}")
    print("")
    print("Continue work from this directory:")
    print(f"  cd {worktree}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

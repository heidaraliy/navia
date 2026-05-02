# Agent Instruction Index

Read only the instruction files that match the task.

| Path or task | Instruction file |
| --- | --- |
| before implementation, commit, push, worktree setup, or PR workflow | `pre-worktree-pr.instructions.md` |
| full feature-to-PR pipeline, autonomous implementation, or multi-agent orchestration | `accuracy-pipeline.instructions.md` |
| `AGENTS.md`, `.codex/skills/**`, `tools/agents/**`, hooks, evals | `agent-config.instructions.md` |
| `internal/app/**`, `internal/ui/**`, terminal layout, keybindings, Bubble Tea flows | `go-tui.instructions.md` |
| per-frame rendering, editor render paths, syntax highlighting, preview/diff rendering, tabs, render caches | `render-performance.instructions.md` |
| `internal/fs/**`, `internal/config/**`, delete/move/copy/search/preview behavior | `filesystem-safety.instructions.md` |
| `.github/**`, releases, install docs, git helpers, publishing | `repo-automation.instructions.md` |

When a task spans domains, read all matching files and load the corresponding skills.

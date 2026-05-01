# Implementation Accuracy Review

Use before finalizing an implementation diff.

- Does the diff solve the requested behavior without widening scope?
- Are filesystem operations path-bounded and covered by temp-dir tests?
- Are UI keybindings, help text, and status messages consistent?
- Are package boundaries still clear?
- Did validation run from the feature worktree?
- Are unrun checks reported with a reason?
- Is the PR description honest about risk and manual checks?

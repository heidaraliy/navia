# Accuracy Pipeline Instructions

Use for non-trivial implementation, safety-sensitive behavior, multi-agent orchestration, or draft PR publishing.

## Pipeline

1. Preflight branch and worktree state.
2. Build a context bundle with local search.
3. Produce a plan with domain routing and validation gates.
4. Audit the plan before edits.
5. Implement in dependency order.
6. Run targeted validation, then broader validation.
7. Review the diff for correctness, safety, and test gaps.
8. Commit, push, and open a draft PR when requested.

## Context Bundle

Gather only facts needed for the task:

- owning packages and nearby tests
- current CLI/TUI behavior
- filesystem and config safety risks
- validation commands likely required
- public docs or CI impact

When subagents are available, use independent explorers for code context, risk review, and test discovery. Explorer output must be factual and path-grounded.

## Implementation Slices

Parallelize only when write sets do not overlap. Tell implementation workers they are not alone in the codebase and must not revert unrelated changes.

Useful slices:

- TUI behavior and rendering
- filesystem/config safety
- docs and examples
- CI/release automation
- tests and review

## Stop Rules

Stop and report when:

- the request conflicts with a hard invariant
- ownership or safety behavior cannot be identified locally
- validation fails after three focused fixes
- continuing would stage, revert, or overwrite unrelated dirty work

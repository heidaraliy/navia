# Repository Guidelines

## Project Structure & Module Organization

`navia` is a Go 1.22 command-line terminal UI application. The executable entry point is `cmd/navia/main.go`. Internal packages live under `internal/`:

- `internal/app`: Bubble Tea model, update loop, and view composition.
- `internal/config`: user configuration loading and validation.
- `internal/fs`: filesystem scanning, preview, search, copy, move, and trash behavior.
- `internal/git`: git status helpers.
- `internal/shellteach`: command suggestion/help logic.
- `internal/ui`: shared layout and Lip Gloss styles.

Tests are colocated with the packages they cover using `*_test.go` files. There is no separate assets directory at present.

## Build, Test, and Development Commands

- `go run ./cmd/navia`: run the app against the current directory.
- `go run ./cmd/navia /path/to/root`: run the app against a specific root.
- `go run ./cmd/navia --version`: print the current version.
- `go test ./...`: run all package tests.
- `go test ./internal/fs -run TestRecursiveSearchFilesAndText`: run one focused test.
- `go build ./cmd/navia`: compile the CLI binary.
- `go fmt ./...`: format Go source before committing.

## Coding Style & Naming Conventions

Use standard Go formatting and idioms. Keep package names short and lowercase (`fs`, `app`, `config`). Exported identifiers should have clear domain names such as `ScanDir`, `BuildPreview`, or `SafeDelete`; unexported helpers should stay package-local unless shared behavior justifies promotion. Prefer table-driven tests when adding multiple cases, and keep filesystem tests isolated with `t.TempDir()` and `t.Setenv()`.

## Testing Guidelines

The project uses Go's built-in `testing` package. Add or update colocated `*_test.go` files for behavioral changes, especially in `internal/fs`, `internal/config`, `internal/git`, and command-generation logic. Tests should avoid depending on the developer's machine state; create temporary files, directories, and environment variables inside the test.

## Commit & Pull Request Guidelines

No local git history is available in this checkout, so use concise imperative commit messages, for example `Add recursive text search tests` or `Fix config fallback handling`. Pull requests should include a short summary, test results such as `go test ./...`, and screenshots or terminal recordings when changing the Bubble Tea UI. Link related issues when applicable and call out config, filesystem, or destructive-operation behavior explicitly.

## Security & Configuration Tips

Be careful with code that deletes, moves, or copies paths. Preserve the existing safe-delete behavior that moves files under the Navia trash location, and validate path boundaries with helpers such as `IsSubpath` when adding filesystem operations. Do not commit local config, generated binaries, or temporary test output.

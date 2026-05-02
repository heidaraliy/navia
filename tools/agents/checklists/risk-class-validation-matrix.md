# Risk Class Validation Matrix

| Change type | Minimum validation |
| --- | --- |
| Docs only | `git diff --check` |
| Agent config | `validate_agent_config.py`, `bash -n`, `git diff --check` |
| Go logic | targeted package test, `go test ./...` |
| Filesystem operations | focused temp-dir tests, `go test ./internal/fs`, `go test ./...` |
| TUI behavior | targeted tests when possible, `go test ./...`, manual terminal check |
| Render/performance hot path | large-input test or benchmark when practical, `go test ./...`, inspect allocation/cache bounds |
| CI/release | local syntax inspection, `go test ./...`, hosted CI after push |

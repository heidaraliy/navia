# Navia

Navia is a terminal file navigator for people who want a fast visual workflow
while gradually learning the shell commands behind file operations.

It is written in Go with Bubble Tea and Lip Gloss. Current features include tree
navigation, file previews, recursive file and text search, create/rename/copy/cut
and paste actions, safe delete, editor launching, git-aware project context, and
shell command hints after file operations.

## Install

For the latest tagged Go module release:

```bash
go install github.com/heidaraliy/navia/cmd/navia@latest
```

For a specific tagged release, replace `v0.1.0` with the version you want:

```bash
go install github.com/heidaraliy/navia/cmd/navia@v0.1.0
```

When maintainers push a `v*` tag, the release workflow publishes Linux,
macOS, and Windows binary archives with SHA-256 checksums on the GitHub release
page.

To run from source:

```bash
git clone https://github.com/heidaraliy/navia.git
cd navia
go run ./cmd/navia
```

## Usage

```bash
navia
navia /path/to/project
navia --version
```

Common keys:

- `j` / `k` or arrow keys: move through entries
- `enter` / `l`: expand or collapse directories
- `h` / `backspace`: collapse or jump to the parent
- `/`: search recursively from the current directory
- `tab`: toggle file-name and text search while searching
- `n` / `N`: create a file or directory
- `r`: rename
- `y`, `x`, `p`: copy, cut, and paste
- `d`: safe delete
- `e`: open the selected file in `$VISUAL`, `$EDITOR`, or `nvim`
- `?`: help
- `q`: quit

## Configuration

Navia reads `~/.config/navia/config.toml`, or
`$XDG_CONFIG_HOME/navia/config.toml` when `XDG_CONFIG_HOME` is set.

```toml
show_hidden = false
editor = "nvim"
safe_delete = true
sort_dirs_first = true
preview_max_bytes = 262144
```

## Development

```bash
go test ./...
go run ./cmd/navia
go build ./cmd/navia
```

Run `go fmt ./...` before committing Go changes.

## Releases

Maintainers publish a release by pushing a `v*` tag, such as `v0.1.0`. The
tagged release workflow runs tests, builds archives for supported desktop
targets, generates checksums, and creates the GitHub release.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for contributor workflow and
[AGENTS.md](AGENTS.md) for agent-specific orchestration. Keep changes small,
tested, and explicit about filesystem safety.

## License

MIT

# Navia

Navia is a terminal micro-IDE: a fast project explorer, previewer, recursive
search tool, modal editor, git review surface, and shell-command teacher in one
keyboard-driven Go TUI.

The goal is the useful part of a VS Code-style project workspace without the
heavy startup path, extension bloat, and background machinery that can make a
simple edit feel slow. Neovim is excellent, but it asks you to become a power
user and assemble the surrounding project UI yourself. Navia takes the opposite
shape: open a directory and get the project tree, previews, editing, search,
git, and file operations immediately.

Navia is early-stage software, but it is already more than a file navigator. It
is meant to be the terminal home base for small-to-medium project work.

## What Navia Gives You

- A project cockpit: tree navigation, expandable directories, drill-in roots,
  hidden-file filtering, ignore-name filtering, and a persistent preview pane.
- Rich previews: directory summaries, text previews, image metadata, binary-file
  detection, size/mod-time details, truncation limits, and Chroma syntax
  highlighting for source files.
- Recursive search: file-name search and text search from inside the UI, plus
  startup modes for opening directly into either search surface.
- A built-in modal editor: tabs, dirty-state tracking, vim-style normal/insert/
  visual modes, counts, motions, yank/delete/paste, undo/redo, search,
  substitution, save/quit commands, and jump history.
- Lightweight code intelligence: optional Go LSP support through `gopls` for
  definition and reference jumps.
- Git review mode: status summary, formatted diff previews, auto-refresh,
  stage/unstage, restore/remove, commit, and push from the same interface.
- Safe file operations: create, rename, copy, cut, paste, and safe delete, with
  safe delete moving files into Navia trash by default.
- Shell literacy: after file operations, Navia shows the equivalent shell command
  so the visual workflow still teaches the command-line workflow.

## Install

Navia requires Go 1.22 or newer.

For the latest tagged Go module release:

```bash
go install github.com/heidaraliy/navia/cmd/navia@latest
```

For a specific tagged release:

```bash
go install github.com/heidaraliy/navia/cmd/navia@v0.1.0
```

You can also build from source:

```bash
git clone https://github.com/heidaraliy/navia.git
cd navia
go run ./cmd/navia
```

Tagged releases attach Linux, macOS, and Windows binary archives plus SHA-256
checksums to the GitHub release page.

## Usage

```bash
navia
navia /path/to/project
navia -d
navia -s "search text"
navia -f "file name"
navia --version
```

Startup modes:

- `navia`: open the current directory as a project workspace.
- `navia /path/to/project`: open a specific directory.
- `navia -d`: start in git diff mode.
- `navia -s "query"`: start in recursive text search.
- `navia -f "query"`: start in recursive file-name search.
- `navia --version`: print the installed version.

Common keys:

- `j` / `k` or arrow keys: move through entries.
- `enter` / `l`: expand directories or open search results.
- `h` / `backspace`: collapse or jump to the parent.
- `L` / `shift+enter`: make the selected directory the root.
- `/`: search recursively from the current directory.
- `tab`: toggle file-name and text search while searching.
- `g`: go to a path.
- `n` / `N`: create a file or directory.
- `r`: rename.
- `y`, `x`, `p`: copy, cut, and paste.
- `d`: safe delete.
- `e` / `c`: open the selected file in the Navia editor.
- `D`: open git diff mode.
- `?`: help.
- `q`: quit.

Diff mode keys:

- `s` / `u`: stage or unstage the selected file.
- `R` / `D`: restore or remove the selected file.
- `c` / `p`: commit or push the current branch.
- `r`: refresh manually.
- `esc`: return to the tree.

Editor keys:

- `i`, `a`, `I`, `A`, `o`, `O`: enter insert mode.
- `h`, `j`, `k`, `l`, `w`, `b`, `e`: move the cursor.
- `gg`, `G`, `:number`: jump by file position.
- `v` / `V`: visual or visual-line selection.
- `y`, `d`, `c`, `p`: yank, delete, change, and paste.
- `u` / `ctrl+r`: undo and redo.
- `/`, `n`, `N`: search inside the open buffer.
- `gd` / `gr`: jump to definition or references when LSP is available.
- `ctrl+o` / `ctrl+i`: jump backward or forward through editor history.
- `:w`, `:q`, `:wq`, `:qa`: save and close commands.
- `:e path`: open another file in a tab.
- `:bn` / `:bp`, `gt` / `gT`: move between editor tabs.
- `:theme`: list themes; `:theme navia` switches the syntax theme.
- `:nvim`: open the active buffer in your configured external editor.
- `ctrl+w h`, `ctrl+w l`, `ctrl+w o`: focus panes or use editor-only view.

Press `?` inside Navia for the full in-app key reference.

## Configuration

Navia reads `~/.config/navia/config.toml`, or
`$XDG_CONFIG_HOME/navia/config.toml` when `XDG_CONFIG_HOME` is set.

```toml
show_hidden = false
editor = "nvim"
safe_delete = true
sort_dirs_first = true
preview_max_bytes = 262144
editor_max_bytes = 1048576
enable_lsp = true
gopls_command = "gopls"
theme = "navia"
ignore_names = ".git,node_modules,.next,dist,build,target,.cache"
```

`editor` defaults to `$VISUAL`, then `$EDITOR`, then `nvim`. Safe delete is on
by default and uses `$XDG_DATA_HOME/navia/trash` or
`~/.local/share/navia/trash`.

## Development

```bash
go test ./...
go run ./cmd/navia
go build ./cmd/navia
```

Run `go fmt ./...` before committing Go changes.

## Releases

Maintainers publish tagged releases by pushing a `v*` tag, such as `v0.1.0`.
The release workflow runs formatting checks, `go vet ./...`, `go test ./...`,
builds supported desktop archives, writes checksums, and creates the GitHub
release.

See [docs/releasing.md](docs/releasing.md) for the release checklist.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for contributor workflow and
[AGENTS.md](AGENTS.md) for agent-specific orchestration. Keep changes small,
tested, and explicit about filesystem safety.

## License

MIT

<div align="center">

<h1>
  <img src="docs/assets/navia-logo.svg" alt="Navia" width="486">
</h1>

<p><strong>A microIDE in your terminal.</strong></p>

<p>
Navia is a terminal micro-IDE: a fast project explorer, previewer, recursive
search tool, modal editor, and git review surface, in one keyboard-driven Go TUI.
</p>

</div>

## What Navia Gives You

- A project cockpit: tree navigation, expandable directories, drill-in roots,
  hidden-file filtering, ignore-name filtering, and a persistent preview pane.
- Rich previews: directory summaries, text previews, image metadata, binary-file
  detection, size/mod-time details, truncation limits, and Chroma syntax
  highlighting for source files.
- Recursive search: file-name search and text search from inside the UI, plus
  startup modes for opening directly into either search surface.
- A built-in modal editor: tabs, dirty-state tracking, vim-style normal/insert/
  visual modes, counts, motions, yank/delete/paste, undo/redo, rich Markdown
  highlighting with task-checkbox toggles, search, substitution, save/quit
  commands, and jump history.
- Lightweight code intelligence: optional Go LSP support through `gopls` for
  definition and reference jumps.
- Git review mode: status summary, formatted diff previews, auto-refresh,
  stage/unstage, restore/remove, commit, and push from the same interface.
- Safe file operations: create, rename, copy, cut, paste, and safe delete, with
  safe delete moving files into Navia trash by default.

## Install

Users do not need Go to run Navia.

Install the latest release with:

```bash
curl -fsSL https://raw.githubusercontent.com/heidaraliy/navia/main/install.sh | sh
```

Or install with Homebrew:

```bash
brew install heidaraliy/tap/navia
```

Manual install is also supported. Download the archive for your platform from
the latest GitHub release, extract it, and put the `navia` binary somewhere on
your `PATH`.

Example for Apple Silicon Macs:

```bash
tar -xzf navia_0.1.0_darwin_arm64.tar.gz
mkdir -p ~/.local/bin
install -m 0755 navia_0.1.0_darwin_arm64/navia ~/.local/bin/navia
navia --version
```

Use the matching archive name for other platforms: `darwin_amd64` for Intel
Macs, `linux_amd64` for most Linux PCs, and `linux_arm64` for ARM Linux
machines.

Windows:

1. Download `navia_0.1.0_windows_amd64.zip`, or `windows_arm64` for ARM
   Windows.
2. Extract the zip.
3. Move `navia.exe` to a directory on your `PATH`.
4. Run `navia --version` in PowerShell.

SHA-256 checksums are attached to each tagged release.

### Install From Source

Source installs require Go 1.22 or newer.

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
- `space`: toggle the current Markdown task checkbox.
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

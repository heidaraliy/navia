<div align="center">

<h1>
  <img src="docs/assets/navia-logo.svg" alt="Navia" width="486">
</h1>

<p><strong>A micro-IDE in your terminal.</strong></p>

<p>
Navia is a fast, keyboard-driven development environment for users
that desire a programming experience contained within the terminal,
but prefer navigation that succinctly executes <strong>intent</strong>.
</p>

</div>

---

## Why Navia Exists

Selfishly, I built Navia for myself. The terminal is powerful, but project navigation is still weirdly fragmented,
at least for someone who was taught to program entirely in VSCode. With LLMs and CLI tools like Claude Code and
Codex becoming vital tools, I started using the terminal more, in tandem with VSCode. 

Eventually, the back-and-forth swapping became a nuisance, and while jumping from `cd` to `ls` to `find` to `grep` to 
`nvim` to `git diff` is great, it's not exactly what I'd call "fun". Feels more like starting with a 6mm socket and 
trying every size until you get to the right one.

Navia gives you one focused surface for the stuff you do constantly inside a codebase:

- move around the tree
- preview files without opening them
- search file names and file contents
- make quick edits
- review diffs
- stage, commit, and push changes
- safely move, rename, copy, paste, and delete files

It's definitely *not* trying to replace your editor or shell.

It *is* trying to make the space between them way cleaner.

## What You Get

### Project Navigation

Navia gives you a persistent project tree with fast keyboard movement,
expandable directories, drill-in roots, hidden-file filtering, ignored-name
filtering, and a preview pane that follows your selection.

You can use it as a lightweight file navigator, a project browser, or a quick
way to understand a repo you just opened.

### Rich Previews

Directories show useful summaries. Text files show readable previews. Source
files get Chroma syntax highlighting. Binary files are detected instead of
dumped into your terminal like garbage.

Navia also shows file size, modified time, image metadata, and truncates large
previews so your terminal stays responsive.

### Recursive Search

Search by file name or by text content directly inside the TUI.

You can also launch straight into search mode:

```bash
navia -s "search text"
navia -f "file name"
```

### Built-In Modal Editor

Navia includes a modal editor for quick edits without leaving the workspace.

It supports tabs, dirty-state tracking, vim-style normal/insert/visual modes,
counts, motions, yank/delete/paste, undo/redo, search, substitution, save/quit
commands, jump history, and Markdown task-checkbox toggles.

For bigger edits, use your actual editor:

```vim
:nvim
```

By default, Navia resolves your editor from `$VISUAL`, then `$EDITOR`, then
`nvim`.

### Lightweight Code Intelligence

Navia currently ships with lightweight Go LSP support through `gopls`.

LSP features:

- jump to definition with `gd`
- find references with `gr`
- jump backward/forward through editor history

### Git Review Mode

Navia has a built-in git review surface for the normal loop:

- see repo status
- preview formatted diffs
- stage and unstage files
- restore or remove changed files
- commit
- push
- refresh automatically

Launch directly into diff mode:

```bash
navia -d
```

### Safe File Operations

Navia supports create, rename, copy, cut, paste, and delete.

Safe delete is enabled by default. Deleted files are moved into Navia trash
instead of being immediately destroyed.

## Install

You do not need Go to run Navia.

Install the latest release:

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

Use the matching archive for your system:

- `darwin_arm64` for Apple Silicon Macs
- `darwin_amd64` for Intel Macs
- `linux_amd64` for most Linux PCs
- `linux_arm64` for ARM Linux machines
- `windows_amd64` for most Windows PCs
- `windows_arm64` for ARM Windows machines

### Windows

1. Download `navia_0.1.0_windows_amd64.zip`, or the ARM archive if needed.
2. Extract the zip.
3. Move `navia.exe` to a directory on your `PATH`.
4. Run this in PowerShell:

```powershell
navia --version
```

SHA-256 checksums are attached to each tagged release.

## Install From Source

Source installs require Go 1.22 or newer.

Install the latest tagged Go module release:

```bash
go install github.com/heidaraliy/navia/cmd/navia@latest
```

Install a specific tagged release:

```bash
go install github.com/heidaraliy/navia/cmd/navia@v0.1.0
```

Run from source:

```bash
git clone https://github.com/heidaraliy/navia.git
cd navia
go run ./cmd/navia
```

Build from source:

```bash
go build ./cmd/navia
```

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

- `navia`: open the current directory.
- `navia /path/to/project`: open a specific directory.
- `navia -d`: start in git diff mode.
- `navia -s "query"`: start in recursive text search.
- `navia -f "query"`: start in recursive file-name search.
- `navia --version`: print the installed version.

## Common Keys

Tree mode:

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

Diff mode:

- `s` / `u`: stage or unstage the selected file.
- `R` / `D`: restore or remove the selected file.
- `c` / `p`: commit or push the current branch.
- `r`: refresh manually.
- `esc`: return to the tree.

Editor mode:

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
- `:theme`: list themes.
- `:theme navia`: switch to the Navia syntax theme.
- `:nvim`: open the active buffer in your configured external editor.
- `ctrl+w h`, `ctrl+w l`, `ctrl+w o`: focus panes or use editor-only view.

Press `?` inside Navia for the full in-app key reference.

## Configuration

Navia reads config from:

```text
~/.config/navia/config.toml
```

Or, when `XDG_CONFIG_HOME` is set:

```text
$XDG_CONFIG_HOME/navia/config.toml
```

Example config:

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

Safe delete is enabled by default.

Deleted files are moved to:

```text
$XDG_DATA_HOME/navia/trash
```

Or, when `XDG_DATA_HOME` is not set:

```text
~/.local/share/navia/trash
```

## Development

```bash
go test ./...
go run ./cmd/navia
go build ./cmd/navia
```

Run this before committing Go changes:

```bash
go fmt ./...
```

## Releases

Maintainers publish releases by pushing a `v*` tag, such as:

```bash
v0.1.0
```

The release workflow runs formatting checks, `go vet ./...`, `go test ./...`,
builds supported desktop archives, writes checksums, and creates the GitHub
release.

See [docs/releasing.md](docs/releasing.md) for the release checklist.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for contributor workflow and
[AGENTS.md](AGENTS.md) for agent-specific orchestration.

Keep changes small, tested, and explicit about filesystem safety.

## License

MIT

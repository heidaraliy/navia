# Navia

Navia is a fast, read-only terminal file navigator and Git diff explorer.

It gives you one focused surface for browsing a project, previewing and searching files, reviewing working-tree changes or previous commits, and handing real edits to Neovim. Navia never creates, renames, copies, deletes, stages, restores, commits, or pushes files.

## Run

```sh
navia [path]
navia -d [path]
```

The default mode is the filesystem navigator. `-d` opens the Git differ directly. A path may name either a directory or a file; file paths start with that file selected.

## Navigator

- Expandable project tree with hidden/ignored-name filtering.
- Syntax-highlighted, size-bounded text previews and safe binary/image metadata.
- Recursive filename and content search from `/`; press `Tab` to switch search type.
- `Enter` expands or collapses a directory and opens a file in `$VISUAL`, `$EDITOR`, or `nvim`.
- The preview is never focused: scroll it with `Ctrl-j/k`, `Ctrl-↑/↓`, or `PgUp/PgDn`.

| Key | Action |
| --- | --- |
| `j/k`, `↑/↓` | Select a tree row. |
| `J/K`, `Shift-↑/↓` | Page through tree rows. |
| `Enter` | Expand/collapse a directory or open a file externally. |
| `l`, `→` | Expand a directory; do nothing on files. |
| `h`, `←`, `Backspace` | Collapse or move to the parent. |
| `/` | Search; `Tab` switches filename/content search. |
| `D` | Open the Git differ. |
| `F` / `f` | Toggle fullscreen tree / preview. |
| `?` | Show keybindings. |
| `q` | Quit. |

## Git differ

Navia compares `HEAD` with staged, unstaged, and untracked working-tree changes. It shows file and line totals, per-file statistics, syntax-highlighted unified or side-by-side diffs, binary metadata, end-preserving long paths, mouse scrolling, and a draggable center divider.

Press `c` to choose `Working Tree` or a previous first-parent commit. Commit pages contain 50 entries; `Load more…` appends the next page. A selected commit is compared with its first parent, and a root commit is compared with Git’s empty tree.

| Key | Action |
| --- | --- |
| `j/k`, `↑/↓` | Select a changed file. |
| `J/K`, `Shift-↑/↓` | Page through changed files. |
| `Ctrl-j/k`, `Ctrl-↑/↓`, `PgUp/PgDn` | Page through the diff. |
| `/` | Search changed paths; Enter includes file contents. |
| `v` | Toggle unified and side-by-side layouts. |
| `c` | Open commit history. |
| `Enter`, `Ctrl-o` | Open the selected file externally. Historical/deleted files open read-only temporary snapshots. |
| `F` / `f` | Toggle fullscreen file list / diff. |
| `r` | Refresh. |
| `Esc` | Return to the navigator. |
| `q` | Quit. |

## Configuration

Navia reads `~/.config/navia/config.toml`, or `$XDG_CONFIG_HOME/navia/config.toml`:

```toml
show_hidden = false
editor = "nvim"
sort_dirs_first = true
preview_max_bytes = 262144
theme = "navia"
ignore_names = ".git, node_modules, .next, dist, build, target, .cache"
```

Obsolete v1 keys are ignored for compatibility.

## Install and development

```sh
go run ./cmd/navia
go install ./cmd/navia
go test ./...
go vet ./...
```

Source builds require Go 1.22 or newer. `./install.sh` installs the latest published release archive; release archives and Homebrew installs do not require Go.

Navia is licensed under the MIT License.

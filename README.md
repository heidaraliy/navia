# Navia

Navia is a file navigation and diff viewer.

## Features

- Read-only filesystem navigation
- File previews and search
- Syntax-highlighted Git diffs
- Working-tree and commit history views
- Opens files in `$VISUAL`, `$EDITOR`, or Neovim

## Usage

```sh
navia [path]
navia -d [path]
```

`navia` opens the file navigator. `navia -d` opens the Git diff viewer.

Press `?` inside Navia to view all keybindings.

## Install

```sh
go install github.com/heidaraliy/navia/cmd/navia@latest
```

Or install the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/heidaraliy/navia/main/install.sh | sh
```

## Development

```sh
go test ./...
go vet ./...
go run ./cmd/navia
```

MIT licensed.

#!/usr/bin/env sh
set -eu

repo="heidaraliy/navia"
install_dir="${NAVIA_INSTALL_DIR:-$HOME/.local/bin}"
requested_version="${NAVIA_VERSION:-latest}"

usage() {
  cat <<'USAGE'
Usage: install.sh

Installs the latest Navia release archive for this machine.

Environment:
  NAVIA_VERSION       Release tag to install. Defaults to latest.
  NAVIA_INSTALL_DIR   Install directory. Defaults to ~/.local/bin.
USAGE
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "navia install: missing required command: $1" >&2
    exit 1
  fi
}

need curl
need tar

case "$(uname -s)" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *)
    echo "navia install: unsupported OS: $(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *)
    echo "navia install: unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

if [ "$requested_version" = "latest" ]; then
  tag="$(
    curl -fsSL "https://api.github.com/repos/$repo/releases/latest" |
      sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
      head -n 1
  )"
else
  tag="$requested_version"
fi

if [ -z "${tag:-}" ]; then
  echo "navia install: could not resolve release tag" >&2
  exit 1
fi

case "$tag" in
  v*) version="${tag#v}" ;;
  *) version="$tag"; tag="v$tag" ;;
esac

name="navia_${version}_${os}_${arch}"
archive="$name.tar.gz"
checksums="navia_${version}_checksums.txt"
base_url="https://github.com/$repo/releases/download/$tag"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/navia-install.XXXXXX")"

cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT INT TERM

echo "Downloading $archive"
curl -fsSL "$base_url/$archive" -o "$tmp/$archive"
curl -fsSL "$base_url/$checksums" -o "$tmp/$checksums"

checksum_line="$(grep "  $archive\$" "$tmp/$checksums" || true)"
if [ -z "$checksum_line" ]; then
  echo "navia install: checksum entry missing for $archive" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$tmp" && printf '%s\n' "$checksum_line" | sha256sum -c -)
elif command -v shasum >/dev/null 2>&1; then
  (cd "$tmp" && printf '%s\n' "$checksum_line" | shasum -a 256 -c -)
else
  echo "navia install: sha256sum or shasum not found; skipping checksum verification" >&2
fi

tar -xzf "$tmp/$archive" -C "$tmp"

mkdir -p "$install_dir"
if command -v install >/dev/null 2>&1; then
  install -m 0755 "$tmp/$name/navia" "$install_dir/navia"
else
  cp "$tmp/$name/navia" "$install_dir/navia"
  chmod 0755 "$install_dir/navia"
fi

echo "Installed navia $version to $install_dir/navia"
"$install_dir/navia" --version

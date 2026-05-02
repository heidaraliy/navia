#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C
export LANG=C

usage() {
  cat <<'USAGE'
Usage: build_release.sh <version>

Builds Navia release archives into dist/.

Environment:
  DIST_DIR   Output directory. Defaults to dist.
  TARGETS    Space-separated GOOS/GOARCH pairs. Defaults to:
             linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

version="${1:-}"
if [[ -z "$version" ]]; then
  usage >&2
  exit 2
fi

version="${version#v}"
if [[ ! "$version" =~ ^[0-9]+[.][0-9]+[.][0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "release version must look like v1.2.3 or 1.2.3, got: $version" >&2
  exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
dist_input="${DIST_DIR:-dist}"
if [[ -z "$dist_input" || "$dist_input" =~ ^/+$ || "$dist_input" == "." || "$dist_input" == ".." ]]; then
  echo "DIST_DIR must name a build output directory, got: $dist_input" >&2
  exit 2
fi
if [[ "$dist_input" == /* ]]; then
  dist_dir="$dist_input"
else
  dist_dir="$repo_root/$dist_input"
fi
dist_base="$(basename "$dist_dir")"
if [[ "$dist_base" == "." || "$dist_base" == ".." ]]; then
  echo "DIST_DIR must name a build output directory, got: $dist_input" >&2
  exit 2
fi
mkdir -p "$(dirname "$dist_dir")"
dist_dir="$(cd "$(dirname "$dist_dir")" && pwd -P)/$dist_base"
home_dir=""
if [[ -n "${HOME:-}" && -d "$HOME" ]]; then
  home_dir="$(cd "$HOME" && pwd -P)"
fi
if [[ "$dist_dir" == "/" || "$dist_dir" == "$repo_root" || (-n "$home_dir" && "$dist_dir" == "$home_dir") ]]; then
  echo "refusing to clean unsafe DIST_DIR: $dist_dir" >&2
  exit 2
fi
targets="${TARGETS:-linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64}"
commit="$(git -C "$repo_root" rev-parse --short=12 HEAD)"

rm -rf -- "$dist_dir"
mkdir -p "$dist_dir"

for target in $targets; do
  goos="${target%/*}"
  goarch="${target#*/}"
  if [[ -z "$goos" || -z "$goarch" || "$goos" == "$goarch" ]]; then
    echo "invalid target: $target" >&2
    exit 2
  fi

  name="navia_${version}_${goos}_${goarch}"
  staging="$dist_dir/$name"
  mkdir -p "$staging"

  binary="navia"
  if [[ "$goos" == "windows" ]]; then
    binary="navia.exe"
  fi

  echo "building $target"
  (
    cd "$repo_root"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build \
      -trimpath \
      -ldflags "-s -w -X main.version=$version" \
      -o "$staging/$binary" \
      ./cmd/navia
  )

  cp "$repo_root/README.md" "$repo_root/LICENSE" "$staging/"

  (
    cd "$dist_dir"
    if [[ "$goos" == "windows" ]]; then
      zip -qr "$name.zip" "$name"
    else
      tar -czf "$name.tar.gz" "$name"
    fi
    rm -rf "$name"
  )
done

(
  cd "$dist_dir"
  shasum -a 256 navia_* > "navia_${version}_checksums.txt"
)

cat > "$dist_dir/RELEASE_NOTES.md" <<NOTES
Navia v$version

Commit: $commit

Install from source with:

    go install github.com/heidaraliy/navia/cmd/navia@v$version

Binary archives and SHA-256 checksums are attached to this release.
NOTES

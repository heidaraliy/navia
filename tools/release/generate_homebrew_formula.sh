#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C
export LANG=C

usage() {
  cat <<'USAGE'
Usage: generate_homebrew_formula.sh <version> <checksums-file>

Writes a Homebrew formula for Navia to stdout.

Example:
  tools/release/generate_homebrew_formula.sh v0.1.0 dist/navia_0.1.0_checksums.txt > packaging/homebrew/navia.rb
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

version="${1:-}"
checksums="${2:-}"
if [[ -z "$version" || -z "$checksums" ]]; then
  usage >&2
  exit 2
fi

version="${version#v}"
tag="v$version"
if [[ ! "$version" =~ ^[0-9]+[.][0-9]+[.][0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "version must look like v1.2.3 or 1.2.3, got: $version" >&2
  exit 2
fi
if [[ ! -f "$checksums" ]]; then
  echo "checksums file not found: $checksums" >&2
  exit 2
fi

sha_for() {
  local asset="$1"
  awk -v asset="$asset" '$2 == asset { print $1 }' "$checksums"
}

require_sha() {
  local asset="$1"
  local value="$2"

  if [[ -z "$value" ]]; then
    echo "missing checksum for $asset in $checksums" >&2
    exit 2
  fi
  if [[ ! "$value" =~ ^[0-9A-Fa-f]{64}$ ]]; then
    echo "checksum for $asset must be exactly 64 hex characters" >&2
    exit 2
  fi
}

darwin_amd64="navia_${version}_darwin_amd64.tar.gz"
darwin_arm64="navia_${version}_darwin_arm64.tar.gz"
linux_amd64="navia_${version}_linux_amd64.tar.gz"
linux_arm64="navia_${version}_linux_arm64.tar.gz"

sha_darwin_amd64="$(sha_for "$darwin_amd64")"
sha_darwin_arm64="$(sha_for "$darwin_arm64")"
sha_linux_amd64="$(sha_for "$linux_amd64")"
sha_linux_arm64="$(sha_for "$linux_arm64")"

require_sha "$darwin_amd64" "$sha_darwin_amd64"
require_sha "$darwin_arm64" "$sha_darwin_arm64"
require_sha "$linux_amd64" "$sha_linux_amd64"
require_sha "$linux_arm64" "$sha_linux_arm64"

cat <<FORMULA
# typed: strict
# frozen_string_literal: true

# Homebrew formula for Navia.
class Navia < Formula
  desc "Terminal micro-IDE for project navigation, editing, search, and git review"
  homepage "https://github.com/heidaraliy/navia"
  version "$version"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/heidaraliy/navia/releases/download/$tag/$darwin_arm64"
      sha256 "$sha_darwin_arm64"
    else
      url "https://github.com/heidaraliy/navia/releases/download/$tag/$darwin_amd64"
      sha256 "$sha_darwin_amd64"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/heidaraliy/navia/releases/download/$tag/$linux_arm64"
      sha256 "$sha_linux_arm64"
    else
      url "https://github.com/heidaraliy/navia/releases/download/$tag/$linux_amd64"
      sha256 "$sha_linux_amd64"
    end
  end

  def install
    bin.install "navia"
  end

  test do
    assert_match "navia $version", shell_output("#{bin}/navia --version")
  end
end
FORMULA

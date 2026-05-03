#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/navia-release-hardening.XXXXXX")"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT INT TERM

checksums="$tmp/checksums.txt"

valid_sha="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
valid_upper_sha="0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF"
cat > "$checksums" <<EOF
$valid_sha  navia_0.1.0_darwin_amd64.tar.gz
$valid_upper_sha  navia_0.1.0_darwin_arm64.tar.gz
$valid_sha  navia_0.1.0_linux_amd64.tar.gz
$valid_sha  navia_0.1.0_linux_arm64.tar.gz
EOF

"$repo_root/tools/release/generate_homebrew_formula.sh" v0.1.0 "$checksums" > "$tmp/navia.rb"
grep -q "sha256 \"$valid_sha\"" "$tmp/navia.rb"
grep -q "sha256 \"$valid_upper_sha\"" "$tmp/navia.rb"

cat > "$checksums" <<EOF
not-a-sha  navia_0.1.0_darwin_amd64.tar.gz
$valid_sha  navia_0.1.0_darwin_arm64.tar.gz
$valid_sha  navia_0.1.0_linux_amd64.tar.gz
$valid_sha  navia_0.1.0_linux_arm64.tar.gz
EOF

if "$repo_root/tools/release/generate_homebrew_formula.sh" v0.1.0 "$checksums" > "$tmp/bad.rb" 2> "$tmp/bad.err"; then
  echo "expected invalid checksum to fail" >&2
  exit 1
fi
grep -q "must be exactly 64 hex characters" "$tmp/bad.err"

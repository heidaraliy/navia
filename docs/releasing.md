# Releasing Navia

This checklist is for maintainers cutting a tagged Navia release.

## Version Choice

Use semantic versions with a leading `v` tag. For the first public release,
`v0.1.0` is the expected starting point unless there is a reason to mark the
project as pre-release.

## Release Flow

1. Prepare release-facing docs or automation changes on a feature branch.
2. Open a pull request into `main` and wait for CI to pass.
3. Merge the pull request.
4. From the main checkout, update `main`:

```bash
git checkout main
git pull --ff-only origin main
```

5. Run local validation:

```bash
go test ./...
TARGETS="darwin/arm64" DIST_DIR="$(mktemp -d)/navia-dist" tools/release/build_release.sh v0.1.0
```

6. Create and push the tag:

```bash
git tag -a v0.1.0 -m "Navia v0.1.0"
git push origin v0.1.0
```

7. Watch the `Tagged Release` workflow in GitHub Actions.
8. Confirm the GitHub release exists, has archives for Linux, macOS, and
   Windows, and includes `navia_0.1.0_checksums.txt`.
9. Smoke-test a downloaded binary archive:

```bash
tmp="$(mktemp -d)"
tar -xzf navia_0.1.0_darwin_arm64.tar.gz -C "$tmp"
"$tmp/navia_0.1.0_darwin_arm64/navia" --version
```

10. Smoke-test the tagged source install:

```bash
go install github.com/heidaraliy/navia/cmd/navia@v0.1.0
navia --version
```

11. Generate and publish the Homebrew formula:

```bash
gh release download v0.1.0 --pattern 'navia_0.1.0_checksums.txt' --dir /tmp/navia-release
tools/release/generate_homebrew_formula.sh v0.1.0 /tmp/navia-release/navia_0.1.0_checksums.txt > packaging/homebrew/navia.rb
```

The formula generator rejects missing or non-hex SHA-256 values before writing
formula output. Do not hand-edit checksum values around that validation.

Copy the generated formula to `heidaraliy/homebrew-tap` as `Formula/navia.rb`,
then validate the tap with:

```bash
brew install heidaraliy/tap/navia
navia --version
brew test heidaraliy/tap/navia
brew uninstall navia
```

12. Smoke-test the install script:

```bash
NAVIA_INSTALL_DIR="$(mktemp -d)" sh install.sh
```

The install script must verify the downloaded archive with `sha256sum` or
`shasum`; it fails before downloading when neither checksum tool is available.

## What The Tag Does

Pushing a `v*` tag starts `.github/workflows/release.yml`. The workflow:

- checks out the tagged commit;
- installs Go 1.22;
- downloads modules;
- checks formatting;
- runs `go vet ./...`;
- runs `go test ./...`;
- runs `tools/release/build_release.sh`;
- creates the GitHub release with generated archives, checksums, and notes.

Do not tag a commit before the release-ready README and validation changes have
merged into `main`. The tag is the source of truth for both Go module installs
and the GitHub release artifacts.

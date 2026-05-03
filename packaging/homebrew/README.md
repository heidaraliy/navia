# Homebrew Packaging

Navia's Homebrew formula is generated from release checksums.

```bash
gh release download v0.1.0 --pattern 'navia_0.1.0_checksums.txt' --dir /tmp/navia-release
tools/release/generate_homebrew_formula.sh v0.1.0 /tmp/navia-release/navia_0.1.0_checksums.txt > packaging/homebrew/navia.rb
```

The generated formula is intended for the `heidaraliy/homebrew-tap` repository
at `Formula/navia.rb`. The generator validates every emitted `sha256` as a
64-character hexadecimal SHA-256 value.

Validate tap changes with:

```bash
brew style heidaraliy/tap
brew install heidaraliy/tap/navia
brew test heidaraliy/tap/navia
```

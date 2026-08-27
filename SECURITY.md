# Security Policy

## Supported Versions

Navia is early-stage open source software. Security fixes target the latest
tagged release when one is available; otherwise they target the `main` branch.

## Reporting A Vulnerability

Please do not open a public issue for a vulnerability that could cause data loss, command execution, or unsafe file access. Email the maintainer or use a private GitHub security advisory when available.

Include:

- affected commit or version
- operating system and terminal
- reproduction steps
- expected impact

## Safety-Sensitive Areas

The highest-risk areas are path boundary checks, bounded file previews, external-editor launching, Git subprocesses, config parsing, and future automation that may run external commands. Navia does not provide filesystem or Git mutation commands.

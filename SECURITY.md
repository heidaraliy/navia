# Security Policy

## Supported Versions

Navia is early-stage open source software. Until tagged releases exist, security fixes target the `main` branch.

## Reporting A Vulnerability

Please do not open a public issue for a vulnerability that could cause data loss, command execution, or unsafe file access. Email the maintainer or use a private GitHub security advisory when available.

Include:

- affected commit or version
- operating system and terminal
- reproduction steps
- expected impact

## Safety-Sensitive Areas

The highest-risk areas are filesystem mutation, path boundary checks, editor launching, config parsing, and future automation that may run external commands.

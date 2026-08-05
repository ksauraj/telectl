---
title: Contributing Guide
nav_order: 20
---

# Contributing Guide

How to contribute to telectl. The full canonical version lives in
[`CONTRIBUTING.md`](https://github.com/ksauraj/telectl/blob/main/CONTRIBUTING.md)
in the repo — this page is the wiki edition.

## Code of conduct

This project adheres to the
[Contributor Covenant](https://github.com/ksauraj/telectl/blob/main/CODE_OF_CONDUCT.md).
By participating you agree to uphold it. Report unacceptable behavior to
gitsauraj@gmail.com.

## Getting started

1. Fork the repository on GitHub.
2. Clone your fork and add the upstream remote:
   ```bash
   git clone https://github.com/YOUR_USERNAME/telectl.git
   cd telectl
   git remote add upstream https://github.com/ksauraj/telectl.git
   ```
3. Create a branch for your work:
   ```bash
   git checkout -b feat/your-feature
   ```

See [Development Setup](Development-Setup) for prerequisites and build
instructions.

## Making changes

- Keep changes focused; one logical change per PR.
- Update tests alongside code — the repo has dedicated unit tests for
  flag parsing, rich-text rendering, and callback parsing that pin
  hard-won behavior (see [Testing Guide](Testing-Guide)).
- Run the full check pipeline before pushing:
  ```bash
  make fmt
  make vet
  make lint
  make test
  ```

## Commit messages

- Use conventional commits: `fix:`, `feat:`, `docs:`, `refactor:`, `test:`,
  `chore:`.
- Reference the issue/PR context in the body when relevant.
- Commits are authored as `ksauraj <gitsauraj@gmail.com>` by maintainers.

## Pull request process

1. Push your branch and open a PR against `main`.
2. CI runs automatically: go mod tidy check, formatting, vet, tests, lint,
   build, smoke test, and (on tags) the multi-platform release build.
3. **A green CI is required before merge.**
4. Two-eyeball review applies for anything touching security (impersonation,
   RBAC, token handling).

## Good first issues

Look for issues labeled `good first issue` / `help wanted`. Focus areas that
are well-isolated:

- The `allowed_commands` allowlist in config (must stay in sync with
  registered handlers — there are tests for this).
- Menu callback-data encoding (`internal/menus`).
- Log/exec flag parsing (`internal/handlers`).

## Documentation

- User-facing docs live in `docs/wiki/` (this wiki's source). If your change
  alters a command, a config key, or a security behavior, update the
  corresponding page.
- Wiki pages are also mirrored to the GitHub wiki and the GitHub Pages site;
  the `docs/wiki/` copies are the source of truth.

## Release process

See [Release Process](Release-Process) for tagging, versioning, and CI.
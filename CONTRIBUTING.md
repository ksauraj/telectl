# Contributing to telectl

Thank you for your interest in contributing to telectl! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Code Style](#code-style)
- [Commit Messages](#commit-messages)
- [Pull Request Process](#pull-request-process)
- [Release Process](#release-process)

## Code of Conduct

This project adheres to the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code. Please report unacceptable behavior to conduct@ksauraj.com.

## Getting Started

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/telectl.git
   cd telectl
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/ksauraj/telectl.git
   ```

## Development Setup

### Prerequisites

- Go 1.23+
- Docker (for container testing)
- Kind (for local Kubernetes testing)
- Helm 3.12+
- golangci-lint v2.1.6
- goimports
- staticcheck

### Quick Setup

```bash
# Install development tools
make dev-setup

# Verify setup
make check
```

### Manual Tool Installation

```bash
# golangci-lint (v2.1.6 - pinned to CI version)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.1.6

# goimports
go install golang.org/x/tools/cmd/goimports@latest

# staticcheck
go install honnef.co/go/tools/cmd/staticcheck@latest

# mockgen (for generating mocks)
go install github.com/golang/mock/mockgen@latest
```

## Making Changes

### Branch Naming

Use descriptive branch names:
- `feat/short-description` - New features
- `fix/short-description` - Bug fixes
- `docs/short-description` - Documentation updates
- `refactor/short-description` - Code refactoring
- `ci/short-description` - CI/CD changes
- `chore/short-description` - Maintenance tasks

### Workflow

1. Create a new branch from `main`:
   ```bash
   git checkout main
   git pull upstream main
   git checkout -b feat/your-feature-name
   ```

2. Make your changes with tests

3. Run the full check pipeline:
   ```bash
   make check
   ```

4. Commit with conventional commit messages (see below)

5. Push to your fork and open a PR

## Testing

### Unit Tests

```bash
# Run all tests with race detector
make test

# Run specific package tests
go test -v -race ./internal/bot/...
go test -v -race ./internal/k8s/...
```

### Integration Tests

```bash
# Run with Kind cluster (requires kind installed)
make test-integration
```

### Coverage

```bash
make coverage
# Opens coverage.html in browser
```

## Code Style

### Go Code

- Follow standard Go formatting: `gofmt` (enforced by CI)
- Use `goimports` for import organization
- Run `golangci-lint` before committing (v2.1.6 config in `.golangci.yml`)
- Run `staticcheck` for additional static analysis

### Configuration

The project uses `.golangci.yml` v2 format with:
- Core linters enabled (errcheck, staticcheck, govet, etc.)
- Formatters: gofmt, goimports
- Disabled linters for existing codebase issues (documented in config)

### Architecture Principles

1. **No kubectl subprocess** - All K8s operations via client-go
2. **Callback data protocol** - Defined in `internal/menus/menus.go`
3. **Single pane model** - Callbacks edit, never send new messages
4. **Context switching is session-scoped** - No ~/.kube/config writes
5. **One symbol vocabulary** - Unicode glyphs in `formatters/symbols.go`

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Formatting, missing semicolons, etc.
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `perf`: Performance improvement
- `test`: Adding missing tests
- `chore`: Maintenance, dependencies, build changes
- `ci`: CI/CD changes

### Examples

```
feat(bot): add /about command with version info

Adds /about command showing version, build info, K8s server version,
and project links. Updates main menu text and bot menu button.

Closes #42
```

```
fix(k8s): handle scale subresource for deployments

Deployment scale operations require deployments/scale verb.
Adds to ClusterRole and Role templates.

Fixes #123
```

```
docs(helm): add impersonation configuration example

Adds complete values-prod.yaml example with impersonation
user mapping for per-user RBAC.
```

## Pull Request Process

### Before Submitting

- [ ] All tests pass (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] Formatting correct (`make fmt`)
- [ ] Vet passes (`make vet`)
- [ ] Commit messages follow conventional format
- [ ] Branch is up to date with `main`
- [ ] CHANGELOG.md updated (for user-facing changes)

### PR Requirements

1. **Clear title** following conventional commits
2. **Description** explaining what and why
3. **Related issues** linked (e.g., "Closes #123")
4. **Screenshots** for UI changes
5. **Test evidence** for new functionality

### Review Process

1. Automated checks must pass (CI)
2. At least one maintainer approval required
3. All conversations resolved
4. Branch protection rules enforced

### After Merge

- Delete the branch (auto-deleted on merge)
- Update local main: `git checkout main && git pull upstream main`

## Release Process

### Versioning

Follows [Semantic Versioning](https://semver.org/):
- `MAJOR.MINOR.PATCH` (e.g., `v1.2.3`)
- Pre-releases: `v1.0.0-alpha.1`, `v1.0.0-beta.1`, `v1.0.0-rc.1`

### Creating a Release

1. Update version in Makefile:
   ```bash
   # Edit VERSION in Makefile
   ```

2. Create release tag:
   ```bash
   make release
   # Or manually:
   git tag -a v0.2.0 -m "Release v0.2.0"
   git push origin v0.2.0
   ```

3. GitHub Actions will:
   - Build multi-platform binaries
   - Build and push Docker image to GHCR
   - Create GitHub Release with artifacts
   - Generate release notes

### Release Checklist

- [ ] All CI checks passing on main
- [ ] CHANGELOG.md updated
- [ ] Version bumped in Makefile
- [ ] Release notes drafted
- [ ] Tag pushed to origin

## Getting Help

- **Questions**: Open a [Discussion](https://github.com/ksauraj/telectl/discussions)
- **Bugs**: Open an [Issue](https://github.com/ksauraj/telectl/issues/new/choose)
- **Security**: See [SECURITY.md](SECURITY.md)

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).
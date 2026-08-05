# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Comprehensive Helm chart with full RBAC support
- Per-user Kubernetes impersonation for RBAC
- `/about` command with version, build info, and project links
- Enhanced welcome message with project tagline
- Bot menu button with About, Help, and Settings
- Impersonation permissions in Helm chart
- GHCR (GitHub Container Registry) for Docker images
- Dependabot configuration for automated dependency updates
- GitHub issue templates (bug, feature, security)
- Pull request template
- Branch protection documentation
- CODEOWNERS file
- Contributing guide
- Code of Conduct (Contributor Covenant v2.1)
- Security policy
- Wiki documentation structure

### Changed
- **BREAKING**: Docker image moved from Docker Hub to GHCR (`ghcr.io/ksauraj/telectl`)
- **BREAKING**: golangci-lint upgraded from v1.64.8 to v2.1.6
- **BREAKING**: Helm chart default image repository changed to GHCR
- CI workflow updated for golangci-lint v2 compatibility
- Makefile updated to use GHCR
- Documentation restructured with wiki

### Fixed
- RBAC permissions for `deployments/scale` subresource
- RBAC permissions for `pods/exec` subresource
- golangci-lint v2 configuration format
- gofmt issues on 3 files

## [v0.1.0-alpha.1] - 2026-08-05

### Added
- Initial alpha release
- Telegram bot for Kubernetes cluster management
- Interactive menu-driven UI
- Resource browsing (Pods, Deployments, Services, etc.)
- Logs with follow, previous container, tail, since
- Exec with interactive shell and command support
- Context management (list, switch, view)
- Monitoring (top pods/nodes, events, watch)
- Inline queries
- Dry-run mode
- Rate limiting
- Helm chart for Kubernetes deployment
- In-cluster authentication support

---

## Release Types

- **Alpha**: Early preview, unstable APIs, not for production
- **Beta**: Feature complete, APIs stabilizing, limited production use
- **RC**: Release candidate, production ready pending final testing
- **Stable**: Production ready, semantic versioning guaranteed

## Version Format

`v<major>.<minor>.<patch>[-<prerelease>]`

Examples:
- `v0.1.0-alpha.1`
- `v0.1.0-beta.2`
- `v1.0.0-rc.1`
- `v1.0.0`
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

## [v0.1.0-beta.0] - 2026-08-05

First beta: feature-complete CLI/menu surface with per-user RBAC via
impersonation. Promoted from alpha per maintainer decision — the mutating
actions are now RBAC-enforced end to end.

### Fixed
- **Rich tables**: KeyValue field names now render as real bold. The
  `**marker**` was double-escaped by the table cell escaping pass
  (`**Kind**` → `\*\*Kind\*\*`), so it rendered as literal asterisks.
  Bold markers are now preserved through escaping; cells are escaped
  exactly once.
- **Log truncation kept the wrong end**: when a log body exceeded the
  single-message pane limit, the pane truncated from the *head*, silently
  discarding the newest lines the user asked for. Log panes now truncate
  from the *tail* (old output cut, newest lines kept).
- **Log tail requests were overridden**: `/logs`, `/logs -f` and the menu
  log buttons hardcoded a 100/200-line cap via `FormatPodLogs`, so a
  `--tail 500` request was silently cut to 100. The formatter now only
  caps when no `--tail` is given (the API server limits the fetch when
  one is).

### Security Notes
- Permissions are **not hardcoded** in the bot. Each Telegram user maps to a
  k8s identity (user + groups); the bot acts as that identity and Kubernetes
  RBAC enforces the role. To grant/revoke access, change the ClusterRole /
  RoleBinding — no code change or redeploy required.

## [v0.1.0-alpha.2] - 2026-08-05

### Fixed
- **Security**: All bot actions (scale, delete, restart, drain, cordon,
  logs, describe, list) now run through the per-user **impersonated** k8s
  client instead of the bot's base client. Previously the menu-driven
  mutations used the base client (which holds the chart's broad ClusterRole),
  so a read-only mapped user could scale/delete regardless of their RBAC
  role. Now the API server's RBAC decides allow/deny based on the identity
  each Telegram user is mapped to.
- **RBAC**: The chart's `readonly-user` ClusterRole is now bound to the
  `viewers` **group** (the identity read-only users are impersonated as).
  Role changes take effect without touching the bot.
- **Docker build**: The multi-stage Dockerfile hardcoded `GOARCH=amd64`, so
  the `linux/arm64` image shipped an amd64 binary. It now uses the `TARGETARCH`
  build arg so each platform image contains the correct binary, and QEMU is
  registered in CI so arm64 build stages can run on the amd64 runner.
- **Audit logging**: Every user action (delete, restart, scale, exec) logs
  `telegram_user_id` plus resource details, and impersonated-client selection
  is logged at Info level, so logs show *who is doing what*.

### Security Notes
- Permissions are **not hardcoded** in the bot. Each Telegram user maps to a
  k8s identity (user + groups); the bot acts as that identity and Kubernetes
  RBAC enforces the role. To grant/revoke access, change the ClusterRole /
  RoleBinding — no code change or redeploy required.

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
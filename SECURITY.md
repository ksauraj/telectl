# Security Policy

## Supported Versions

We release security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| main    | :white_check_mark: |
| v0.1.x  | :white_check_mark: |
| < v0.1  | :x:                |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

### Private Disclosure

**Do not open a public issue** for security vulnerabilities. Instead:

1. **Email**: security@ksauraj.com
2. **GitHub Security Advisory**: Use the "Report a vulnerability" tab in the Security section of this repository

### What to Include

Please provide as much information as possible:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Affected versions
- Suggested fix (if any)

### Response Timeline

| Phase | Timeline |
|-------|----------|
| Acknowledgment | Within 48 hours |
| Initial Assessment | Within 5 business days |
| Fix Development | Within 30 days (varies by severity) |
| Coordinated Disclosure | After fix is released |

### Severity Classification

We use CVSS 3.1 for severity classification:

- **Critical** (9.0-10.0): Immediate patch release
- **High** (7.0-8.9): Patch within 7 days
- **Medium** (4.0-6.9): Patch within 30 days
- **Low** (0.1-3.9): Patch in next scheduled release

## Security Features

### Default Secure Defaults

- **Non-root container**: Runs as UID 1000
- **Read-only root filesystem**: `readOnlyRootFilesystem: true`
- **Dropped capabilities**: `ALL` capabilities dropped
- **No privilege escalation**: `allowPrivilegeEscalation: false`
- **RBAC**: Cluster-scoped or namespace-scoped with least privilege

### Authentication & Authorization

- **Telegram user allowlist**: Only configured user IDs can access
- **Per-user rate limiting**: Configurable requests/minute
- **Kubernetes impersonation**: Optional per-user K8s RBAC
- **Dry-run mode**: Log mutations without executing

### Secrets Management

- Bot token stored in Kubernetes Secret
- Helm secret has `helm.sh/resource-policy: keep`
- No secrets in container image or config maps
- Environment variables for sensitive config

## Supply Chain Security

### Dependencies

- `go mod verify` in CI
- Go module checksums verified
- Dependencies updated regularly via Dependabot
- Minimal dependency footprint

### Build Process

- Reproducible builds with `CGO_ENABLED=0`
- Multi-stage Docker builds
- SBOM generation (planned)
- SLSA compliance (planned)

### Distribution

- GitHub Releases with checksums (SHA256)
- Docker images on GHCR with signed tags (planned)
- Homebrew formula (planned)

## Threat Model

### In Scope

- Bot token compromise
- Kubernetes RBAC escalation
- Callback data injection
- Rate limiting bypass
- Log injection

### Out of Scope

- Telegram platform vulnerabilities
- Kubernetes cluster compromise (outside bot RBAC)
- Network-level attacks (assume trusted network)
- Physical access to infrastructure

## Security Checklist for Contributors

- [ ] No secrets in code or config
- [ ] Input validation on all user data
- [ ] Proper error handling (no stack traces to users)
- [ ] Rate limiting on all public endpoints
- [ ] Secure defaults in Helm chart
- [ ] Tests for security-relevant code

## Contact

Security team: security@ksauraj.com

For non-security issues, please use the standard issue tracker.
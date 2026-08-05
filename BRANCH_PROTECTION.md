# Branch Protection Rules

This document describes the branch protection configuration for the telectl repository.

## Protected Branches

### `main` Branch

The `main` branch is protected with the following rules:

| Rule | Setting |
|------|---------|
| Require pull request reviews before merging | ✅ Required |
| Required approving reviews | 1 |
| Dismiss stale PR approvals when new commits are pushed | ✅ Enabled |
| Require review from code owners | ✅ Enabled |
| Require status checks to pass before merging | ✅ Required |
| Require branches to be up to date before merging | ✅ Required |
| Required status checks | `Go Mod Tidy`, `Test`, `Lint` |
| Require conversation resolution before merging | ✅ Enabled |
| Require signed commits | ❌ Not required |
| Require linear history | ✅ Enabled |
| Include administrators | ✅ Enabled |
| Restrict who can push to matching branches | ✅ Enabled |
| Allow force pushes | ❌ Disabled |
| Allow deletions | ❌ Disabled |

### `feat/*` Branches

Feature branches are not protected but follow conventions:
- Named: `feat/short-description`
- Must pass CI before merging to main
- Deleted after merge

## Required Status Checks

The following GitHub Actions jobs must pass:

1. **Go Mod Tidy** - Verifies `go.mod` and `go.sum` are tidy
2. **Test** - Runs all tests with race detector
3. **Lint** - Runs golangci-lint (v2.1.6)

## Code Owners

Defined in [CODEOWNERS](CODEOWNERS):

```
# Global owners
* @ksauraj

# Helm chart
charts/ @ksauraj

# CI/CD
.github/ @ksauraj

# Documentation
docs/ @ksauraj
```

## Merge Requirements

### Standard Merge

1. Create PR from feature branch to `main`
2. All CI checks pass
3. At least 1 approval from code owner
4. All conversations resolved
5. Branch up to date with `main`
6. Squash and merge (recommended) or regular merge

### Hotfix Process

For urgent production fixes:

1. Create branch from `main`: `hotfix/description`
2. Make minimal fix
3. Fast-track review (1 approval still required)
4. Merge to `main`
5. Tag release immediately

## Branch Naming Conventions

| Prefix | Purpose | Protection |
|--------|---------|------------|
| `feat/` | New features | CI only |
| `fix/` | Bug fixes | CI only |
| `docs/` | Documentation | CI only |
| `refactor/` | Code refactoring | CI only |
| `perf/` | Performance | CI only |
| `test/` | Test improvements | CI only |
| `chore/` | Maintenance | CI only |
| `ci/` | CI/CD changes | CI only |
| `hotfix/` | Urgent fixes | Fast-track |
| `release/` | Release prep | CI only |

## Enforcement

Branch protection is enforced via GitHub Settings → Branches → Branch protection rules.

To view current rules:
```
Settings → Branches → Branch protection rules → main
```

## Automation

### Auto-merge

Not enabled. All merges require manual approval.

### Dependabot

Dependabot PRs for dependency updates:
- Labeled: `dependencies`, `gomod`/`github-actions`/`docker`
- Auto-merge: Disabled (requires manual review)
- Auto-assign: @ksauraj

### Stale Branches

- Auto-delete merged branches: ✅ Enabled
- Stale PRs (30 days): Auto-close with comment

## Emergency Override

In case of emergency (e.g., critical production issue):
1. Maintainer can bypass protection via GitHub UI
2. Must document reason in PR
3. Follow-up PR to add tests/fixes within 48 hours

## Compliance

These rules ensure:
- Code quality through reviews
- Test coverage maintained
- No direct pushes to main
- Linear history for bisecting
- Audit trail for all changes
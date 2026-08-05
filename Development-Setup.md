---
title: Development Setup
nav_order: 19
---

# Development Setup

Everything you need to build, run, and debug telectl locally.

## Prerequisites

- **Go 1.23+** (see `go.mod`)
- **Docker** (for the container build and kind)
- **kind** (local Kubernetes cluster for integration testing)
- **golangci-lint** (for `make lint`)
- **staticcheck** (optional; for `make staticcheck`)
- **goimports** (optional; `make fmt` skips it gracefully if absent)

## Clone & build

```bash
git clone https://github.com/ksauraj/telectl.git
cd telectl
make build          # builds ./telectl into the repo root
```

The Makefile's `build` target compiles `cmd/telectl` with version/commit/date
baked in via `-ldflags`.

## Install dev tools

```bash
make dev-setup      # installs golangci-lint, staticcheck, goimports
```

(If `dev-setup` isn't defined, install golangci-lint manually:
`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`.)

## Run the bot locally

```bash
make run-dev
# or, with a config file:
./telectl --config /path/to/config.yaml --log-level debug
```

The bot connects using your local kubeconfig by default. Start it with
`--log-level debug` to see which config file it loads and how it connects.

Useful subcommands for a quick check without a bot:

```bash
./telectl version        # version, commit, build date
./telectl config         # effective config (secrets redacted)
./telectl contexts       # list kubeconfig contexts
```

## Local test cluster (kind)

```bash
kind create cluster --name telectl-dev
kubectl cluster-info
```

For testing against a realistic setup:

1. Create a namespace + a sample workload:
   ```bash
   kubectl create ns production
   kubectl create deployment frontend -n production --image=nginx --replicas=3
   ```
2. Set up the RBAC identities + impersonator role (see the
   [manual test runbook](https://github.com/ksauraj/telectl-manual-test) or
   [Kubernetes RBAC](Kubernetes-RBAC)).
3. Run the bot locally against the kind cluster and point
   `kubernetes.kubeconfig_path` at the kind kubeconfig.

## Repository layout

```
cmd/telectl/          # CLI: root command, version/config/contexts subcommands
internal/bot/         # bot core: dispatch, menus, panes, per-user clients
internal/handlers/    # command handlers (get, logs, exec, top, …)
internal/menus/       # menu builder, callback-data encode/decode
internal/k8s/         # client-go wrapper: contexts, impersonation, verbs
internal/config/      # config schema + viper loading
internal/tg/          # Telegram API + rich text (markdown) rendering
internal/types/       # shared types: resource map, sessions, state
internal/utils/       # formatters etc.
charts/telectl/       # Helm chart
docs/                 # documentation (this wiki's source)
```

## Working on menus or rich text

The menu/rich-text code has dedicated tests
(`internal/tg/richdoc_test.go`, `internal/menus/`). Run them focused:

```bash
go test ./internal/tg/ ./internal/menus/ -v
```

## Building the Docker image

```bash
make docker-build     # ghcr.io/ksauraj/telectl:<version> + :latest
```

Multi-arch is handled in CI via buildx + QEMU; the Dockerfile compiles per
`TARGETARCH` so each platform ships its own binary.
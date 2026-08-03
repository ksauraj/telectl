# telectl

A Telegram bot for Kubernetes cluster management. Talks to the API server
directly through client-go — no kubectl subprocess, no shell escaping, no
binary to ship in the container image.

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![CI](https://github.com/ksauraj/telectl/actions/workflows/ci.yml/badge.svg)](https://github.com/ksauraj/telectl/actions)
[![Docker](https://img.shields.io/badge/Docker-ksauraj%2Ftelectl-2496ED?logo=docker)](https://hub.docker.com/r/ksauraj/telectl)

## What it does

**Browse resources** — tap through Pods, Deployments, Services, ReplicaSets,
Namespaces, Nodes, ConfigMaps, Secrets, PVCs, PVs, Ingresses with pagination.
Each resource opens a detail pane with context-aware action buttons.

**Per-resource actions** — Describe, Labels, Selector, Endpoints, Rollout
history, Pods (for workloads), Logs, Exec, Scale, Cordon/Uncordon, Drain,
Restart, Delete. Every button is wired; none silently does nothing.

**Logs** — view, follow (`-f`), previous container (`-p`), tail, since.
Container selection from a button keyboard.

**Exec** — interactive shell or single command. The container flag (`-c`) is
parsed correctly: `/exec pod sh -c "echo hi"` runs `sh -c "echo hi"` inside
the pod, not `sh` with the container set to `echo hi`.

**Context management** — list contexts, switch in-session (does not rewrite
`~/.kube/config`), show current config.

**Monitoring** — `/top pods|nodes` (requires metrics-server), `/events`,
`/watch`.

**Inline queries** — type `@yourbot pods -n kube-system` in any chat.

**Security** — user allowlist, per-user rate limiting, dry-run mode.

## Quick start

### Prerequisites

- Go 1.23+
- A Telegram bot token from [@BotFather](https://t.me/BotFather)
- A kubeconfig with access to your cluster

### Install from source

```bash
git clone https://github.com/ksauraj/telectl
cd telectl
make build
```

### Install with Go

```bash
go install github.com/ksauraj/telectl/cmd/telectl@latest
```

### Docker

```bash
docker pull ksauraj/telectl:latest
```

## Configuration

### Minimal config file

```yaml
# ~/.config/telectl/telectl.yaml
telegram:
  bot_token: "YOUR_BOT_TOKEN_FROM_BOTFATHER"
  allowed_user_ids: [123456789]   # your Telegram user ID; omit to allow everyone
```

Find your Telegram user ID by messaging [@userinfobot](https://t.me/userinfobot).

Config file search order: `--config` flag → `$HOME/.config/telectl/` →
`$HOME/.config/` → `/etc/telectl/`. The file must be named `telectl.yaml`
(or `.yml`, `.json`, `.toml`). telectl refuses to start if it accidentally
picks up a kubeconfig or the compiled binary.

### Environment variables

All settings can be overridden with environment variables:

```bash
export TELEGRAM_BOT_TOKEN="your-token"
export KUBECONFIG=/path/to/kubeconfig
export ALLOWED_USER_IDS="123456789,987654321"
```

### Running

```bash
# With a config file
./telectl --config ~/.config/telectl/telectl.yaml

# With environment variables only
TELEGRAM_BOT_TOKEN=xxx KUBECONFIG=~/.kube/config ./telectl

# Docker
docker run --rm -it \
  -v ~/.kube:/home/telectl/.kube:ro \
  -v ~/.config/telectl/telectl.yaml:/app/config.yaml:ro \
  -e TELEGRAM_BOT_TOKEN=xxx \
  ksauraj/telectl:latest
```

## Usage

### Menu navigation

Send `/start` to open the main menu. Everything is reachable by tapping
buttons — no command syntax required. See
[docs/MENU_GUIDE.md](docs/MENU_GUIDE.md) for the full walkthrough and
[docs/CONFIGURATION.md](docs/CONFIGURATION.md) for the complete configuration reference.

### Typed commands

```
/get pods [-n namespace] [-o wide|json|yaml] [-l selector] [-A]
/get deployments -n production -o wide

/describe pods my-pod [-n namespace]

/logs my-pod [-c container] [-f] [-p] [--tail N] [--since 5m]

/exec my-pod [-c container] [-- command args...]
/exec my-pod -- sh -c "cat /etc/hosts"

/contexts
/use-context production-cluster
/config

/top pods|nodes
/events [-n namespace]

/restart deployment my-app [-n namespace]
/scale deployment my-app 5 [-n namespace]
```

### Inline queries

```
@yourbot pods
@yourbot pods -n kube-system
@yourbot deployments nginx -n production
@yourbot nodes
@yourbot services -l app=nginx
```

Supported aliases: `po`/`pod`, `deploy`/`deployment`, `svc`/`service`,
`rs`/`replicaset`, `ns`/`namespace`, `no`/`node`, `cm`/`configmap`,
`pvc`, `pv`, `ing`/`ingress`, `ev`/`event`, and their plurals.

## Configuration reference

| Key | Env var | Default | Description |
|-----|---------|---------|-------------|
| `telegram.bot_token` | `TELEGRAM_BOT_TOKEN` | — | **Required.** Bot token from @BotFather |
| `telegram.allowed_user_ids` | `ALLOWED_USER_IDS` | `[]` | Comma-separated Telegram user IDs. Empty = allow everyone (not recommended in production) |
| `telegram.admin_user_ids` | `ADMIN_USER_IDS` | `[]` | Admin user IDs for privileged operations |
| `kubernetes.kubeconfig_path` | `KUBECONFIG` | `~/.kube/config` | Path to kubeconfig |
| `kubernetes.context` | — | current context | Override the active context at startup |
| `kubernetes.default_namespace` | — | `default` | Namespace used when `-n` is not given |
| `kubernetes.dry_run` | — | `false` | Log what would happen without making API calls |
| `logging.level` | — | `info` | `debug`, `info`, `warn`, `error` |
| `bot.rate_limit` | — | `30` | Max requests per user per minute |
| `bot.enable_menu_button` | — | `true` | Show the persistent menu button in the chat header |
| `bot.enable_reply_keyboard` | — | `true` | Show the persistent reply keyboard |
| `bot.menu_page_size` | — | `10` | Resources shown per page in list views |
| `bot.max_message_length` | — | `4096` | Telegram's hard limit; long output is split |

## Security

**Always set `allowed_user_ids` in production.** Without it, anyone who
discovers your bot token can run kubectl-equivalent commands against your
cluster.

The bot uses the kubeconfig's credentials directly, so its RBAC permissions
are exactly those of the configured user or service account. Scope them to
what the bot actually needs.

`dry_run: true` logs every mutating operation without executing it — useful
for auditing what the bot would do before granting it write access.

Context switching (`/use-context`) rebuilds the in-process API clients but
does not write to `~/.kube/config`, so it affects only this bot session and
does not change what `kubectl` sees on the same machine.

## Architecture

```
Telegram user
     │
     ▼
  tg.RealBot          ← thin wrapper around go-telegram/bot
     │
     ▼
  bot.Bot             ← update routing, session management, rate limiting
  ├── handlers/       ← typed command implementations (/get, /logs, /exec, …)
  ├── bot/detail.go   ← per-resource detail pane and its 25 action verbs
  └── menus/          ← keyboard builders and callback data protocol
     │
     ▼
  k8s.Client          ← client-go wrapper; never shells out to kubectl
  ├── client.go       ← list/get/watch, exec, port-forward, restart
  ├── nodeops.go      ← cordon, uncordon, drain (eviction subresource)
  └── workloadops.go  ← selector resolution, rollout history, scale, endpoints
     │
     ▼
  Kubernetes API server
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full breakdown,
including the testing approach and how to extend each layer.

### Key design decisions

**No kubectl subprocess.** Every operation goes through client-go. This means
no PATH dependency, no shell injection surface, and correct context handling
for concurrent users.

**Callback data is a protocol.** The `menu:<type>:<field>:…` format is
declared as a field table in `menus/menus.go`. The dispatcher reads from the
same table. A test walks every button every keyboard can render and asserts
each verb has a handler — the invariant whose violation caused the original
sixteen dead buttons.

**Context switching is session-scoped.** Switching context rebuilds the API
clients in memory but does not rewrite `~/.kube/config`. Two operators on the
same machine cannot clobber each other's active context.

**kubeconfig saves go through clientcmd.WriteToFile.** The internal
`clientcmdapi.Config` type marshals to a format kubectl cannot read. Using the
versioned writer prevents kubeconfig corruption on context switch.

## Development

```bash
# Build
make build

# Test (with race detector)
make test

# Lint
make lint

# Format + vet + lint + test
make check

# Cross-compile for all platforms
make build-all

# Development setup (installs golangci-lint, goimports, staticcheck)
make dev-setup
```

### Running locally

```bash
make run-dev   # requires TELEGRAM_BOT_TOKEN and KUBECONFIG in env
```

### Project layout

```
cmd/telectl/
├── main.go              # entry point: signal handling, logger init
└── cmd/root.go          # cobra CLI: flags, config loading, bot.New

internal/
├── bot/
│   ├── bot.go           # update routing, command dispatch, session management
│   ├── detail.go        # per-resource detail pane (29 action verbs)
│   └── *_test.go        # bot-level integration tests with fake Telegram
├── config/
│   └── config.go        # Viper config, env binding, startup validation
├── handlers/
│   ├── base.go          # shared flag parser (parseFlags, valueFlag)
│   ├── get.go           # /get
│   ├── describe.go      # /describe
│   ├── logs.go          # /logs (parseLogFlags)
│   ├── exec.go          # /exec (parseExecArgs)
│   ├── context.go       # /contexts, /use-context, /config, /portforward
│   ├── monitoring.go    # /top, /events, /watch
│   ├── operations.go    # /restart, /scale
│   └── inline_query.go  # inline query handler
├── k8s/
│   ├── client.go        # list/get/watch, exec, port-forward, restart
│   ├── nodeops.go       # cordon, uncordon, drain
│   └── workloadops.go   # selector, rollout history, scale, endpoints
├── menus/
│   ├── menus.go         # keyboard builders, ParseCallbackData field table
│   └── tokens.go        # token store (64-byte callback_data limit)
├── tg/
│   ├── bot.go           # RealBot: SendText, SendRich, EditRich, …
│   └── types.go         # Message, InlineKeyboard, RichDoc, …
├── types/
│   └── types.go         # ResourceMap (alias → GVR), UserSession, RateLimiter
└── utils/formatters/
    ├── formatters.go    # table rendering, status emoji, TruncateString
    ├── rich.go          # RichDoc builder (headings, tables, code, details)
    └── richdetail.go    # per-resource rich renderers (drain result, rollout, …)

pkg/kubeconfig/
├── kubeconfig.go        # parse, SwitchContext, Save (via clientcmd.WriteToFile)
└── save_test.go         # round-trip and corruption-prevention tests

.github/workflows/ci.yml # tidy → test → lint → build → release
Dockerfile               # multi-stage, alpine final image
Makefile
.golangci.yml
config.yaml.example
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/go-telegram/bot` | Telegram Bot API |
| `github.com/spf13/cobra` | CLI |
| `github.com/spf13/viper` | Configuration |
| `go.uber.org/zap` | Structured logging |
| `k8s.io/client-go` | Kubernetes API client |
| `k8s.io/api`, `k8s.io/apimachinery` | Kubernetes types |

## Contributing

```bash
git clone https://github.com/ksauraj/telectl
cd telectl
make dev-setup
# make your changes
make check          # must pass before opening a PR
```

## License

Apache License 2.0 — see [LICENSE](LICENSE).

## Author

Sauraj Kumar Singh ([@ksauraj](https://github.com/ksauraj))

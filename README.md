# k8s-telegram-bot

A production-ready Telegram bot for Kubernetes cluster management. Built with Go for performance and reliability.

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Build Status](https://github.com/ksauraj/k8s-telegram-bot/workflows/CI/badge.svg)](https://github.com/ksauraj/k8s-telegram-bot/actions)
[![Docker](https://img.shields.io/badge/Docker-ksauraj/k8s-telegram-bot-2496ED?logo=docker)](https://hub.docker.com/r/ksauraj/k8s-telegram-bot)

## Features

### Interactive Menu System
- **Bot Menu Button**: Persistent menu in chat header with quick access to all features
- **Reply Keyboard**: Always-visible bottom bar with main actions (Resources, Logs, Exec, Contexts, Monitor, Operations, Settings)
- **Inline Navigation**: Kubernetes resource browsing with pagination, similar to k9s
- **Action Keyboards**: Context-aware buttons per resource (Logs, Exec, Describe, Scale, Restart, Delete)
- **Namespace Selector**: Visual namespace switching with current namespace indicator

### Resource Management
- List resources: Pods, Deployments, Services, ReplicaSets, Namespaces, Nodes, ConfigMaps, Secrets, PVCs, PVs, Ingresses
- Describe resources: Detailed information with events
- Multiple output formats: Table, Wide, JSON, YAML, Name-only
- Label selectors and field selectors: Filter resources precisely
- All namespaces support: -A flag for cluster-wide queries

### Logs and Debugging
- View logs: /logs <pod> [-c container] [--tail N] [--since TIME]
- Follow logs: Real-time log streaming with -f flag
- Previous container logs: Access logs from crashed containers with -p
- Timestamps: Optional timestamp display

### Interactive Execution
- Exec into containers: /exec <pod> [-c container] [command]
- Interactive shell: Start persistent shell sessions
- Command history: Session-based command tracking

### Port Forwarding
- Local port forwarding: Access services locally
- Multiple ports: Forward multiple ports simultaneously

### Context Management
- List contexts: View all kubeconfig contexts with details
- Switch contexts: /use-context <name> with inline keyboard
- Current context indicator: Visual feedback in context list

### Monitoring
- Resource usage: /top pods|nodes (requires metrics-server)
- Events: Cluster events with filtering
- Watch mode: Real-time resource change notifications

### Operations
- Restart deployments: Rolling restart via annotation update
- Scale deployments: /scale deployment <name> <replicas>

### Security and Access Control
- User allowlist: Restrict bot access to specific users
- Admin commands: Separate admin user list for sensitive operations
- Rate limiting: Per-user request throttling
- Dry-run mode: Test commands without making changes

## Quick Start

### Prerequisites
- Go 1.23+
- Telegram Bot Token (from @BotFather)
- Kubernetes cluster access (kubeconfig)

### Installation

#### From Source
```bash
git clone https://github.com/ksauraj/k8s-telegram-bot
cd k8s-telegram-bot
make build
```

#### Using Docker
```bash
docker pull ksauraj/k8s-telegram-bot:latest
```

#### Using Go Install
```bash
go install github.com/ksauraj/k8s-telegram-bot/cmd/k8sbot@latest
```

### Configuration

1. Copy the example config:
```bash
cp config.yaml.example ~/.config/k8s-telegram-bot.yaml
```

2. Edit the config with your bot token:
```yaml
telegram:
  bot_token: "YOUR_BOT_TOKEN_FROM_BOTFATHER"
  allowed_user_ids: [123456789]  # Optional: your Telegram user ID
```

3. Or use environment variables:
```bash
export TELEGRAM_BOT_TOKEN="your-token"
export KUBECONFIG=/path/to/kubeconfig
export ALLOWED_USER_IDS="123456789,987654321"
```

### Running

```bash
# Direct binary
./k8s-telegram-bot --config ~/.config/k8s-telegram-bot.yaml

# With Docker
docker run --rm -it \
  -v ~/.kube:/home/botuser/.kube:ro \
  -v ~/.config/k8s-telegram-bot.yaml:/app/config.yaml:ro \
  ksauraj/k8s-telegram-bot:latest
```

## Usage Examples

### Interactive Menu System

**Start the bot and use the menu:**
```
/start
```

This shows the main menu with reply keyboard. Tap buttons to navigate:
- Resources: Browse all resource types with pagination
- Logs: View pod logs with follow/previous options
- Exec: Execute commands in containers
- Port Forward: Forward local ports to pods
- Contexts: Switch kubeconfig contexts
- Monitor: Top pods/nodes, events, watch
- Operations: Restart, scale deployments
- Settings: Namespace, context, theme settings

**Bot Menu Button** (in chat header): Tap for quick command access.

### Inline Queries
Use the bot from any chat by typing:
```
@yourbot pods
@yourbot pods -n kube-system
@yourbot deployments nginx
@yourbot services -l app=nginx
@yourbot nodes
```

### Basic Resource Queries
```bash
# List all pods in default namespace
/get pods

# List pods in specific namespace
/get pods -n kube-system

# Get wide output with node/IP info
/get pods -o wide

# Filter by labels
/get pods -l app=nginx

# Get all resources across namespaces
/get pods -A

# JSON output for scripting
/get deployment my-app -o json
```

### Logs
```bash
# View recent logs
/logs my-pod

# Follow logs in real-time
/logs my-pod -f

# Specific container
/logs my-pod -c nginx

# Last 100 lines
/logs my-pod --tail 100

# Logs from last hour
/logs my-pod --since 1h

# Previous container (crashed)
/logs my-pod -p
```

### Exec
```bash
# Interactive shell
/exec my-pod

# Specific container
/exec my-pod -c nginx

# Run single command
/exec my-pod -- ls -la /var/log

# With namespace
/exec my-pod -n production -- cat /etc/nginx/nginx.conf
```

### Context Management
```bash
# List all contexts
/contexts

# Switch context (with inline keyboard)
/use-context production-cluster

# Show current config
/config
```

### Monitoring
```bash
# Pod resource usage
/top pods

# Node resource usage
/top nodes

# Sort by memory
/top pods --sort memory

# Recent events
/events

# Events in specific namespace
/events -n production
```

### Operations
```bash
# Restart deployment
/restart deployment my-app

# Scale deployment
/scale deployment my-app 5
```

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   Telegram      │────▶│  k8s-telegram-bot │────▶│  Kubernetes API │
│   User          │     │  (Go)            │     │  Server         │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                              │
                              ▼
                        ┌──────────────────┐
                        │  Kubeconfig      │
                        │  Parser          │
                        └──────────────────┘
```

### Components
- **bot/**: Telegram bot core, update handling, session management
- **handlers/**: Command implementations
- **k8s/**: Kubernetes client wrapper with typed operations
- **pkg/kubeconfig/**: Kubeconfig parsing and manipulation
- **internal/config/**: Configuration management (Viper)
- **internal/utils/**: Formatters, helpers
- **internal/menus/**: Menu and keyboard builder

## Development

### Setup
```bash
make dev-setup
```

### Run Tests
```bash
make test
```

### Lint and Format
```bash
make check
```

### Build for All Platforms
```bash
make build-all
```

## Configuration Reference

| Setting | Env Var | Default | Description |
|---------|---------|---------|-------------|
| telegram.bot_token | TELEGRAM_BOT_TOKEN | - | Required: Bot token |
| telegram.allowed_user_ids | ALLOWED_USER_IDS | [] | Allowed user IDs |
| telegram.admin_user_ids | ADMIN_USER_IDS | [] | Admin user IDs |
| kubernetes.kubeconfig_path | KUBECONFIG | ~/.kube/config | Kubeconfig path |
| kubernetes.default_namespace | - | default | Default namespace |
| kubernetes.dry_run | - | false | Dry-run mode |
| logging.level | - | info | Log level |
| bot.rate_limit | - | 30 | Requests/minute |
| bot.enable_menu_button | - | true | Enable bot menu button |
| bot.enable_reply_keyboard | - | true | Enable reply keyboard |
| bot.menu_page_size | - | 10 | Items per page in lists |

## Security Considerations

1. **Bot Token**: Keep your bot token secret. Use environment variables.
2. **User Allowlist**: Always configure allowed_user_ids in production.
3. **RBAC**: The bot uses the kubeconfig's credentials. Ensure the kubeconfig has appropriate RBAC permissions.
4. **Network**: Run the bot in a secure network. Consider using a VPN or private network.
5. **Dry-run**: Use dry_run: true for testing in production clusters.

## Project Structure

```
/home/ubuntu/.hermes/workspace/k8s-telegram-bot/
├── cmd/k8sbot/
│   ├── main.go              # Entry point
│   └── cmd/root.go          # Cobra CLI commands
├── internal/
│   ├── bot/
│   │   └── bot.go           # Core bot with menu integration
│   ├── config/
│   │   └── config.go        # Viper config with menu settings
│   ├── handlers/
│   │   ├── base.go          # Shared handler utilities
│   │   ├── start.go         # /start handler
│   │   ├── help.go          # /help handler
│   │   ├── version.go       # /version handler
│   │   ├── get.go           # /get handler (all resources)
│   │   ├── describe.go      # /describe handler
│   │   ├── logs.go          # /logs handler
│   │   ├── exec.go          # /exec handler
│   │   ├── context.go       # /contexts, /use-context, /config, /portforward
│   │   ├── monitoring.go    # /top, /events, /watch
│   │   ├── operations.go    # /restart, /scale
│   │   ├── menu_handlers.go # /resources, /monitor, /operations, /settings
│   │   ├── inline_query.go  # Inline query handler
│   │   └── handlers.go      # Package marker
│   ├── k8s/
│   │   └── client.go        # Kubernetes client wrapper
│   ├── menus/
│   │   ├── menus.go         # Menu/keyboard builder
│   │   └── menus_test.go    # Unit tests for callback parsing
│   └── utils/
│       ├── formatters.go    # Output formatters
│       └── formatters_test.go # Formatter tests
├── pkg/kubeconfig/
│   ├── kubeconfig.go        # Kubeconfig parser
│   └── kubeconfig_test.go   # Parser tests
├── .github/workflows/ci.yml # CI/CD pipeline
├── Dockerfile               # Multi-stage build
├── Makefile                 # Build, test, lint, release targets
├── config.yaml.example      # Example configuration
├── .env.example             # Environment variables template
├── .golangci.yml            # Linter configuration
├── .gitignore               # Git ignore rules
├── LICENSE                  # Apache 2.0 license
├── README.md                # This file
└── scripts/
    ├── setup.sh             # Development setup
    └── release.sh           # Release automation
```

## Dependencies

### Core
- github.com/go-telegram-bot-api/telegram-bot-api/v5 - Telegram Bot API wrapper
- github.com/spf13/cobra - CLI framework
- github.com/spf13/viper - Configuration management
- go.uber.org/zap - Structured logging

### Kubernetes
- k8s.io/client-go - Kubernetes Go client
- k8s.io/apimachinery - Kubernetes API machinery
- k8s.io/api - Kubernetes API types
- sigs.k8s.io/yaml - YAML utilities
- gopkg.in/yaml.v3 - YAML parsing

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run `make check` to ensure code quality
5. Submit a pull request

## License

Apache License 2.0 - See LICENSE for details.

## Author

Sauraj Kumar Singh (@ksauraj)
- DevOps Engineer
- Kubernetes Enthusiast

## Acknowledgments

- client-go - Kubernetes Go client
- telegram-bot-api - Telegram Bot API wrapper
- Cobra - CLI framework
- Viper - Configuration management
- Zap - Structured logging
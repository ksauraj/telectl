# Installation Guide

## Prerequisites

- **Go 1.23+** (for building from source)
- **Telegram Bot Token** from [@BotFather](https://t.me/BotFather)
- **Kubernetes cluster** (v1.28+)
- **kubectl** configured with cluster access
- **Helm 3.12+** (for Kubernetes deployment)

## Installation Methods

### 1. Helm Chart (Recommended for Kubernetes)

```bash
# Add the Helm repository
helm repo add telectl https://ksauraj.github.io/telectl/
helm repo update

# Install with required values
helm install telectl telectl/telectl \
  --namespace telectl \
  --create-namespace \
  --set telegram.botToken="YOUR_BOT_TOKEN" \
  --set telegram.allowedUserIds="{123456789}"
```

### 2. Local Chart (From Source)

```bash
git clone https://github.com/ksauraj/telectl
cd telectl

helm install telectl ./charts/telectl \
  --namespace telectl \
  --create-namespace \
  --set telegram.botToken="YOUR_BOT_TOKEN" \
  --set telegram.allowedUserIds="{123456789}"
```

### 3. Docker

```bash
# Pull from GHCR
docker pull ghcr.io/ksauraj/telectl:latest

# Run with config
docker run --rm -it \
  -v ~/.kube:/home/telectl/.kube:ro \
  -v ~/.config/telectl/telectl.yaml:/app/config.yaml:ro \
  -e TELEGRAM_BOT_TOKEN="YOUR_BOT_TOKEN" \
  ghcr.io/ksauraj/telectl:latest
```

### 4. From Source (Go)

```bash
# Build
git clone https://github.com/ksauraj/telectl
cd telectl
make build

# Run
./telectl --config config.yaml
```

### 5. Go Install

```bash
go install github.com/ksauraj/telectl/cmd/telectl@latest
```

## Configuration

### Required Settings

| Setting | Description | Example |
|---------|-------------|---------|
| `telegram.bot_token` | Bot token from @BotFather | `123456789:ABCdef...` |
| `telegram.allowed_user_ids` | Your Telegram user ID | `[123456789]` |

### Getting Your Telegram User ID

1. Message [@userinfobot](https://t.me/userinfobot)
2. Copy the numeric ID it returns

### Config File Locations (in order of precedence)

1. `--config` flag
2. `$HOME/.config/telectl/telectl.yaml`
3. `$HOME/.config/telectl.yaml`
4. `/etc/telectl/telectl.yaml`

### Minimal Config Example

```yaml
# ~/.config/telectl/telectl.yaml
telegram:
  bot_token: "123456789:ABCdefGhIjKlMnOpQrStUvWxYz"
  allowed_user_ids: [123456789]
```

## Verification

### Check Deployment

```bash
# Check pod status
kubectl get pods -n telectl -l app.kubernetes.io/name=telectl

# Check logs
kubectl logs -n telectl -l app.kubernetes.io/name=telectl
```

### Test Bot

1. Open Telegram
2. Search for your bot username
3. Send `/start`
4. You should see the main menu

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Bot doesn't respond | Check logs: `kubectl logs -n telectl -l app.kubernetes.io/name=telectl` |
| "User not allowed" | Verify `allowed_user_ids` matches your Telegram ID |
| RBAC errors | Check `kubectl auth can-i get pods --as=system:serviceaccount:telectl:telectl` |
| Image pull errors | Verify GHCR image exists: `docker pull ghcr.io/ksauraj/telectl:latest` |

## Next Steps

- [Configuration Reference](Configuration-Reference) - Complete configuration options
- [Helm Chart Guide](Helm-Chart-Guide) - Advanced Helm deployment
- [Security](Security) - Production security hardening
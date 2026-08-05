# Docker Deployment

Run telectl as a container you manage yourself (not via the Helm chart).
Multi-arch images (`linux/amd64`, `linux/arm64`) are published to GHCR.

## Pull the image

```bash
docker pull ghcr.io/ksauraj/telectl:v0.1.0-beta.0
```

Tags follow the release policy — use the latest
[release](https://github.com/ksauraj/telectl/releases) tag (e.g.
`v0.1.0-beta.0`). `latest` is not pinned; prefer an explicit immutable tag.

## Run it with a mounted config

```bash
# 1. Write your config
cat > config.yaml <<'EOF'
telegram:
  bot_token: "YOUR_BOT_TOKEN"
  allowed_user_ids: [YOUR_ADMIN_TELEGRAM_ID]
kubernetes:
  kubeconfig_path: "/app/kubeconfig"
EOF

# 2. Run, mounting config + kubeconfig
docker run -d --name telectl \
  -v "$PWD/config.yaml:/app/config.yaml:ro" \
  -v "$PWD/kubeconfig":/app/kubeconfig:ro \
  ghcr.io/ksauraj/telectl:v0.1.0-beta.0
```

- The image's `ENTRYPOINT` already passes `--config /app/config.yaml` and sets
  `TELECTL_CONFIG=/app/config.yaml`, so mounting your config there means it's
  picked up automatically.
- Mount your kubeconfig into the container and point
  `kubernetes.kubeconfig_path` at it.

## Run in-cluster (ServiceAccount instead of kubeconfig)

Inside a pod, telectl auto-detects the cluster and uses `rest.InClusterConfig()`
— you **do not mount a kubeconfig**. Mount **only** the config:

```bash
docker run -d --name telectl \
  -v "$PWD/config.yaml":/app/config.yaml:ro \
  ghcr.io/ksauraj/telectl:v0.1.0-beta.0
```

The image runs as non-root user `telectl` (uid 1000), so give it a
ServiceAccount token — this is exactly what the
[Helm chart](Helm-Chart-Guide) wires up for you.

## Build your own image

```bash
docker build -t telectl:local .
```

Requires Go 1.23+ (builder stage). The `ARG TARGETARCH` in the Dockerfile
means multi-platform builds (`docker buildx build --platform linux/amd64,linux/arm64 …`)
compile the correct binary per platform.

## Config / secrets hygiene

- Keep `config.yaml` out of the image; mount it read-only.
- The bot token is a **cluster credential** (it holds kubeconfig powers).
  Prefer a Secret + mounted file, or the Helm chart's Secret, over baking it
  into the image.
- See [Security](Security) and [Production Checklist](Production-Checklist).

## Comparison with Helm

For an in-cluster Deployment with RBAC + impersonation, the
[Helm chart](Helm-Chart-Guide) is the recommended, declarative path — it
creates the ServiceAccount, Secret, ConfigMap, and RBAC for you. `docker run`
gives you full manual control but leaves RBAC wiring to you. See
[Two Deployment Modes](Two-Deployment-Modes).
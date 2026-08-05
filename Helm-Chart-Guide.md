---
title: Helm Chart Guide
nav_order: 11
---

# Helm Chart Guide

Complete guide for deploying telectl with the official Helm chart.

## Chart Repository

```bash
helm repo add telectl https://ksauraj.github.io/telectl/
helm repo update
```

Or use the local chart from the repository:
```bash
git clone https://github.com/ksauraj/telectl
cd telectl
helm install telectl ./charts/telectl ...
```

## Quick Install

```bash
helm install telectl telectl/telectl \
  --namespace telectl \
  --create-namespace \
  --set telegram.botToken="YOUR_BOT_TOKEN" \
  --set telegram.allowedUserIds="{123456789}"
```

## Configuration Options

### Image

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Container image repository | `ghcr.io/ksauraj/telectl` |
| `image.tag` | Image tag (defaults to chart appVersion) | `""` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `imagePullSecrets` | Image pull secrets for private registries | `[]` |

### Service Account

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceAccount.create` | Create ServiceAccount | `true` |
| `serviceAccount.annotations` | ServiceAccount annotations | `{}` |
| `serviceAccount.name` | ServiceAccount name (empty = auto-generated) | `""` |

### RBAC

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rbac.create` | Create RBAC resources | `true` |
| `rbac.clusterScoped` | Cluster-wide (true) or namespace-scoped (false) | `true` |
| `rbac.allowedNamespaces` | Namespaces for namespace-scoped RBAC | `[]` |

**Cluster-scoped** (default): Creates `ClusterRole` + `ClusterRoleBinding` with access to all namespaces.

**Namespace-scoped**: Creates `Role` + `RoleBinding` limited to specific namespaces:
```yaml
rbac:
  clusterScoped: false
  allowedNamespaces: ["default", "production", "staging"]
```

### Telegram

| Parameter | Description | Default |
|-----------|-------------|---------|
| `telegram.botToken` | **Required.** Bot token from @BotFather | `""` |
| `telegram.allowedUserIds` | Comma-separated Telegram user IDs | `[]` |
| `telegram.adminUserIds` | Admin user IDs for privileged operations | `[]` |

### Kubernetes Client

| Parameter | Description | Default |
|-----------|-------------|---------|
| `kubernetes.kubeconfigPath` | Path to kubeconfig (empty = in-cluster) | `""` |
| `kubernetes.defaultNamespace` | Default namespace for commands | `default` |
| `kubernetes.context` | Override kubeconfig context | `""` |
| `kubernetes.timeout` | Request timeout in seconds | `30` |
| `kubernetes.dryRun` | Log mutations without executing | `false` |
| `kubernetes.clusterName` | Display name for cluster | `""` |
| `kubernetes.burst` | client-go burst limit | `10` |
| `kubernetes.qps` | client-go QPS limit | `5.0` |

### Impersonation (Per-User RBAC)

When enabled, the bot impersonates different K8s users based on Telegram user ID.

> **Design assumption (important):** the bot does **not** hardcode any
> permission logic. Each Telegram user is mapped to a Kubernetes identity
> (user + groups) in the config; the bot then acts *as that identity* for
> **every** action, and **Kubernetes RBAC decides** whether the request
> succeeds or is rejected with `Forbidden`. This means:
>
> - Granting/revoking a user's access = editing the ClusterRole / RoleBinding
>   that their impersonated identity is bound to. No code change, no redeploy.
> - Read-only users will get errors like `User "readonly-user" cannot patch
>   resource "deployments/scale"` when they try to scale or delete — that is
>   the API server enforcing RBAC, not a bot-level check.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `impersonation.enabled` | Enable impersonation feature | `false` |
| `impersonation.defaultUser` | Default user to impersonate | `""` |
| `impersonation.defaultGroups` | Default groups to impersonate | `[]` |
| `impersonation.userMapping` | Per-Telegram-user K8s user/group mapping | `{}` |

**Example:**
```yaml
impersonation:
  enabled: true
  defaultUser: "system:serviceaccount:default:readonly"
  defaultGroups: ["viewers"]
  userMapping:
    "123456789":    # Your admin Telegram ID
      user: "admin-user"
      groups: ["system:masters", "platform-team"]
    "987654321":    # Developer
      user: "dev-user"
      groups: ["developers"]
```

### Logging

| Parameter | Description | Default |
|-----------|-------------|---------|
| `logging.level` | Log level (debug/info/warn/error) | `info` |
| `logging.format` | Log format (json/console) | `json` |
| `logging.output` | Log output (stdout/stderr) | `stdout` |

### Bot

| Parameter | Description | Default |
|-----------|-------------|---------|
| `bot.maxMessageLength` | Max message length | `4096` |
| `bot.commandPrefix` | Command prefix | `/` |
| `bot.enableMarkdown` | Enable MarkdownV2 | `true` |
| `bot.rateLimit` | Max requests per user per minute | `30` |
| `bot.enableMenuButton` | Show persistent menu button | `true` |
| `bot.enableReplyKeyboard` | Show persistent reply keyboard | `true` |
| `bot.menuPageSize` | Resources per page | `10` |
| `bot.allowedCommands` | Allowed command names | See values.yaml |

### Resources

```yaml
resources:
  limits:
    cpu: 200m
    memory: 256Mi
  requests:
    cpu: 50m
    memory: 64Mi
```

### Security

```yaml
podSecurityContext:
  runAsNonRoot: true
  runAsUser: 1000
  runAsGroup: 1000
  fsGroup: 1000

securityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
  readOnlyRootFilesystem: true
```

## Complete Example: Production Values

```yaml
# values-prod.yaml
image:
  repository: ghcr.io/ksauraj/telectl
  tag: "v0.1.0"
  pullPolicy: IfNotPresent

serviceAccount:
  create: true
  annotations:
    # For AWS IRSA
    # eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/telectl

rbac:
  create: true
  clusterScoped: true

telegram:
  botToken: "123456789:ABCdefGhIjKlMnOpQrStUvWxYz"
  allowedUserIds: [123456789, 987654321]
  adminUserIds: [123456789]

kubernetes:
  defaultNamespace: "default"
  timeout: 30
  dryRun: false
  clusterName: "prod-cluster-1"
  burst: 20
  qps: 10.0

impersonation:
  enabled: true
  defaultUser: "system:serviceaccount:default:readonly"
  defaultGroups: ["viewers"]
  userMapping:
    "123456789":
      user: "admin-user"
      groups: ["system:masters", "platform-team"]
    "987654321":
      user: "dev-user"
      groups: ["developers"]

logging:
  level: "info"
  format: "json"
  output: "stdout"

bot:
  maxMessageLength: 4096
  commandPrefix: "/"
  enableMarkdown: true
  rateLimit: 30
  enableMenuButton: true
  enableReplyKeyboard: true
  menuPageSize: 10
  allowedCommands:
    - "start"
    - "help"
    - "about"
    - "version"
    - "get"
    - "describe"
    - "logs"
    - "exec"
    - "portforward"
    - "contexts"
    - "use-context"
    - "config"
    - "top"
    - "events"
    - "watch"
    - "restart"
    - "scale"
    - "resources"
    - "monitor"
    - "operations"
    - "settings"

resources:
  limits:
    cpu: 200m
    memory: 256Mi
  requests:
    cpu: 50m
    memory: 64Mi

# Optional: for private registry
# imagePullSecrets:
#   - name: ghcr-secret
```

Install with:
```bash
helm install telectl ./charts/telectl \
  --namespace telectl \
  --create-namespace \
  -f values-prod.yaml
```

## Verification

```bash
# Check pod
kubectl get pods -n telectl -l app.kubernetes.io/name=telectl

# Check logs
kubectl logs -n telectl -l app.kubernetes.io/name=telectl

# Verify RBAC
kubectl auth can-i get pods --as=system:serviceaccount:telectl:telectl

# Check config
kubectl get configmap -n telectl telectl-config -o yaml
```

## Upgrading

```bash
# Update to latest chart
helm repo update
helm upgrade telectl telectl/telectl -n telectl

# Or with local chart
helm upgrade telectl ./charts/telectl -n telectl -f values-prod.yaml
```

## Uninstalling

```bash
helm uninstall telectl -n telectl
# Note: Secrets with botToken are kept by default (helm.sh/resource-policy: keep)
# To fully remove: kubectl delete secret telectl-secrets -n telectl
```

## Chart Structure

```
charts/telectl/
├── Chart.yaml           # Chart metadata
├── values.yaml          # Default configuration values
├── templates/
│   ├── _helpers.tpl     # Template helpers
│   ├── deployment.yaml  # Deployment with config/secret volumes
│   ├── secret.yaml      # Bot token + user IDs (resource-policy: keep)
│   ├── configmap.yaml   # Generated telectl.yaml config
│   ├── serviceaccount.yaml
│   ├── role.yaml        # ClusterRole/Role + ClusterRoleBinding/RoleBinding
│   └── NOTES.txt        # Post-install instructions
└── README.md            # This guide
```

## Customization

### Private Registry

```yaml
image:
  repository: your-registry.com/telectl
  tag: "v0.1.0"
imagePullSecrets:
  - name: your-registry-secret
```

### Node Placement

```yaml
nodeSelector:
  kubernetes.io/arch: amd64
tolerations:
  - key: "dedicated"
    operator: "Equal"
    value: "monitoring"
    effect: "NoSchedule"
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
              - key: app.kubernetes.io/name
                operator: In
                values:
                  - telectl
          topologyKey: kubernetes.io/hostname
```

### Extra Environment Variables

```yaml
extraEnv:
  - name: CUSTOM_VAR
    value: "custom-value"
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| ImagePullBackOff | Check `image.repository`, `image.tag`, and `imagePullSecrets` |
| RBAC errors | Verify `rbac.clusterScoped` and `rbac.allowedNamespaces` |
| Bot not starting | Check logs: `kubectl logs -n telectl -l app.kubernetes.io/name=telectl` |
| Config not applied | Verify ConfigMap: `kubectl get configmap -n telectl telectl-config -o yaml` |
| Impersonation not working | Ensure `impersonation.enabled: true` and user mapping is correct |
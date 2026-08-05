# Helm Chart Installation

This document covers installing telectl via the official Helm chart.

## Prerequisites

- Kubernetes cluster (v1.28+)
- Helm v3.12+
- A Telegram bot token from [@BotFather](https://t.me/BotFather)
- Your Telegram user ID (get it from [@userinfobot](https://t.me/userinfobot))

## Quick Install

```bash
# Add the Helm repository (or use local chart)
helm repo add telectl https://ksauraj.github.io/telectl/
helm repo update

# Install with required values
helm install telectl telectl/telectl \
  --namespace telectl \
  --create-namespace \
  --set telegram.botToken="YOUR_BOT_TOKEN" \
  --set telegram.allowedUserIds="{123456789}" \
  --set rbac.clusterScoped=true
```

## Using the Local Chart

If you prefer to use the chart from the source repository:

```bash
git clone https://github.com/ksauraj/telectl
cd telectl

helm install telectl ./charts/telectl \
  --namespace telectl \
  --create-namespace \
  --set telegram.botToken="YOUR_BOT_TOKEN" \
  --set telegram.allowedUserIds="{123456789}" \
  --set rbac.clusterScoped=true
```

## Configuration Options

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Container image repository | `ghcr.io/ksauraj/telectl` |
| `image.tag` | Image tag (defaults to chart appVersion) | `""` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `imagePullSecrets` | Image pull secrets for private registries | `[]` |
| `serviceAccount.create` | Create ServiceAccount | `true` |
| `serviceAccount.annotations` | ServiceAccount annotations | `{}` |
| `serviceAccount.name` | ServiceAccount name (empty = auto-generated) | `""` |
| `rbac.create` | Create RBAC resources | `true` |
| `rbac.clusterScoped` | Cluster-wide (true) or namespace-scoped (false) | `true` |
| `rbac.allowedNamespaces` | Namespaces for namespace-scoped RBAC | `[]` |
| `telegram.botToken` | **Required.** Bot token from @BotFather | `""` |
| `telegram.allowedUserIds` | Comma-separated Telegram user IDs | `[]` |
| `telegram.adminUserIds` | Admin user IDs for privileged operations | `[]` |
| `kubernetes.kubeconfigPath` | Path to kubeconfig (empty = in-cluster) | `""` |
| `kubernetes.defaultNamespace` | Default namespace for commands | `default` |
| `kubernetes.context` | Override kubeconfig context | `""` |
| `kubernetes.timeout` | Request timeout in seconds | `30` |
| `kubernetes.dryRun` | Log mutations without executing | `false` |
| `kubernetes.clusterName` | Display name for cluster | `""` |
| `kubernetes.burst` | client-go burst limit | `10` |
| `kubernetes.qps` | client-go QPS limit | `5.0` |
| `impersonation.enabled` | Enable per-user K8s impersonation | `false` |
| `impersonation.defaultUser` | Default user to impersonate | `""` |
| `impersonation.defaultGroups` | Default groups to impersonate | `[]` |
| `impersonation.userMapping` | Per-Telegram-user K8s user/group mapping | `{}` |
| `logging.level` | Log level (debug/info/warn/error) | `info` |
| `logging.format` | Log format (json/console) | `json` |
| `logging.output` | Log output (stdout/stderr) | `stdout` |
| `bot.maxMessageLength` | Max message length | `4096` |
| `bot.commandPrefix` | Command prefix | `/` |
| `bot.enableMarkdown` | Enable MarkdownV2 | `true` |
| `bot.rateLimit` | Max requests per user per minute | `30` |
| `bot.enableMenuButton` | Show persistent menu button | `true` |
| `bot.enableReplyKeyboard` | Show persistent reply keyboard | `true` |
| `bot.menuPageSize` | Resources per page | `10` |
| `bot.allowedCommands` | Allowed command names | See values.yaml |
| `resources` | Pod resource limits/requests | `{}` |
| `podSecurityContext` | Pod security context | See values.yaml |
| `securityContext` | Container security context | See values.yaml |
| `nodeSelector` | Node selector | `{}` |
| `tolerations` | Pod tolerations | `[]` |
| `affinity` | Pod affinity | `{}` |
| `extraEnv` | Additional environment variables | `[]` |
| `extraVolumeMounts` | Additional volume mounts | `[]` |
| `extraVolumes` | Additional volumes | `[]` |

## Complete Example

```yaml
# values-prod.yaml
image:
  repository: ghcr.io/ksauraj/telectl
  tag: "v0.1.0"
  pullPolicy: IfNotPresent

serviceAccount:
  create: true
  annotations:
    # Add cloud provider annotations for IRSA, Workload Identity, etc.
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
    "123456789":    # Your admin Telegram ID
      user: "admin-user"
      groups: ["system:masters", "platform-team"]
    "987654321":    # Developer
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

# Optional: node placement
# nodeSelector:
#   kubernetes.io/arch: amd64
# tolerations:
#   - key: "dedicated"
#     operator: "Equal"
#     value: "monitoring"
#     effect: "NoSchedule"
```

Install with:

```bash
helm install telectl ./charts/telectl \
  --namespace telectl \
  --create-namespace \
  -f values-prod.yaml
```

## RBAC Modes

### Cluster-Scoped (Default)

Creates `ClusterRole` and `ClusterRoleBinding` with access to all namespaces. Suitable for cluster administrators.

```yaml
rbac:
  clusterScoped: true
```

### Namespace-Scoped

Creates `Role` and `RoleBinding` limited to specific namespaces.

```yaml
rbac:
  clusterScoped: false
  allowedNamespaces: ["default", "production", "staging"]
```

## Impersonation for Per-User RBAC

When `impersonation.enabled: true`, the bot uses Kubernetes impersonation to act as different K8s users based on the Telegram user ID. This allows different Telegram users to have different K8s permissions through a single bot instance.

### Prerequisites

The bot's ServiceAccount needs permission to impersonate:

```yaml
# Automatically added when impersonation.enabled: true
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: telectl-impersonator
rules:
- apiGroups: [""]
  resources: ["users", "groups", "serviceaccounts"]
  verbs: ["impersonate"]
```

### User Mapping

```yaml
impersonation:
  enabled: true
  defaultUser: "system:serviceaccount:default:readonly"
  defaultGroups: ["viewers"]
  userMapping:
    "123456789":    # Admin's Telegram ID
      user: "admin-user"
      groups: ["system:masters"]
    "987654321":    # Developer's Telegram ID
      user: "dev-user"
      groups: ["developers"]
```

## Verification

After installation, verify the deployment:

```bash
# Check pod status
kubectl get pods -n telectl -l app.kubernetes.io/name=telectl

# Check logs
kubectl logs -n telectl -l app.kubernetes.io/name=telectl

# Verify RBAC
kubectl auth can-i get pods --as=system:serviceaccount:telectl:telectl
```

## Upgrading

```bash
# Update to latest chart version
helm repo update
helm upgrade telectl telectl/telectl -n telectl

# Or with local chart and custom values
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
└── README.md            # This file
```

## Image Repository

The chart uses `ghcr.io/ksauraj/telectl` by default. Images are built and pushed automatically on tagged releases via GitHub Actions.

To use a different registry or private image:

```yaml
image:
  repository: your-registry.com/telectl
  tag: "v0.1.0"
imagePullSecrets:
  - name: your-registry-secret
```
# Try It Locally

Not sure if telectl is for you? The fastest way to find out is a **local
cluster** — kind, minikube, k3d, k3s, or MicroK8s — with the Helm chart.
You get a real Kubernetes API server, real RBAC, and a real bot, all on one
machine, with nothing to clean up afterwards except a delete command.

This guide is self-contained: create a cluster, apply the RBAC, install the
chart, and talk to the bot. If any step is unclear, the
[Helm Chart Guide](Helm-Chart-Guide) and
[Kubernetes RBAC](Kubernetes-RBAC) pages have the deeper explanations.

## What you need

| Tool | Why |
|------|-----|
| A local cluster (see table below) | the API server the bot talks to |
| `helm` v3+ | installs the chart |
| `kubectl` | inspecting the cluster while testing |
| Docker | kind/k3d/minikube(container driver) run clusters in containers |
| Telegram bot token | from @BotFather — the only real "secret" |
| Your Telegram user ID | from @userinfobot — the allow-list |

telectl talks to the Kubernetes API directly through client-go. It works
against **any** cluster that gives it a kubeconfig and RBAC — local clusters
are just the cheapest way to try it.

---

## Step 1 — Create a local cluster

Pick one row. All of these give you a cluster telectl can manage; the
difference is only how the cluster is created and what its kubeconfig context
is called.

### kind (Kubernetes in Docker)

```bash
kind create cluster --name telectl-test
```

- Requires Docker.
- Context: `kind-telectl-test` (auto-selected as current).
- Kubeconfig: `kind get kubeconfig --name telectl-test` or use the merged
  default `~/.kube/config`.

### minikube

```bash
minikube start --driver=docker      # or --driver=virtualbox / hyperkit / kvm2
```

- Context: `minikube` (auto-selected).
- Kubeconfig: `minikube update-context` writes it into `~/.kube/config`.

### k3d (k3s in Docker)

```bash
k3d cluster create telectl-test
```

- Requires Docker. Lightweight (k3s).
- Context: `k3d-telectl-test` (auto-selected).

### k3s (native single-node)

```bash
curl -sfL https://get.k3s.io | sh -s - --write-kubeconfig-mode 644
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
```

- Runs as a systemd service; no Docker needed.
- Context: `default` (auto-selected via the exported kubeconfig).

### MicroK8s

```bash
snap install microk8s --classic
microk8s status --wait-ready
microk8s config > ~/.kube/microk8s-config   # or use the generated config
export KUBECONFIG=$HOME/.kube/microk8s-config
```

- Snap-based; runs its own containerd.
- Context: `microk8s`.

### Docker Desktop (built-in Kubernetes)

```bash
# Enable in Docker Desktop: Settings -> Kubernetes -> Enable Kubernetes
```

- Context: `docker-desktop` (auto-selected once enabled).

### Rancher Desktop

```bash
# Enable in the app: Kubernetes -> Enable Kubernetes (dockerd or containerd)
```

- Context: `rancher-desktop`.

Verify whatever you chose:

```bash
kubectl cluster-info
kubectl get nodes
```

One node, `Ready`. If `kubectl` used a different context than your new
cluster, point it explicitly: `kubectl config use-context <context>`.

---

## Step 2 — Create a sample workload

Something to look at while testing:

```bash
kubectl create namespace production
kubectl create deployment frontend -n production --image=nginx --replicas=3
kubectl get deploy,pods -n production
```

`frontend` is what you will scale, restart, and read logs from in the test
steps below.

---

## Step 3 — Apply the RBAC (identities + impersonation)

The Helm chart creates the **bot's** ServiceAccount, ClusterRole, and the
impersonate rule. It does **not** create the identities the bot impersonates
(`admin-user`, `readonly-user`) or their role bindings — you create those.
This separation is intentional: the chart owns the bot, you own the people.

Save this as `rbac.yaml` and apply it:

```yaml
# -----------------------------------------------------------------------------
# rbac.yaml — identities for per-user RBAC (2-user setup: admin + read-only)
# -----------------------------------------------------------------------------

# Read-only identity
apiVersion: v1
kind: ServiceAccount
metadata:
  name: readonly-user
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: readonly-user
rules:
- apiGroups: [""]
  resources: ["pods", "services", "configmaps", "secrets", "namespaces",
              "nodes", "persistentvolumeclaims", "persistentvolumes",
              "events", "endpoints", "serviceaccounts"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments", "replicasets", "statefulsets", "daemonsets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["networking.k8s.io"]
  resources: ["ingresses"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods", "nodes"]
  verbs: ["get", "list", "watch"]
---
# Bind the read-only role to the GROUP the bot impersonates ("viewers").
# This is the binding that makes impersonation actually work.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: readonly-user-group
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: viewers
roleRef:
  kind: ClusterRole
  name: readonly-user
  apiGroup: rbac.authorization.k8s.io
---
# Admin identity
apiVersion: v1
kind: ServiceAccount
metadata:
  name: admin-user
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: admin-user
subjects:
- kind: ServiceAccount
  name: admin-user
  namespace: default
roleRef:
  kind: ClusterRole
  name: cluster-admin
  apiGroup: rbac.authorization.k8s.io
---
# The bot's impersonation permission: allow the bot's SA ("telectl") to
# impersonate exactly the identities above.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: telectl-impersonator
rules:
- apiGroups: [""]
  resources: ["users", "groups", "serviceaccounts"]
  verbs: ["impersonate"]
  resourceNames:
  - "admin-user"
  - "readonly-user"
  - "system:serviceaccount:default:readonly"
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: telectl-impersonator
subjects:
- kind: ServiceAccount
  name: telectl
  namespace: default
roleRef:
  kind: ClusterRole
  name: telectl-impersonator
  apiGroup: rbac.authorization.k8s.io
```

Apply and sanity-check:

```bash
kubectl apply -f rbac.yaml

# Read-only user should read, but not mutate:
kubectl auth can-i get pods -n production --as=readonly-user --as-group=viewers      # yes
kubectl auth can-i delete pods -n production --as=readonly-user --as-group=viewers   # no

# Admin can do everything:
kubectl auth can-i delete pods -n production --as=admin-user --as-group=system:masters  # yes
```

> **Why the group binding matters:** the bot impersonates the *user*
> `readonly-user` with *group* `viewers`. Binding the read-only role only to
> the ServiceAccount leaves the impersonated identity with **zero**
> permissions — the classic "can't read anything" bug. The `readonly-user-group`
> binding above fixes it. Details:
> [Kubernetes RBAC](Kubernetes-RBAC#the-critical-gotcha).

---

## Step 4 — Write the Helm values

Save as `values.yaml`. Replace `REPLACE_WITH_BOT_TOKEN` with your BotFather
token and the user IDs with yours. The image defaults to the published GHCR
release — no local builds, no `pullPolicy: Never`:

```yaml
# -----------------------------------------------------------------------------
# values.yaml — 2-user setup: one admin, one read-only
# -----------------------------------------------------------------------------
image:
  repository: ghcr.io/ksauraj/telectl
  tag: v0.1.0-beta.0
  pullPolicy: IfNotPresent     # pulls from GHCR; correct for kind/minikube/k3d

serviceAccount:
  create: true

rbac:
  create: true
  clusterScoped: true

telegram:
  botToken: "REPLACE_WITH_BOT_TOKEN_FROM_BOTFATHER"
  allowedUserIds: ["YOUR_ADMIN_TELEGRAM_ID", "YOUR_READONLY_TELEGRAM_ID"]   # YOUR admin + read-only IDs
  adminUserIds: ["YOUR_ADMIN_TELEGRAM_ID"]                    # UX hint only

kubernetes:
  defaultNamespace: "default"
  timeout: 30
  dryRun: false

impersonation:
  enabled: true
  defaultUser: "system:serviceaccount:default:readonly"
  defaultGroups: ["viewers"]
  userMapping:
    "YOUR_ADMIN_TELEGRAM_ID":
      user: "admin-user"
      groups: ["system:masters"]
    "YOUR_READONLY_TELEGRAM_ID":
      user: "readonly-user"
      groups: ["viewers"]

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
```

> The Telegram user IDs must be **quoted strings** — unquoted, YAML renders
> large IDs in scientific notation (`1.28889517e+09`) and the mapping breaks.
> See [Configuration Reference](Configuration-Reference#telegram).

---

## Step 5 — Install the chart

```bash
helm repo add telectl https://ksauraj.github.io/telectl/charts
helm repo update telectl

helm install telectl telectl/telectl \
  --namespace default \
  -f values.yaml \
  --wait --timeout 180s
```

Watch it come up:

```bash
kubectl get pods -l app.kubernetes.io/name=telectl
kubectl logs deploy/telectl --tail=20
```

Expect a startup line like:

```
INFO  Starting telectl   {"version": "v0.1.0-beta.0", "dry_run": false, "allowed_users": 2}
```

and no errors about the API server. If the pod is `ImagePullBackOff`, your
cluster can't reach GHCR (common on restrictive networks) — see
[Troubleshooting](Troubleshooting#bot-not-responding).

---

## Step 6 — Test from Telegram

1. Open your bot, press **Start**.
2. **Admin user:** `/get pods -A`, then `/scale deployment frontend 5 -n production`
   — should succeed.
3. **Read-only user:** `/get pods -A` works; `/scale ...` and delete return
   **Forbidden** — that's RBAC doing its job.
4. Check the audit trail in the bot's logs (each mutating action carries
   `telegram_user_id`):

```bash
kubectl logs deploy/telectl | grep "User action"
```

```
INFO  User action: scale resource  {"telegram_user_id": YOUR_ADMIN_TELEGRAM_ID, ...}
```

---

## Cleanup

```bash
helm uninstall telectl          # removes the Deployment, RBAC, Secret
kubectl delete -f rbac.yaml     # removes the identities + bindings

# and/or delete the cluster entirely:
kind delete cluster --name telectl-test      # kind
minikube delete                              # minikube
k3d cluster delete telectl-test              # k3d
sudo systemctl stop k3s && sudo /usr/local/bin/k3s-uninstall.sh   # k3s
microk8s stop && sudo snap remove microk8s   # MicroK8s
```

> Note: the Helm Secret is kept on `helm uninstall` (it carries
> `helm.sh/resource-policy: keep`). On a throwaway local cluster that's
> irrelevant, but on a real one delete it explicitly if you're rotating the
> token: `kubectl delete secret telectl -n default`.

---

## Which local cluster should you pick?

| Cluster | Best for | Docker needed | Notes |
|---------|----------|---------------|-------|
| **kind** | CI-like, reproducible tests | yes | the fastest to create/delete; most used in this project's own tests |
| **minikube** | closest to a "real" single-node cluster | optional (has its own drivers) | feature-rich: addons, dashboard, multi-driver |
| **k3d** | lightweight + fast | yes | k3s-in-Docker, tiny footprint |
| **k3s** | native single-node without Docker | no | systemd service; good for ARM (Raspberry Pi) too |
| **MicroK8s** | Ubuntu workstations | no | snap-based; includes addons (dns, dashboard, registry) |
| **Docker Desktop** | macOS/Windows users who already run Docker | built-in | one toggle to enable Kubernetes |
| **Rancher Desktop** | macOS/Windows with a GUI preference | built-in | containserd or dockerd backend |

All of them produce a standard kubeconfig that telectl consumes unchanged —
there is nothing cluster-specific in the bot. If you have any other
conformant Kubernetes (K0s, Talos, Civo, EKS, GKE, AKS, …), the same three
steps apply: create cluster, apply `rbac.yaml`, `helm install` with
`values.yaml`.

## Related

- [Helm Chart Guide](Helm-Chart-Guide) — every chart value and RBAC option
- [Kubernetes RBAC](Kubernetes-RBAC) — roles, bindings, impersonation
- [Two Deployment Modes](Two-Deployment-Modes) — Helm pod vs normal-user binary
- [Troubleshooting](Troubleshooting) — if something doesn't come up
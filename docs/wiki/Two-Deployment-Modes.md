# Two Ways to Run telectl

telectl can be deployed two ways, and they differ in **who your bot is** in
the cluster. The decision shapes your security model, what the config looks
like, and how many clusters you can manage.

## Setup A — Helm (bot runs as a pod inside the cluster)

The bot is a Deployment inside one of your clusters. It uses an
**in-cluster ServiceAccount** and **per-user impersonation**.

| Aspect | What happens |
|--------|--------------|
| **Credential** | The pod's ServiceAccount token (mounted automatically). No kubeconfig needed. |
| **Who the bot "is"** | A ServiceAccount in your cluster. |
| **How users are separated** | `impersonation.enabled: true` maps each Telegram user → a k8s identity (`user` + `groups`). The bot re-issues every request with `Impersonate-User` / `Impersonate-Group` headers, and **Kubernetes RBAC** decides yes/no. |
| **Config source** | Helm `values.yaml` → ConfigMap + Secret; the bot reads `/app/config.yaml`. |
| **Config present in cluster?** | The kubeconfig-based keys (`kubeconfig_path`) are **ignored**; the bot auto-detects it's in-cluster and uses `rest.InClusterConfig()`. |
| **Number of clusters** | **One.** The pod's ServiceAccount only exists in, and is only valid for, that cluster. Multi-cluster switching is not possible. |
| **Deploy/upgrade** | `helm upgrade`. |
| **Best for** | Running the bot alongside the cluster it manages, as a managed, declarative, per-user-RBAC setup. |

### Minimal helm install

```bash
helm install telectl telectl/telectl \
  --namespace telectl --create-namespace \
  --set telegram.botToken="YOUR_BOT_TOKEN" \
  --set impersonation.enabled=true
```

> Full walkthrough with RBAC roles/bindings and impersonation: see
> [Impersonation & RBAC](Impersonation-and-RBAC) and the
> [Helm Chart Guide](Helm-Chart-Guide).

---

## Setup B — Normal user (bot runs as a binary on your workstation)

|telectl| runs as a regular process on your machine (or a container you manage
yourself, not via the chart). It authenticates with **your** kubeconfig,
exactly like `kubectl`.

| Aspect | What happens |
|--------|--------------|
| **Credential** | Your kubeconfig — `$KUBECONFIG`, `~/.kube/config`, or an explicit `kubernetes.kubeconfig_path`. No ServiceAccount involved. |
| **Who the bot "is"** | **You.** Your user/cert/SA identity, from the context you pick. |
| **How users are separated** | No impersonation needed normally. The bot runs with your privileges, so every Telegram user that reaches it is acting as you. (For multi-user separation you'd put the binary behind something that already restricts callers, or run it again per role.) |
| **Config source** | A local YAML file discovered at `~/.config/telectl/`, `/etc/telectl/`, etc. (`kubernetes.kubeconfig_path`, `kubernetes.context`, …). |
| **Config present in cluster?** | No. It reads your kubeconfig path whenever set; otherwise falls back to default loading rules (`$KUBECONFIG`, `~/.kube/config`). |
| **Number of clusters** | **Many.** The `contexts` / `use-context` commands switch which cluster the bot talks to. |
| **Deployment** | Download the release binary (or `docker run` your own image) and run it. |
| **`/contexts`** | Lists every context in the kubeconfig as a button for quick switching. |
| **`/use-context <name>`** | Switches the active context **in-process** — it does *not* rewrite the file on disk, so per-session switching is cheap and safe. |

### How cluster switching works

The bot builds its client from a context in the kubeconfig. Switching context
re-builds the REST config + clientset for the chosen context and swaps it in
only after every step succeeds, so a failed switch leaves the bot usable
against the previous cluster. There are two APIs:

- `SwitchContext(name)` — in-memory only; does not touch the kubeconfig file.
- `SwitchContextPersistent(name)` — also writes `current-context` to the
  kubeconfig so it survives a restart.

### Run from binary

```bash
# point it at your real kubeconfig (or rely on defaults)
telectl bot --config ~/.config/telectl/config.yaml
```

```yaml
telegram:
  botToken: "YOUR_BOT_TOKEN"
kubernetes:
  # optional; default = $KUBECONFIG / ~/.kube/config
  kubeconfig_path: "/home/you/.kube/config"
  context: "my-prod-cluster"        # optional initial context
impersonation:
  enabled: false                    # you are the identity; no impersonation
```

---

## Side-by-side

| | **Setup A — Helm pod** | **Setup B — normal user** |
|---|---|---|
| Where it runs | inside the cluster | your machine / your own container |
| Credential | SA token (injected) | your kubeconfig |
| Identity | a ServiceAccount | you |
| Per-user RBAC | impersonation, RBAC decides | one identity = you |
| Multi-cluster | **no** | **yes** (`contexts` / `use-context`) |
| Config | ConfigMap+Secret via Helm | local config file |
| kubeconfig key | ignored (auto in-cluster) | used |
| Primary use case | managed, multi-user org bot | personal, multi-cluster dev tool |

---

## Which one should you pick?

- **Choose Helm (Setup A)** if the bot manages a cluster that lives somewhere
  permanent and more than one person uses it — RBAC impersonation gives you
  real per-user permissions enforced by Kubernetes itself.
- **Choose local binary (Setup B)** if it's your personal tool and you want
  to hop between several clusters (dev, staging, prod, multiple tenants)
  from one bot session without redeploying anything.

> Note: Setup B does *not* apply the per-user impersonation map; the bot uses
> your identity for every user you allow. For shared machines treat the bot's
> Telegram token as carrying the same trust/power as your kubeconfig.
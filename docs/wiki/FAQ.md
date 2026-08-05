# FAQ

Frequently asked questions about telectl.

## General

### Do I need kubectl installed?
No. telectl talks to the API server directly through client-go. kubectl is
neither required nor bundled. It *does* need a valid kubeconfig (or an
in-cluster ServiceAccount).

### Why does telectl hold my full kubeconfig permissions?
When you run it as a normal user, the bot literally is your identity — it
uses the context from your kubeconfig. That's the point. Treat the bot token
like a cluster credential. For per-user separation, use
[Impersonation & RBAC](Impersonation-and-RBAC).

### Is it a plugin for kubectl? A dashboard?
Neither. It's a standalone operator that happens to use client-go.

## Security & permissions

### My read-only user can read nothing — why?
The classic bug: the role is bound to the *ServiceAccount* but the bot
impersonates the *user* `readonly-user` with *group* `viewers`. Bind the role
to the **`viewers` group** — not just the SA. See
[Kubernetes RBAC](Kubernetes-RBAC#the-critical-gotcha).

### How do I stop strangers using the bot?
Set `telegram.allowed_user_ids`. An empty list lets *anyone who finds the
bot* operate the cluster.

### Can a read-only user delete pods?
By RBAC, no (the API server returns Forbidden). But make sure the bot routes
**every** action (including menu actions) through the impersonated client —
an early bug did mutating menu actions with the bot's own broad client.
Current builds impersonate all actions.

### What is `dry_run`?
With `dry_run: true`, mutating operations are logged but never applied, and
replies say so explicitly. Run it this way first to see what the bot would do.

## Commands & behavior

### What does `/scale` need?
A deployment or replicaset. `/scale deployment frontend 5 -n production`.
Only deployments/replicasets scale; others reply "Not scalable".

### Why does `/top` sometimes say "Metrics unavailable"?
It needs **metrics-server** in the cluster. Without it the metrics API returns
404; telectl reports it as the expected condition rather than a raw error.

### Why "No events"? 
Events expire after about an hour. A quiet namespace legitimately has none.

### The bot doesn't reply. Why?
Check the pod/process logs. Common causes: wrong bot token, your ID missing
from `allowed_user_ids`, or (in-cluster) a missing ServiceAccount. For Helm:
`kubectl logs -n <ns> deploy/telectl`.

### "Conflict: terminated by other getUpdates"
Two instances are both long-polling. There must be exactly one running pod.

## Configuration

### Where is the config file?
`~/.config/telectl/telectl.yaml`, `~/.config/telectl.yaml`, `/etc/telectl/telectl.yaml`,
or `--config path`. Full reference:
[Configuration Reference](Configuration-Reference).

### How do I override a key per environment?
Uppercase, replace `.` with `_`, prefix `TELECTL_`:
`kubernetes.dry_run → TELECTL_KUBERNETES_DRY_RUN`. Credentials keep
conventional names (`TELEGRAM_BOT_TOKEN`, `KUBECONFIG`, `ALLOWED_USER_IDS`).

### My Telegram ID shows as `1.28889517e+09` in YAML?
Quote the ID in YAML (`"YOUR_ADMIN_TELEGRAM_ID"`) or the file / `--set` may render it in
scientific notation.

## Deployment

### Helm pod vs local binary?
Helm = in-cluster pod, uses a ServiceAccount + impersonation, single cluster.
Local binary = your kubeconfig, acts as you, multi-cluster. Full comparison:
[Two Deployment Modes](Two-Deployment-Modes).

### Does the bot auto-pick in-cluster config?
Yes. With no kubeconfig path, it tries `rest.InClusterConfig()` first, then
falls back to default loading rules (`$KUBECONFIG`, `~/.kube/config`).

### How do I switch clusters?
`/contexts` lists them; `/use-context <name>` switches in-process (no file
rewrite). See [Context Management](Context-Management).

## Contributing

### How do I contribute? Where are the tests?
See [Contributing Guide](Contributing-Guide) and
[Testing Guide](Testing-Guide). CI runs formatting, vet, tests, lint, and
build on every PR; green CI is required to merge.
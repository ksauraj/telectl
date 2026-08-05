# Production Checklist

Everything to verify before pointing telectl at anything you care about.

## Before you deploy

- [ ] **Bot token is a secret.** The token grants kubeconfig-level powers.
      Never commit it, never bake it into an image. Use the Helm Secret or a
      mounted file, read-only.
- [ ] **`telegram.allowed_user_ids` is set.** An empty list means *everyone
      who finds the bot* can operate the cluster. Set it to your real IDs
      (get them from @userinfobot).
- [ ] **`dry_run: false` is deliberate.** Run with `dry_run: true` first and
      watch the logs — mutations are logged, never applied — before enabling
      real operations.
- [ ] **Per-user RBAC is configured** (`impersonation.enabled: true` +
      `user_mapping`), not a shared admin identity for everyone. The
      **default user for unmapped users is least-privilege** (e.g.
      `system:serviceaccount:default:readonly` + `viewers`).
- [ ] **Group bindings match the impersonated groups.** Remember: the bot
      impersonates `user` + `groups`; the role must be bound to the **Group**
      (not just the SA). Verify with `kubectl auth can-i … --as-group=…`.
- [ ] **The bot's impersonator role is scoped with `resourceNames`** so it
      can only impersonate the identities you actually map, not arbitrary
      users.

## After deploy

- [ ] `kubectl get pods` shows the bot pod `Running` (1 replica — two pods
      polling getUpdates conflict).
- [ ] Startup log shows `Starting telectl` with the expected version,
      `dry_run`, and `allowed_users` count.
- [ ] `kubectl logs` shows per-user action lines (they include
      `telegram_user_id`, resource, namespace) — verify **every mutating
      action is logged with the user**.
- [ ] **Test both identities from chat:**
  - admin user: scale a workload up and down, delete a test pod
  - read-only user: same commands must return **Forbidden**
- [ ] `kubectl auth can-i --list --as=system:serviceaccount:default:<bot-sa>`
      shows `impersonate` on exactly the mapped identities.

## Operations

- [ ] **Backups / gitops:** telectl can mutate a cluster from chat. If you
      need audit or rollback, pair it with your existing change-management
      (GitOps, backups, etc.) — the bot doesn't gate on them.
- [ ] **Log retention:** per-user action logs are the audit trail; ship pod
      logs to your log collector.
- [ ] **Token rotation:** rotate the bot token periodically, and when any
      past user loses access (their Telegram session is the access).
- [ ] **Upgrades:** follow [Upgrading](Upgrading) — config shape is stable
      across the 0.1.x line; pick the release ladder (alpha → beta → stable)
      deliberately.

## In-cluster (Helm) specifics

- [ ] Values quote the Telegram user IDs (`"YOUR_ADMIN_TELEGRAM_ID"`) — YAML otherwise
      renders big IDs in scientific notation (`1.28889517e+09`).
- [ ] If you change values that touch the Secret, delete the old Secret first
      (it carries `helm.sh/resource-policy: keep`).
- [ ] `--wait --timeout` installs; a 60s tool timeout may *report* cancelled
      while the release actually completes — check `helm status`.

## Security

- [ ] See [Security](Security) for the full threat model; [Kubernetes RBAC](Kubernetes-RBAC)
      for the permission details.
---
title: Troubleshooting
nav_order: 24
---

# Troubleshooting

Common issues and how to resolve them.

## Bot Not Responding

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| No reply at all | bot token wrong / not allowed | Check logs; verify `telegram.botToken` and `telegram.allowedUserIds` |
| `Conflict: terminated by other getUpdates` | two bot instances running (e.g. old pod lingering during rollout) | Ensure only one replica; wait for the old pod to terminate |
| `Forbidden` on everything | impersonated identity has no bindings | See [Impersonation & RBAC](Impersonation-and-RBAC) — bind the role to the `viewers` group |

## RBAC / Impersonation

```bash
kubectl get pod -n telectl -l app.kubernetes.io/name=telectl
kubectl logs -n telectl -l app.kubernetes.io/name=telectl   # look for "Using impersonated K8s client"

# Who can the bot impersonate?
kubectl auth can-i --list --as=system:serviceaccount:default:telectl | grep impersonate

# Manual check of a read-only identity
kubectl --as=system:serviceaccount:default:telectl get pods -n production --as=readonly-user --as-group=viewers
```

If a read-only user can suddenly mutate, or an admin can't read:

- The **role binding** is the source of truth, not the bot. `kubectl get clusterrolebindings | grep -i readonly`.
- The binding must reference the **identity the bot impersonates** (user or
  group), not some unrelated ServiceAccount.

## Logs Truncation

- `--tail N` is now respected (the formatter no longer caps below it). If you
  still see fewer lines, the single-message pane limit (~3500 chars with
  margin) applies — log panes keep the **newest** lines and cut the head.
- For full output, run `/logs <pod> --tail 1000` and scroll; very long output
  is chunked by the sender.

## Tables Showing `**Field**`

- Fixed in `v0.1.0-beta.0`. If you still see literal asterisks, you're on an
  older image — upgrade: `kubectl rollout restart deployment/telectl` after
  pulling the new tag.

## Config Not Applied

```bash
kubectl get configmap -n telectl telectl-config -o yaml
kubectl exec -it -n telectl deploy/telectl -- telectl config
```

## Image Pull Back Off

```bash
kubectl describe pod -n telectl -l app.kubernetes.io/name=telectl
```

- GHCR is **public** now, so no `imagePullSecrets` needed for public pulls.
- Verify the tag exists: `kubectl get image --namespace default` or check
  [GHCR](https://github.com/ksauraj/telectl/pkgs/container/telectl).

---

[🔙 Home](Home)
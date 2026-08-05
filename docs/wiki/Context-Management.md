# Context Management

telectl can switch between multiple Kubernetes clusters from a single bot
session, driven by the **contexts in the kubeconfig it runs against**.

## How it works

A kubeconfig can hold many contexts, each pairing a *cluster*, a *user*,
and a *namespace*. telectl builds its API client from the current context.
Switching context rebuilds the client for the new cluster **in-process** and
swaps it in only after every step succeeds — so a failed switch leaves the
bot usable against the previous cluster instead of half-broken.

## Listing contexts

```
/contexts
```

Shows every context in the kubeconfig as a button: cluster, user, namespace,
and which one is current. Tap one to switch.

You can also list them from the shell (binary subcommand):

```
telectl contexts
```

## Switching context

**In-session (in-process):**

```
/use-context <name>
```

Rebuilds the REST config + clientset for `<name>` and switches the *bot
session* to it. **It does not rewrite the kubeconfig file on disk** — which
is deliberate: `~/.kube/config` is shared with `kubectl` and every other tool
(and potentially other users of the bot), so telectl won't clobber it.

**Persistent (writes disk):**

From **Settings → Context**, or the code's `SwitchContextPersistent` API —
also writes `current-context` back to the kubeconfig so the choice survives a
bot restart.

The Settings pane always shows the live **Context** and **Namespace** at the
top, so you know which cluster you're pointing at.

## Session scoping

- Context and namespace are **session-scoped** — per chat, for you. Two users
  of the same bot can each be on a different cluster.
- Under impersonation, the impersonated identity still applies per-user; the
  context just changes *which* cluster the (impersonated) request goes to.

## Example: a dev + prod workflow

```bash
# kubeconfig has contexts: dev-cluster, staging, prod-live
```

```
/contexts                # see all three
/use-context dev-cluster # browse dev pods
/use-context prod-live   # switch to prod for a check
```

## Gotchas

- The bot can only switch between contexts present in its kubeconfig. Add
  contexts to that file (or `KUBECONFIG`) to make them available.
- Context switching is *client-level*; it doesn't change which identity each
  Telegram user impersonates. RBAC still applies per user on *every* cluster
  — a read-only role stays read-only no matter which context you switch to.
- A failed switch logs the reason and keeps you on the previous cluster.

## Related

- [Two Deployment Modes](Two-Deployment-Modes) — multi-cluster is a natural
  fit for the normal-user (local binary) deployment; the in-cluster Helm pod
  is single-cluster by nature.
- [Command Reference](Command-Reference) — `/contexts`, `/use-context`.
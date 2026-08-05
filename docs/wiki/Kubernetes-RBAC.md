# Kubernetes RBAC

How Kubernetes RBAC governs what telectl — and each of its Telegram users —
is allowed to do. This is the security core of the bot.

## The principle

**telectl never decides who may do what. Kubernetes RBAC does.**

The bot talks to the API server through client-go using the identities
described below. For every operation, the API server evaluates RBAC for the
*impersonated* identity and answers allow or deny. The bot has zero hardcoded
permissions.

## Two layers of RBAC

### 1. The bot's own identity

The bot runs as one identity (a ServiceAccount in the cluster, or your
kubeconfig user). That identity needs:

- read/write on the resources the bot manages (pods, deployments, …)
- `impersonate` on the identities it will act as (for per-user RBAC)

The Helm chart generates this via `rbac.create: true` (a ClusterRole +
ClusterRoleBinding + the impersonate rule).

### 2. The impersonated identities (per-user RBAC)

When impersonation is on, each Telegram user maps to a K8s identity
(`user` + `groups`) via `impersonation.user_mapping`. The bot sends
`Impersonate-User` / `Impersonate-Group` headers, and the API server checks
**that** identity's bindings.

| Object | Kind | Purpose |
|--------|------|---------|
| `readonly-user` (SA) | ServiceAccount | identity that only reads |
| `readonly-user` (role) | ClusterRole | `get/list/watch` only, no mutation, no `/scale` |
| `readonly-user` (binding→SA) | ClusterRoleBinding | direct checks `--as=...:readonly-user` |
| `readonly-user-group` | ClusterRoleBinding | binds the role to **Group `viewers`** — this is what impersonation matches |
| `admin-user` (SA) | ServiceAccount | identity with full access |
| `admin-user` binding | ClusterRoleBinding | SA → `cluster-admin` |
| `telectl-impersonator` | ClusterRole | lets the bot's SA impersonate exactly `admin-user`, `readonly-user`, `sa:readonly` |
| `telectl-impersonator` binding | ClusterRoleBinding | binds the role to the bot SA |

## The critical gotcha: SA binding vs Group binding

There are **two** ways to bind a role, and mixing them up breaks
impersonation:

- **Bind a ServiceAccount** → matches an identity *acting as that SA*.
- **Bind a Group** (e.g. `viewers`) → matches an identity *carrying that
  group*.

The bot impersonates the **user** `readonly-user` with **group** `viewers`.
If you only bind the role to the SA `readonly-user`, the impersonated user
has **zero** permissions — the classic "can't read anything" bug. You must
**also** bind the role to the `viewers` **group**.

```yaml
# The fix — bind the role to the GROUP the bot impersonates:
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: readonly-user-group
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: viewers          # the group the bot sends
roleRef:
  kind: ClusterRole
  name: readonly-user
  apiGroup: rbac.authorization.k8s.io
```

## Restricting WHICH identities the bot may impersonate

The impersonator role restricts `resourceNames` so the bot can't impersonate
arbitrary identities:

```yaml
- apiGroups: [""]
  resources: ["users", "groups", "serviceaccounts"]
  verbs: ["impersonate"]
  resourceNames:
  - "admin-user"
  - "readonly-user"
  - "system:serviceaccount:default:readonly"
```

> Note: the Helm chart's `impersonate` rule (when `impersonation.enabled`)
> grants impersonate broadly; for tight deployments, add `resourceNames` to
> lock the bot down to only the identities you actually map (see
> [Impersonation](Impersonation-and-RBAC)).

## Verify with `kubectl auth can-i`

Faithful pre-check (uses the same impersonation headers the bot sends):

```bash
# read-only user (group viewers)
kubectl auth can-i get pods -n production --as=readonly-user --as-group=viewers     # yes
kubectl auth can-i delete pods -n production --as=readonly-user --as-group=viewers   # no
kubectl auth can-i scale deploy frontend -n production --as=readonly-user --as-group=viewers  # no

# admin
kubectl auth can-i delete pods -n production --as=admin-user --as-group=system:masters  # yes
```

## Settings that turn RBAC on/off

| Config | Effect |
|--------|--------|
| `rbac.create: true` (Helm) | create the bot's ClusterRole + binding + impersonate rule |
| `impersonation.enabled: true` | route every request through per-user impersonation |
| `impersonation.user_mapping` | map each Telegram user → identity |

## Related

- [Impersonation & RBAC](Impersonation-and-RBAC) — the security-model deep dive
- [Security](Security) — threat model + least-privilege
- [Helm Chart Guide](Helm-Chart-Guide) — chart RBAC options
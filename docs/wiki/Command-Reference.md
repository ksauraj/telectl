# Command Reference

Every typed command telectl understands, with syntax, flags, and examples.
All commands start with the prefix from `bot.command_prefix` (default `/`).

## Command families

| Family | Commands |
|--------|----------|
| Info | `/start`, `/help`, `/about`, `/version`, `/config` |
| Browse | `/get`, `/describe`, `/resources` |
| Logs & exec | `/logs`, `/exec`, `/portforward` |
| Cluster | `/contexts`, `/use-context`, `/top`, `/events`, `/watch` |
| Operations | `/restart`, `/scale` |
| Menus | `/monitor`, `/operations`, `/settings` |

## Shared flags

Most browse/operate commands accept these kubectl-style flags (in both
separated and `=` forms):

| Flag | Meaning |
|------|---------|
| `-n, --namespace <ns>` | Namespace (defaults to session namespace, then `kubernetes.default_namespace`) |
| `-A, --all-namespaces` | All namespaces (overrides `-n`) |
| `-o, --output <fmt>` | `wide`, `json`, `yaml` (default: compact table) |
| `-l, --selector <expr>` | Label selector, e.g. `app=api,env=prod` |
| `--field-selector <expr>` | Field selector, e.g. `status.phase=Running` |

---

## `/start`
Greeting + the main reply keyboard.

## `/help`
Command reference summary.

## `/about`
Version, links, project info.

## `/version`
Build version, commit, and build date.

## `/config`
Prints the **effective** configuration (secrets redacted): which kubeconfig,
context, namespace, dry-run state, impersonation status, and the cluster
display name.

## `/get <resource> [name] [-n ns] [-o fmt] [-l selector] [--field-selector ...]`

List or fetch resources, kubectl-style.

```
/get pods -A
/get pods -n production -l app=api
/get deployment frontend -o yaml
/get nodes -o wide
```

Supported resources (aliases accepted):
`pods/po`, `deployments/deploy`, `services/svc`, `replicasets/rs`,
`namespaces/ns`, `nodes/no`, `configmaps/cm`, `secrets`,
`persistentvolumeclaims/pvc`, `persistentvolumes/pv`, `ingresses/ing`,
`events/ev`.

## `/describe <resource> [name] [-n ns]`

Detailed view: metadata, status conditions, events, and a readable summary.
Without a name, describes the resource type's notable objects.

## `/resources`

Opens the interactive resource browser menu (see
[Menu Navigation](Menu-Navigation)). Alias of tapping **Resources**.

## `/logs <pod> [-n ns] [-c container] [-f] [-p] [--tail N] [--since TIME]`

Read pod logs.

| Flag | Meaning |
|------|---------|
| `-c, --container <name>` | Specific container in the pod |
| `-f, --follow` | Stream new lines (bounded; stops with `/cancel`) |
| `-p, --previous` | Logs from the previous (terminated) container instance |
| `--tail N` | Last N lines. **Respected exactly** — no silent cap |
| `--since <time>` | Logs since a time (e.g. `10m`, `1h`, RFC3339) |

```
/logs api-74b6f5d9c9-abcde
/logs api-74b6f5d9c9-abcde -n production -c sidecar --tail 500
```

## `/exec <pod> [-n ns] [-c container] [-- command args...]`

Run a command inside a pod container.

- No command → interactive shell (`sh`).
- Everything after the pod name (or after `--`) is the command; the
  command's own flags are passed through untouched.

```
/exec api-abcde                     # shell
/exec api-abcde -n production -- printenv
/exec api-abcde -c sidecar ls -la
```

## `/portforward <pod> <local:remote> [-n ns]`

Forward a local port to a pod port. Output gives the forwarding address.

## `/contexts`

Lists every context in the kubeconfig with its cluster, user, and namespace,
as buttons — tap to switch. See [Context Management](Context-Management).

## `/use-context <name>`

Switch the active context **in-process** (does not rewrite the kubeconfig
file on disk). Persistent switch is available from the Settings menu's
context picker.

```
/use-context my-prod-cluster
```

## `/top <pods|nodes> [-n ns]`

CPU/memory usage. Requires metrics-server in the cluster; otherwise it says
so explicitly (rather than surfacing a raw 404).

```
/top pods -n production
/top nodes
```

## `/events [-n ns]`

Recent events in the namespace (events expire after ~1h, so a quiet
namespace may show none).

## `/watch <resource> [-n ns]`

Follows changes to a resource and pushes updates as they happen. Runs as a
command (not in the pane) because it streams; stop with `/cancel`.

```
/watch pods -n production
```

## `/restart deployment <name> [-n ns]`

Rolling restart of a workload (deployment/statefulset/daemonset).

```
/restart deployment frontend -n production
```

## `/scale deployment <name> <replicas> [-n ns]`

Scale a deployment or replicaset to a replica count.

```
/scale deployment frontend 5 -n production
```

## `/monitor`, `/operations`, `/settings`

Open the corresponding menu panes (see [Menu Navigation](Menu-Navigation)).

---

## Command allowlist

`bot.allowed_commands` controls which commands dispatch. The default list
(and the source of truth for valid names) is:

```
start help about version get describe logs exec portforward
contexts use-context config top events watch restart scale
resources monitor operations settings
```

Note the key is `portforward` (not `port-forward`). A command missing from
the list is rejected as *not allowed* even though its handler exists — if you
trim this list, trim it deliberately.
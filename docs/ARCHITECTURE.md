# Architecture

How telectl is put together, and why. The *why* matters more than the *what*
here: several of these choices look arbitrary until you know which failure they
prevent.

---

## Request flow

```
Telegram
   │  long poll (getUpdates)
   ▼
go-telegram/bot            update dispatch
   │
   ▼
internal/tg.RealBot        thin wrapper: SendText, SendRich, EditText, EditRich
   │
   ▼
internal/bot.Bot           routing, allowlist, rate limit, session
   ├─ handleMessage        ─────▶ handlers/*        typed commands
   ├─ handleCallbackQuery  ─────▶ dispatchCallback  inline buttons
   └─ handleInlineQuery    ─────▶ handlers/inline_query.go
                                        │
                                        ▼
                               internal/k8s.Client
                                        │
                                        ▼
                              Kubernetes API server
```

Menu buttons and typed commands converge on the same handlers. A button that
lists pods calls the same code `/get pods` does, so the two cannot drift.

---

## Packages

| Package | Responsibility |
|---|---|
| `cmd/telectl` | Entry point, signal handling, build metadata |
| `cmd/telectl/cmd` | Cobra CLI: flags, config load, subcommands |
| `internal/bot` | Update routing, dispatch, sessions, detail pane |
| `internal/handlers` | One file per command family |
| `internal/k8s` | client-go wrapper: the only package that talks to the cluster |
| `internal/menus` | Keyboard builders and the callback-data protocol |
| `internal/tg` | Telegram transport and the Rich Markdown builder |
| `internal/types` | `ResourceMap` (alias → GVR), session, rate limiter |
| `internal/config` | Viper configuration and startup validation |
| `internal/utils/formatters` | Rendering: tables, rich documents, status glyphs |
| `pkg/kubeconfig` | Kubeconfig parsing and safe writing |

`internal/k8s` is the only package that touches the cluster. Everything above it
works with `k8s.ResourceInfo`, a flattened view carrying name, namespace, kind,
labels, annotations, status and the raw unstructured object.

---

## Design decisions

### No kubectl subprocess

Every Kubernetes operation goes through client-go. Nothing in the codebase calls
`os/exec`, and `depguard` enforces that in CI:

```yaml
no-subprocess:
  deny:
    - pkg: "os/exec"
      desc: "telectl must not spawn processes; use client-go against the API server"
```

Consequences:

- The container image ships no kubectl.
- No shell quoting to get wrong. `/exec pod sh -c "rm -rf /"` runs inside the
  target container via the exec subresource; it is never interpreted by a shell
  on the host.
- Cordon is a strategic merge patch. Drain is a sequence of eviction-subresource
  calls. Rollout history is reconstructed from the ReplicaSets a Deployment owns
  — the same derivation `kubectl rollout history` performs, because there is no
  history API.

One caveat: if the operator's kubeconfig declares an `exec` credential plugin
(`kubelogin`, `gke-gcloud-auth-plugin`, `aws-iam-authenticator`), client-go
invokes it. That dependency comes from the kubeconfig, not from telectl.

### Callback data is a declared protocol

Telegram inline buttons carry an opaque `callback_data` string, capped at **64
bytes**. Exceeding it makes Telegram reject the **entire keyboard** with
`BUTTON_DATA_INVALID` — not the offending button, the whole thing. A pod list
would render as buttonless text with no error visible to the user.

Two mechanisms address this.

**A token store** (`menus/tokens.go`). Data over 64 bytes is replaced with
`menu:t:<hash>` and the original kept in memory. `MenuBuilder.btn` is the single
choke point every button goes through, so nothing can bypass it:

```go
func (mb *MenuBuilder) btn(text, data string) tg.InlineKeyboardButton {
    return tg.InlineButtonData(text, mb.tokens.Shorten(data))
}
```

Tokens are per-process. After a restart, an old button resolves to nothing and
the user is told to send `/start` rather than being met with silence.

**A field table** (`menus/menus.go`). The format is positional —
`menu:<type>:<f0>:<f1>:…` — and the mapping from position to struct field is
data:

```go
var callbackFields = map[string][]func(*CallbackAction, string){
    "action": {
        func(a *CallbackAction, v string) { a.Action = v },
        func(a *CallbackAction, v string) { a.ResourceType = v },
        func(a *CallbackAction, v string) { a.Namespace = v },
        func(a *CallbackAction, v string) { a.Name = v },
        ...
    },
}
```

This was previously a chain of `if len(parts) >= N` checks, and every bug in the
area was a field landing one slot off. A button that omitted its resource-type
field shifted `Name` into `Namespace` and left `Name` empty — and the
dispatcher's `if action.Name == "" { return }` guard then dropped the tap in
silence. All buttons are now built by one constructor:

```go
func actionData(verb, resourceType, namespace, name string, args ...string) string
```

### The verb set is enumerable

`bot/detail.go` routes detail-pane verbs through a map, not a switch:

```go
var detailVerbs = map[string]func(*Bot, context.Context, detailReq){...}
```

A switch would work identically at runtime. The map exists so a test can
enumerate the verbs and assert that **every button any keyboard renders has a
handler**, and conversely that every handler is reachable from some button. That
invariant's violation was the original sixteen dead buttons: rendered, never
dispatched, replying "that action is not available yet" — a defect
indistinguishable from a working button until someone tapped it.

### Rich Messages, with a fallback

Bot API 10.1 added Rich Messages, which render real tables natively instead of
monospace text in a code block. telectl builds Rich Markdown with
`tg.RichDoc` and sends it via `sendRichMessage`.

Every rich send is paired with a plain-text fallback:

```go
func (b *Bot) SendRich(chatID int64, markdown, fallback string) {
    if _, err := b.tgBot.SendRich(ctx, chatID, markdown, nil); err != nil {
        b.logger.Warn("Rich message rejected, falling back to text", ...)
        b.SendLongMessage(chatID, fallback)
    }
}
```

A server or client that rejects rich content degrades to readable text rather
than silence.

Wire-format note worth recording: the *receive* type `models.RichMessage`
carries a block tree, but the *send* type `models.InputRichMessage` takes a
markup string. An earlier implementation posted the receive shape and could
never have worked.

### Context switching is session-scoped

`SwitchContext` rebuilds the clientset, dynamic client and REST config, then
swaps them in **only after every step succeeds** — a failed switch leaves the
client working against the previous context.

It deliberately does not write `current-context` back to `~/.kube/config`. That
file is shared with kubectl and with everything else running as that user; a
chat message should not silently repoint someone's terminal at production. A
persistent variant exists (`SwitchContextPersistent`) but nothing in the chat
path calls it.

An earlier version rebuilt nothing: the clients were constructed once in
`NewClient`, so switching reported success while continuing to talk to the old
cluster until restart.

### Kubeconfig writes go through clientcmd

`kubeconfig.Save` must use `clientcmd.WriteToFile`, never `yaml.Marshal(kc.Raw)`.
`kc.Raw` is a `*clientcmdapi.Config` — the *internal* representation. Marshalling
it directly emits lowercased Go field names (`authinfos` instead of `users`),
maps keyed by name where v1 requires lists, and the internal-only
`locationoforigin` field. The result is unreadable by every Kubernetes tool,
including kubectl:

```
json: cannot unmarshal object into Go struct field Config.clusters
of type []v1.NamedCluster
```

This destroyed a real kubeconfig during development. `Save` now also refuses to
write when nothing is loaded, or when the config has no clusters and no
contexts, so a partially-initialised struct cannot overwrite a populated file.

### Cluster-scoped resources take no namespace

Passing a namespace when querying nodes, namespaces or PVs makes the API server
look for a namespaced variant that does not exist, and answer "the server could
not find the requested resource" — which reads like a broken cluster rather than
a bad query. `types.IsClusterScoped` is the single list, consulted by both the
menu and command paths. Three divergent copies existed before, and one of them
omitted `persistentvolumes`.

### Display width, not byte length

Tables are aligned by **display columns**, counting emoji and CJK as two. Two
related bugs came from ignoring this: `displayWidth` treating a status emoji as
one column shifted every following cell, and truncation slicing by byte cut
multi-byte runes in half, producing invalid UTF-8 that Telegram rejects outright.
Truncation is now rune-aware (`truncateToWidth`, `TruncateString`).

---

## Testing approach

Roughly 315 tests. Two properties they aim for:

**Each regression test fails when its fix is reverted.** Verified by hand for
the routing fix, the callback field shapes, the token store, `Save`'s format,
cordon's patch, the column-table aliasing, the log container plumbing, and the
age formatter. A test that passes against the bug is worse than no test — it
reports safety that is not there.

**Assertions target observable state, not chat text.** The detail-pane tests
assert on cluster state after the action (`IsNodeCordoned`,
`GetDeploymentReplicas`) and on the API requests actually issued (which
container a log request named), because the fake clientset returns identical
canned output regardless. An earlier version of the per-container log test
passed even with the container name dropped.

### Fakes

- **Telegram**: `internal/bot/routing_e2e_test.go` runs an `httptest` server
  that parses the `multipart/form-data` the library actually sends and records
  every call, including `reply_markup` structure. A `failMethod` hook drives the
  rich-message fallback path.
- **Kubernetes**: `k8s.NewClientForTest` accepts a `kubernetes.Interface` and a
  `dynamic.Interface`, so `k8sfake` and `dynfake` can back the client. The
  production constructor needs a reachable cluster and a kubeconfig on disk,
  which made the menu verbs untestable; `Client.clientset` is the interface
  rather than `*kubernetes.Clientset` for exactly this reason.
- **Scale subresource**: the fake object tracker has no notion of subresources —
  a GET on `deployments/scale` returns the Deployment, and client-go's generated
  fake then asserts it to `*autoscalingv1.Scale` and panics. `installScaleReactors`
  in the bot tests serves scale from `spec.replicas` the way a real API server
  does.

---

## Extending

### A new command

1. Add a handler in `internal/handlers/` implementing
   `Handle(ctx, msg, args, session) error`.
2. Register it in `Bot.registerHandlers`.
3. Add the name to `bot.allowed_commands` in `setDefaults` — a command absent
   from that list is rejected as "not allowed" even though its handler exists.
4. Add it to the help text in `handlers/help.go`.

### A new detail-pane verb

1. Add the handler to `detailVerbs` in `internal/bot/detail.go`.
2. Render a button for it in `menus.MenuBuilder.kindActionRows`, built with
   `act(verb)` so the callback shape stays correct.
3. Describe it in `formatters.RichHelpForResource` so the pane's ❓ Help
   explains it.

The reachability tests will fail if you add a button with no handler, or a
handler with no button.

### A new resource type

Add it to `types.ResourceMap` — alias, plural and short form. That single map
drives the command parsers, the menu builders and the inline queries. Add it to
`clusterScopedResources` too if it has no namespace.

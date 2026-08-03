# Menu-Driven Usage Guide

telectl can be driven entirely by tapping buttons — no command syntax required.
This guide covers the three interactive surfaces, how navigation works, and what
each button does.

Typed commands remain fully supported and are documented in the README. Menu
buttons and typed commands run the *same* handlers, so behaviour is identical
either way.

---

## The three surfaces

Telegram gives a bot three distinct UI affordances. telectl uses all three, and
they behave differently — knowing which is which saves confusion.

| Surface | Where it appears | What it does |
|---|---|---|
| **Command menu** | `/` button next to the message box | Telegram's own command list. Tapping inserts and sends a command. |
| **Reply keyboard** | Replaces your phone keyboard, bottom of screen | Persistent shortcut bar. Tapping *sends a text message* with that label. |
| **Inline keyboard** | Buttons attached under a bot message | Real navigation. Tapping edits the message in place. |

The inline keyboard is the main interface. The reply keyboard is a shortcut bar
for jumping between top-level sections.

### Enabling / disabling

Both keyboards are controlled from config (`config.yaml`):

```yaml
bot:
  enable_menu_button: true      # the "/" command menu
  enable_reply_keyboard: true   # the persistent bottom bar
  menu_page_size: 10            # items per page in resource lists
```

With `enable_reply_keyboard: false`, the main menu sends a single message with
inline buttons only. Everything remains reachable.

---

## Getting started

Send `/start`. You get two messages:

1. A status line showing your current **cluster context** and **namespace**,
   with the persistent bottom bar attached.
2. A `Choose a section:` message carrying the main inline menu.

```
🤖 telectl

Cluster: minikube
Namespace: default

Choose an action below, or use /help for the full command reference.
```

```
┌──────────────┬──────────────┐
│ 📦 Resources │ 📊 Monitor   │
├──────────────┼──────────────┤
│ 🔧 Operations│ ⚙️ Settings  │
├──────────────┴──────────────┤
│          ❓ Help            │
└─────────────────────────────┘
```

Send `/start` again at any point to get back here.

---

## Navigation model

Inline buttons **edit the current message** rather than posting a new one. The
menu behaves like a single pane that swaps content, so the chat does not fill up
as you browse.

Consequences worth knowing:

- Scrolling up to an **older** menu message and tapping its buttons still works.
  That message updates in place, not the newest one.
- Results that are *content* rather than *navigation* — a resource description,
  a log dump, an error — arrive as **new** messages, so they persist in history.
- Every menu screen has a route back: `🔙`, `🏠 Main`, or `🔙 Main Menu`.

---

## 📦 Resources

`📦 Resources` opens the resource type picker:

```
┌────────────┬─────────────────┬──────────────┐
│ 📦 Pods    │ 🚀 Deployments  │ 🔌 Services  │
├────────────┼─────────────────┼──────────────┤
│ 📋 ReplicaSets │ 📁 Namespaces │ 🖥️ Nodes   │
├────────────┼─────────────────┼──────────────┤
│ ⚙️ ConfigMaps │ 🔐 Secrets   │ 💾 PVCs      │
├────────────┼─────────────────┼──────────────┤
│ 🌐 Ingresses │ 📅 Events     │ 💾 PVs       │
├────────────┴─────────────────┴──────────────┤
│              🔙 Main Menu                    │
└──────────────────────────────────────────────┘
```

Picking a type lists objects of that type in your **current namespace**.
Cluster-scoped types (nodes, namespaces, PVs) ignore the namespace and always
list cluster-wide.

### Reading a resource list

Each item is a button prefixed with a status colour:

| Icon | Meaning |
|---|---|
| 🟢 | Running / Active / Ready |
| 🟡 | Pending / Creating |
| 🔴 | Failed / Error / CrashLoopBackOff |
| 🔵 | Succeeded / Completed |
| 🟠 | Terminating |
| ⚪ | Unknown |

Names longer than 20 characters are truncated with `...` — Telegram caps button
label width.

### Switching namespace

Resource lists show the active namespace as a button in the bottom row
(`🌐 default`, or `🌐 All NS`). Tap it to open the namespace picker:

```
┌─────────────────────────────┐
│  ✅ 🌐 All Namespaces        │
├───────────┬────────┬────────┤
│ ✅ default│ kube-…  │ prod   │
├───────────┴────────┴────────┤
│ ⬅️ Prev │ 📄 1/3 │ Next ➡️  │
├─────────────────────────────┤
│           🔙 Back            │
└─────────────────────────────┘
```

Picking a namespace re-runs the current list in that namespace. The choice is
stored in your session, so every later command and menu uses it until you change
it again. `🌐 All Namespaces` clears the filter and lists cluster-wide.

The same picker is available from `⚙️ Settings → 🌐 Namespace` when you are not
already browsing a list.

Cluster-scoped kinds (nodes, namespaces, PVs) ignore this setting — they always
list cluster-wide, because a namespace filter is meaningless for them.

### Pagination and list actions

The bottom rows of a list are controls, not items:

```
┌─────────┬──────────┬──────────┐
│ ⬅️ Prev │  📄 2/5  │ Next ➡️  │
├─────────┼──────────┼──────────┤
│ 🔄 Refresh │ 🌐 default │ 🔙 Types │
└─────────┴──────────┴──────────┘
```

- **⬅️ Prev / Next ➡️** — page through. `📄 2/5` is a label, not a button.
- **🔄 Refresh** — re-query the cluster. Use after a restart or scale.
- **🌐 <namespace>** — open the namespace picker (above).
- **🔙 Types** — back to the type picker.

Page size comes from `bot.menu_page_size` (default 10).

### Per-resource actions

Tapping a resource in the list opens a **detail pane** in the same message: a
compact summary (kind, status, age, and a few facts that answer the first
question you ask) plus the action keyboard for that kind. It no longer dumps a
full describe — that is now one tap away behind **📝 Describe**.

Every kind gets the inspection row:

| Button | Effect |
|---|---|
| 📝 Describe | Full detail (spec, status, labels, annotations), as a new message |
| 🏷️ Labels | Labels and annotations on their own |
| 📅 Events | Recent events naming this object |
| 🗑️ Delete | Opens a confirmation prompt — never deletes immediately |

Type-specific actions:

| Type | Actions |
|---|---|
| **Pods** | 📋 Logs · 🖥️ Exec · 🔌 Forward · (per-container log buttons when a pod has >1 container) · 🖥️ Node (jump to the node it runs on) |
| **Deployments** | 🔄 Restart · 📈 Scale · 📋 Pods · 🎯 Selector · 📜 History · 📄 YAML |
| **ReplicaSets** | 📋 Pods · 📈 Scale · 🎯 Selector |
| **Services** | 📋 Endpoints · 🎯 Selector · 🔌 Forward |
| **Nodes** | 📊 Top · 📋 Pods · 🔧 Cordon / 🔓 Uncordon (whichever changes something) · 💤 Drain (confirmed) |
| **Namespaces** | 📋 Resources (object counts per kind) · 🌐 Switch to |
| **Every pane** | ❓ Help — explains each button for the current kind |

Details on the notable verbs:

- **📋 Pods** (deployment/ReplicaSet) — resolves the live selector and lists
  the pods it matches, each still tappable. **📋 Pods** on a node lists
  everything scheduled there across all namespaces.
- **🎯 Selector** — shows the selector and the pods it currently matches, so
  you can tell a wrong selector from missing pods at a glance.
- **📋 Endpoints** — a service's backing addresses, split into **ready** and
  **not ready**. A service with only not-ready endpoints looks healthy in a
  list but serves no traffic.
- **📜 History** — the deployment's revisions, reconstructed from the
  ReplicaSets it owns (the same way `kubectl rollout history` does it), newest
  first, with the current revision marked ✅.
- **📄 YAML** — the live manifest as YAML, read-only. There is no apply-from-
  chat: writing a manifest back could silently overwrite a change someone made
  seconds earlier, with no diff and no lock.
- **📊 Top** (node) — CPU/memory from metrics-server. Without metrics-server,
  you get an explanation, not a raw 404.
- **🔧 Cordon / 🔓 Uncordon** — only the button that *changes* the node's
  state is shown: a cordoned node offers Uncordon, an open node offers Cordon.
- **💤 Drain** — asks for confirmation first, then cordons and evicts. Pods
  owned by DaemonSets and static/mirror pods are left in place; evictions
  respect PodDisruptionBudgets, so refused evictions are reported, not
  escalated to a delete. The node stays cordoned if anything failed.
- **📋 Logs** (pod) — opens a chooser: last 50 / 100 / 500 lines, 👁️ Follow
  (a fresh 200-line snapshot — live streaming is not practical in chat), or
  ⏮️ Previous (the previous container instance; usually empty until a restart).
- **📈 Scale** — quick counts (0, 1, 2, 3, 5, 10) with the current count
  marked, plus a Custom button that shows the `/scale` command form.
  ReplicaSet scaling warns that a Deployment owner will revert the change.

Every mutating verb (cordon, drain, scale, delete) reports what it did, notes
when it was a dry run, and re-renders the detail pane so the keyboard reflects
the new state.

### "Not yet wired" — nothing left

Every button rendered by the keyboard builders now has a dispatcher branch, and
a test enforces it: it walks every keyboard the menu builder can produce,
collects the action verb from each button's callback data, and fails if any verb
has no handler. Removing a handler fails that test by name.

If you ever tap a button and see **"⚠️ That action is not available yet."**,
that is a bug — please report it.

### Deletion is always confirmed

`🗑️ Delete` never acts on the first tap. It replaces the pane with:

```
⚠️ Delete pods my-pod in default?

This cannot be undone.

┌────────────────┬───────────┐
│ ✅ Yes, Delete │ ❌ Cancel │
└────────────────┴───────────┘
```

`❌ Cancel` returns to the resource view. `✅ Yes, Delete` performs the delete,
reports the result, and refreshes the list.

With `kubernetes.dry_run: true` (or `--dry-run`), the confirm button reports
`🧪 Dry run: would delete ...` and changes nothing.

---

## 📊 Monitor

| Button | Equivalent command |
|---|---|
| 📊 Top Pods | `/top pods` |
| 🖥️ Top Nodes | `/top nodes` |
| 📅 Events | `/events` |
| 👁️ Watch | shows `/watch` usage |

`Top` requires **metrics-server** in the cluster; without it you get an error
from the API rather than a table.

---

## 🔧 Operations

Restart and scale need a target name, which buttons can't supply on their own,
so these buttons reply with the exact command to copy:

| Button | Reply |
|---|---|
| 🔄 Restart Deployment | `/restart deployment <name> [-n namespace]` |
| 📈 Scale Deployment | `/scale deployment <name> <replicas> [-n namespace]` |
| 🗑️ Delete Resource | Points you at 📦 Resources, where delete is confirmed |
| ✏️ Edit Resource | Not implemented yet |

To restart **without typing a name**, go through
`📦 Resources → Deployments → <deployment> → 🔄 Restart`. That path is fully
wired and needs no typing.

`📈 Scale` from a deployment's detail view currently replies with the exact
`/scale` command pre-filled with that deployment's name and namespace — the
quick-preset buttons (0, 1, 2, 3, 5, 10) exist in the keyboard builder but their
dispatch (`scaleset`) is not wired yet.

---

## ⚙️ Settings

| Button | Effect |
|---|---|
| ⚙️ Context | Lists kubeconfig contexts as buttons — tap one to switch |
| 🌐 Namespace | Opens the namespace picker (see Switching namespace) |
| 🎨 Theme / 🔔 Notifications | Placeholders, not implemented |

Context switching applies immediately and is reflected in the main menu header.

**Scope:** switching context rebuilds the bot's own connection only. It does
**not** rewrite `~/.kube/config`, because that file is shared with `kubectl` and
with every other user of the bot — a chat message should not repoint another
person's tooling. Your `kubectl` current-context is unaffected.

---

## The persistent bottom bar

When `enable_reply_keyboard` is on:

```
┌──────────────┬───────────┬──────────┐
│ 📦 Resources │ 📋 Logs   │ 🖥️ Exec  │
├──────────────┼───────────┼──────────┤
│🔌 Port Forward│⚙️ Contexts│📊 Monitor│
├──────────────┼───────────┼──────────┤
│ 🔧 Operations│ ⚙️ Settings│ ❓ Help  │
└──────────────┴───────────┴──────────┘
```

These send their label as a normal message, which the bot recognises. Both the
emoji label and the bare word work — `📦 Resources` and `resources` are
equivalent, so you can type instead of tap.

`📋 Logs`, `🖥️ Exec`, and `🔌 Port Forward` reply with usage text, because each
needs a pod name. For logs without typing, use
`📦 Resources → Pods → <pod> → 📋 Logs`.

---

## Rich Messages

Output that used to be a monospace code-block table is now sent with
`sendRichMessage` (Bot API 10.1+), so Telegram draws a **native table** with real
columns, borders and horizontal scrolling instead of pre-formatted text.

What became rich:

| Surface | Rendering |
|---|---|
| `/get <type>` | Native table, resource-specific columns, status dots |
| `/get <type> <name>`, `/describe` | Summary table + collapsible Labels / Annotations / Details |
| `/top pods`, `/top nodes` | Metrics table with CPU and memory columns |
| `/events` | Table with type emoji, reason, object, message, age |
| `/contexts` | Table marking the active context, with switch buttons |
| `/config`, `/version` | Key/value table |
| `/logs`, `/exec` | Real code block with a copy button |
| Menu → Resources → *type* | Table above the buttons for the current page |

`-o json`, `-o yaml` and `-o name` stay plain text — they are machine-readable
formats you asked for verbatim.

### Fallback

If the server rejects a rich message — older Bot API, or rich messages not
available for that chat — the bot automatically re-sends the previous plain-text
rendering. You still get an answer; the downgrade is recorded in the log:

```
WARN  Rich message rejected, falling back to text  {"error": "..."}
```

Inline keyboards are preserved across the fallback, so buttons keep working
either way.

### A note on escaping

Resource names, labels and event messages are free-form and routinely contain
`|`, `*`, `_` and backticks. All cluster-supplied text is escaped before it
reaches the table, so a pod named `weird|name` cannot split a row into extra
columns.

---

Each user gets an independent session holding current namespace, current
context, and menu position. Your namespace changes never affect another user.

Access is gated by `telegram.allowed_user_ids` (or `--allowed-users`). If the
list is **empty, every user is allowed** — always set it for a bot reachable by
others. Unauthorized users are refused for both typed commands and button taps.

Rate limiting (`bot.rate_limit`, default 30/min) applies per user.

---

## Troubleshooting

**Buttons don't appear, only text.**
The keyboard failed to send. Restart with `--log-level=debug` and look for
`Failed to send ...`. Verify `bot.enable_reply_keyboard` / `enable_menu_button`.

**Tapping a button does nothing.**
Debug logs show `Dispatching callback` with the callback data for every tap. No
line means the tap never arrived; `Unparseable callback data` means the button's
payload is malformed.

**"That menu is from an earlier session."**
Long callback payloads are stored server-side and the button carries a short
token, because Telegram rejects any keyboard containing `callback_data` over 64
bytes. Tokens live in memory, so buttons from before a restart no longer resolve.
Send `/start` for a fresh menu.

**`BUTTON_DATA_INVALID` in the logs.**
A generated button exceeded the 64-byte limit, and Telegram rejects the *entire*
keyboard rather than one button — so the list renders with no buttons at all. All
generated buttons now route through the token store; if you see this again, a new
keyboard builder is bypassing `MenuBuilder.btn`.

**A command reports "not allowed".**
`bot.allowed_commands` is gating it. An empty list allows everything; otherwise
the command must be listed. Note the key is `portforward`, not `port-forward`.

**Menu shows `Cluster: unknown`.**
The kubeconfig context could not be read. Check `kubernetes.kubeconfig_path` and
that `/contexts` lists what you expect.

**Nothing in the logs at all.**
Pass `--log-level=debug`. Every inbound message logs `Received message` and
every tap logs `Dispatching callback`.

---

## Command ↔ menu equivalence

| Menu path | Command |
|---|---|
| 📦 Resources → Pods | `/get pods` |
| 📦 Resources → Pods → *pod* → 📝 Describe | `/describe pods <name>` |
| 📦 Resources → Pods → *pod* → 📋 Logs | `/logs <pod>` |
| 📦 Resources → Deployments → *deploy* → 🔄 Restart | `/restart deployment <name>` |
| 📊 Monitor → Top Pods | `/top pods` |
| 📊 Monitor → Events | `/events` |
| ⚙️ Settings → Context | `/contexts` |
| ❓ Help | `/help` |

Both paths call the same handler, so output and permissions are identical.

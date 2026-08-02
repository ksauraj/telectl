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

Tapping a resource in the list runs **Describe** on it directly, posting the
full detail as a new message. The detail keyboard adds:

| Button | Effect |
|---|---|
| 📝 Describe | Full detail, as a new message |
| 🗑️ Delete | Opens a confirmation prompt — never deletes immediately |

Type-specific actions:

| Type | Actions |
|---|---|
| **Pods** | 📋 Logs · 🖥️ Exec · 🔌 Port Forward |
| **Deployments** | 🔄 Restart · 📈 Scale |
| **Services** | 🔌 Port Forward |
| **Nodes** | *(see "Not yet wired" below)* |
| **Namespaces** | 🗑️ Delete |
| **ReplicaSets** | *(see "Not yet wired" below)* |

### Not yet wired

Several buttons are rendered by the keyboard builders but have no dispatcher
branch yet. Tapping one replies **"⚠️ That action is not available yet."**
rather than doing nothing silently:

`cordon` · `uncordon` · `drain` · `nodepods` · `top` (from a node) ·
`endpoints` · `history` · `edit` · `pods` (from a deployment) · `rspods` ·
`rsscale` · `scaleset` · `scalecustom` · `logsfollow` · `logsprevious` ·
`nsresources`

The equivalent typed commands still work where one exists — e.g. `/top nodes`
for node metrics, `/scale deployment <name> <n>` for an exact replica count.

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

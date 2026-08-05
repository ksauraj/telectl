---
title: Menu Navigation
nav_order: 5
---

# Menu Navigation

telectl's menus are **single-pane navigation**: every menu lives in one
message that edits itself in place as you tap buttons. No message spam, no
back-stack confusion — the current view and its buttons are always the
message in front of you.

## The two keyboards

| Keyboard | Where | What it does |
|----------|-------|--------------|
| **Reply keyboard** (bottom bar) | persistent | Quick entry to each section |
| **Inline keyboard** (buttons under the message) | inside the pane | Navigate the current view |

The reply keyboard has: **Resources**, **Logs**, **Exec**, **Port Forward**,
**Contexts**, **Monitor**, **Operations**, **Settings**, **Help**.

The bot menu (☰ button, top-left) mirrors the main commands.

## Navigation flow

```
Main pane
 ├─ Resources ── Resource type ── List (paged) ── Resource detail ── actions
 ├─ Logs       ── pick pod ── pick container ── tail picker (50/100/500)
 ├─ Exec       ── pick pod ── confirm command
 ├─ Port Forward
 ├─ Contexts   ── context picker
 ├─ Monitor    ── Top (pods/nodes) · Events · Watch
 ├─ Operations ── Restart · Scale · Delete · Edit
 └─ Settings   ── Namespace · Context · Theme* · Notifications*
     (* not implemented yet — shows "Not available yet")
```

## Resources (browse)

1. Tap **Resources** → pick a resource type: Pods, Deployments, Services,
   ReplicaSets, Namespaces, Nodes, ConfigMaps, Secrets, PVCs, PVs, Ingresses,
   Events.
2. A **paged list** of objects (namespace picker above, `menu_page_size`
   items per page).
3. Tap an object → **detail pane**: compact summary + action buttons.

The detail pane's actions depend on the kind:

| Kind | Actions |
|------|---------|
| Pod | Describe, Labels, Events, Logs (per container), Exec, Port Forward, Delete |
| Deployment/RS | Describe, Labels, Events, Logs, Restart, Scale, Delete |
| Node | Describe, Top, Cordon/Uncordon, Drain |
| Other | Describe, Labels, Events, Delete |

Mutating actions are gated:

- **Delete** asks *Yes, delete / Cancel* first.
- **Drain** asks *Yes, drain* and explains the node will be cordoned and its
  pods evicted.
- **Scale** opens a chooser with quick replicas (0/1/3/5…) plus a custom
  value.

## Monitor

Top (pods/nodes usage), Events (recent), Watch (usage hint — runs as a typed
command because it streams).

## Operations

Signposts to the typed commands — the operations menu has no selected
resource, so each button shows the `/command` syntax to use.

## Settings

Shows the current **Context** and **Namespace**; buttons to switch either.
The context picker lists every kubeconfig context as a button; switching
rebuilds the bot's clients **in-process** without touching
`~/.kube/config`.

## What the buttons mean (glyphs)

Buttons carry small glyph markers that convey meaning at a glance:

| Glyph | Meaning |
|-------|---------|
| ▶ action | navigates / performs an action |
| ◀ back | one level up |
| ⌂ home | back to the main pane |
| ✕ destructive | delete/drain (gated behind confirmation) |
| ✓ selected | the current pick (e.g. the active namespace) |

## Namespace scoping

- The namespace switcher appears in list views; **All namespaces** is an
  option. Cluster-scoped resources (nodes, PVs, namespaces) always list
  cluster-wide regardless of the picker.
- The chosen namespace is **session-scoped**: it persists for you until you
  change it (per-chat, not per-user across chats).

## Tips

- Prefer **tap navigation** over typing — the pane re-renders in place.
- The pane is capped in size; if a list is long, use the pager, or `/get`
  with `-o wide`/filters for a full view.
- `message is not modified` errors are expected (idempotent taps) and are
  logged at debug level, not surfaced.
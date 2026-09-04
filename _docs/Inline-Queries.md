---
title: Inline Queries
nav_order: 7
---

# Inline Queries

telectl supports **inline mode**: type `@telectl <query>` in *any* chat to
look up a resource without opening the bot. Results render as tappable cards
that insert a summary into the chat — handy for pasting a pod's status or a
service's endpoint into a conversation.

## How it works

1. In any Telegram chat, type:
   ```
   @telectl pods
   ```
2. Telegram calls the bot's inline handler with the query.
3. telectl lists matching resources as inline results; tap one to send it.

The first token is the **resource type**, optionally followed by a name and/or
namespace:

```
@telectl pods
@telectl deployment frontend
@telectl pods -n production
@telectl service my-svc
```

## Query grammar

`<resource> [name] [-n <namespace>]` (same resource aliases as `/get`).

- **No resource** → the bot returns an inline help card.
- **Unknown resource** → inline help suggesting valid types.
- **With a name** → a single-resource result summarizing that object.
- **Without a name** → one result per matching resource in the namespace
  (or all, when `-n` is given and applies).

## Example results

- `@telectl pods -n production`
  → one card per Pod in `production`, titled with its name and namespace.
- `@telectl deployment frontend`
  → a card for that Deployment with its desired/ready replicas.

## Notes & limits

- Results are capped at Telegram's inline limit (max ~50 results).
- A result that can't be built (e.g. resource not found) returns a clear
  error card rather than failing silently.
- **Permissions are the same as the bot** — inline queries use the same
  impersonated client as the normal commands, so RBAC still applies. A
  read-only user gets read-only inline cards.
- Inline mode must be enabled for the bot in @BotFather
  (`/setinline`). It is controlled by the same
  [allowed users](Configuration-Reference#telegram) allow-list.

## Relationship to menu/typed commands

| Entry point | When to use |
|-------------|-------------|
| `/get`, `/describe` | deep inspection in the workspace |
| Menu / **Resources** | browsing and mutating |
| **Inline `@telectl …`** | a quick look-up dropped into any chat |

Inline queries are read-only lookups — they don't mutate anything.
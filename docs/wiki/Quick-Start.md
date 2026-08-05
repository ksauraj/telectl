# Quick Start

Get telectl running in five minutes — from nothing to your first
`/resources` view in Telegram.

## What you need

- A Kubernetes cluster you can reach (`kubectl cluster-info` works).
- A Telegram bot token from [@BotFather](https://t.me/BotFather).
- Your Telegram user ID (ask [@userinfobot](https://t.me/userinfobot)).
- The `telectl` binary (or a container image).

## 1. Create a bot with BotFather

1. Open [@BotFather](https://t.me/BotFather), send `/newbot`, pick a name and
   username.
2. Copy the token it gives you — it looks like `123456789:AA...`.

## 2. Get your user ID

Message [@userinfobot](https://t.me/userinfobot). It replies with your numeric
ID. **You must put this ID in the config** — without it, the bot will refuse
to talk to you.

## 3. Write a config file

```yaml
# ~/.config/telectl/telectl.yaml
telegram:
  bot_token: "YOUR_BOT_TOKEN"
  allowed_user_ids: [YOUR_USER_ID]   # e.g. [YOUR_ADMIN_TELEGRAM_ID]

kubernetes:
  kubeconfig_path: "~/.kube/config"  # optional; defaults to $KUBECONFIG
```

The bot searches config in this order: `~/.config/telectl/telectl.yaml`,
`~/.config/telectl.yaml`, `/etc/telectl/telectl.yaml`, or an explicit
`--config path`. See [Configuration Reference](Configuration-Reference).

## 4. Run it

```bash
telectl
```

You should see:

```
INFO  Starting telectl   {"version": "0.1.0-beta.0", "commit": "...", "dry_run": false}
```

> If you see `No allowed users configured`, every Telegram user who finds the
> bot can operate your cluster. Fix `allowed_user_ids` before going further.

## 5. Open the chat

1. Find your bot on Telegram and press **Start**.
2. Tap the **☰ menu** button (or send `/resources`).
3. Pick a resource type — **Pods** — and browse.

Try these next:

| Command | What it does |
|---------|--------------|
| `/get pods -A` | list pods in every namespace |
| `/logs <pod> --tail 50` | last 50 log lines of a pod |
| `/top pods` | pod CPU/memory (needs metrics-server) |
| `/contexts` | switch cluster from the kubeconfig |
| `/config` | show the effective config (redacted) |

## Securing it (don't skip)

- The bot holds **your** kubeconfig permissions. A leaked bot token is a
  leaked cluster credential — keep `bot_token` out of version control.
- If multiple people use the bot, enable per-user RBAC via
  [Impersonation & RBAC](Impersonation-and-RBAC) so each user only gets
  their own role.
- For production hardening see the [Production Checklist](Production-Checklist).

## Next steps

- [Installation Guide](Installation-Guide) — binaries, Docker, Helm
- [Configuration Reference](Configuration-Reference) — every option
- [Command Reference](Command-Reference) — all typed commands
- [Menu Navigation](Menu-Navigation) — using the interactive menus
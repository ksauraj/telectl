# Configuration Reference

Every configuration key telectl understands, where the config file lives,
and how environment overrides work. This page is generated from the config
schema in `internal/config/config.go` — if a key isn't here, the bot doesn't
read it.

## Config file discovery

telectl searches these locations, in order (first match wins):

1. An explicit path from `--config /path/to/telectl.yaml`
2. `~/.config/telectl/telectl.yaml`
3. `~/.config/telectl.yaml`
4. `/etc/telectl/telectl.yaml`

> **Deliberate design choice:** the working directory and `$HOME` are *not*
> searched. `~/.kube/config` and stray YAML files in the home directory would
> be misparsed as bot config, producing confusing errors like
> `yaml: control characters are not allowed` (kubeconfigs contain binary cert
> data). A binary named `telectl` in the repo root would also collide with the
> config name.

A complete annotated example lives in the repo as
[`config.yaml.example`](https://github.com/ksauraj/telectl/blob/main/config.yaml.example).

## Environment overrides

Every key can be overridden by an environment variable: uppercase the key
path, replace `.` with `_`, prefix with `TELECTL_`:

```
bot.rate_limit      ->  TELECTL_BOT_RATE_LIMIT
kubernetes.dry_run  ->  TELECTL_KUBERNETES_DRY_RUN
logging.level       ->  TELECTL_LOGGING_LEVEL
```

Credentials keep their **conventional unprefixed names**:

```
telegram.bot_token  ->  TELEGRAM_BOT_TOKEN
kubeconfig path     ->  KUBECONFIG
allowed user ids    ->  ALLOWED_USER_IDS   (comma-separated)
```

CLI flags outrank config file and environment (see
[CLI flags](#cli-flags) below).

---

## `telegram`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `bot_token` | string | `""` | Bot token from @BotFather. **Required.** Env: `TELEGRAM_BOT_TOKEN`. |
| `allowed_user_ids` | []int64 | `[]` | Telegram user IDs permitted to use the bot. **Empty = everyone who finds the bot can operate the cluster.** Env: `ALLOWED_USER_IDS`. |
| `admin_user_ids` | []int64 | `[]` | Reserved for privileged operations (display/UX only — real authority is RBAC). Env: `ADMIN_USER_IDS`. |
| `parse_mode` | string | `"MarkdownV2"` | Message parse mode: `MarkdownV2`, `HTML`, or `""` for plain text. |
| `webhook_url` | string | `""` | **Read but unused** — webhook mode is not implemented; the bot uses long polling. |
| `webhook_port` | int | `8443` | **Read but unused** — same as above. |

## `kubernetes`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `kubeconfig_path` | string | `""` | Path to kubeconfig. Empty → `$KUBECONFIG`, then `~/.kube/config`. Env: `KUBECONFIG`. |
| `default_namespace` | string | `"default"` | Namespace used when a command doesn't pass `-n`/`--namespace`. |
| `context` | string | `""` | Kubeconfig context to use. Empty → the kubeconfig's current context. |
| `timeout` | int | `30` | API request timeout in seconds. |
| `dry_run` | bool | `false` | Log mutating operations instead of performing them. Replies say so explicitly. |
| `impersonate_user` | string | `""` | **Read but unused** — use the `impersonation:` block instead. |
| `impersonate_groups` | []string | `[]` | **Read but unused** — same as above. |
| `burst` | int | `10` | client-go request burst (rate limiting). |
| `qps` | float | `5.0` | client-go requests per second. |
| `cluster_name` | string | `""` | Display name shown in `/config`, `/version`, main menu. Empty → kubeconfig context name. |

## `impersonation`

Per-user Kubernetes RBAC. When enabled, the bot impersonates a K8s identity
mapped from the Telegram user ID — **Kubernetes RBAC, not the bot, decides
who may do what.**

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | bool | `false` | Enable impersonation. |
| `default_user` | string | `""` | Identity to impersonate for unmapped users. e.g. `system:serviceaccount:default:readonly`. |
| `default_groups` | []string | `[]` | Groups to carry for unmapped users. e.g. `[viewers]`. |
| `user_mapping` | map | `{}` | `telegram_user_id → {user, groups}`. |

```yaml
impersonation:
  enabled: true
  default_user: "system:serviceaccount:default:readonly"
  default_groups: ["viewers"]
  user_mapping:
    "YOUR_ADMIN_TELEGRAM_ID":
      user: "admin-user"
      groups: ["system:masters"]
    "YOUR_READONLY_TELEGRAM_ID":
      user: "readonly-user"
      groups: ["viewers"]
```

> ⚠️ The mapped `groups` must match actual ClusterRoleBindings, or the
> impersonated identity has no permissions. See
> [Impersonation & RBAC](Impersonation-and-RBAC).

## `logging`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `level` | string | `"info"` | `debug`, `info`, `warn`, `error`. |
| `format` | string | `"json"` | `json` or `console`. |
| `output` | string | `"stdout"` | `stdout` or `stderr`. |

## `bot`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `max_message_length` | int | `4096` | Telegram's message limit. |
| `command_prefix` | string | `"/"` | Prefix for typed commands. |
| `enable_markdown` | bool | `true` | Render rich formatting. |
| `rate_limit` | int | `30` | Per-user, per-minute message throttle. |
| `allowed_commands` | []string | (all) | Commands the bot dispatches. **Must stay in sync with registered handlers** — a command missing here is rejected as "not allowed" even though its handler exists. |
| `enable_menu_button` | bool | `true` | Register the bot menu (☰) commands. |
| `enable_reply_keyboard` | bool | `true` | Persistent reply keyboard under the input box. |
| `menu_page_size` | int | `10` | Items per page in list views. |

---

## CLI flags

```
telectl [flags]

Flags:
      --config string        Config file (searches $HOME/.config/telectl/, ...)
      --token string         Telegram bot token (or TELEGRAM_BOT_TOKEN)
      --kubeconfig string    Path to kubeconfig (or KUBECONFIG)
      --allowed-users string Comma-separated Telegram user IDs
      --log-level string     debug | info | warn | error   (default "info")
      --dry-run              Log mutations instead of performing them
```

Subcommands:

```
telectl                # run the bot (default)
telectl version        # print version + commit + build date
telectl config         # print effective configuration (redacted) and exit
telectl contexts       # list kubeconfig contexts and exit
```

Flags outrank config file and environment.

## Validating your config

```bash
telectl config --config /path/to/telectl.yaml
```

Prints the effective, merged configuration (secrets redacted) so you can see
exactly what the bot will use. Also run the bot with `--log-level debug` to
see which config file was loaded at startup.
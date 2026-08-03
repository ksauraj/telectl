# Configuration

telectl reads configuration from three sources. Later sources win:

1. **Defaults** compiled into the binary
2. **Config file** — `telectl.yaml`, found by search or named with `--config`
3. **Environment variables**
4. **Command-line flags**

So a `--token` flag beats `TELEGRAM_BOT_TOKEN`, which beats `telegram.bot_token`
in the file, which beats the built-in default.

---

## The config file

### Where telectl looks

With no `--config` flag, these directories are searched in order for a file
named `telectl` with a recognised config extension:

1. `$HOME/.config/telectl/`
2. `$HOME/.config/`
3. `/etc/telectl/`

The first match wins. `--config /path/to/file.yaml` skips the search entirely.

Recognised extensions: `.yaml`, `.yml`, `.json`, `.toml`, `.ini`, `.env`,
`.properties`, `.hcl`, `.tfvars`.

### What is deliberately *not* searched

**`$HOME` itself.** Operators keep unrelated YAML there, most notably
`~/.kube/config`. Viper would happily try to parse it as bot config and fail
with `yaml: control characters are not allowed` — the embedded certificate data
is binary. The error names neither file, so it reads as a telectl bug.

**The current working directory.** `make build` puts a binary named `telectl` in
the repo root, which collides with the config base name. Viper would find the
executable and try to parse it as YAML.

Both cases are additionally guarded: if a config read fails, telectl inspects
what it actually opened and says so rather than passing the opaque parse error
through:

```
refusing to use "/home/you/.kube/config" as bot config: it looks like a
kubeconfig file. Pass --config /path/to/telectl.yaml explicitly

refusing to use "/home/you/telectl" as bot config: not a recognized config
file (ext=""). Pass --config /path/to/telectl.yaml explicitly
```

### Minimal file

```yaml
telegram:
  bot_token: "123456:ABC-DEF..."
  allowed_user_ids: [123456789]
```

### Full file

Every key with its default. Omit anything you do not need.

```yaml
telegram:
  # Required. From @BotFather.
  bot_token: ""

  # Telegram user IDs permitted to use the bot. An empty list allows
  # EVERYONE who can find the bot — see Security below.
  allowed_user_ids: []

  # Reserved for privileged operations.
  admin_user_ids: []

  # Default parse mode for outgoing messages.
  parse_mode: "MarkdownV2"

  # Webhook mode is not yet wired; telectl uses long polling.
  webhook_url: ""
  webhook_port: 8443

kubernetes:
  # Defaults to $KUBECONFIG, then ~/.kube/config.
  kubeconfig_path: ""

  # Context to use at startup. Empty means the kubeconfig's current-context.
  context: ""

  # Namespace used when a command omits -n.
  default_namespace: "default"

  # Log mutating operations instead of performing them.
  dry_run: false

  # API request timeout, in seconds.
  timeout: 30

  # client-go rate limiting against the API server.
  burst: 10
  qps: 5.0

  # Not yet wired.
  impersonate_user: ""
  impersonate_groups: []

logging:
  level: "info"     # debug, info, warn, error
  format: "json"    # json or console
  output: "stdout"

bot:
  # Telegram's hard cap. Longer output is split across messages.
  max_message_length: 4096

  command_prefix: "/"
  enable_markdown: true

  # Requests per user per minute.
  rate_limit: 30

  # Interactive surfaces.
  enable_menu_button: true
  enable_reply_keyboard: true

  # Resources per page in list views.
  menu_page_size: 10

  # Commands the bot will dispatch. A handler exists for each of these;
  # removing one disables that command without removing its code.
  allowed_commands:
    - start
    - help
    - version
    - get
    - describe
    - logs
    - exec
    - portforward
    - contexts
    - use-context
    - config
    - top
    - events
    - watch
    - restart
    - scale
    - resources
    - monitor
    - operations
    - settings
```

---

## Environment variables

### Credentials keep conventional names

These are read unprefixed, because they are the names the surrounding ecosystem
already uses:

| Variable | Maps to |
|---|---|
| `TELEGRAM_BOT_TOKEN` | `telegram.bot_token` |
| `KUBECONFIG` | `kubernetes.kubeconfig_path` |
| `ALLOWED_USER_IDS` | `telegram.allowed_user_ids` |
| `ADMIN_USER_IDS` | `telegram.admin_user_ids` |

The two ID lists are comma-separated:

```bash
export ALLOWED_USER_IDS="123456789,987654321"
```

A malformed entry in `ALLOWED_USER_IDS` is skipped; a malformed entry passed to
`--allowed-users` is a startup error. The flag is stricter on purpose — an ID
typed on the command line is being set deliberately, and silently dropping it
would quietly change who can operate the cluster.

### Everything else uses the TELECTL_ prefix

Any config key can be overridden by upper-casing it and replacing dots with
underscores:

| Variable | Maps to |
|---|---|
| `TELECTL_BOT_RATE_LIMIT` | `bot.rate_limit` |
| `TELECTL_KUBERNETES_DRY_RUN` | `kubernetes.dry_run` |
| `TELECTL_LOGGING_LEVEL` | `logging.level` |
| `TELECTL_KUBERNETES_DEFAULT_NAMESPACE` | `kubernetes.default_namespace` |
| `TELECTL_BOT_MENU_PAGE_SIZE` | `bot.menu_page_size` |

---

## Command-line flags

```
--config string          Config file (skips the search path)
--token string           Telegram bot token
--kubeconfig string      Path to kubeconfig
--allowed-users string   Comma-separated Telegram user IDs
--log-level string       debug, info, warn, error   (default "info")
--dry-run                Log mutating operations instead of performing them
```

`--log-level` only overrides the config file when actually passed. Its "info"
default would otherwise clobber a `logging.level: debug` set in the file.

### Subcommands

```
telectl              Run the bot
telectl version      Print version, commit and build date
telectl config       Print the effective configuration (bot token redacted)
telectl contexts     List kubeconfig contexts, then exit
```

`telectl config` is the fastest way to find out what telectl actually loaded —
it prints the merged view, so a value coming from the environment shows up
there. The bot token is redacted to a four-character prefix, because this
output is routinely pasted into issue reports.

---

## Verifying a configuration

```bash
# What did telectl actually load?
telectl config

# Can it reach the cluster, and which contexts does it see?
telectl contexts

# Start with maximum verbosity
telectl --log-level debug
```

At startup telectl logs the kubeconfig path, the resolved context, whether
dry-run is on, and how many users are on the allowlist. If the allowlist is
empty, it logs a warning — see below.

---

## Security

### Always set an allowlist

With `allowed_user_ids` empty, **every Telegram user who finds the bot can
operate the cluster**. Telegram bot usernames are discoverable and tokens leak.
telectl logs a warning at startup when no allowlist is configured:

```
No allowed users configured — every Telegram user who finds this bot can
operate the cluster. Set --allowed-users or telegram.allowed_user_ids.
```

Get your own ID from [@userinfobot](https://t.me/userinfobot).

### RBAC is the real boundary

telectl performs every operation with the kubeconfig's credentials, so it can
do exactly what that identity is permitted to do. The allowlist controls *who
can ask*; RBAC controls *what happens*. Treat RBAC as the security boundary and
scope the bot's service account to what it actually needs.

For a read-mostly deployment, a role granting `get`/`list`/`watch` plus
`pods/log` is enough for everything except scale, restart, delete, cordon and
drain.

### Dry run

`kubernetes.dry_run: true` (or `--dry-run`) makes every mutating operation log
what it would have done and return success without calling the API. Useful for
confirming what the bot would do before granting write access. The chat replies
say so explicitly (`🧪 Dry run — nothing was changed`), so it is not mistaken
for a real change.

### Context switching does not touch your kubeconfig

`/use-context` rebuilds telectl's in-memory API clients. It deliberately does
**not** write `current-context` back to `~/.kube/config`, because that file is
shared with kubectl and with anything else running as that user — a chat
message should not silently repoint someone's terminal at a different cluster.

The persistent variant exists in the code (`SwitchContextPersistent`) but is not
reachable from chat.

---

## Deployment notes

### In-cluster

Mount a service account token and point telectl at it, or supply a kubeconfig
as a secret. The container runs as UID 1000 (`telectl`), so mount the
kubeconfig readable by that user:

```yaml
volumeMounts:
  - name: kubeconfig
    mountPath: /home/telectl/.kube
    readOnly: true
env:
  - name: TELEGRAM_BOT_TOKEN
    valueFrom:
      secretKeyRef:
        name: telectl
        key: bot-token
  - name: ALLOWED_USER_IDS
    value: "123456789"
```

### Credential plugins

If your kubeconfig uses an `exec` credential plugin (`kubelogin` for AKS,
`gke-gcloud-auth-plugin` for GKE, `aws-iam-authenticator` for EKS), that binary
must be present in the container. telectl itself never spawns processes, but
client-go will invoke the plugin your kubeconfig declares — the dependency comes
from the kubeconfig, not from telectl.

The stock image does not include any credential plugin. Either bake one in or
use a kubeconfig with a long-lived token or client certificate.

### Rate limits

Two separate limits apply:

- `bot.rate_limit` throttles per Telegram user, protecting the cluster from a
  chat-driven request flood.
- `kubernetes.qps` / `kubernetes.burst` throttle telectl's own client-go
  requests against the API server.

Raising the first without the second just moves the queue.

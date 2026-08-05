# Upgrading

How to upgrade telectl safely. The project uses immutable,
Kubernetes-style SemVer tags, so an upgrade is always *to a specific new
number*, never a re-tag of the same one. See [Release Process](Release-Process).

## Config compatibility

Within the **0.1.x** line the config schema is stable — your existing
`config.yaml` / Helm values keep working. Read the
[CHANGELOG](https://github.com/ksauraj/telectl/blob/main/CHANGELOG.md) for
any `**BREAKING**` notes (e.g. the move from Docker Hub to GHCR earlier
changed the image repository).

## Before you upgrade

- [ ] Read the CHANGELOG entries between your current and target version —
  especially any `**BREAKING**` lines.
- [ ] Back up your kubeconfig-backed access (the images/binaries are
  replaces; config and RBAC are what you own).
- [ ] If you use impersonation, confirm your ClusterRoles/Bindings still match
  the identities in `impersonation.user_mapping` (they're independent of the
  bot version and carry over unchanged).

## Binaries (normal user deployment)

```bash
# 1. check current
telectl version

# 2. download the new binary from the target release
#    (see https://github.com/ksauraj/telectl/releases), or rebuild:
cd telectl
git fetch origin
git checkout main && git pull
make build

# 3. restart the process
./telectl --config /path/to/config.yaml
```

Your config file is untouched — just swap the binary and restart.

## Docker

```bash
docker pull ghcr.io/ksauraj/telectl:v0.1.0-beta.0   # your target tag
docker stop telectl && docker rm telectl
docker run ... ghcr.io/ksauraj/telectl:v0.1.0-beta.0
```

## Helm (in-cluster)

### Update the chart repo

```bash
helm repo update telectl
helm search repo telectl --versions
```

### Upgrade the release

```bash
helm upgrade telectl telectl/telectl \
  --namespace default \
  -f values-bot.yaml \
  --set image.tag=v0.1.0-beta.0
```

**Known Helm gotchas** (see also [Helm Chart Guide](Helm-Chart-Guide)):

- If you change values that touch the bot **Secret**, delete the old Secret
  first — it carries `helm.sh/resource-policy: keep`, so Helm won't update
  it and your new values won't apply. `kubectl delete secret telectl -n default`.
- Big Telegram user IDs must stay **quoted** in values/`--set` to avoid
  scientific-notation corruption.
- If `helm upgrade` says "cannot re-use a name that is still in use",
  `helm uninstall` then wait a few seconds before re-installing.
- A `--wait --timeout` that *reports* cancelled may still have completed —
  verify with `helm status telectl`.

### Verify the upgrade

```bash
helm history telectl
kubectl get pods -n default -l app.kubernetes.io/name=telectl
kubectl logs -n default deploy/telectl --tail=20   # confirm the new version
```

The pod log's startup line shows the version you upgraded to.

## Rolling back

Helm keeps history; roll back to a prior revision:

```bash
helm history telectl
helm rollback telectl <REVISION>
```

For binaries/Docker, roll back = run the previous version (you kept the old
artifact). Tags are immutable, so the previous version is always still
downloadable.

## Downgrade caveats

Downgrading within 0.1.x is safe. If you ever go back across a `**BREAKING**`
change (e.g. an image-repository change), re-apply the config values that
change with it.
# Testing Guide

How telectl is tested — locally and in CI — and how to add tests that stick.

## Test layers

| Layer | Where | Command |
|-------|-------|---------|
| Unit | `internal/...` | `make test-unit` (`go test -short ./internal/...`) |
| Full suite | everything | `make test` (`go test -race -coverprofile=coverage.out ./...`) |
| Integration (opt-in) | tagged `//go:build integration` | `make test-integration` (`go test -tags=integration ./...`) |
| Lint | whole repo | `make lint` (golangci-lint), `make vet`, `make staticcheck` |
| Coverage | report | `make coverage` → `coverage.html` |

## What the tests pin (read these before editing)

These test files encode non-obvious behavior — change them deliberately:

- `internal/handlers/base_flags_test.go`
  - `TestParseFlags` — every flag spelling (`-n x`, `-n=x`, `--namespace x`,
    `--namespace=x`), `-A` clearing a namespace, dangling flags, unknown
    flags falling through, "last flag wins".
  - `TestParseExecArgs` — the pod/container/command boundary: `--` separates
    the command, the command's own flags pass through untouched, `-n` is
    stripped *before* the command (a former bug), joined forms, dangling
    container flags.
- `internal/tg/richdoc_test.go` — rich-text escaping, bold markers surviving
  table cells, Markdown control characters in cluster-supplied text.
- `internal/menus/` — callback-data encode/decode round-trips, unknown
  callbacks parse without panicking, token resolution across restarts.
- `internal/types/clusterscope_test.go` — every resource alias classifies
  consistently with its GVR (cluster-scoped vs namespaced).

## CI pipeline

The CI workflow (`ci.yml`) runs on every push/PR:

1. **Go Mod Tidy** — `go mod tidy` must be a no-op; checksums verified.
2. **Test** — formatting check (`gofmt`), `go vet`, full test suite with
   race detector, coverage upload.
3. **Lint** — golangci-lint.
4. **Build** — compile + smoke test the binary.
5. **Build Release Binaries** — all platforms (amd64/arm64 ×
   linux/darwin/windows), used on tags.

## Writing a good test here

- Prefer **table-driven tests** (the repo's existing style).
- Test the *behavior users depend on*, not just the happy path: dangling
  flags, unknown resources, missing metrics-server, Forbidden responses,
  impersonated-client routing.
- The k8s client is built against an interface (`kubernetes.Interface`), so
  you can inject a fake and exercise verbs (cordon, drain, scale) without a
  cluster.
- For rich-text/menu tests, golden assertions on the rendered output.

## Running one focused test

```bash
go test ./internal/handlers/ -run TestParseExecArgs -v
go test ./internal/tg/ -v
```

## Before you push

```bash
make fmt && make vet && make lint && make test
```

A green local run plus green CI is the bar for merge.
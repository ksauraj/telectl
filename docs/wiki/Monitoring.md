# Monitoring

telectl surfaces cluster health three ways: **resource usage**, **events**,
and **watching**.

## Top — resource usage

```
/top pods [-n namespace]
/top nodes
```

Shows CPU/memory for pods or nodes, sourced from the metrics API
(`metrics.k8s.io`).

- Requires **metrics-server** installed in the cluster. If it's absent,
  telectl says *"Metrics unavailable — this needs metrics-server installed"*
  and suggests alternating rather than surfacing a raw API 404.
- Node metrics are cluster-scoped (always all nodes); pod metrics follow your
  current namespace.

Menu path: **Monitor → Top**.

## Events — cluster activity

```
/events [-n namespace]
```

Lists the most recent events in the namespace: object, type, reason, and a
human-readable message. Useful for spotting failed pulls, evictions, or
readiness gates.

- Events **expire after about an hour**, so a quiet namespace may legitimately
  show *"No events"* — telectl calls this out so it isn't mistaken for a fault.
- Menu path: **Monitor → Events**.

## Watch — live updates

```
/watch <resource> [-n namespace]
```

Follows a resource (e.g. `pods`) and pushes updates as objects change. Because
it streams, it runs as a **typed command**, not inside a menu pane.

- Stop with `/cancel`.
- Output uses the session's impersonated client, so a read-only user only
  sees what their role permits.

## Menu entry point

**Monitor** (reply keyboard / menu) → pick **Top**, **Events**, or
**Watch**. Top and Events render into the single pane; Watch shows the
command syntax (it can't live in the pane because it streams).

## Grafana / dashboards?

telectl is a chat operator, not a dashboard. For long-lived metrics dashboards
pair it with your existing monitoring stack (Prometheus + Grafana, etc.);
the `/top` and `/watch` commands are for quick, on-the-go checks from chat.
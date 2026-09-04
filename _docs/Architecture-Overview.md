---
title: Architecture Overview
nav_order: 16
---

# Architecture Overview

telectl is a **direct-API** Kubernetes bot: the binary runs inside (or next
to) your cluster and talks to the API server with client-go. No agent pods,
no webhook relays, no kubectl subprocesses — every operation is a native
Kubernetes API call.

## Component Diagram

```mermaid
flowchart TB
    subgraph User["Outside the cluster"]
        U1[Telegram user 1]
        U2[Telegram user 2]
        U3[Telegram user N]
    end

    subgraph Cloud["Telegram"]
        API[Bot API server]
    end

    subgraph K8sCluster["Your Kubernetes cluster"]
        subgraph NS["namespace: telectl"]
            Pod[telectl pod]
            SA[ServiceAccount: telectl]
            Secret[Secret: bot token + user IDs]
            CM[ConfigMap: telectl.yaml]
            ImpCR[ClusterRole: telectl-impersonator]
        end

        APIServer[(API server)]

        ImpUserA[Identity: admin-user<br/>groups: system:masters]
        ImpUserB[Identity: readonly-user<br/>groups: viewers]

        RBAC1[ClusterRoleBinding<br/>readonly-user → viewers]
        RBAC2[ClusterRoleBinding<br/>admin-user → system:masters]
    end

    U1 -->|HTTPS long-poll| API
    U2 -->|HTTPS long-poll| API
    U3 -->|HTTPS long-poll| API
    API -->|getUpdates| Pod

    Pod -->|uses token| SA
    Pod -->|reads| Secret
    Pod -->|reads| CM
    Pod -->|impersonates| APIServer

    APIServer -->|checks| ImpCR
    ImpUserA --> RBAC2
    ImpUserB --> RBAC1
```

## Request Flow (mutating action, e.g. scale)

```mermaid
sequenceDiagram
    participant U as Telegram user
    participant B as telectl bot
    participant K as API server
    participant R as RBAC

    U->>B: tap "Scale to 10" on deployment pane
    B->>B: resolve callback → applyScale
    B->>B: look up user mapping → identity (user + groups)
    B->>K: PATCH deployments/scale as impersonated identity
    K->>R: authorize(user, groups, verb, resource)
    alt allowed
        R-->>K: allow
        K-->>B: 200 OK
        B->>U: re-render detail pane (new replica count)
    else forbidden
        R-->>K: deny
        K-->>B: 403 Forbidden
        B->>U: show Forbidden error in pane
    end
```

## Key Design Decisions

| Decision | Why |
|----------|-----|
| **Direct API, no kubectl** | One binary, no subprocess management, typed clients |
| **Impersonation for per-user RBAC** | The bot holds one ServiceAccount; each Telegram user is mapped to a k8s identity, and **Kubernetes RBAC decides** what they may do |
| **Single message pane** | Menu navigation edits one message in place; verbs render into the pane (TUI-style) |
| **Rich messages with plain fallback** | Native tables/headings where the Bot API supports them; graceful fallback to text |
| **GHCR multi-arch images** | `linux/amd64` + `linux/arm64` built with buildx `TARGETARCH` |

## Data Flow for a Typed Command

```mermaid
flowchart LR
    A[/logs pod --tail 50/] --> B[LogsHandler]
    B --> C[parseLogFlags]
    C --> D[getK8sClient session]
    D --> E[GetPodLogs]
    E --> F[FormatPodLogs respects --tail]
    F --> G[SendRich + text fallback]
    G --> H[Telegram]
```

## Package Layout

```
cmd/telectl/          entrypoint (Cobra CLI)
internal/bot/         bot core: callbacks, panes, detail verbs
internal/handlers/    typed command handlers (logs, exec, scale, restart…)
internal/k8s/         client-go wrapper, impersonated clients
internal/config/      viper config, impersonation mapping
internal/menus/       menu/keyboard builders, callback token store
internal/tg/          Telegram transport, rich messages
internal/types/       shared types, resource map
internal/utils/       formatters, helpers
charts/telectl/       Helm chart
```

---

Next: [How It Works](How-It-Works) · [Impersonation & RBAC](Impersonation-and-RBAC)
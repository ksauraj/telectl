---
title: Home
nav_order: 1
---

telectl is a **Telegram bot for Kubernetes cluster management**. Browse pods,
deployments, services and more; read logs; scale, restart and exec — all from
chat, with **per-user RBAC via Kubernetes impersonation**.

```mermaid
flowchart LR
    User([Telegram user]) -->|message / callback| TG[Telegram Bot API]
    TG --> Bot[telectl pod]
    Bot --> K8s[(Kubernetes API server)]
    K8s -->|RBAC decision| Bot
```

## 📚 Documentation

### Getting Started
- [Quick Start](Quick-Start) — running in 5 minutes
- [Installation Guide](Installation-Guide) — binaries, Docker, Helm
- [Configuration Reference](Configuration-Reference) — every config option

### User Guides
- [Menu Navigation](Menu-Navigation) — interactive menus
- [Command Reference](Command-Reference) — all typed commands
- [Inline Queries](Inline-Queries) — `@telectl …` lookups in any chat
- [Context Management](Context-Management) — switch clusters in-session
- [Monitoring](Monitoring) — top, events, watch

### Deployment
- [Try It Locally](Try-It-Locally) — kind, minikube, k3d, k3s, MicroK8s
- [Helm Chart Guide](Helm-Chart-Guide) — full Helm reference + RBAC + impersonation
- [Docker Deployment](Docker-Deployment) — running with Docker
- [Two Deployment Modes](Two-Deployment-Modes) — Helm pod vs normal user
- [Kubernetes RBAC](Kubernetes-RBAC) — setting up permissions
- [Impersonation & RBAC](Impersonation-and-RBAC) — per-user permissions
- [Production Checklist](Production-Checklist) — pre-deployment checklist

### Development
- [Architecture Overview](Architecture-Overview) — components, data flow, diagrams
- [How It Works](How-It-Works) — deep dive into menus, commands, sessions
- [Development Setup](Development-Setup) — local dev environment
- [Contributing Guide](Contributing-Guide) — how to contribute
- [Testing Guide](Testing-Guide) — unit, integration, e2e
- [Release Process](Release-Process) — how releases work
- [Versioning & Releases](Versioning-and-Releases) — SemVer, K8s-style pre-releases

### Operations
- [Security](Security) — tokens, impersonation, least-privilege
- [Troubleshooting](Troubleshooting) — common issues
- [FAQ](FAQ) — frequently asked questions
- [Upgrading](Upgrading) — version upgrade procedures
- [Adopters](Adopters) — who's using telectl

## 🔗 Quick Links
- [GitHub Repository](https://github.com/ksauraj/telectl)
- [Releases](https://github.com/ksauraj/telectl/releases)
- [GHCR Images](https://github.com/ksauraj/telectl/pkgs/container/telectl)

## ⚙️ Helm Chart Repository

Add the chart repo directly from GitHub Pages:

```bash
helm repo add telectl https://ksauraj.github.io/telectl/charts
helm repo update
helm install telectl telectl/telectl \
  --namespace telectl --create-namespace \
  --set telegram.botToken="YOUR_BOT_TOKEN"
```

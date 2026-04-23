# AGENTS.md

This file provides guidance to AI coding agents when working with code in this repository.

## Project Overview

`civo-node-agent` monitors Kubernetes nodes in a Civo cluster and triggers automatic recovery actions (currently hard reboot via the Civo API) when nodes fail health checks.

### Deployment

`civo-node-agent` is designed to run as a daemon process on the control plane VM, which is the preferred deployment. A Helm chart (`charts/`) is also provided so it can run as a single-replica Deployment in `kube-system` if needed.

By default the agent runs in **monitor-only mode** (logs recovery actions without executing them). Set `CIVO_NODE_AGENT_MONITOR_ONLY=false` to enable actual reboots.

## Build & Test Commands

```bash
# Build (CGO disabled — no C dependencies; required for static binary on the CP VM)
CGO_ENABLED=0 go build -o node-agent ./

# Run all tests
go test ./...

# Before completing any task, always run:
go fmt ./...
go vet ./...
go test ./...
```

No linter is configured in CI.

## Architecture

**Entrypoint** (`main.go`): Reads env vars + `--kubeconfig` flag, sets up JSON structured logging (slog), registers Prometheus metrics, starts the metrics HTTP server, constructs an Executor + Checkers + Watcher, and runs the watcher with graceful SIGTERM/SIGINT shutdown.

### Packages

- **`pkg/watcher/`** — Orchestrator. Sets up a Node Informer (filtered by optional node pool label selector) and runs a 10s ticker reconcile loop driving the state machine (`Unknown → Healthy → Unhealthy → WaitingReboot → Failed`).
- **`pkg/health/`** — Health checkers.
- **`pkg/operation/`** — Recovery executors (Civo API reboot; nop executor used as safe default).
- **`pkg/metrics/`** — Prometheus metrics (all `civo_` prefixed). Defined once in `metrics.go`.

## Release

Tags matching `v*.*.*` trigger `.github/workflows/release-image.yaml`, which builds multi-arch Docker images via goreleaser and publishes to Docker Hub. The same binary is also uploaded to Civo object storage for CP VM installations (handled outside this repository).

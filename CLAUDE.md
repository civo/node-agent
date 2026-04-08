# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Kubernetes node agent for Civo cloud that monitors cluster nodes and triggers automatic hard reboots via the Civo API when nodes become NotReady or lose expected GPU capacity. Deployed as a single-replica Deployment in kube-system via Helm.

## Build & Test Commands

```bash
# Build
go build -o node-agent ./

# Run all tests
go test ./...

# Run a single test
go test ./pkg/watcher/ -run TestName

# Build Docker image (dry-run)
goreleaser release --snapshot --skip=publish --clean
```

No linter is configured in CI.

## Architecture

**Entrypoint** (`main.go`): Reads env vars, sets up JSON structured logging (slog), creates a Watcher, and runs it with graceful SIGTERM/SIGINT shutdown.

**Core package** (`pkg/watcher/`):
- `watcher.go` — Main loop polls every 10 seconds. For each node matching the node pool label (`kubernetes.civo.com/civo-node-pool={nodePoolID}`), checks if the node is NotReady or has fewer GPUs than desired. If a reboot is warranted (and cooldown window hasn't elapsed), calls `HardRebootInstance` via the Civo API.
- `options.go` — Functional options pattern (`WithKubernetesClient`, `WithCivoClient`, etc.) for dependency injection and configuration.
- `fake.go` — `FakeClient` implementing `civogo.Clienter` for testing.
- `watcher_test.go` — Tests use fake Kubernetes client (`k8s.io/client-go/kubernetes/fake`) and `FakeClient` for Civo API.

**Reboot safeguards**: Tracks last reboot time per node in a `sync.Map`. Skips reboot if the node's Ready/NotReady condition transitioned recently or a reboot command was sent within the configurable time window (default 40 minutes).

## Required Environment Variables

`CIVO_API_KEY`, `CIVO_REGION`, `CIVO_CLUSTER_ID`, `CIVO_NODE_POOL_ID` — see `.env.example`.

Optional: `CIVO_API_URL`, `CIVO_NODE_DESIRED_GPU_COUNT`, `CIVO_NODE_REBOOT_TIME_WINDOW_MINUTES`.

## Deployment

Helm chart in `charts/`. Secrets are expected in `civo-node-agent` and `civo-api-access` Kubernetes secrets.

```bash
helm upgrade -n kube-system --install node-agent ./charts
```

## Key Dependencies

- `github.com/civo/civogo` — Civo cloud API client
- `k8s.io/client-go` — Kubernetes client (in-cluster config by default)

## Release

Tags matching `v*.*.*` trigger `.github/workflows/release-image.yaml`, which builds multi-arch Docker images via goreleaser and publishes to Docker Hub.

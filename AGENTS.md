# AGENTS.md

This file provides guidance to AI coding agents when working with code in this repository.

## Project Overview

`civo-node-agent` monitors Kubernetes nodes in a Civo cluster and triggers automatic recovery actions (currently hard reboot via the Civo API) when nodes fail health checks.

### Deployment target

**The recommended deployment model is a daemon process running on the control plane VM**, installed and managed outside this repository (e.g. by a provisioning script that downloads the binary to the VM and registers it with the init system).

A Helm chart (`charts/`) is included for running the agent as a single-replica Deployment in `kube-system`, but **this is not the primary or recommended deployment mode**. When making design decisions, prioritize the CP VM daemon use case (e.g. kubeconfig path, logging, filesystem assumptions).

By default the agent runs in **monitor-only mode** (logs recovery actions without executing them). Set `CIVO_NODE_AGENT_MONITOR_ONLY=false` to enable actual reboots.

## Build & Test Commands

```bash
# Build (CGO disabled — no C dependencies; required for static binary on the CP VM)
CGO_ENABLED=0 go build -o civo-node-agent ./

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

- **`pkg/watcher/`** — Orchestrator. Sets up a Node Informer (filtered by optional node pool label selector) and runs a 10s ticker reconcile loop driving a state machine (`Unknown → Healthy → Unhealthy → WaitingReboot`).
- **`pkg/health/`** — Health checkers (`HealthChecker` interface: `Name()`, `Check() (healthy, reason)`, `Threshold()`).
- **`pkg/operation/`** — Recovery executors (`Executor` interface; `civoExecutor` for Civo API, `nopExecutor` as safe default).
- **`pkg/metrics/`** — Prometheus metrics (all `civo_` prefixed). Defined once in `metrics.go`.

### Design conventions

- Package-boundary types are exposed via interfaces; concrete structs are unexported.
- `NodeState` fields are private, mutated only through `StateStore` transition methods.
- Functional options for configuration. Test-only options are unexported (`withNowFunc`, `withNodeLister`).
- All timestamps stored in UTC (`nowFunc` defaults to `time.Now().UTC()`).
- Tests follow the Civo Go testing conventions (`description` field, verb-driven descriptions, `test` iterator, mock init inside `t.Run`).

## Known Limitations

The following are intentionally not implemented in the current state and must be addressed before enabling `monitorOnly=false` in production or expanding recovery beyond reboot:

- **No cluster-wide blast-radius protection.** There is no concurrent-reboot cap, no unhealthy-rate circuit breaker, and no PDB awareness. A cluster-wide outage (CNI glitch, object storage failure, region issue) could cause every node to be rebooted simultaneously.
- **Standard nodes retry reboot indefinitely.** `PhaseDrain → PhaseReplace` is defined in the state machine but not wired up. A persistently broken node will be rebooted forever with no upper bound on `rebootCount`.
- **Civo API calls do not propagate `context.Context`.** The civogo library does not accept a context, and `Reboot()` currently discards it. A hung API call will block the reconcile tick.
- **No retry/backoff on Civo API errors.** A failed reboot is retried immediately on the next tick, which can hammer the Civo API during an outage.

## Release

Tags matching `v*.*.*` trigger `.github/workflows/release-image.yaml`, which builds multi-arch Docker images via goreleaser and publishes to Docker Hub. The same binary is also uploaded to Civo object storage for CP VM installations (handled outside this repository).

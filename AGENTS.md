# AGENTS.md

This file provides guidance to AI coding agents when working with code in this repository.

## Project Overview

Kubernetes node agent for Civo cloud that monitors cluster nodes and triggers automatic recovery actions (currently hard reboot via the Civo API) when nodes fail health checks. Deployed as a single-replica Deployment in `kube-system` via Helm.

By default the agent runs in **monitor-only mode** (logs recovery actions without executing them). Set `CIVO_NODE_AGENT_MONITOR_ONLY=false` (or `monitorOnly: false` in Helm) to enable actual reboots.

## Build & Test Commands

```bash
# Build (CGO disabled — no C dependencies)
CGO_ENABLED=0 go build -o civo-node-agent ./

# Run all tests
go test ./...

# Run a single test
go test ./pkg/watcher/ -run TestName

# Before completing any task
go fmt ./...
go vet ./...
go test ./...

# Build Docker image (dry-run)
goreleaser release --snapshot --skip=publish --clean
```

No linter is configured in CI.

## Architecture

**Entrypoint** (`main.go`): Reads env vars + `--kubeconfig` flag, sets up JSON structured logging (slog), registers Prometheus metrics, starts the metrics HTTP server, constructs an Executor + Checkers + Watcher, and runs the watcher with graceful SIGTERM/SIGINT shutdown.

### Packages

- **`pkg/watcher/`** — Orchestrator.
  - `watcher.go` — Sets up a Node Informer (filtered by optional node pool label selector), runs a 10s ticker reconcile loop.
  - `state.go` — `NodePhase` enum (`Unknown`, `Healthy`, `Unhealthy`, `WaitingReboot`, future: `Drain`, `Replace`), `NodeState` with private fields + getters, `StateStore` with transition methods (`MarkUnhealthy`, `MarkWaitingReboot`, `Reset`, `Cleanup`).
  - `options.go` — Functional options (`WithExecutor`, `WithCheckers`, `WithMonitorOnly`, `WithNodePoolIDs`, etc.). Test-only options are unexported (`withNowFunc`, `withNodeLister`).
- **`pkg/health/`** — Health checkers.
  - `HealthChecker` interface returns `(healthy bool, reason string)` plus `Threshold()`.
  - `nodeReadyChecker` (5min), `diskPressureChecker` (30min), `ciliumChecker` (10min, skips non-Cilium CNI via `NetworkUnavailable` reason), `gpuChecker` (10min, uses `nvidia.com/gpu.count` label vs allocatable GPU count; auto-skips non-GPU nodes).
  - `HasGPU(node)` helper — reads `nvidia.com/gpu.count` label; used by watcher to mark nodes as GPU for reboot-wait differentiation.
- **`pkg/operation/`** — Recovery executors.
  - `Executor` interface with `Reboot(ctx, nodeName)`.
  - `civoExecutor` implements via Civo API (`FindKubernetesClusterInstance` + `HardRebootInstance`).
  - `nopExecutor` is the safe default to prevent nil-pointer dereference.
- **`pkg/metrics/`** — Prometheus metrics (all `civo_` prefixed).

### Reconcile loop

For each node matched by the label selector (or all nodes if `nodePoolIDs` is empty):

1. Run each `HealthChecker`; record `civo_node_agent_health_check_total{node, checker, result}` with the checker's reason as result.
2. If all checkers pass and the node was previously unhealthy → `Reset`, log `Node recovered`, update phase metrics.
3. If any checker failed:
   - Track `isGPUNode` (from `nvidia.com/gpu.count` label).
   - Compute `minThreshold` across failed checkers.
   - State transitions:
     - `Healthy → Unhealthy`: mark + log `Node unhealthy detected`.
     - `Unhealthy → WaitingReboot`: after `minThreshold` elapsed, optionally call `executor.Reboot` (skipped in monitor-only), log `Reboot initiated`, increment `recovery_actions_total`.
     - `WaitingReboot → WaitingReboot`: after `rebootWaitMinutes` (standard) or `gpuRebootWaitMinutes` (GPU), retry reboot, log `Reboot retry`. In monitor-only the per-tick `Waiting for reboot effect` log is suppressed.
4. Cleanup state and Prometheus labels for nodes no longer in the cluster.

### Reboot safeguards

- `monitorOnly=true` by default (fail-safe).
- Per-node cooldown via `rebootWaitMinutes` / `gpuRebootWaitMinutes`.
- `NopExecutor` default prevents accidental reboots when no executor is configured.

**Known limitations (tracked as TODOs / future PRs):**

- No cluster-wide blast-radius protection (no concurrent-reboot cap, no unhealthy-rate circuit breaker, no PDB awareness). Required before enabling `monitorOnly=false` in production.
- Standard nodes retry reboot indefinitely; `PhaseDrain → PhaseReplace` is defined but not wired up.
- Civo API calls do not propagate `context.Context` (civogo library limitation). `Reboot()` discards the ctx; a hung API call blocks the reconcile tick.
- No retry/backoff on Civo API errors.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CIVO_API_KEY` | — | Civo API key (read from `civo-api-access` secret in Helm deployment). |
| `CIVO_API_URL` | — | Civo API URL. |
| `CIVO_CLUSTER_ID` | — | Civo cluster ID (exposed as label in `civo_node_agent_info`). |
| `CIVO_REGION` | — | Civo region. |
| `CIVO_NODE_POOL_IDS` | empty | Comma-separated node pool IDs. Empty = watch all nodes. |
| `CIVO_NODE_AGENT_MONITOR_ONLY` | `true` | Monitor-only mode (log but don't reboot). |
| `CIVO_NODE_AGENT_METRICS_PORT` | `9625` | Prometheus metrics HTTP port (validated to 1024–65535). |
| `CIVO_NODE_REBOOT_WAIT_MINUTES` | `10` | Reboot wait between retries for standard nodes. |
| `CIVO_GPU_NODE_REBOOT_WAIT_MINUTES` | `40` | Reboot wait between retries for GPU nodes. |

## Command Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--kubeconfig` | `/etc/rancher/k3s/k3s.yaml` | Path to kubeconfig. Empty = in-cluster config. The Helm Deployment passes `--kubeconfig=""`. |
| `--version` | | Print the agent version and exit. |

## Prometheus Metrics

All metrics use the `civo_` prefix.

| Metric | Type | Labels |
|--------|------|--------|
| `civo_node_agent_info` | Gauge (always 1) | `version`, `cluster_id` |
| `civo_node_agent_health_check_total` | Counter | `node`, `checker`, `result` (low-cardinality reason) |
| `civo_node_agent_recovery_actions_total` | Counter | `node`, `action`, `mode` (`monitor` / `active`) |
| `civo_node_agent_recovery_failures_total` | Counter | `node`, `action` |
| `civo_node_agent_reconcile_errors_total` | Counter | `reason` |
| `civo_node_agent_node_unhealthy_duration_seconds` | Gauge | `node` |
| `civo_node_agent_recovery_phase` | Gauge | `node`, `phase` (value = 1 for current phase, 0 for others) |

Per-node metric labels are cleaned up (`DeletePartialMatch`) when a node is removed from the cluster. `civo_node_agent_info` is explicitly deleted on graceful shutdown.

## Deployment

```bash
helm upgrade -n kube-system --install node-agent ./charts
```

- API credentials come from the existing `civo-api-access` secret (auto-provisioned by Civo on every Civo Kubernetes cluster).
- Non-sensitive config lives in `values.yaml` (`nodePoolIDs`, `rebootWaitMinutes`, `gpuRebootWaitMinutes`, `monitorOnly`, `metricsPort`).

## Key Dependencies

- `github.com/civo/civogo` — Civo cloud API client (no context support, used with goroutine+timeout workaround TBD).
- `k8s.io/client-go` — Kubernetes client. Uses a SharedInformer filtered by `kubernetes.civo.com/civo-node-pool` label when `CIVO_NODE_POOL_IDS` is set.
- `github.com/prometheus/client_golang` — Prometheus instrumentation.

## Release

Tags matching `v*.*.*` trigger `.github/workflows/release-image.yaml`, which builds multi-arch Docker images via goreleaser and publishes to Docker Hub.

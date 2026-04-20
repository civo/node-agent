# Node Agent

`node-agent` monitors the health of Kubernetes nodes and can automatically reboot VM instances when necessary. A reboot is triggered when a node fails one or more health checks (e.g. `NodeReady`, GPU count, Cilium, DiskPressure) for a configured threshold.

By default it runs in **monitor-only** mode, logging recovery actions without executing them. Set `monitorOnly=false` to enable actual reboots.

## Prerequisites: `civo-api-access` Secret

The `civo-api-access` secret is automatically provisioned by Civo in the `kube-system` namespace of every Civo Kubernetes cluster. It contains the API credentials and cluster identity used by `node-agent`:

| Key | Description |
|-----|-------------|
| `api-key`  | Civo API key used for reboot operations. |
| `api-url` | Civo API URL. |
| `cluster-id` | The ID of this Civo Kubernetes cluster. |
| `region` | The Civo region this cluster runs in. |

No manual setup is required — `node-agent` reads these values directly from the existing secret.

## NVIDIA GPU Operator (GPU clusters only)

The GPU health check relies on the `nvidia.com/gpu.count` label added by the NVIDIA GPU Feature Discovery component. Follow the Civo documentation to install the NVIDIA GPU Operator on your cluster:

[Installing the NVIDIA GPU Operator](https://github.com/civo/docs/blob/main/content/docs/kubernetes/advanced/gpu-config.md#installing-the-nvidia-gpu-operator)

## Install `node-agent` chart

You will need to clone this repository in order to have access to the charts directory. In your terminal, change directory to your cloned `node-agent` repo directory, then run:

```bash
helm upgrade -n kube-system --install node-agent ./charts
```

To enable active recovery (actually reboot nodes):

```bash
helm upgrade -n kube-system --install node-agent ./charts --set monitorOnly=false
```

## Configuration

### Helm values (`values.yaml`)

| Value | Default | Description |
|-------|---------|-------------|
| `nodePoolIDs` | `""` | Comma-separated node pool IDs to watch. Empty means all nodes. |
| `rebootWaitMinutes` | `10` | Minutes to wait after rebooting a standard node before retrying. |
| `gpuRebootWaitMinutes` | `40` | Minutes to wait after rebooting a GPU node before retrying. |
| `monitorOnly` | `true` | If `true`, log recovery actions without executing them. Set `false` to enable reboots. |
| `metricsPort` | `9625` | Port for the Prometheus metrics endpoint. |

### Health checkers

| Checker | Condition | Threshold |
|---------|-----------|-----------|
| `NodeReady` | `NodeReady == True` | 5 min |
| `DiskPressure` | `DiskPressure != True` | 30 min |
| `CiliumAgent` | `NetworkUnavailable == False` with reason `CiliumIsUp` (skipped for non-Cilium CNI) | 10 min |
| `GPU` | `allocatable["nvidia.com/gpu"]` equals `nvidia.com/gpu.count` label (skipped for non-GPU nodes) | 10 min |

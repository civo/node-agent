# node-agent

`node-agent` monitors the health of Kubernetes nodes and can conditional restart the node vm instances. It will conduct a node restart when a Kubernetes node either becomes NotReady or when the number of available GPUs per node reduces below a configured expectation.

## Installation

export API_KEY="your-api-key"
export NODE_POOL_ID="your-node-pool-id"
export GPU_COUNT="your-gpu-count"

kubectl patch secret civo-api-access -n kube-system --type='merge' -p='{"stringData": {"api-key-1": "'"$API_KEY"'", "node-pool-id": "'"$NODE_POOL_ID"'", "gpu-count": "'"$GPU_COUNT"'"}}'

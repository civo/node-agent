# Node Agent

`node-agent` monitors the health of Kubernetes nodes and can automatically restart VM instances when necessary. It triggers a restart under the following conditions:  

- A node enters the **NotReady** state.  
- The number of available GPUs per node falls below a configured threshold.  

## Installation

Set the required environment variables:  

```bash
export API_KEY="your-api-key"
export NODE_POOL_ID="your-node-pool-id"
export GPU_COUNT="your-gpu-count"

kubectl patch secret civo-api-access -n kube-system --type='merge' \
    -p='{"stringData": {"api-key-1": "'"$API_KEY"'", "node-pool-id": "'"$NODE_POOL_ID"'", "gpu-count": "'"$GPU_COUNT"'"}}'

helm upgrade --install node-agent ./charts
```

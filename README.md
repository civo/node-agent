# Node Agent

`node-agent` monitors the health of Kubernetes nodes and can automatically restart VM instances when necessary. It triggers a restart under the following conditions:  

- A node enters the **NotReady** state.  
- The number of available GPUs per node falls below a configured threshold.  


## Set Your `civo-node-agent` Secret

```
export CIVO_DESIRED_GPU_COUNT="12"
export CIVO_NODE_POOL_ID="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxxxxxx"
export CIVO_API_KEY="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
kubectl -n kube-system delete secret civo-node-agent --ignore-not-found
kubectl -n kube-system create secret generic civo-node-agent
kubectl -n kube-system patch secret civo-node-agent -n kube-system --type='merge' \
    -p='{"stringData": {"civo-api-key": "'"$CIVO_API_KEY"'", "node-pool-id": "'"$CIVO_NODE_POOL_ID"'", "desired-gpu-count": "'"$CIVO_DESIRED_GPU_COUNT"'"}}'
```

## Install `node-agent` chart

```bash
helm upgrade -n kube-system --install node-agent ./charts
```

## Configuration Details

The following configurations are stored in the `node-agent` secret in the `kube-system` namespace.

`node-pool-id`: The ID of your Kubernetes node pool which you want monitored. To collect this value, go to the [civo kubernetes dashboard](https://dashboard.civo.com/kubernetes), select your cluster, and click copy next to your pool id.

`desired-gpu-count`: This value is intended to match the number of GPUs per node. If you had a 2-node cluster with 8 GPU total, you would set this value to 4 to represent the number of GPUs per node.

`civo-api-key`: The civo api key to use when automatically rebooting nodes. To collect this value, go to toue [civo settings security tab](https://dashboard.civo.com/security).

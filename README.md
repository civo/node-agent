# Node Agent

`node-agent` monitors the health of Kubernetes nodes and can automatically restart VM instances when necessary. It triggers a restart under the following conditions:  

- A node enters the **NotReady** state.  
- The number of available GPUs per node falls below a configured threshold.  

## Configuration

`node_pool_id` [xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxxxxxx]:
The ID of your Kubernetes node pool which you want monitored. To collect this value, go to the [civo kubernetes dashboard](https://dashboard.civo.com/kubernetes), select your cluster, and click copy next to your pool id.

`node_desired_gpu_count`
This value is intended to match the number of GPUs per node. If you had a 2-node cluster with 8 GPU total, you would set this value to 4 to represent the number of GPUs per node.

# Installation

```bash
helm upgrade --install --set node_pool_id=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxxxxxx --set node_desired_gpu_count=8 node-agent ./charts
```

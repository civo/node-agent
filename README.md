# node-agent

`node-agent` monitors the health of Kubernetes nodes and can conditional restart the node vm instances. It will conduct a node restart when a Kubernetes node either becomes NotReady or when the number of available GPUs per node reduces below a configured expectation.

## Installation


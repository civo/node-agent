package health

import (
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	gpuResourceName  = "nvidia.com/gpu"
	gpuCountLabel    = "nvidia.com/gpu.count"
	gpuReplicasLabel = "nvidia.com/gpu.replicas"
	gpuThreshold     = 10 * time.Minute
)

// gpuChecker reports healthy when the node's allocatable GPU count
// matches the expected count from the nvidia.com/gpu.count label
// multiplied by the nvidia.com/gpu.replicas label (GPU sharing such as time-slicing).
// If the count label is not present, the node is not a GPU node and the check is skipped.
type gpuChecker struct{}

func (c *gpuChecker) Name() string             { return "GPU" }
func (c *gpuChecker) Threshold() time.Duration { return gpuThreshold }

func (c *gpuChecker) Check(node *corev1.Node) (bool, string) {
	count, ok := expectedGPUCount(node)
	if !ok || count == 0 {
		return true, "NonGPUNode"
	}
	expected := int64(count) * int64(gpuReplicas(node))

	quantity, exists := node.Status.Allocatable[gpuResourceName]
	if !exists || quantity.IsZero() {
		return false, "GPUCountMismatch"
	}

	actual, ok := quantity.AsInt64()
	if !ok {
		return false, "NoAllocatableGPU"
	}

	if actual == expected {
		return true, "GPUCountMatch"
	}
	return false, "GPUCountMismatch"
}

// HasGPU returns true if the node has the nvidia.com/gpu.count label
// with a positive value, indicating it is a GPU node regardless of
// current GPU health.
func HasGPU(node *corev1.Node) bool {
	n, ok := expectedGPUCount(node)
	return ok && n > 0
}

// expectedGPUCount reads the nvidia.com/gpu.count label from the node.
func expectedGPUCount(node *corev1.Node) (int, bool) {
	v, exists := node.Labels[gpuCountLabel]
	if !exists {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// gpuReplicas reads the nvidia.com/gpu.replicas label from the node.
// It returns 1 when the label is absent or invalid, i.e. the GPUs are not shared.
func gpuReplicas(node *corev1.Node) int {
	v, exists := node.Labels[gpuReplicasLabel]
	if !exists {
		return 1
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

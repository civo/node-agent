package health

import (
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	gpuResourceName = "nvidia.com/gpu"
	gpuCountLabel   = "nvidia.com/gpu.count"
	gpuThreshold    = 10 * time.Minute
)

// gpuChecker reports healthy when the node's allocatable GPU count
// matches the expected count from the nvidia.com/gpu.count label.
// If the label is not present, the node is not a GPU node and the check is skipped.
type gpuChecker struct{}

func (c *gpuChecker) Name() string             { return "GPU" }
func (c *gpuChecker) Threshold() time.Duration { return gpuThreshold }

func (c *gpuChecker) Check(node *corev1.Node) (bool, string) {
	expected, ok := expectedGPUCount(node)
	if !ok || expected == 0 {
		return true, "Non-GPU node"
	}

	quantity, exists := node.Status.Allocatable[gpuResourceName]
	if !exists || quantity.IsZero() {
		return false, fmt.Sprintf("Expected %d but got 0", expected)
	}

	actual, ok := quantity.AsInt64()
	if !ok {
		return false, "No allocatable GPU count"
	}

	if actual == int64(expected) {
		return true, fmt.Sprintf("%d/%d", actual, expected)
	}
	return false, fmt.Sprintf("Expected %d but got %d", expected, actual)
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

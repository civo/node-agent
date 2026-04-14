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
)

// gpuChecker reports healthy when the node's allocatable GPU count
// matches the expected count from the nvidia.com/gpu.count label.
// If the label is not present, the node is not a GPU node and the check is skipped.
type gpuChecker struct{}

func (c *gpuChecker) Name() string             { return "GPU" }
func (c *gpuChecker) Threshold() time.Duration { return 10 * time.Minute }

func (c *gpuChecker) Check(node *corev1.Node) (bool, string) {
	expected, ok := expectedGPUCount(node)
	if !ok || expected == 0 {
		return true, "non-GPU node"
	}

	quantity, exists := node.Status.Allocatable[gpuResourceName]
	if !exists || quantity.IsZero() {
		return false, fmt.Sprintf("expected %d but got 0", expected)
	}

	actual, ok := quantity.AsInt64()
	if !ok {
		return false, "failed to read allocatable GPU count"
	}

	if actual == int64(expected) {
		return true, fmt.Sprintf("%d/%d", actual, expected)
	}
	return false, fmt.Sprintf("expected %d but got %d", expected, actual)
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

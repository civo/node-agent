package health

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

const gpuResourceName = "nvidia.com/gpu"

// gpuChecker reports healthy when the node's allocatable GPU count
// matches the desired count. If desiredCount is 0 the check always passes.
type gpuChecker struct {
	desiredCount int
}

func (c *gpuChecker) Name() string             { return "GPU" }
func (c *gpuChecker) Threshold() time.Duration { return 10 * time.Minute }

func (c *gpuChecker) Check(node *corev1.Node) bool {
	if c.desiredCount == 0 {
		return true
	}

	quantity, exists := node.Status.Allocatable[gpuResourceName]
	if !exists || quantity.IsZero() {
		return false
	}

	gpuCount, ok := quantity.AsInt64()
	if !ok {
		return false
	}

	return gpuCount == int64(c.desiredCount)
}

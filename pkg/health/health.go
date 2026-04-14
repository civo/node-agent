package health

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

// HealthChecker determines whether a single aspect of a node is healthy.
type HealthChecker interface {
	// Name returns a human-readable identifier for this checker (e.g. "NodeReady").
	Name() string
	// Check returns whether the node is healthy and a reason string.
	// On success the reason is empty. On failure it describes what went wrong.
	Check(node *corev1.Node) (healthy bool, reason string)
	// Threshold returns how long this checker must continuously fail
	// before a recovery action is triggered.
	Threshold() time.Duration
}

// NewDefaultCheckers returns the enabled health checkers.
// GPU checker auto-skips non-GPU nodes by checking the nvidia.com/gpu.count label.
// Cilium checker auto-skips nodes without CiliumAgentIsReady condition.
func NewDefaultCheckers() []HealthChecker {
	return []HealthChecker{
		&nodeReadyChecker{},
		&diskPressureChecker{},
		&ciliumChecker{},
		&gpuChecker{},
	}
}

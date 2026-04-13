package health

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

// HealthChecker determines whether a single aspect of a node is healthy.
type HealthChecker interface {
	// Name returns a human-readable identifier for this checker (e.g. "NodeReady").
	Name() string
	// Check returns true if the node is healthy for this checker's concern.
	Check(node *corev1.Node) bool
	// Threshold returns how long this checker must continuously fail
	// before a recovery action is triggered.
	Threshold() time.Duration
}

// NewDefaultCheckers returns the enabled health checkers.
// GPU checker is always included; it auto-skips non-GPU nodes
// by checking for the nvidia.com/gpu.count label.
func NewDefaultCheckers() []HealthChecker {
	return []HealthChecker{
		&nodeReadyChecker{},
		&diskPressureChecker{},
		&gpuChecker{},
	}
}

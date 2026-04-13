package health

import corev1 "k8s.io/api/core/v1"

// HealthChecker determines whether a single aspect of a node is healthy.
type HealthChecker interface {
	// Name returns a human-readable identifier for this checker (e.g. "NodeReady").
	Name() string
	// Check returns true if the node is healthy for this checker's concern.
	Check(node *corev1.Node) bool
}

// NewDefaultCheckers returns the enabled health checkers.
// GPUChecker is included only when desiredGPUCount > 0.
func NewDefaultCheckers(desiredGPUCount int) []HealthChecker {
	checkers := []HealthChecker{
		&nodeReadyChecker{},
		&diskPressureChecker{},
	}
	if desiredGPUCount > 0 {
		checkers = append(checkers, &gpuChecker{desiredCount: desiredGPUCount})
	}
	return checkers
}

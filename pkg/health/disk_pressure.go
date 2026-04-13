package health

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

// diskPressureChecker reports healthy when the node does not have disk pressure.
type diskPressureChecker struct{}

func (c *diskPressureChecker) Name() string             { return "DiskPressure" }
func (c *diskPressureChecker) Threshold() time.Duration { return 30 * time.Minute }

func (c *diskPressureChecker) Check(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeDiskPressure {
			return cond.Status != corev1.ConditionTrue
		}
	}
	return true
}

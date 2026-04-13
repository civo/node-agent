package health

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

// nodeReadyChecker reports healthy when the node's NodeReady condition is True.
type nodeReadyChecker struct{}

func (c *nodeReadyChecker) Name() string             { return "NodeReady" }
func (c *nodeReadyChecker) Threshold() time.Duration { return 10 * time.Minute }

func (c *nodeReadyChecker) Check(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

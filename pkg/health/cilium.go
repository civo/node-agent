package health

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

const ciliumAgentConditionType corev1.NodeConditionType = "CiliumAgentIsReady"

// ciliumChecker reports healthy when the Cilium agent is ready.
// If the CiliumAgentIsReady condition is not present (Cilium not installed),
// the check is skipped and returns healthy.
type ciliumChecker struct{}

func (c *ciliumChecker) Name() string             { return "CiliumAgent" }
func (c *ciliumChecker) Threshold() time.Duration { return 10 * time.Minute }

func (c *ciliumChecker) Check(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == ciliumAgentConditionType {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return true
}

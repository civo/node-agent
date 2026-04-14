package health

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	ciliumReadyReason = "CiliumIsUp"
	ciliumThreshold   = 10 * time.Minute
)

// ciliumChecker reports healthy when the Cilium-managed NetworkUnavailable
// condition is False. If the condition's reason is not "CiliumIsUp"
// (i.e. a different CNI manages the condition), the check is skipped.
type ciliumChecker struct{}

func (c *ciliumChecker) Name() string             { return "CiliumAgent" }
func (c *ciliumChecker) Threshold() time.Duration { return ciliumThreshold }

func (c *ciliumChecker) Check(node *corev1.Node) (bool, string) {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeNetworkUnavailable {
			if cond.Reason != ciliumReadyReason {
				return true, cond.Reason
			}
			return cond.Status == corev1.ConditionFalse, cond.Reason
		}
	}
	return true, "NetworkUnavailable condition not found"
}

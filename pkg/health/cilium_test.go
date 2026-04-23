package health

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCiliumChecker_Threshold(t *testing.T) {
	c := &ciliumChecker{}
	if got := c.Threshold(); got != 10*time.Minute {
		t.Errorf("got %v, want %v", got, 10*time.Minute)
	}
}

func TestCiliumChecker_Name(t *testing.T) {
	c := &ciliumChecker{}
	if got := c.Name(); got != "CiliumAgent" {
		t.Errorf("got %q, want %q", got, "CiliumAgent")
	}
}

func TestCiliumChecker_Check(t *testing.T) {
	tests := []struct {
		description string
		node        *corev1.Node
		want        bool
	}{
		{
			description: "returns true when NetworkUnavailable is False with CiliumIsUp",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeNetworkUnavailable,
							Status: corev1.ConditionFalse,
							Reason: ciliumReadyReason,
						},
					},
				},
			},
			want: true,
		},
		{
			description: "returns false when NetworkUnavailable is True with CiliumIsUp",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeNetworkUnavailable,
							Status: corev1.ConditionTrue,
							Reason: ciliumReadyReason,
						},
					},
				},
			},
			want: false,
		},
		{
			description: "skips check when NetworkUnavailable has non-Cilium reason",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeNetworkUnavailable,
							Status: corev1.ConditionFalse,
							Reason: "FlannelIsUp",
						},
					},
				},
			},
			want: true,
		},
		{
			description: "returns true when condition is absent",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{},
				},
			},
			want: true,
		},
	}

	c := &ciliumChecker{}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			if got, _ := c.Check(test.node); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

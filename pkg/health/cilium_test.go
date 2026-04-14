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
		name string
		node *corev1.Node
		want bool
	}{
		{
			name: "Returns true when NetworkUnavailable is False with CiliumIsUp",
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
			name: "Returns false when NetworkUnavailable is True with CiliumIsUp",
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
			name: "Returns true when NetworkUnavailable has non-Cilium reason (skip)",
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
			name: "Returns true when condition is absent",
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := c.Check(tt.node); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

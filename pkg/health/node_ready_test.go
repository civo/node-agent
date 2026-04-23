package health

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNodeReadyChecker_Threshold(t *testing.T) {
	c := &nodeReadyChecker{}
	if got := c.Threshold(); got != 5*time.Minute {
		t.Errorf("got %v, want %v", got, 5*time.Minute)
	}
}

func TestNodeReadyChecker_Name(t *testing.T) {
	c := &nodeReadyChecker{}
	if got := c.Name(); got != "NodeReady" {
		t.Errorf("got %q, want %q", got, "NodeReady")
	}
}

func TestNodeReadyChecker_Check(t *testing.T) {
	tests := []struct {
		description string
		node        *corev1.Node
		want        bool
	}{
		{
			description: "returns true when NodeReady condition is True",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
				},
			},
			want: true,
		},
		{
			description: "returns false when NodeReady condition is False",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
					},
				},
			},
			want: false,
		},
		{
			description: "returns false when no conditions present",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{},
				},
			},
			want: false,
		},
		{
			description: "returns false when only non-NodeReady conditions present",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
					},
				},
			},
			want: false,
		},
	}

	c := &nodeReadyChecker{}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			if got, _ := c.Check(test.node); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

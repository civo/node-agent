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
		name string
		node *corev1.Node
		want bool
	}{
		{
			name: "Returns true when NodeReady condition is True",
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
			name: "Returns false when NodeReady condition is False",
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
			name: "Returns false when no conditions present",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{},
				},
			},
			want: false,
		},
		{
			name: "Returns false when only non-NodeReady conditions present",
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := c.Check(tt.node); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

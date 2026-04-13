package health

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDiskPressureChecker_Name(t *testing.T) {
	c := &diskPressureChecker{}
	if got := c.Name(); got != "DiskPressure" {
		t.Errorf("got %q, want %q", got, "DiskPressure")
	}
}

func TestDiskPressureChecker_Check(t *testing.T) {
	tests := []struct {
		name string
		node *corev1.Node
		want bool
	}{
		{
			name: "Returns true when DiskPressure is False (no pressure)",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
					},
				},
			},
			want: true,
		},
		{
			name: "Returns false when DiskPressure is True (under pressure)",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeDiskPressure, Status: corev1.ConditionTrue},
					},
				},
			},
			want: false,
		},
		{
			name: "Returns true when no conditions present",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{},
				},
			},
			want: true,
		},
		{
			name: "Returns true when only non-DiskPressure conditions present",
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
	}

	c := &diskPressureChecker{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.Check(tt.node); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

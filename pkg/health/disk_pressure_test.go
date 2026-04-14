package health

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDiskPressureChecker_Threshold(t *testing.T) {
	c := &diskPressureChecker{}
	if got := c.Threshold(); got != 30*time.Minute {
		t.Errorf("got %v, want %v", got, 30*time.Minute)
	}
}

func TestDiskPressureChecker_Name(t *testing.T) {
	c := &diskPressureChecker{}
	if got := c.Name(); got != "DiskPressure" {
		t.Errorf("got %q, want %q", got, "DiskPressure")
	}
}

func TestDiskPressureChecker_Check(t *testing.T) {
	tests := []struct {
		description string
		node        *corev1.Node
		want        bool
	}{
		{
			description: "returns true when DiskPressure is False (no pressure)",
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
			description: "returns false when DiskPressure is True (under pressure)",
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
			description: "returns true when no conditions present",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{},
				},
			},
			want: true,
		},
		{
			description: "returns true when only non-DiskPressure conditions present",
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
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			if got, _ := c.Check(test.node); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

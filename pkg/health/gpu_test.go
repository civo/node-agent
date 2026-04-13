package health

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGPUChecker_Name(t *testing.T) {
	c := NewGPUChecker(8)
	if got := c.Name(); got != "GPU" {
		t.Errorf("got %q, want %q", got, "GPU")
	}
}

func TestGPUChecker_Check(t *testing.T) {
	tests := []struct {
		name    string
		desired int
		node    *corev1.Node
		want    bool
	}{
		{
			name:    "Returns true when GPU count matches desired",
			desired: 8,
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						gpuResourceName: resource.MustParse("8"),
					},
				},
			},
			want: true,
		},
		{
			name:    "Returns true when desired is 0 (check skipped)",
			desired: 0,
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{},
				},
			},
			want: true,
		},
		{
			name:    "Returns false when GPU count is less than desired",
			desired: 8,
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						gpuResourceName: resource.MustParse("7"),
					},
				},
			},
			want: false,
		},
		{
			name:    "Returns false when GPU count is zero",
			desired: 8,
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						gpuResourceName: resource.MustParse("0"),
					},
				},
			},
			want: false,
		},
		{
			name:    "Returns false when no GPU resource in allocatable",
			desired: 8,
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewGPUChecker(tt.desired)
			if got := c.Check(tt.node); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewDefaultCheckers(t *testing.T) {
	t.Run("GPU disabled when desiredCount is 0", func(t *testing.T) {
		checkers := NewDefaultCheckers(0)
		if len(checkers) != 2 {
			t.Fatalf("expected 2 checkers, got %d", len(checkers))
		}
		if checkers[0].Name() != "NodeReady" {
			t.Errorf("expected NodeReady checker first, got %q", checkers[0].Name())
		}
		if checkers[1].Name() != "DiskPressure" {
			t.Errorf("expected DiskPressure checker second, got %q", checkers[1].Name())
		}
	})

	t.Run("GPU enabled when desiredCount is positive", func(t *testing.T) {
		checkers := NewDefaultCheckers(8)
		if len(checkers) != 3 {
			t.Fatalf("expected 3 checkers, got %d", len(checkers))
		}
		if checkers[0].Name() != "NodeReady" {
			t.Errorf("expected NodeReady checker first, got %q", checkers[0].Name())
		}
		if checkers[1].Name() != "DiskPressure" {
			t.Errorf("expected DiskPressure checker second, got %q", checkers[1].Name())
		}
		if checkers[2].Name() != "GPU" {
			t.Errorf("expected GPU checker third, got %q", checkers[2].Name())
		}
	})
}

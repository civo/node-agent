package health

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGPUChecker_Name(t *testing.T) {
	c := &gpuChecker{}
	if got := c.Name(); got != "GPU" {
		t.Errorf("got %q, want %q", got, "GPU")
	}
}

func TestGPUChecker_Check(t *testing.T) {
	tests := []struct {
		name string
		node *corev1.Node
		want bool
	}{
		{
			name: "Returns true when allocatable matches label count",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node-01",
					Labels: map[string]string{gpuCountLabel: "8"},
				},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						gpuResourceName: resource.MustParse("8"),
					},
				},
			},
			want: true,
		},
		{
			name: "Returns true when gpu.count label is absent (non-GPU node)",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{},
				},
			},
			want: true,
		},
		{
			name: "Returns true when gpu.count label is 0",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node-01",
					Labels: map[string]string{gpuCountLabel: "0"},
				},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{},
				},
			},
			want: true,
		},
		{
			name: "Returns false when allocatable is less than label count",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node-01",
					Labels: map[string]string{gpuCountLabel: "8"},
				},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						gpuResourceName: resource.MustParse("7"),
					},
				},
			},
			want: false,
		},
		{
			name: "Returns false when allocatable GPU is zero",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node-01",
					Labels: map[string]string{gpuCountLabel: "8"},
				},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						gpuResourceName: resource.MustParse("0"),
					},
				},
			},
			want: false,
		},
		{
			name: "Returns false when allocatable GPU resource is missing",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node-01",
					Labels: map[string]string{gpuCountLabel: "8"},
				},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{},
				},
			},
			want: false,
		},
		{
			name: "Returns true when gpu.count label is invalid",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node-01",
					Labels: map[string]string{gpuCountLabel: "invalid"},
				},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{},
				},
			},
			want: true,
		},
	}

	c := &gpuChecker{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.Check(tt.node); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewDefaultCheckers(t *testing.T) {
	checkers := NewDefaultCheckers()
	if len(checkers) != 4 {
		t.Fatalf("expected 4 checkers, got %d", len(checkers))
	}
	expected := []string{"NodeReady", "DiskPressure", "CiliumAgent", "GPU"}
	for i, name := range expected {
		if checkers[i].Name() != name {
			t.Errorf("checkers[%d]: expected %q, got %q", i, name, checkers[i].Name())
		}
	}
}

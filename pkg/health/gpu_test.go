package health

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGPUChecker_Threshold(t *testing.T) {
	c := &gpuChecker{}
	if got := c.Threshold(); got != 10*time.Minute {
		t.Errorf("got %v, want %v", got, 10*time.Minute)
	}
}

func TestHasGPU(t *testing.T) {
	tests := []struct {
		description string
		node        *corev1.Node
		want        bool
	}{
		{
			description: "returns true when gpu.count label is positive",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node-01",
					Labels: map[string]string{gpuCountLabel: "8"},
				},
			},
			want: true,
		},
		{
			description: "returns false when gpu.count label is absent",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
			},
			want: false,
		},
		{
			description: "returns false when gpu.count label is 0",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node-01",
					Labels: map[string]string{gpuCountLabel: "0"},
				},
			},
			want: false,
		},
		{
			description: "returns false when gpu.count label is invalid",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node-01",
					Labels: map[string]string{gpuCountLabel: "invalid"},
				},
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			if got := HasGPU(test.node); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestGPUChecker_Name(t *testing.T) {
	c := &gpuChecker{}
	if got := c.Name(); got != "GPU" {
		t.Errorf("got %q, want %q", got, "GPU")
	}
}

func TestGPUChecker_Check(t *testing.T) {
	tests := []struct {
		description string
		node        *corev1.Node
		want        bool
	}{
		{
			description: "returns true when allocatable matches label count",
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
			description: "returns true when gpu.count label is absent (non-GPU node)",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{},
				},
			},
			want: true,
		},
		{
			description: "returns true when gpu.count label is 0",
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
			description: "returns false when allocatable is less than label count",
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
			description: "returns false when allocatable GPU is zero",
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
			description: "returns false when allocatable GPU resource is missing",
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
			description: "returns true when gpu.count label is invalid",
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
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			if got, _ := c.Check(test.node); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
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

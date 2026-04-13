package watcher

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/civo/node-agent/pkg/health"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/fake"
)

// --- Test helpers ---

// fakeNodeLister implements listerscorev1.NodeLister for testing.
type fakeNodeLister struct {
	nodes []*corev1.Node
	err   error
}

func (l *fakeNodeLister) List(selector labels.Selector) ([]*corev1.Node, error) {
	if l.err != nil {
		return nil, l.err
	}
	return l.nodes, nil
}

func (l *fakeNodeLister) Get(name string) (*corev1.Node, error) {
	for _, n := range l.nodes {
		if n.Name == name {
			return n, nil
		}
	}
	return nil, fmt.Errorf("node %q not found", name)
}

// mockExecutor implements operation.Executor for testing.
type mockExecutor struct {
	rebootFunc func(ctx context.Context, nodeName string) error
	calls      []string
}

func (m *mockExecutor) Reboot(ctx context.Context, nodeName string) error {
	m.calls = append(m.calls, nodeName)
	if m.rebootFunc != nil {
		return m.rebootFunc(ctx, nodeName)
	}
	return nil
}

// alwaysFailChecker is a HealthChecker that always reports unhealthy.
type alwaysFailChecker struct{ name string }

func (c *alwaysFailChecker) Name() string            { return c.name }
func (c *alwaysFailChecker) Check(*corev1.Node) bool { return false }

// --- Test variables ---

var (
	testClusterID               = "test-cluster-123"
	testNodePoolID              = "test-node-pool"
	testNodeDesiredGPUCount     = "8"
	testRebootTimeWindowMinutes = time.Duration(40)
)

// newTestNode creates a node for testing with common defaults.
func newTestNode(name string, ready corev1.ConditionStatus, gpuCount int) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				nodePoolLabelKey: testNodePoolID,
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: ready},
			},
		},
	}
	if gpuCount > 0 {
		node.Status.Allocatable = corev1.ResourceList{
			gpuResourceName: resource.MustParse(strconv.Itoa(gpuCount)),
		}
	}
	return node
}

// newTestWatcher creates a watcher with sensible test defaults and the given options.
func newTestWatcher(t *testing.T, opts ...Option) *watcher {
	t.Helper()
	baseOpts := []Option{
		WithKubernetesClient(fake.NewSimpleClientset()),
		WithExecutor(&mockExecutor{}),
	}
	w, err := NewWatcher(t.Context(),
		testClusterID, testNodePoolID,
		append(baseOpts, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return w.(*watcher)
}

// --- TestNew ---

func TestNew(t *testing.T) {
	type args struct {
		clusterID  string
		nodePoolID string
		opts       []Option
	}
	type test struct {
		name      string
		args      args
		checkFunc func(*watcher) error
		wantErr   bool
	}

	tests := []test{
		{
			name: "Returns no error when given valid input",
			args: args{
				clusterID:  testClusterID,
				nodePoolID: testNodePoolID,
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithExecutor(&mockExecutor{}),
					WithDesiredGPUCount(testNodeDesiredGPUCount),
				},
			},
			checkFunc: func(w *watcher) error {
				if w.clusterID != testClusterID {
					return fmt.Errorf("clusterID mismatch: got %s, want %s", w.clusterID, testClusterID)
				}
				cnt, err := strconv.Atoi(testNodeDesiredGPUCount)
				if err != nil {
					return err
				}
				if w.nodeDesiredGPUCount != cnt {
					return fmt.Errorf("nodeDesiredGPUCount mismatch: got %d, want %d", w.nodeDesiredGPUCount, cnt)
				}
				if w.nodeSelector == nil || w.nodeSelector.MatchLabels[nodePoolLabelKey] != testNodePoolID {
					return fmt.Errorf("nodeSelector mismatch: got %v, want %s", w.nodeSelector, testNodePoolID)
				}
				if w.client == nil {
					return fmt.Errorf("client is nil")
				}
				if w.rebootTimeWindowMinutes != testRebootTimeWindowMinutes {
					return fmt.Errorf("rebootTimeWindowMinutes mismatch: got %v, want %v", w.rebootTimeWindowMinutes, testRebootTimeWindowMinutes)
				}
				if !w.monitorOnly {
					return fmt.Errorf("monitorOnly should default to true")
				}
				if w.states == nil {
					return fmt.Errorf("states is nil")
				}
				if w.nowFunc == nil {
					return fmt.Errorf("nowFunc is nil")
				}
				return nil
			},
		},
		{
			name: "Returns no error when input is invalid, but default value is set",
			args: args{
				clusterID:  testClusterID,
				nodePoolID: testNodePoolID,
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithExecutor(&mockExecutor{}),
					WithDesiredGPUCount("invalid"),
					WithDesiredGPUCount("-1"),
					WithRebootTimeWindowMinutes("invalid time"),
					WithRebootTimeWindowMinutes("0"),
				},
			},
			checkFunc: func(w *watcher) error {
				if w.nodeDesiredGPUCount != 0 {
					return fmt.Errorf("nodeDesiredGPUCount mismatch: got %d, want %d", w.nodeDesiredGPUCount, 0)
				}
				if w.rebootTimeWindowMinutes != testRebootTimeWindowMinutes {
					return fmt.Errorf("rebootTimeWindowMinutes mismatch: got %v, want %v", w.rebootTimeWindowMinutes, testRebootTimeWindowMinutes)
				}
				return nil
			},
		},
		{
			name: "Returns an error when clusterID is missing",
			args: args{
				nodePoolID: testNodePoolID,
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithExecutor(&mockExecutor{}),
				},
			},
			wantErr: true,
		},
		{
			name: "Returns an error when nodePoolID is missing",
			args: args{
				clusterID: testClusterID,
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithExecutor(&mockExecutor{}),
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w, err := NewWatcher(t.Context(),
				test.args.clusterID,
				test.args.nodePoolID,
				test.args.opts...)
			if (err != nil) != test.wantErr {
				t.Errorf("error = %v, wantErr %v", err, test.wantErr)
			}

			if !test.wantErr {
				if w == nil {
					t.Errorf("expected non-nil object, but got nil")
					return
				}
				obj := w.(*watcher)
				if test.checkFunc != nil {
					if err := test.checkFunc(obj); err != nil {
						t.Errorf("checkFunc error: %v", err)
					}
				}
			}
		})
	}
}

// --- State machine transition tests ---

func TestRun_HealthyNodeStaysHealthy(t *testing.T) {
	node := newTestNode("node-01", corev1.ConditionTrue, 8)
	w := newTestWatcher(t,
		WithNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers(8)),
	)

	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	state, ok := w.states.Get("node-01")
	if !ok {
		t.Fatal("state should exist for node-01")
	}
	if state.Phase() != PhaseHealthy {
		t.Errorf("got phase %v, want PhaseHealthy", state.Phase())
	}
}

func TestRun_UnhealthyDetection(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	node := newTestNode("node-01", corev1.ConditionFalse, 8)
	w := newTestWatcher(t,
		WithNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers(8)),
		WithNowFunc(func() time.Time { return now }),
	)

	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	state, _ := w.states.Get("node-01")
	if state.Phase() != PhaseUnhealthy {
		t.Errorf("got phase %v, want PhaseUnhealthy", state.Phase())
	}
	if !state.UnhealthySince().Equal(now) {
		t.Errorf("got unhealthySince %v, want %v", state.UnhealthySince(), now)
	}
}

func TestRun_RebootTriggerActiveMode(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	node := newTestNode("node-01", corev1.ConditionFalse, 8)
	exec := &mockExecutor{}
	w := newTestWatcher(t,
		WithNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers(8)),
		WithExecutor(exec),
		WithMonitorOnly(false),
		WithUnhealthyThresholdMinutes("10"),
		WithNowFunc(func() time.Time { return now }),
	)

	// First run: detect unhealthy.
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	// Advance past threshold.
	now = now.Add(11 * time.Minute)

	// Second run: should trigger reboot.
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	state, _ := w.states.Get("node-01")
	if state.Phase() != PhaseWaitingReboot {
		t.Errorf("got phase %v, want PhaseWaitingReboot", state.Phase())
	}
	if state.RebootCount() != 1 {
		t.Errorf("got rebootCount %d, want 1", state.RebootCount())
	}
	if len(exec.calls) != 1 || exec.calls[0] != "node-01" {
		t.Errorf("expected 1 reboot call for node-01, got %v", exec.calls)
	}
}

func TestRun_RebootSkippedInReportMode(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	node := newTestNode("node-01", corev1.ConditionFalse, 8)
	exec := &mockExecutor{}
	w := newTestWatcher(t,
		WithNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers(8)),
		WithExecutor(exec),
		WithMonitorOnly(true),
		WithUnhealthyThresholdMinutes("10"),
		WithNowFunc(func() time.Time { return now }),
	)

	// First run: detect unhealthy.
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	// Advance past threshold.
	now = now.Add(11 * time.Minute)

	// Second run: should transition to WaitingReboot but NOT call executor.
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	state, _ := w.states.Get("node-01")
	if state.Phase() != PhaseWaitingReboot {
		t.Errorf("got phase %v, want PhaseWaitingReboot", state.Phase())
	}
	if len(exec.calls) != 0 {
		t.Errorf("expected no reboot calls in report mode, got %v", exec.calls)
	}
}

func TestRun_RecoveryAfterReboot(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	node := newTestNode("node-01", corev1.ConditionFalse, 8)
	w := newTestWatcher(t,
		WithNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers(8)),
		WithMonitorOnly(false),
		WithUnhealthyThresholdMinutes("10"),
		WithNowFunc(func() time.Time { return now }),
	)

	// Run 1: detect unhealthy.
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Run 2: trigger reboot.
	now = now.Add(11 * time.Minute)
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	// Node recovers.
	node.Status.Conditions[0].Status = corev1.ConditionTrue
	now = now.Add(5 * time.Minute)

	// Run 3: should detect recovery.
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	state, _ := w.states.Get("node-01")
	if state.Phase() != PhaseHealthy {
		t.Errorf("got phase %v, want PhaseHealthy", state.Phase())
	}
	if state.RebootCount() != 0 {
		t.Errorf("got rebootCount %d, want 0 after recovery", state.RebootCount())
	}
}

func TestRun_RebootRetry(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	node := newTestNode("node-01", corev1.ConditionFalse, 8)
	exec := &mockExecutor{}
	w := newTestWatcher(t,
		WithNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers(8)),
		WithExecutor(exec),
		WithMonitorOnly(false),
		WithUnhealthyThresholdMinutes("10"),
		WithRebootTimeWindowMinutes("40"),
		WithNowFunc(func() time.Time { return now }),
	)

	// Run 1: detect unhealthy.
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Run 2: trigger first reboot.
	now = now.Add(11 * time.Minute)
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	// Still unhealthy, but within reboot window → no retry.
	now = now.Add(30 * time.Minute)
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(exec.calls) != 1 {
		t.Fatalf("expected 1 reboot call before window expires, got %d", len(exec.calls))
	}

	// Advance past reboot window → retry.
	now = now.Add(11 * time.Minute)
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	state, _ := w.states.Get("node-01")
	if state.Phase() != PhaseWaitingReboot {
		t.Errorf("got phase %v, want PhaseWaitingReboot", state.Phase())
	}
	if state.RebootCount() != 2 {
		t.Errorf("got rebootCount %d, want 2", state.RebootCount())
	}
	if len(exec.calls) != 2 {
		t.Errorf("expected 2 reboot calls, got %d", len(exec.calls))
	}
}

func TestRun_GPUMismatchTriggersUnhealthy(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	node := newTestNode("node-01", corev1.ConditionTrue, 7) // 7 GPUs, desired 8
	w := newTestWatcher(t,
		WithNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers(8)),
		WithNowFunc(func() time.Time { return now }),
	)

	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	state, _ := w.states.Get("node-01")
	if state.Phase() != PhaseUnhealthy {
		t.Errorf("got phase %v, want PhaseUnhealthy", state.Phase())
	}
	if !state.IsGPUNode() {
		t.Error("expected isGPUNode to be true for node with 7 GPUs")
	}
}

func TestRun_RebootErrorContinuesProcessing(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	node := newTestNode("node-01", corev1.ConditionFalse, 0)
	exec := &mockExecutor{
		rebootFunc: func(_ context.Context, _ string) error {
			return fmt.Errorf("reboot API error")
		},
	}
	w := newTestWatcher(t,
		WithNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers(0)),
		WithExecutor(exec),
		WithMonitorOnly(false),
		WithUnhealthyThresholdMinutes("10"),
		WithNowFunc(func() time.Time { return now }),
	)

	// Run 1: detect unhealthy.
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Run 2: threshold exceeded, reboot fails → should not error out, stays PhaseUnhealthy.
	now = now.Add(11 * time.Minute)
	if err := w.run(t.Context()); err != nil {
		t.Fatal("run should not return error on reboot failure")
	}

	state, _ := w.states.Get("node-01")
	if state.Phase() != PhaseUnhealthy {
		t.Errorf("got phase %v, want PhaseUnhealthy (reboot failed, no transition)", state.Phase())
	}
}

func TestRun_NodeListError(t *testing.T) {
	w := newTestWatcher(t,
		WithNodeLister(&fakeNodeLister{err: fmt.Errorf("list error")}),
		WithCheckers(health.NewDefaultCheckers(0)),
	)

	if err := w.run(t.Context()); err == nil {
		t.Error("expected error from node list failure")
	}
}

func TestRun_StaleStateCleanup(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	node := newTestNode("node-01", corev1.ConditionFalse, 0)
	lister := &fakeNodeLister{nodes: []*corev1.Node{node}}
	w := newTestWatcher(t,
		WithNodeLister(lister),
		WithCheckers(health.NewDefaultCheckers(0)),
		WithNowFunc(func() time.Time { return now }),
	)

	// Run 1: detect node-01 unhealthy.
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, ok := w.states.Get("node-01"); !ok {
		t.Fatal("state should exist for node-01")
	}

	// Node removed from cluster.
	lister.nodes = nil

	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	if _, ok := w.states.Get("node-01"); ok {
		t.Error("state for node-01 should be cleaned up after removal")
	}
}

func TestRun_UnhealthyWithinThresholdNoReboot(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	node := newTestNode("node-01", corev1.ConditionFalse, 0)
	exec := &mockExecutor{}
	w := newTestWatcher(t,
		WithNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers(0)),
		WithExecutor(exec),
		WithMonitorOnly(false),
		WithUnhealthyThresholdMinutes("10"),
		WithNowFunc(func() time.Time { return now }),
	)

	// Run 1: detect unhealthy.
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	// Run 2: still within threshold → no reboot.
	now = now.Add(5 * time.Minute)
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	state, _ := w.states.Get("node-01")
	if state.Phase() != PhaseUnhealthy {
		t.Errorf("got phase %v, want PhaseUnhealthy", state.Phase())
	}
	if len(exec.calls) != 0 {
		t.Errorf("expected no reboot calls within threshold, got %v", exec.calls)
	}
}

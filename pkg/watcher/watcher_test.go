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
type alwaysFailChecker struct {
	name      string
	threshold time.Duration
}

func (c *alwaysFailChecker) Name() string                      { return c.name }
func (c *alwaysFailChecker) Check(*corev1.Node) (bool, string) { return false, "always fail" }
func (c *alwaysFailChecker) Threshold() time.Duration          { return c.threshold }

// --- Test variables ---

var (
	testNodePoolID        = "test-node-pool"
	testRebootWaitMinutes = time.Duration(10) * time.Minute
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
		node.Labels["nvidia.com/gpu.count"] = strconv.Itoa(gpuCount)
		node.Status.Allocatable = corev1.ResourceList{
			"nvidia.com/gpu": resource.MustParse(strconv.Itoa(gpuCount)),
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
	w, err := NewWatcher(append(baseOpts, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return w.(*watcher)
}

// --- TestNew ---

func TestNew(t *testing.T) {
	type args struct {
		opts []Option
	}
	type test struct {
		description string
		args        args
		checkFunc   func(*watcher) error
		wantErr     bool
	}

	tests := []test{
		{
			description: "returns no error when given valid input",
			args: args{
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithExecutor(&mockExecutor{}),
					WithNodePoolIDs(testNodePoolID),
				},
			},
			checkFunc: func(w *watcher) error {
				if w.nodeLabelSelector == nil || w.nodeLabelSelector.MatchLabels[nodePoolLabelKey] != testNodePoolID {
					return fmt.Errorf("nodeLabelSelector mismatch: got %v, want %s", w.nodeLabelSelector, testNodePoolID)
				}
				if w.client == nil {
					return fmt.Errorf("client is nil")
				}
				if w.rebootWaitMinutes != testRebootWaitMinutes {
					return fmt.Errorf("rebootTimeWindowMinutes mismatch: got %v, want %v", w.rebootWaitMinutes, testRebootWaitMinutes)
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
			description: "returns no error when input is invalid, but default value is set",
			args: args{
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithExecutor(&mockExecutor{}),
					WithRebootWaitMinutes("invalid time"),
					WithRebootWaitMinutes("0"),
				},
			},
			checkFunc: func(w *watcher) error {
				if w.rebootWaitMinutes != testRebootWaitMinutes {
					return fmt.Errorf("rebootTimeWindowMinutes mismatch: got %v, want %v", w.rebootWaitMinutes, testRebootWaitMinutes)
				}
				return nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			w, err := NewWatcher(test.args.opts...)
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
		withNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers()),
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
		withNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers()),
		withNowFunc(func() time.Time { return now }),
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
		withNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers()),
		WithExecutor(exec),
		WithMonitorOnly("false"),
		withNowFunc(func() time.Time { return now }),
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
		withNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers()),
		WithExecutor(exec),
		WithMonitorOnly("true"),
		withNowFunc(func() time.Time { return now }),
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
		withNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers()),
		WithMonitorOnly("false"),
		withNowFunc(func() time.Time { return now }),
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
		withNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers()),
		WithExecutor(exec),
		WithMonitorOnly("false"),
		WithGPURebootWaitMinutes("40"),
		withNowFunc(func() time.Time { return now }),
	)

	// Run 1: detect unhealthy.
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Run 2: trigger first reboot (GPU checker threshold is 10min).
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
	node := newTestNode("node-01", corev1.ConditionTrue, 8)
	// Simulate GPU failure: label says 8 but only 7 allocatable.
	node.Status.Allocatable["nvidia.com/gpu"] = resource.MustParse("7")
	w := newTestWatcher(t,
		withNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers()),
		withNowFunc(func() time.Time { return now }),
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
		withNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers()),
		WithExecutor(exec),
		WithMonitorOnly("false"),
		withNowFunc(func() time.Time { return now }),
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
		withNodeLister(&fakeNodeLister{err: fmt.Errorf("list error")}),
		WithCheckers(health.NewDefaultCheckers()),
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
		withNodeLister(lister),
		WithCheckers(health.NewDefaultCheckers()),
		withNowFunc(func() time.Time { return now }),
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
		withNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers()),
		WithExecutor(exec),
		WithMonitorOnly("false"),
		withNowFunc(func() time.Time { return now }),
	)

	// Run 1: detect unhealthy.
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	// Run 2: still within threshold → no reboot.
	now = now.Add(3 * time.Minute)
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

func TestRun_RebootRetryLimitExceeded_TransitionsToFailed(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	node := newTestNode("node-01", corev1.ConditionFalse, 0)
	exec := &mockExecutor{}
	w := newTestWatcher(t,
		withNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers()),
		WithExecutor(exec),
		WithMonitorOnly("false"),
		WithRebootWaitMinutes("10"),
		WithMaxRebootRetries("3"),
		withNowFunc(func() time.Time { return now }),
	)

	// Run 1: detect unhealthy.
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Run 2: threshold exceeded → first reboot (rebootCount=1).
	now = now.Add(6 * time.Minute)
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Reboot retries 2 and 3.
	for i := 0; i < 2; i++ {
		now = now.Add(11 * time.Minute)
		if err := w.run(t.Context()); err != nil {
			t.Fatal(err)
		}
	}

	state, _ := w.states.Get("node-01")
	if state.RebootCount() != 3 {
		t.Fatalf("expected rebootCount=3 after 3 reboots, got %d", state.RebootCount())
	}
	if state.Phase() != PhaseWaitingReboot {
		t.Fatalf("expected PhaseWaitingReboot after %d reboots, got %v", state.RebootCount(), state.Phase())
	}

	// Next retry should exceed the limit → PhaseFailed.
	now = now.Add(11 * time.Minute)
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	state, _ = w.states.Get("node-01")
	if state.Phase() != PhaseFailed {
		t.Errorf("got phase %v, want PhaseFailed", state.Phase())
	}
	if len(exec.calls) != 3 {
		t.Errorf("expected exactly 3 reboot calls (no further reboots after Failed), got %d", len(exec.calls))
	}
}

func TestRun_MonitorOnlyDoesNotIncrementRebootCount(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	node := newTestNode("node-01", corev1.ConditionFalse, 0)
	w := newTestWatcher(t,
		withNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers()),
		WithMonitorOnly("true"),
		WithRebootWaitMinutes("10"),
		WithMaxRebootRetries("3"),
		withNowFunc(func() time.Time { return now }),
	)

	// Run the state machine through several reboot cycles.
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		now = now.Add(11 * time.Minute)
		if err := w.run(t.Context()); err != nil {
			t.Fatal(err)
		}
	}

	state, _ := w.states.Get("node-01")
	if state.RebootCount() != 0 {
		t.Errorf("rebootCount should stay 0 in monitor-only mode, got %d", state.RebootCount())
	}
	if state.Phase() == PhaseFailed {
		t.Errorf("monitor-only mode must not transition to PhaseFailed; got phase %v", state.Phase())
	}
}

func TestRun_RecoverFromFailed(t *testing.T) {
	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	node := newTestNode("node-01", corev1.ConditionFalse, 0)
	exec := &mockExecutor{}
	w := newTestWatcher(t,
		withNodeLister(&fakeNodeLister{nodes: []*corev1.Node{node}}),
		WithCheckers(health.NewDefaultCheckers()),
		WithExecutor(exec),
		WithMonitorOnly("false"),
		WithRebootWaitMinutes("10"),
		WithMaxRebootRetries("1"),
		withNowFunc(func() time.Time { return now }),
	)

	// Drive to Failed: detect → first reboot (rebootCount=1) → retry exceeds limit.
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(6 * time.Minute)
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(11 * time.Minute)
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	if st, _ := w.states.Get("node-01"); st.Phase() != PhaseFailed {
		t.Fatalf("expected PhaseFailed, got %v", st.Phase())
	}

	// Node recovers.
	node.Status.Conditions[0].Status = corev1.ConditionTrue
	now = now.Add(time.Minute)
	if err := w.run(t.Context()); err != nil {
		t.Fatal(err)
	}

	st, _ := w.states.Get("node-01")
	if st.Phase() != PhaseHealthy {
		t.Errorf("expected PhaseHealthy after recovery, got %v", st.Phase())
	}
	if st.RebootCount() != 0 {
		t.Errorf("rebootCount should reset to 0 after recovery, got %d", st.RebootCount())
	}
}

func TestBuildNodeSelector(t *testing.T) {
	tests := []struct {
		description string
		nodePoolIDs []string
		wantNil     bool
		wantLabels  map[string]string
		wantInExpr  bool
	}{
		{
			description: "returns nil for empty IDs",
			wantNil:     true,
		},
		{
			description: "returns MatchLabels for single ID",
			nodePoolIDs: []string{"pool-1"},
			wantLabels:  map[string]string{nodePoolLabelKey: "pool-1"},
		},
		{
			description: "returns MatchExpressions In for multiple IDs",
			nodePoolIDs: []string{"pool-1", "pool-2"},
			wantInExpr:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			sel := buildNodeSelector(test.nodePoolIDs)
			if test.wantNil {
				if sel != nil {
					t.Errorf("expected nil selector, got %v", sel)
				}
				return
			}
			if sel == nil {
				t.Fatal("expected non-nil selector")
			}
			if test.wantLabels != nil {
				for k, v := range test.wantLabels {
					if sel.MatchLabels[k] != v {
						t.Errorf("MatchLabels[%s] = %q, want %q", k, sel.MatchLabels[k], v)
					}
				}
			}
			if test.wantInExpr {
				if len(sel.MatchExpressions) != 1 {
					t.Fatalf("expected 1 MatchExpression, got %d", len(sel.MatchExpressions))
				}
				expr := sel.MatchExpressions[0]
				if expr.Key != nodePoolLabelKey {
					t.Errorf("key = %q, want %q", expr.Key, nodePoolLabelKey)
				}
				if len(expr.Values) != len(test.nodePoolIDs) {
					t.Errorf("values count = %d, want %d", len(expr.Values), len(test.nodePoolIDs))
				}
			}
		})
	}
}

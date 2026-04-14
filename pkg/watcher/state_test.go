package watcher

import (
	"testing"
	"time"
)

func TestNodePhaseString(t *testing.T) {
	tests := []struct {
		phase NodePhase
		want  string
	}{
		{PhaseUnknown, "Unknown"},
		{PhaseHealthy, "Healthy"},
		{PhaseUnhealthy, "Unhealthy"},
		{PhaseReboot, "Reboot"},
		{PhaseWaitingReboot, "WaitingReboot"},
		{PhaseDrain, "Drain"},
		{PhaseReplace, "Replace"},
		{NodePhase(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.phase.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNodePhaseZeroValue(t *testing.T) {
	var phase NodePhase
	if phase != PhaseUnknown {
		t.Errorf("zero value of NodePhase should be PhaseUnknown, got %v", phase)
	}
}

func TestStateStoreGetOrCreate(t *testing.T) {
	s := NewStateStore()

	st := s.GetOrCreate("node-01")
	if st.Phase() != PhaseHealthy {
		t.Errorf("new state should be PhaseHealthy, got %v", st.Phase())
	}

	st2 := s.GetOrCreate("node-01")
	if st != st2 {
		t.Error("GetOrCreate should return the same pointer for existing node")
	}
}

func TestStateStoreGet(t *testing.T) {
	s := NewStateStore()

	_, ok := s.Get("nonexistent")
	if ok {
		t.Error("Get should return false for nonexistent node")
	}

	s.GetOrCreate("node-01")
	st, ok := s.Get("node-01")
	if !ok {
		t.Error("Get should return true for existing node")
	}
	if st.Phase() != PhaseHealthy {
		t.Errorf("got phase %v, want PhaseHealthy", st.Phase())
	}
}

func TestStateStoreDelete(t *testing.T) {
	s := NewStateStore()
	s.GetOrCreate("node-01")

	s.Delete("node-01")
	_, ok := s.Get("node-01")
	if ok {
		t.Error("node should be deleted")
	}

	// Deleting nonexistent node should not panic.
	s.Delete("nonexistent")
}

func TestStateStoreRange(t *testing.T) {
	s := NewStateStore()
	s.GetOrCreate("node-01")
	s.GetOrCreate("node-02")
	s.GetOrCreate("node-03")

	visited := make(map[string]bool)
	s.Range(func(name string, _ *NodeState) bool {
		visited[name] = true
		return true
	})

	if len(visited) != 3 {
		t.Errorf("Range should visit 3 nodes, visited %d", len(visited))
	}
}

func TestStateStoreRangeEarlyStop(t *testing.T) {
	s := NewStateStore()
	s.GetOrCreate("node-01")
	s.GetOrCreate("node-02")
	s.GetOrCreate("node-03")

	count := 0
	s.Range(func(_ string, _ *NodeState) bool {
		count++
		return false
	})

	if count != 1 {
		t.Errorf("Range should stop after first call when fn returns false, visited %d", count)
	}
}

func TestStateStoreMarkUnhealthy(t *testing.T) {
	s := NewStateStore()
	s.GetOrCreate("node-01")

	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	s.MarkUnhealthy("node-01", now)

	st, _ := s.Get("node-01")
	if st.Phase() != PhaseUnhealthy {
		t.Errorf("got phase %v, want PhaseUnhealthy", st.Phase())
	}
	if !st.UnhealthySince().Equal(now) {
		t.Errorf("got unhealthySince %v, want %v", st.UnhealthySince(), now)
	}
}

func TestStateStoreMarkUnhealthyNonexistent(t *testing.T) {
	s := NewStateStore()
	// Should not panic.
	s.MarkUnhealthy("nonexistent", time.Now())
}

func TestStateStoreMarkWaitingReboot(t *testing.T) {
	s := NewStateStore()
	s.GetOrCreate("node-01")

	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	s.MarkWaitingReboot("node-01", now)

	st, _ := s.Get("node-01")
	if st.Phase() != PhaseWaitingReboot {
		t.Errorf("got phase %v, want PhaseWaitingReboot", st.Phase())
	}
	if !st.LastRebootTime().Equal(now) {
		t.Errorf("got lastRebootTime %v, want %v", st.LastRebootTime(), now)
	}
	if st.RebootCount() != 1 {
		t.Errorf("got rebootCount %d, want 1", st.RebootCount())
	}

	// Retry increments count.
	later := now.Add(time.Hour)
	s.MarkWaitingReboot("node-01", later)

	st, _ = s.Get("node-01")
	if st.RebootCount() != 2 {
		t.Errorf("got rebootCount %d after retry, want 2", st.RebootCount())
	}
	if !st.LastRebootTime().Equal(later) {
		t.Errorf("got lastRebootTime %v after retry, want %v", st.LastRebootTime(), later)
	}
}

func TestStateStoreMarkWaitingRebootNonexistent(t *testing.T) {
	s := NewStateStore()
	// Should not panic.
	s.MarkWaitingReboot("nonexistent", time.Now())
}

func TestStateStoreUpdateCheckerInfo(t *testing.T) {
	s := NewStateStore()
	s.GetOrCreate("node-01")

	checkers := []string{"NodeReady", "GPU"}
	s.UpdateCheckerInfo("node-01", checkers, true)

	st, _ := s.Get("node-01")
	if !st.IsGPUNode() {
		t.Error("expected isGPUNode to be true")
	}
}

func TestStateStoreUpdateCheckerInfoNonexistent(t *testing.T) {
	s := NewStateStore()
	// Should not panic.
	s.UpdateCheckerInfo("nonexistent", []string{"NodeReady"}, false)
}

func TestStateStoreReset(t *testing.T) {
	s := NewStateStore()
	s.GetOrCreate("node-01")

	now := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	s.MarkUnhealthy("node-01", now)
	s.UpdateCheckerInfo("node-01", []string{"NodeReady"}, true)
	s.MarkWaitingReboot("node-01", now)

	s.Reset("node-01")

	st, ok := s.Get("node-01")
	if !ok {
		t.Fatal("node should still exist after Reset")
	}
	if st.Phase() != PhaseHealthy {
		t.Errorf("got phase %v, want PhaseHealthy", st.Phase())
	}
	if st.RebootCount() != 0 {
		t.Errorf("got rebootCount %d, want 0", st.RebootCount())
	}
	if !st.UnhealthySince().IsZero() {
		t.Error("unhealthySince should be zero after Reset")
	}
	if !st.LastRebootTime().IsZero() {
		t.Error("lastRebootTime should be zero after Reset")
	}
	if st.IsGPUNode() {
		t.Error("isGPUNode should be false after Reset")
	}
}

func TestStateStoreResetNonexistent(t *testing.T) {
	s := NewStateStore()
	// Should not panic and should not create an entry.
	s.Reset("nonexistent")

	_, ok := s.Get("nonexistent")
	if ok {
		t.Error("Reset on nonexistent node should not create an entry")
	}
}

func TestStateStoreResetReplacesPointer(t *testing.T) {
	s := NewStateStore()
	old := s.GetOrCreate("node-01")

	s.Reset("node-01")

	current, _ := s.Get("node-01")
	if old == current {
		t.Error("Reset should replace the map entry with a new pointer")
	}
}

func TestStateStoreCleanup(t *testing.T) {
	s := NewStateStore()
	s.GetOrCreate("node-01")
	s.GetOrCreate("node-02")
	s.GetOrCreate("node-03")

	active := map[string]struct{}{
		"node-01": {},
		"node-03": {},
	}
	s.Cleanup(active)

	if _, ok := s.Get("node-01"); !ok {
		t.Error("node-01 should still exist")
	}
	if _, ok := s.Get("node-02"); ok {
		t.Error("node-02 should be removed")
	}
	if _, ok := s.Get("node-03"); !ok {
		t.Error("node-03 should still exist")
	}
}

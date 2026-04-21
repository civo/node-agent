package watcher

import (
	"sync"
	"time"
)

// NodePhase represents the current recovery phase of a node.
type NodePhase int

const (
	PhaseUnknown       NodePhase = iota // 0 - unknown/uninitialized
	PhaseHealthy                        // 1 - node is healthy
	PhaseUnhealthy                      // 2 - checker(s) failing, waiting for threshold
	PhaseReboot                         // 3 - reboot command issued
	PhaseWaitingReboot                  // 4 - waiting for reboot to take effect
	PhaseDrain                          // 5 - future: draining pods
	PhaseReplace                        // 6 - future: replace issued
	PhaseFailed                         // 7 - recovery gave up (exceeded retries); awaits manual intervention or natural recovery
)

// String returns the string representation of a NodePhase.
func (p NodePhase) String() string {
	switch p {
	case PhaseUnknown:
		return "Unknown"
	case PhaseHealthy:
		return "Healthy"
	case PhaseUnhealthy:
		return "Unhealthy"
	case PhaseReboot:
		return "Reboot"
	case PhaseWaitingReboot:
		return "WaitingReboot"
	case PhaseDrain:
		return "Drain"
	case PhaseReplace:
		return "Replace"
	case PhaseFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// NodeState holds the recovery state for a single node.
// All fields are private; read via getters, mutate via StateStore methods.
type NodeState struct {
	mu             sync.RWMutex
	phase          NodePhase
	unhealthySince time.Time
	lastRebootTime time.Time
	rebootCount    int
	failedCheckers []string
	isGPUNode      bool
}

func (s *NodeState) Phase() NodePhase {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.phase
}
func (s *NodeState) UnhealthySince() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.unhealthySince
}
func (s *NodeState) LastRebootTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastRebootTime
}
func (s *NodeState) RebootCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rebootCount
}
func (s *NodeState) IsGPUNode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isGPUNode
}

// StateStore is a concurrency-safe store for per-node recovery state.
type StateStore struct {
	mu    sync.RWMutex
	nodes map[string]*NodeState
}

// NewStateStore creates a new empty StateStore.
func NewStateStore() *StateStore {
	return &StateStore{
		nodes: make(map[string]*NodeState),
	}
}

// GetOrCreate returns the NodeState for the given node name,
// creating a new one (PhaseHealthy) if it does not exist.
func (s *StateStore) GetOrCreate(name string) *NodeState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.nodes[name]; ok {
		return st
	}
	st := &NodeState{phase: PhaseHealthy}
	s.nodes[name] = st
	return st
}

// Get returns the NodeState for the given node name and whether it was found.
func (s *StateStore) Get(name string) (*NodeState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.nodes[name]
	return st, ok
}

// Delete removes the state entry for the given node name.
func (s *StateStore) Delete(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.nodes, name)
}

// Range calls fn for each node state entry. If fn returns false, iteration stops.
func (s *StateStore) Range(fn func(name string, state *NodeState) bool) {
	s.mu.RLock()
	snapshot := make(map[string]*NodeState, len(s.nodes))
	for name, state := range s.nodes {
		snapshot[name] = state
	}
	s.mu.RUnlock()
	for name, state := range snapshot {
		if !fn(name, state) {
			return
		}
	}
}

// UpdateCheckerInfo updates the failed checker names and GPU flag for a node.
func (s *StateStore) UpdateCheckerInfo(name string, failedCheckers []string, isGPUNode bool) {
	st, ok := s.Get(name)
	if !ok {
		return
	}
	st.mu.Lock()
	st.failedCheckers = failedCheckers
	st.isGPUNode = isGPUNode
	st.mu.Unlock()
}

// MarkUnhealthy transitions a node to PhaseUnhealthy and records when it became unhealthy.
func (s *StateStore) MarkUnhealthy(name string, now time.Time) {
	st, ok := s.Get(name)
	if !ok {
		return
	}
	st.mu.Lock()
	st.phase = PhaseUnhealthy
	st.unhealthySince = now
	st.mu.Unlock()
}

// MarkWaitingReboot transitions a node to PhaseWaitingReboot, records the
// reboot time, and increments the reboot counter.
func (s *StateStore) MarkWaitingReboot(name string, now time.Time) {
	st, ok := s.Get(name)
	if !ok {
		return
	}
	st.mu.Lock()
	st.phase = PhaseWaitingReboot
	st.lastRebootTime = now
	st.rebootCount++
	st.mu.Unlock()
}

// MarkFailed transitions a node to PhaseFailed after recovery attempts were exhausted.
func (s *StateStore) MarkFailed(name string) {
	st, ok := s.Get(name)
	if !ok {
		return
	}
	st.mu.Lock()
	st.phase = PhaseFailed
	st.mu.Unlock()
}

// Reset replaces the node's state with a fresh PhaseHealthy entry.
func (s *StateStore) Reset(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.nodes[name]; ok {
		s.nodes[name] = &NodeState{phase: PhaseHealthy}
	}
}

// Cleanup removes state entries for nodes that are not in the activeNodes set.
func (s *StateStore) Cleanup(activeNodes map[string]struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name := range s.nodes {
		if _, ok := activeNodes[name]; !ok {
			delete(s.nodes, name)
		}
	}
}

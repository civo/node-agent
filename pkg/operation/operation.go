package operation

import "context"

// Executor performs recovery operations on cluster nodes.
type Executor interface {
	Reboot(ctx context.Context, nodeName string) error
}

// nopExecutor is a no-op Executor that does nothing.
// Used as a safe default to prevent nil pointer dereference.
type nopExecutor struct{}

func (e *nopExecutor) Reboot(_ context.Context, _ string) error { return nil }

// NewNopExecutor returns an Executor that performs no operations.
func NewNopExecutor() Executor {
	return &nopExecutor{}
}

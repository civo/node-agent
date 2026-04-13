package operation

import "context"

// Executor performs recovery operations on cluster nodes.
type Executor interface {
	Reboot(ctx context.Context, nodeName string) error
}

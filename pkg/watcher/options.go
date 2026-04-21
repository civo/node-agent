package watcher

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/civo/node-agent/pkg/health"
	"github.com/civo/node-agent/pkg/operation"
	"k8s.io/client-go/kubernetes"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
)

// Option represents a configuration function that modifies watcher object.
type Option func(*watcher)

var defaultOptions = []Option{
	WithMonitorOnly("true"),
	WithExecutor(operation.NewNopExecutor()),
	WithRebootWaitMinutes("10"),
	WithGPURebootWaitMinutes("40"),
}

// WithKubernetesClient returns Option to set Kubernetes API client.
func WithKubernetesClient(client kubernetes.Interface) Option {
	return func(w *watcher) {
		if client != nil {
			w.client = client
		}
	}
}

// WithKubernetesClientConfigPath returns Option to set Kubernetes config path.
func WithKubernetesClientConfigPath(path string) Option {
	return func(w *watcher) {
		if path != "" {
			w.clientCfgPath = path
		}
	}
}

// WithNodePoolIDs returns Option to append node pool IDs to watch.
// Accepts a comma-separated string (e.g. "pool-1,pool-2").
// Can be called multiple times to accumulate IDs.
// Empty string is a no-op. If no IDs are provided across all calls, all nodes are watched.
func WithNodePoolIDs(s string) Option {
	return func(w *watcher) {
		for _, id := range strings.Split(s, ",") {
			if v := strings.TrimSpace(id); v != "" {
				w.nodePoolIDs = append(w.nodePoolIDs, v)
			}
		}
	}
}

// WithRebootWaitMinutes returns Option to set the reboot wait time for standard nodes.
func WithRebootWaitMinutes(s string) Option {
	return func(w *watcher) {
		n, err := strconv.Atoi(s)
		if err == nil && n > 0 {
			w.rebootWaitMinutes = time.Duration(n)
		} else {
			slog.Info("RebootWaitMinutes is invalid", "value", s)
		}
	}
}

// WithGPURebootWaitMinutes returns Option to set the reboot wait time for GPU nodes.
func WithGPURebootWaitMinutes(s string) Option {
	return func(w *watcher) {
		n, err := strconv.Atoi(s)
		if err == nil && n > 0 {
			w.gpuRebootWaitMinutes = time.Duration(n)
		} else {
			slog.Info("GPURebootWaitMinutes is invalid", "value", s)
		}
	}
}

// WithMonitorOnly returns Option to enable or disable monitor-only mode.
// Accepts a string parsable by strconv.ParseBool (e.g. "true", "false", "1", "0").
// Empty or unparsable values are ignored (default: true).
func WithMonitorOnly(s string) Option {
	return func(w *watcher) {
		if v, err := strconv.ParseBool(s); err == nil {
			w.monitorOnly = v
		} else {
			slog.Info("MonitorOnly is invalid", "value", s)
		}
	}
}

// WithCheckers returns Option to set the health checkers.
func WithCheckers(checkers []health.HealthChecker) Option {
	return func(w *watcher) {
		w.checkers = checkers
	}
}

// WithExecutor returns Option to set the recovery executor.
func WithExecutor(exec operation.Executor) Option {
	return func(w *watcher) {
		if exec != nil {
			w.executor = exec
		}
	}
}

// withNowFunc returns Option to override the time source (for testing).
func withNowFunc(fn func() time.Time) Option {
	return func(w *watcher) {
		if fn != nil {
			w.nowFunc = fn
		}
	}
}

// withNodeLister returns Option to inject a node lister (for testing).
// When set, the informer setup is skipped.
func withNodeLister(lister listerscorev1.NodeLister) Option {
	return func(w *watcher) {
		w.nodeLister = lister
	}
}

package watcher

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/civo/node-agent/pkg/health"
	"github.com/civo/node-agent/pkg/operation"
	"k8s.io/client-go/kubernetes"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
)

// Option represents a configuration function that modifies watcher object.
type Option func(*watcher)

var defaultOptions = []Option{
	WithRebootTimeWindowMinutes("40"),
}

// WithKubernetesClient returns Option to set Kubernetes API client.
func WithKubernetesClient(client kubernetes.Interface) Option {
	return func(w *watcher) {
		if client != nil {
			w.client = client
		}
	}
}

// WithKubernetesClient returns Option to set Kubernetes config path.
func WithKubernetesClientConfigPath(path string) Option {
	return func(w *watcher) {
		if path != "" {
			w.clientCfgPath = path
		}
	}
}

// WithNodePoolIDs returns Option to append node pool IDs to watch.
// Can be called multiple times to accumulate IDs.
// If no IDs are provided across all calls, all nodes are watched.
func WithNodePoolIDs(ids []string) Option {
	return func(w *watcher) {
		if len(ids) > 0 {
			w.nodePoolIDs = append(w.nodePoolIDs, ids...)
		}
	}
}

// WithRebootTimeWindowMinutes returns Option to set reboot time window.
func WithRebootTimeWindowMinutes(s string) Option {
	return func(w *watcher) {
		n, err := strconv.Atoi(s)
		if err == nil && n > 0 {
			w.rebootTimeWindowMinutes = time.Duration(n)
		} else {
			slog.Info("RebootTimeWindowMinutes is invalid", "value", s)
		}
	}
}

// WithMonitorOnly returns Option to enable or disable monitor-only mode.
// When true (default), recovery actions are logged but not executed.
func WithMonitorOnly(v bool) Option {
	return func(w *watcher) {
		w.monitorOnly = v
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
		w.executor = exec
	}
}

// WithNowFunc returns Option to override the time source (for testing).
func WithNowFunc(fn func() time.Time) Option {
	return func(w *watcher) {
		if fn != nil {
			w.nowFunc = fn
		}
	}
}

// WithNodeLister returns Option to inject a node lister (for testing).
// When set, the informer setup is skipped.
func WithNodeLister(lister listerscorev1.NodeLister) Option {
	return func(w *watcher) {
		w.nodeLister = lister
	}
}

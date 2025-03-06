package watcher

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/civo/civogo"
	"k8s.io/client-go/kubernetes"
)

// Option represents a configuration function that modifies watcher object.
type Option func(*watcher)

var defaultOptions = []Option{
	WithRebootTimeWindowMinutes("40"),
	WithDesiredGPUCount("0"),
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

// WithCivoClient returns Option to set Civo API client.
func WithCivoClient(client civogo.Clienter) Option {
	return func(w *watcher) {
		if client != nil {
			w.civoClient = client
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

// WithDesiredGPUCount returns Option to set reboot time window.
func WithDesiredGPUCount(s string) Option {
	return func(w *watcher) {
		n, err := strconv.Atoi(s)
		if err == nil && n >= 0 {
			w.nodeDesiredGPUCount = n
		} else {
			slog.Info("DesiredGPUCount is invalid", "value", s)
		}
	}
}

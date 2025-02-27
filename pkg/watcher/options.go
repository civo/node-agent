package watcher

import (
	"github.com/civo/civogo"
	"k8s.io/client-go/kubernetes"
)

// Option represents a configuration function that modifies watcher object.
type Option func(*watcher)

var defaultOptions = []Option{}

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

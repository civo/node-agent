package watcher

import (
	"context"
	"fmt"

	"github.com/civo/civogo"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Version is the current version of the this watcher
var Version string = "0.0.1"

type Watcher interface {
	Run(ctx context.Context) error
}

type watcher struct {
	client        kubernetes.Interface
	civoClient    civogo.Clienter
	nodeName      string
	clientCfgPath string
}

func NewWatcher(opts ...Option) (Watcher, error) {
	w := new(watcher)
	for _, opt := range append(defaultOptions, opts...) {
		opt(w)
	}
	if err := w.setupKubernetesClient(); err != nil {
		return nil, err
	}
	return w, nil
}

// setupKubernetesClient creates Kubernetes client based on the kubeconfig path.
// If kubeconfig path is not empty, the client will be created using that path.
// Otherwise, if the kubeconfig path is empty, the client will be created using the in-clustetr config.
func (w *watcher) setupKubernetesClient() (err error) {
	if w.clientCfgPath != "" && w.client == nil {
		cfg, err := clientcmd.BuildConfigFromFlags("", w.clientCfgPath)
		if err != nil {
			return fmt.Errorf("failed to build kubeconfig from path %q: %w", w.clientCfgPath, err)
		}
		w.client, err = kubernetes.NewForConfig(cfg)
		if err != nil {
			return fmt.Errorf("failed to create kubernetes API client: %w", err)
		}
		return nil
	}

	if w.client == nil {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("failed to load in-cluster kubeconfig: %w", err)
		}
		w.client, err = kubernetes.NewForConfig(cfg)
		if err != nil {
			return fmt.Errorf("failed to create kubernetes API client: %w", err)
		}
	}
	return nil
}

func (w *watcher) setupCivoClient(_ context.Context) error {
	return nil
}

func (w *watcher) Run(ctx context.Context) error {
	return nil
}

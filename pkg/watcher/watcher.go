package watcher

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/civo/civogo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	client      kubernetes.Interface
	civoClient  civogo.Clienter
	clusterName string
	region      string
}

func NewWatcher(ctx context.Context, clusterName, region string, opts ...Option) (Watcher, error) {
	w := new(watcher)
	for _, opt := range append(defaultOptions, opts...) {
		opt(w)
	}
	if err := w.setupKubernetesClient(); err != nil {
		return nil, err
	}
	if err := w.setupCivoClient(ctx); err != nil {
		return nil, err
	}

	return w, nil
}

// setupKubernetesClient creates Kubernetes client based on the kubeconfig path.
// If kubeconfig path is not empty, the client will be created using that path.
// Otherwise, if the kubeconfig path is empty, the client will be created using the in-clustetr config.
func (w *watcher) setupKubernetesClient() (err error) {
	kubeconfig := os.Getenv("KUBECONFIG")

	if kubeconfig != "" && w.client == nil {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return fmt.Errorf("failed to build kubeconfig from path %q: %w", kubeconfig, err)
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
	if os.Getenv("CIVO_API_KEY") == "" {
		return fmt.Errorf("CIVO_API_KEY not set")
	}

	apiKey := os.Getenv("CIVO_API_KEY")
	civoClient, err := civogo.NewClient(apiKey, w.region)
	if err != nil {
		return err
	}
	w.civoClient = civoClient
	return nil
}

func (w *watcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)

	for {
		select {
		case <-ticker.C:
			w.listNodes(ctx)
		}
	}
}

func (w *watcher) listNodes(ctx context.Context) {
	nodes, err := w.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing nodes: %v\n", err)
		return
	}

	cluster, err := w.civoClient.GetKubernetesCluster(w.clusterName)
	if err != nil {
		fmt.Printf("Error getting cluster: %v\n", err)
		return
	}

	fmt.Println("\nNodes List:")
	for _, node := range nodes.Items {
		condition := getNodeCondition(node)
		if condition != "Ready" {
			if err := w.restart(cluster); err != nil {
				fmt.Printf("Error restarting instance: %v\n", err)
			}
		}
	}
}

func getNodeCondition(node v1.Node) string {
	for _, cond := range node.Status.Conditions {
		if cond.Type == v1.NodeReady {
			if cond.Status == v1.ConditionTrue {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}

func (w *watcher) restart(cluster *civogo.KubernetesCluster) error {
	instance, err := w.civoClient.GetKubernetesCluster(cluster.ID)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}

	res, err := w.civoClient.RebootInstance(instance.ID)
	if err != nil {
		return fmt.Errorf("failed to reboot instance: %w", err)
	}

	fmt.Printf("Instance %s is rebooting: %v\n", instance.ID, res)
	return nil
}

package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/civo/civogo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Version is the current version of the this watcher
var Version string = "0.0.1"

const (
	nodePoolLabelKey = "kubernetes.civo.com/civo-node-pool"
	gpuResourceName  = "nvidia.com/gpu"
)

type Watcher interface {
	Run(ctx context.Context) error
}

type watcher struct {
	client        kubernetes.Interface
	civoClient    civogo.Clienter
	clientCfgPath string

	clusterID               string
	region                  string
	apiKey                  string
	apiURL                  string
	nodeDesiredGPUCount     int
	rebootTimeWindowMinutes time.Duration

	nodeSelector *metav1.LabelSelector
}

func NewWatcher(ctx context.Context, apiURL, apiKey, region, clusterID, nodePoolID string, opts ...Option) (Watcher, error) {
	w := &watcher{
		clusterID: clusterID,
		apiKey:    apiKey,
		apiURL:    apiURL,
		region:    region,
	}
	for _, opt := range append(defaultOptions, opts...) {
		opt(w)
	}

	if clusterID == "" {
		return nil, fmt.Errorf("CIVO_CLUSTER_ID not set")
	}
	if nodePoolID == "" {
		return nil, fmt.Errorf("CIVO_NODE_POOL_ID not set")
	}
	if w.civoClient == nil && apiKey == "" {
		return nil, fmt.Errorf("CIVO_API_KEY not set")
	}

	w.nodeSelector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			nodePoolLabelKey: nodePoolID,
		},
	}

	if err := w.setupKubernetesClient(); err != nil {
		return nil, err
	}
	if err := w.setupCivoClient(); err != nil {
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
			return fmt.Errorf("failed to build kubeconfig from path %q: %w", cfg, err)
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

func (w *watcher) setupCivoClient() error {
	if w.civoClient != nil {
		return nil
	}

	client, err := civogo.NewClientWithURL(w.apiKey, w.apiURL, w.region)
	if err != nil {
		return fmt.Errorf("failed to initialise civo client: %w", err)
	}

	userAgent := &civogo.Component{
		ID:      w.clusterID,
		Name:    "node-agent",
		Version: Version,
	}
	client.SetUserAgent(userAgent)

	w.civoClient = client
	return nil
}

func (w *watcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			slog.Info("Started the watcher process...")
			if err := w.run(ctx); err != nil {
				slog.Error("An error occurred while running the watcher process", "error", err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (w *watcher) run(ctx context.Context) error {
	nodes, err := w.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(w.nodeSelector),
	})
	if err != nil {
		return err
	}

	thresholdTime := time.Now().Add(-w.rebootTimeWindowMinutes * time.Minute)

	for _, node := range nodes.Items {
		if !isNodeDesiredGPU(&node, w.nodeDesiredGPUCount) || !isNodeReady(&node) {
			slog.Info("Node is not ready, attempting to reboot", "node", node.GetName())
			if isReadyOrNotReadyStatusChangedAfter(&node, thresholdTime) {
				slog.Info("Skipping reboot because Ready/NotReady status was updated recently", "node", node.GetName())
				continue
			}
			if err := w.rebootNode(node.GetName()); err != nil {
				slog.Error("Failed to reboot Node", "node", node.GetName(), "error", err)
				return fmt.Errorf("failed to reboot node: %w", err)
			}
		}
	}
	return nil
}

func isReadyOrNotReadyStatusChangedAfter(node *corev1.Node, thresholdTime time.Time) bool {
	var lastChangedTime time.Time
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.LastTransitionTime.After(lastChangedTime) {
				lastChangedTime = cond.LastTransitionTime.Time
			}
		}
	}

	slog.Info("Checking if Ready/NotReady status has changed recently",
		"node", node.GetName(),
		"lastTransitionTime", lastChangedTime.String(),
		"thresholdTime", thresholdTime.String())

	if lastChangedTime.IsZero() {
		slog.Error("Node is in an invalid state, NodeReady condition not found", "node", node.GetName())
		return false
	}
	return lastChangedTime.After(thresholdTime)
}

func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			slog.Info("Current Node status", "node", node.GetName(), "type", corev1.NodeReady, "status", cond.Status)
			return cond.Status == corev1.ConditionTrue
		}
	}
	slog.Info("NodeReady condition not found", "node", node.GetName())
	return false
}

func isNodeDesiredGPU(node *corev1.Node, desired int) bool {
	if desired == 0 {
		slog.Info("Desired GPU count is set to 0, so the GPU count check is skipped", "node", node.GetName())
		return true
	}

	quantity, exists := node.Status.Allocatable[gpuResourceName]
	if !exists || quantity.IsZero() {
		slog.Info("Allocatable GPU not found", "node", node.GetName())
		return false
	}

	gpuCount, ok := quantity.AsInt64()
	if !ok {
		slog.Info("Failed to convert allocatable GPU quantity to int64", "node", node.GetName(), "quantity", quantity.String())
		return false
	}

	slog.Info("Checking actual GPU count with desired",
		"node", node.GetName(),
		"actual", gpuCount,
		"desired", strconv.Itoa(desired))

	return gpuCount == int64(desired)
}

func (w *watcher) rebootNode(name string) error {
	instance, err := w.civoClient.FindKubernetesClusterInstance(w.clusterID, name)
	if err != nil {
		return fmt.Errorf("failed to find instance, clusterID: %s, nodeName: %s: %w", w.clusterID, name, err)
	}

	_, err = w.civoClient.HardRebootInstance(instance.ID)
	if err != nil {
		return fmt.Errorf("failed to reboot instance, clusterID: %s, instanceID: %s: %w", w.clusterID, instance.ID, err)
	}
	slog.Info("Instance is rebooting", "instanceID", instance.ID, "node", name)
	return nil
}

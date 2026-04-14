package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/civo/node-agent/pkg/health"
	"github.com/civo/node-agent/pkg/metrics"
	"github.com/civo/node-agent/pkg/operation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const nodePoolLabelKey = "kubernetes.civo.com/civo-node-pool"

type Watcher interface {
	Run(ctx context.Context) error
}

type watcher struct {
	client        kubernetes.Interface
	clientCfgPath string

	nodePoolIDs          []string
	rebootWaitMinutes    time.Duration // Standard nodes (default: 10)
	gpuRebootWaitMinutes time.Duration // GPU nodes (default: 40)

	nodeSelector *metav1.LabelSelector
	nodeLister   listerscorev1.NodeLister

	monitorOnly bool
	checkers    []health.HealthChecker
	executor    operation.Executor
	states      *StateStore
	nowFunc     func() time.Time
}

func NewWatcher(ctx context.Context, opts ...Option) (Watcher, error) {
	w := &watcher{
		monitorOnly: true,
		states:      NewStateStore(),
		nowFunc:     time.Now,
	}
	for _, opt := range append(defaultOptions, opts...) {
		opt(w)
	}

	w.nodeSelector = buildNodeSelector(w.nodePoolIDs)

	if err := w.setupKubernetesClient(); err != nil {
		return nil, err
	}
	return w, nil
}

// setupKubernetesClient creates Kubernetes client based on the kubeconfig path.
// If kubeconfig path is not empty, the client will be created using that path.
// Otherwise, if the kubeconfig path is empty, the client will be created using the in-cluster config.
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

func (w *watcher) setupInformer(ctx context.Context) error {
	if w.nodeLister != nil {
		return nil
	}

	labelSelector := metav1.FormatLabelSelector(w.nodeSelector)
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.client,
		0,
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = labelSelector
		}),
	)

	nodeInformer := factory.Core().V1().Nodes()
	w.nodeLister = nodeInformer.Lister()

	factory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), nodeInformer.Informer().HasSynced) {
		return fmt.Errorf("failed to sync node informer cache")
	}

	slog.Info("Node informer cache synced")
	return nil
}

func (w *watcher) Run(ctx context.Context) error {
	if err := w.setupInformer(ctx); err != nil {
		return err
	}

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
	nodes, err := w.nodeLister.List(labels.Everything())
	if err != nil {
		return err
	}

	now := w.nowFunc()
	activeNodes := make(map[string]struct{}, len(nodes))

	for _, node := range nodes {
		nodeName := node.GetName()
		activeNodes[nodeName] = struct{}{}

		// Run all health checkers and collect failures.
		var failedCheckers []string
		var minThreshold time.Duration
		for _, checker := range w.checkers {
			healthy, reason := checker.Check(node)
			if !healthy {
				failedCheckers = append(failedCheckers, checker.Name())
				if minThreshold == 0 || checker.Threshold() < minThreshold {
					minThreshold = checker.Threshold()
				}
			}
			metrics.HealthCheckTotal.WithLabelValues(nodeName, checker.Name(), reason).Inc()
		}

		state := w.states.GetOrCreate(nodeName)

		// All checkers pass → node is healthy.
		if len(failedCheckers) == 0 {
			if state.Phase() != PhaseHealthy {
				prevPhase := state.Phase()
				slog.Info("Node recovered",
					"node", nodeName,
					"previousPhase", prevPhase.String())
				metrics.NodeUnhealthyDurationSeconds.WithLabelValues(nodeName).Set(0)
				metrics.RecoveryPhase.WithLabelValues(nodeName, prevPhase.String()).Set(0)
				metrics.RecoveryPhase.WithLabelValues(nodeName, PhaseHealthy.String()).Set(1)
				w.states.Reset(nodeName)
			}
			continue
		}

		// At least one checker failed — enter the recovery judgment phase.
		// The state machine decides the next action (wait, reboot, retry)
		// regardless of which specific checker(s) failed.
		isGPUNode := health.HasGPU(node)
		w.states.UpdateCheckerInfo(nodeName, failedCheckers, isGPUNode)

		switch state.Phase() {
		case PhaseHealthy:
			w.states.MarkUnhealthy(nodeName, now)
			slog.Info("Node unhealthy detected",
				"node", nodeName,
				"failedCheckers", failedCheckers)
			metrics.NodeUnhealthyDurationSeconds.WithLabelValues(nodeName).Set(0)
			metrics.RecoveryPhase.WithLabelValues(nodeName, PhaseHealthy.String()).Set(0)
			metrics.RecoveryPhase.WithLabelValues(nodeName, PhaseUnhealthy.String()).Set(1)

		case PhaseUnhealthy:
			metrics.NodeUnhealthyDurationSeconds.WithLabelValues(nodeName).Set(
				now.Sub(state.UnhealthySince()).Seconds())
			if now.Sub(state.UnhealthySince()) < minThreshold {
				continue
			}
			if !w.monitorOnly {
				if err := w.executor.Reboot(ctx, nodeName); err != nil {
					slog.Error("Failed to reboot node", "node", nodeName, "error", err)
					continue
				}
			}
			mode := modeLabel(w.monitorOnly)
			slog.Info("Reboot initiated",
				"node", nodeName,
				"mode", mode,
				"failedCheckers", failedCheckers)
			metrics.RecoveryActionsTotal.WithLabelValues(nodeName, "reboot", mode).Inc()
			metrics.RecoveryPhase.WithLabelValues(nodeName, PhaseUnhealthy.String()).Set(0)
			metrics.RecoveryPhase.WithLabelValues(nodeName, PhaseWaitingReboot.String()).Set(1)
			w.states.MarkWaitingReboot(nodeName, now)

		case PhaseWaitingReboot:
			metrics.NodeUnhealthyDurationSeconds.WithLabelValues(nodeName).Set(
				now.Sub(state.UnhealthySince()).Seconds())
			rebootWait := w.rebootWaitMinutes
			if state.IsGPUNode() {
				rebootWait = w.gpuRebootWaitMinutes
			}
			if now.Sub(state.LastRebootTime()) < rebootWait*time.Minute {
				continue
			}

			// TODO: Standard nodes should transition to PhaseDrain → PhaseReplace
			// instead of retrying reboot indefinitely.
			// GPU nodes must never be replaced; they retry reboot only.
			// See: Recovery Flow — Standard Nodes (Drain → timeout 30min → Replace)

			if !w.monitorOnly {
				if err := w.executor.Reboot(ctx, nodeName); err != nil {
					slog.Error("Failed to reboot node (retry)", "node", nodeName, "error", err)
					continue
				}
			}
			mode := modeLabel(w.monitorOnly)
			slog.Info("Reboot retry",
				"node", nodeName,
				"mode", mode,
				"rebootCount", state.RebootCount()+1,
				"failedCheckers", failedCheckers)
			metrics.RecoveryActionsTotal.WithLabelValues(nodeName, "reboot", mode).Inc()
			w.states.MarkWaitingReboot(nodeName, now)
		}
	}

	w.states.Cleanup(activeNodes)
	return nil
}

func modeLabel(monitorOnly bool) string {
	if monitorOnly {
		return "monitor"
	}
	return "active"
}

// buildNodeSelector builds a LabelSelector based on the given node pool IDs.
//   - empty:    no selector (all nodes)
//   - single:   MatchLabels exact match
//   - multiple: MatchExpressions In operator
func buildNodeSelector(nodePoolIDs []string) *metav1.LabelSelector {
	switch len(nodePoolIDs) {
	case 0:
		return nil
	case 1:
		return &metav1.LabelSelector{
			MatchLabels: map[string]string{
				nodePoolLabelKey: nodePoolIDs[0],
			},
		}
	default:
		return &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      nodePoolLabelKey,
					Operator: metav1.LabelSelectorOpIn,
					Values:   nodePoolIDs,
				},
			},
		}
	}
}

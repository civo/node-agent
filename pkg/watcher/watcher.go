package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/civo/node-agent/pkg/health"
	"github.com/civo/node-agent/pkg/metrics"
	"github.com/civo/node-agent/pkg/operation"
	"github.com/prometheus/client_golang/prometheus"
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
	maxRebootRetries     int           // Give up and transition to PhaseFailed after this many reboots

	nodeLabelSelector *metav1.LabelSelector
	nodeLister        listerscorev1.NodeLister

	monitorOnly bool
	checkers    []health.HealthChecker
	executor    operation.Executor
	states      *StateStore
	nowFunc     func() time.Time
}

func NewWatcher(opts ...Option) (Watcher, error) {
	w := &watcher{
		monitorOnly: true,
		states:      NewStateStore(),
		nowFunc:     func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range append(defaultOptions, opts...) {
		opt(w)
	}

	w.nodeLabelSelector = buildNodeSelector(w.nodePoolIDs)

	if err := w.setupKubernetesClient(); err != nil {
		return nil, err
	}
	return w, nil
}

// setupKubernetesClient creates Kubernetes client based on the kubeconfig path.
// If kubeconfig path is not empty, the client will be created using that path.
// Otherwise, if the kubeconfig path is empty, the client will be created using the in-cluster config.
func (w *watcher) setupKubernetesClient() error {
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

func (w *watcher) setupInformer(ctx context.Context) error {
	if w.nodeLister != nil {
		return nil
	}

	var informerOpts []informers.SharedInformerOption
	if w.nodeLabelSelector != nil {
		labelSelector := metav1.FormatLabelSelector(w.nodeLabelSelector)
		slog.Info("Using node label selector", "selector", labelSelector)
		informerOpts = append(informerOpts, informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = labelSelector
		}))
	} else {
		slog.Info("No node label selector configured, watching all nodes")
	}
	factory := informers.NewSharedInformerFactoryWithOptions(w.client, 0, informerOpts...)

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

	slog.Info("Watcher reconcile loop started")
	for {
		select {
		case <-ticker.C:
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
		metrics.ReconcileErrorsTotal.WithLabelValues("list_nodes").Inc()
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
			metrics.RecoveryPhase.WithLabelValues(nodeName, PhaseHealthy.String()).Set(1)
			if prevPhase := state.Phase(); prevPhase != PhaseHealthy {
				slog.Info("Node recovered",
					"node", nodeName,
					"previousPhase", prevPhase.String())
				metrics.NodeUnhealthyDurationSeconds.WithLabelValues(nodeName).Set(0)
				metrics.RecoveryPhase.WithLabelValues(nodeName, prevPhase.String()).Set(0)
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
		// Healthy → Unhealthy: health check failed for the first time, start tracking.
		case PhaseHealthy:
			w.states.MarkUnhealthy(nodeName, now)
			slog.Info("Node unhealthy detected",
				"node", nodeName,
				"failedCheckers", failedCheckers)
			metrics.NodeUnhealthyDurationSeconds.WithLabelValues(nodeName).Set(0)
			metrics.RecoveryPhase.WithLabelValues(nodeName, PhaseHealthy.String()).Set(0)
			metrics.RecoveryPhase.WithLabelValues(nodeName, PhaseUnhealthy.String()).Set(1)

		// Unhealthy → WaitingReboot: health check still failing and threshold exceeded, issue reboot.
		case PhaseUnhealthy:
			metrics.NodeUnhealthyDurationSeconds.WithLabelValues(nodeName).Set(
				now.Sub(state.UnhealthySince()).Seconds())
			if now.Sub(state.UnhealthySince()) < minThreshold {
				slog.Info("Waiting for unhealthy threshold",
					"node", nodeName,
					"elapsed", now.Sub(state.UnhealthySince()).String(),
					"threshold", minThreshold.String(),
					"failedCheckers", failedCheckers)
				continue
			}
			if !w.monitorOnly {
				if err := w.executor.Reboot(ctx, nodeName); err != nil {
					slog.Error("Failed to reboot node", "node", nodeName, "error", err)
					metrics.RecoveryFailuresTotal.WithLabelValues(nodeName, "reboot").Inc()
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

		// WaitingReboot: health check still failing after reboot, retry after wait window.
		case PhaseWaitingReboot:
			metrics.NodeUnhealthyDurationSeconds.WithLabelValues(nodeName).Set(
				now.Sub(state.UnhealthySince()).Seconds())
			rebootWait := w.rebootWaitMinutes
			if state.IsGPUNode() {
				rebootWait = w.gpuRebootWaitMinutes
			}
			if now.Sub(state.LastRebootTime()) < rebootWait {
				// In monitor-only mode no reboot actually happened, so logging
				// "waiting for reboot effect" every tick would be noisy.
				// The "Reboot retry" log still fires once per rebootWait cycle as a liveness signal.
				if !w.monitorOnly {
					slog.Info("Waiting for reboot effect",
						"node", nodeName,
						"elapsed", now.Sub(state.LastRebootTime()).String(),
						"rebootWait", rebootWait.String(),
						"rebootCount", state.RebootCount(),
						"isGPUNode", state.IsGPUNode())
				}
				continue
			}

			// Retry budget exhausted → give up and transition to PhaseFailed.
			// The node stays in Failed until it naturally recovers (all checkers pass).
			// TODO: Standard nodes could transition to PhaseDrain → PhaseReplace here
			// once that flow is wired up. GPU nodes must stay in Failed (never replaced).
			if state.RebootCount() >= w.maxRebootRetries {
				slog.Warn("Reboot retry limit exceeded, giving up",
					"node", nodeName,
					"rebootCount", state.RebootCount(),
					"maxRebootRetries", w.maxRebootRetries,
					"isGPUNode", state.IsGPUNode(),
					"failedCheckers", failedCheckers)
				metrics.RecoveryPhase.WithLabelValues(nodeName, PhaseWaitingReboot.String()).Set(0)
				metrics.RecoveryPhase.WithLabelValues(nodeName, PhaseFailed.String()).Set(1)
				w.states.MarkFailed(nodeName)
				continue
			}

			if !w.monitorOnly {
				if err := w.executor.Reboot(ctx, nodeName); err != nil {
					slog.Error("Failed to reboot node (retry)", "node", nodeName, "error", err)
					metrics.RecoveryFailuresTotal.WithLabelValues(nodeName, "reboot").Inc()
					continue
				}
			}
			w.states.MarkWaitingReboot(nodeName, now)
			mode := modeLabel(w.monitorOnly)
			slog.Info("Reboot retry",
				"node", nodeName,
				"mode", mode,
				"rebootCount", state.RebootCount(),
				"failedCheckers", failedCheckers)
			metrics.RecoveryActionsTotal.WithLabelValues(nodeName, "reboot", mode).Inc()

		// Failed: recovery attempts exhausted. Wait for natural recovery (all checkers pass).
		// If the node recovers the "all checkers pass" branch above will Reset it back to Healthy.
		case PhaseFailed:
			metrics.NodeUnhealthyDurationSeconds.WithLabelValues(nodeName).Set(
				now.Sub(state.UnhealthySince()).Seconds())
		}
	}

	// Clean up state and metrics for nodes no longer in the cluster.
	w.states.Range(func(name string, _ *NodeState) bool {
		if _, ok := activeNodes[name]; !ok {
			metrics.NodeUnhealthyDurationSeconds.DeleteLabelValues(name)
			metrics.HealthCheckTotal.DeletePartialMatch(prometheus.Labels{"node": name})
			metrics.RecoveryActionsTotal.DeletePartialMatch(prometheus.Labels{"node": name})
			metrics.RecoveryFailuresTotal.DeletePartialMatch(prometheus.Labels{"node": name})
			metrics.RecoveryPhase.DeletePartialMatch(prometheus.Labels{"node": name})
		}
		return true
	})
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

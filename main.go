package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/civo/node-agent/pkg/health"
	"github.com/civo/node-agent/pkg/metrics"
	"github.com/civo/node-agent/pkg/operation"
	"github.com/civo/node-agent/pkg/watcher"
)

var (
	version        = "0.2.0"
	versionInfo    = flag.Bool("version", false, "Print the driver version")
	kubeconfigPath = flag.String("kubeconfig", "/etc/rancher/k3s/k3s.yaml", "Path to kubeconfig file (empty for in-cluster config)")
)

var (
	apiURL               = strings.TrimSpace(os.Getenv("CIVO_API_URL"))
	apiKey               = strings.TrimSpace(os.Getenv("CIVO_API_KEY"))
	region               = strings.TrimSpace(os.Getenv("CIVO_REGION"))
	clusterID            = strings.TrimSpace(os.Getenv("CIVO_CLUSTER_ID"))
	nodePoolIDs          = strings.TrimSpace(os.Getenv("CIVO_NODE_POOL_IDS"))
	rebootWaitMinutes    = strings.TrimSpace(os.Getenv("CIVO_NODE_REBOOT_WAIT_MINUTES"))
	gpuRebootWaitMinutes = strings.TrimSpace(os.Getenv("CIVO_GPU_NODE_REBOOT_WAIT_MINUTES"))
	monitorOnly          = strings.TrimSpace(os.Getenv("CIVO_NODE_AGENT_MONITOR_ONLY"))
	metricsPort          = strings.TrimSpace(os.Getenv("CIVO_NODE_AGENT_METRICS_PORT"))
)

const (
	defaultMetricsPort = 9625
)

func run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	executor, err := operation.NewCivoExecutor(clusterID,
		operation.WithAPIConfig(apiKey, apiURL, region, version))
	if err != nil {
		return fmt.Errorf("failed to initialise executor: %w", err)
	}
	checkers := health.NewDefaultCheckers()

	metrics.Register()
	metrics.Info.WithLabelValues(version, clusterID).Set(1)
	metricsServer := &http.Server{
		Addr:    ":" + metricsPortValue(metricsPort),
		Handler: metrics.Handler(),
	}
	go func() {
		slog.Info("Starting metrics server", "addr", metricsServer.Addr)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Metrics server failed", "error", err)
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := metricsServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("Metrics server shutdown failed", "error", err)
		}
	}()

	w, err := watcher.NewWatcher(ctx,
		watcher.WithNodePoolIDs(nodePoolIDs),
		watcher.WithKubernetesClientConfigPath(*kubeconfigPath),
		watcher.WithExecutor(executor),
		watcher.WithCheckers(checkers),
		watcher.WithMonitorOnly(monitorOnly),
		watcher.WithRebootWaitMinutes(rebootWaitMinutes),
		watcher.WithGPURebootWaitMinutes(gpuRebootWaitMinutes),
	)
	if err != nil {
		return err
	}
	return w.Run(ctx)
}

func main() {
	flag.Parse()
	if *versionInfo {
		slog.Info("node-agent", "version", version)
		return
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil).WithAttrs([]slog.Attr{
		slog.String("clusterID", clusterID),
		slog.String("region", region),
		slog.String("nodePoolIDs", nodePoolIDs),
	})))

	if err := run(context.Background()); err != nil {
		slog.Error("The node-agent encountered a critical error and will exit", "error", err)
		os.Exit(1)
	}
}

func metricsPortValue(s string) string {
	if v, err := strconv.Atoi(s); err == nil && v >= 1024 && v <= 65535 {
		return s
	}
	return strconv.Itoa(defaultMetricsPort)
}

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

	"github.com/civo/node-agent/pkg/health"
	"github.com/civo/node-agent/pkg/metrics"
	"github.com/civo/node-agent/pkg/operation"
	"github.com/civo/node-agent/pkg/watcher"
)

var (
	version     = "0.0.1"
	versionInfo = flag.Bool("version", false, "Print the driver version")
)

var (
	apiURL                  = strings.TrimSpace(os.Getenv("CIVO_API_URL"))
	apiKey                  = strings.TrimSpace(os.Getenv("CIVO_API_KEY"))
	region                  = strings.TrimSpace(os.Getenv("CIVO_REGION"))
	clusterID               = strings.TrimSpace(os.Getenv("CIVO_CLUSTER_ID"))
	nodePoolID              = strings.TrimSpace(os.Getenv("CIVO_NODE_POOL_ID"))
	nodeDesiredGPUCount     = strings.TrimSpace(os.Getenv("CIVO_NODE_DESIRED_GPU_COUNT"))
	rebootTimeWindowMinutes = strings.TrimSpace(os.Getenv("CIVO_NODE_REBOOT_TIME_WINDOW_MINUTES"))
	monitorOnly             = strings.TrimSpace(os.Getenv("CIVO_NODE_MONITOR_ONLY"))
	metricsPort             = strings.TrimSpace(os.Getenv("CIVO_NODE_METRICS_PORT"))
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
	checkers := health.NewDefaultCheckers(parseUintOrZero(nodeDesiredGPUCount))

	monitorOnlyFlag := true
	if v, err := strconv.ParseBool(monitorOnly); err == nil {
		monitorOnlyFlag = v
	}

	metrics.Register()
	go func() {
		port := defaultMetricsPort
		// Exclude well known port and negative integers.
		if v, err := strconv.Atoi(metricsPort); err == nil && v >= 1024 && v <= 65535 {
			port = v
		}
		addr := ":" + strconv.Itoa(port)
		slog.Info("Starting metrics server", "addr", addr)
		if err := http.ListenAndServe(addr, metrics.Handler()); err != nil {
			slog.Error("Metrics server failed", "error", err)
		}
	}()

	w, err := watcher.NewWatcher(ctx, clusterID, nodePoolID,
		watcher.WithExecutor(executor),
		watcher.WithCheckers(checkers),
		watcher.WithMonitorOnly(monitorOnlyFlag),
		watcher.WithRebootTimeWindowMinutes(rebootTimeWindowMinutes),
		watcher.WithDesiredGPUCount(nodeDesiredGPUCount),
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
		slog.String("nodePoolID", nodePoolID),
	})))

	if err := run(context.Background()); err != nil {
		slog.Error("The node-agent encountered a critical error and will exit", "error", err)
		os.Exit(1)
	}
}

func parseUintOrZero(s string) int {
	if s == "" {
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	if v < 0 {
		return 0
	}
	return v
}

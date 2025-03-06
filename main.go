package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/civo/node-agent/pkg/watcher"
)

var versionInfo = flag.Bool("version", false, "Print the driver version")

var (
	apiURL                  = strings.TrimSpace(os.Getenv("CIVO_API_URL"))
	apiKey                  = strings.TrimSpace(os.Getenv("CIVO_API_KEY"))
	region                  = strings.TrimSpace(os.Getenv("CIVO_REGION"))
	clusterID               = strings.TrimSpace(os.Getenv("CIVO_CLUSTER_ID"))
	nodePoolID              = strings.TrimSpace(os.Getenv("CIVO_NODE_POOL_ID"))
	nodeDesiredGPUCount     = strings.TrimSpace(os.Getenv("CIVO_NODE_DESIRED_GPU_COUNT"))
	rebootTimeWindowMinutes = strings.TrimSpace(os.Getenv("CIVO_NODE_REBOOT_TIME_WINDOW_MINUTES"))
)

func run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	w, err := watcher.NewWatcher(ctx, apiURL, apiKey, region, clusterID, nodePoolID,
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
		slog.Info("node-agent", "version", watcher.Version)
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

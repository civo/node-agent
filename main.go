package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/civo/node-agent/pkg/watcher"
)

var versionInfo = flag.Bool("version", false, "Print the driver version")

var (
	apiURL              = strings.TrimSpace(os.Getenv("CIVO_API_URL"))
	apiKey              = strings.TrimSpace(os.Getenv("CIVO_API_KEY"))
	region              = strings.TrimSpace(os.Getenv("CIVO_REGION"))
	clusterID           = strings.TrimSpace(os.Getenv("CIVO_CLUSTER_ID"))
	nodePoolID          = strings.TrimSpace(os.Getenv("CIVO_NODE_POOL_ID"))
	nodeDesiredGPUCount = strings.TrimSpace(os.Getenv("CIVO_NODE_DESIRED_GPU_COUNT"))
)

func run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wg := new(sync.WaitGroup)
	defer wg.Wait()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(c)

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-c
		cancel()
	}()

	w, err := watcher.NewWatcher(ctx, apiURL, apiKey, region, clusterID, nodePoolID)
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

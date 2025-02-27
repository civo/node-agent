package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/civo/node-agent/pkg/watcher"
)

var versionInfo = flag.Bool("version", false, "Print the driver version")

var (
	region      = strings.TrimSpace(os.Getenv("CIVO_REGION"))
	clusterName = strings.TrimSpace(os.Getenv("CIVO_CLUSTER_NAME"))
)

func run(ctx context.Context) error {
	w, err := watcher.NewWatcher(ctx, clusterName, region) // TODO: Add options
	if err != nil {
		return err
	}

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

	return w.Run(ctx)
}

func main() {
	flag.Parse()
	if *versionInfo {
		// TOD: log
		return
	}

	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

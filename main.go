package main

import (
	"context"
	"github.com/jokestax/node-agent/internal/k8s"
	"os"
)

func run() error {
	ctx := context.Background()

	noOfNodesToWatch := os.Getenv("NO_OF_NODES_TO_WATCH")
	kClient, err := k8s.New()
	if err != nil {
		panic(err)
	}
	go func() {
		if err := k8s.WatchNodes(ctx, kClient, noOfNodesToWatch); err != nil {
			panic(err)
		}
	}()
	<-ctx.Done()
	return nil
}

func main() {
	if err := run(); err != nil {
		panic(err)
	}

}

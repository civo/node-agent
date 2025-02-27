package main

import (
	"context"
	"os"

	"github.com/jokestax/node-agent/internal/node"
)

func run() error {
	ctx := context.Background()

	noOfNodesToWatch := os.Getenv("NO_OF_NODES_TO_WATCH")
	nodeClient, err := node.New()
	if err != nil {
		panic(err)
	}
	go func() {
		if err := nodeClient.WatchNodes(ctx, noOfNodesToWatch); err != nil {
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

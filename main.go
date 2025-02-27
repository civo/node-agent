package main

import (
	"context"

	"github.com/civo/node-agent/internal/node"
)

func run() error {
	ctx := context.Background()

	nodeClient, err := node.New(ctx)
	if err != nil {
		panic(err)
	}

	go func() {
		if err := nodeClient.WatchNodes(ctx); err != nil {
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

package node

import (
	"context"
	"fmt"
	"os"

	"github.com/civo/civogo"
	"github.com/civo/node-agent/internal/k8s"
	"github.com/konstructio/workpool"
	"k8s.io/client-go/kubernetes"
)

type NodeClient struct {
	KClient    kubernetes.Interface
	CivoClient *civogo.Client
	Pool       *workpool.Pool
}

func New(ctx context.Context) (*NodeClient, error) {

	pool, err := workpool.Initialize(ctx, 1, 100)
	if err != nil {
		return nil, err
	}
	kClient, err := k8s.New()
	if err != nil {
		return nil, err
	}

	if os.Getenv("CIVO_API_KEY") == "" {
		return nil, fmt.Errorf("CIVO_API_KEY not set")

	}

	apiKey := os.Getenv("CIVO_API_KEY")
	civoClient, err := civogo.NewClient(apiKey, "")
	if err != nil {
		return nil, err
	}

	return &NodeClient{
		KClient:    kClient,
		CivoClient: civoClient,
		Pool:       pool,
	}, nil
}

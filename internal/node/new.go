package node

import (
	"fmt"
	"os"

	"github.com/civo/civogo"
	"github.com/jokestax/node-agent/internal/k8s"
	"k8s.io/client-go/kubernetes"
)

type NodeClient struct {
	KClient    kubernetes.Interface
	CivoClient *civogo.Client
}

func New() (*NodeClient, error) {
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
	}, nil
}

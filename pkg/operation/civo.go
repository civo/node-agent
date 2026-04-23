package operation

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/civo/civogo"
)

// civoExecutor implements Executor using the Civo API.
type civoExecutor struct {
	civoClient civogo.Clienter
	clusterID  string

	apiKey  string
	apiURL  string
	region  string
	version string
}

// NewCivoExecutor creates an Executor that performs recovery actions via the Civo API.
func NewCivoExecutor(clusterID string, opts ...Option) (Executor, error) {
	e := &civoExecutor{clusterID: clusterID}
	for _, opt := range opts {
		opt(e)
	}

	if clusterID == "" {
		return nil, fmt.Errorf("cluster ID must not be empty")
	}

	if e.civoClient != nil {
		return e, nil
	}

	if e.apiKey == "" {
		return nil, fmt.Errorf("API key must not be empty")
	}
	if e.apiURL == "" {
		return nil, fmt.Errorf("API URL must not be empty")
	}

	client, err := civogo.NewClientWithURL(e.apiKey, e.apiURL, e.region)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise civo client: %w", err)
	}
	client.SetUserAgent(&civogo.Component{
		ID:      clusterID,
		Name:    "node-agent",
		Version: e.version,
	})
	e.civoClient = client
	return e, nil
}

func (e *civoExecutor) Reboot(_ context.Context, nodeName string) error {
	instance, err := e.civoClient.FindKubernetesClusterInstance(e.clusterID, nodeName)
	if err != nil {
		return fmt.Errorf("failed to find instance, clusterID: %s, nodeName: %s: %w", e.clusterID, nodeName, err)
	}

	_, err = e.civoClient.HardRebootInstance(instance.ID)
	if err != nil {
		return fmt.Errorf("failed to reboot instance, clusterID: %s, instanceID: %s: %w", e.clusterID, instance.ID, err)
	}

	slog.Info("Instance is rebooting", "instanceID", instance.ID, "node", nodeName)
	return nil
}

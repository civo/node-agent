package operation

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/civo/civogo"
)

// Option represents a configuration function that modifies civoExecutor.
type Option func(*civoExecutor)

// WithAPIConfig returns Option to configure the Civo API credentials and version.
// The client is created internally using these values.
func WithAPIConfig(apiKey, apiURL, region, version string) Option {
	return func(e *civoExecutor) {
		e.apiKey = apiKey
		e.apiURL = apiURL
		e.region = region
		e.version = version
	}
}

// WithClient returns Option to inject a pre-built Civo client (for testing).
func WithClient(client civogo.Clienter) Option {
	return func(e *civoExecutor) {
		e.civoClient = client
	}
}

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

	if e.civoClient != nil {
		return e, nil
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

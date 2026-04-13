package operation

import "github.com/civo/civogo"

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

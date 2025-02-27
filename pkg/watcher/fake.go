package watcher

import "github.com/civo/civogo"

// FakeClient is a test client used for more flexible behavior control
// when FakeClient alone is not sufficient.
type FakeClient struct {
	HardRebootInstanceFunc func(id string) (*civogo.SimpleResponse, error)

	*civogo.FakeClient
}

func (f *FakeClient) HardRebootInstance(id string) (*civogo.SimpleResponse, error) {
	if f.HardRebootInstanceFunc != nil {
		return f.HardRebootInstanceFunc(id)
	}
	return f.FakeClient.HardRebootInstance(id)
}

var _ civogo.Clienter = (*FakeClient)(nil)

package operation

import (
	"errors"
	"testing"

	"github.com/civo/civogo"
)

// fakeClient overrides the Civo API methods needed by CivoExecutor.
type fakeClient struct {
	findFunc   func(clusterID, search string) (*civogo.Instance, error)
	rebootFunc func(id string) (*civogo.SimpleResponse, error)

	*civogo.FakeClient
}

func (f *fakeClient) FindKubernetesClusterInstance(clusterID, search string) (*civogo.Instance, error) {
	if f.findFunc != nil {
		return f.findFunc(clusterID, search)
	}
	return f.FakeClient.FindKubernetesClusterInstance(clusterID, search)
}

func (f *fakeClient) HardRebootInstance(id string) (*civogo.SimpleResponse, error) {
	if f.rebootFunc != nil {
		return f.rebootFunc(id)
	}
	return f.FakeClient.HardRebootInstance(id)
}

var _ civogo.Clienter = (*fakeClient)(nil)

func TestCivoExecutor_Reboot(t *testing.T) {
	tests := []struct {
		name     string
		nodeName string
		client   *fakeClient
		wantErr  bool
	}{
		{
			name:     "Returns nil on successful find and reboot",
			nodeName: "node-01",
			client: &fakeClient{
				findFunc: func(clusterID, search string) (*civogo.Instance, error) {
					return &civogo.Instance{ID: "instance-01"}, nil
				},
				rebootFunc: func(id string) (*civogo.SimpleResponse, error) {
					if id != "instance-01" {
						t.Errorf("instanceID mismatch: got %s, want instance-01", id)
					}
					return new(civogo.SimpleResponse), nil
				},
			},
		},
		{
			name:     "Returns error when instance lookup fails",
			nodeName: "node-01",
			client: &fakeClient{
				findFunc: func(_, _ string) (*civogo.Instance, error) {
					return nil, errors.New("not found")
				},
			},
			wantErr: true,
		},
		{
			name:     "Returns error when hard reboot fails",
			nodeName: "node-01",
			client: &fakeClient{
				findFunc: func(_, _ string) (*civogo.Instance, error) {
					return &civogo.Instance{ID: "instance-01"}, nil
				},
				rebootFunc: func(_ string) (*civogo.SimpleResponse, error) {
					return nil, errors.New("reboot failed")
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec, err := NewCivoExecutor("test-cluster", WithClient(tt.client))
			if err != nil {
				t.Fatal(err)
			}
			err = exec.Reboot(t.Context(), tt.nodeName)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

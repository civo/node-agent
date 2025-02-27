package watcher

import (
	"errors"
	"testing"

	"github.com/civo/civogo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var (
	testClusterID           = "test-cluster-123"
	testRegion              = "lon1"
	testApiKey              = "test-api-key"
	testApiURL              = "https://test.civo.com"
	testNodePoolID          = "test-node-pool"
	testNodeDesiredGPUCount = "8"
)

func TestRun(t *testing.T) {
	type args struct {
		opts                []Option
		nodeDesiredGPUCount string
		nodePoolID          string
	}
	type test struct {
		name       string
		args       args
		beforeFunc func(*watcher)
		wantErr    bool
	}

	tests := []test{
		{
			name: "Returns nil when node GPU count is 8 and no reboot needed",
			args: args{
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithCivoClient(&FakeClient{}),
				},
				nodeDesiredGPUCount: testNodeDesiredGPUCount,
				nodePoolID:          testNodePoolID,
			},
			beforeFunc: func(w *watcher) {
				t.Helper()
				client := w.client.(*fake.Clientset)

				nodes := &corev1.NodeList{
					Items: []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "node-01",
								Labels: map[string]string{
									nodePoolLabelKey: testNodePoolID,
								},
							},
							Status: corev1.NodeStatus{
								Conditions: []corev1.NodeCondition{
									{
										Type:   corev1.NodeReady,
										Status: corev1.ConditionTrue,
									},
									{
										Type:   corev1.NodeReady,
										Status: corev1.ConditionFalse,
									},
								},
								Allocatable: corev1.ResourceList{
									gpuResourceName: resource.MustParse("8"),
								},
							},
						},
					},
				}
				client.Fake.PrependReactor("list", "nodes", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nodes, nil
				})
			},
		},
		{
			name: "Returns nil and triggers reboot when GPU count drops below desired (7 GPUs available)",
			args: args{
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithCivoClient(&FakeClient{}),
				},
				nodeDesiredGPUCount: testNodeDesiredGPUCount,
				nodePoolID:          testNodePoolID,
			},
			beforeFunc: func(w *watcher) {
				t.Helper()
				client := w.client.(*fake.Clientset)

				nodes := &corev1.NodeList{
					Items: []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "node-01",
								Labels: map[string]string{
									nodePoolLabelKey: testNodePoolID,
								},
							},
							Status: corev1.NodeStatus{
								Conditions: []corev1.NodeCondition{
									{
										Type:   corev1.NodeReady,
										Status: corev1.ConditionTrue,
									},
									{
										Type:   corev1.NodeReady,
										Status: corev1.ConditionFalse,
									},
								},
								Allocatable: corev1.ResourceList{
									gpuResourceName: resource.MustParse("7"),
								},
							},
						},
					},
				}
				client.Fake.PrependReactor("list", "nodes", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nodes, nil
				})

				civoClient := w.civoClient.(*FakeClient)
				instance := &civogo.Instance{
					ID: "instance-01",
				}
				civoClient.FindKubernetesClusterInstanceFunc = func(clusterID, search string) (*civogo.Instance, error) {
					return instance, nil
				}
				civoClient.HardRebootInstanceFunc = func(id string) (*civogo.SimpleResponse, error) {
					return new(civogo.SimpleResponse), nil
				}
			},
		},
		{
			name: "Returns nil and triggers reboot when GPU count matches desired but node is not ready",
			args: args{
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithCivoClient(&FakeClient{}),
				},
				nodeDesiredGPUCount: testNodeDesiredGPUCount,
				nodePoolID:          testNodePoolID,
			},
			beforeFunc: func(w *watcher) {
				t.Helper()
				client := w.client.(*fake.Clientset)

				nodes := &corev1.NodeList{
					Items: []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "node-01",
								Labels: map[string]string{
									nodePoolLabelKey: testNodePoolID,
								},
							},
							Status: corev1.NodeStatus{
								Conditions: []corev1.NodeCondition{
									{
										Type:   corev1.NodeReady,
										Status: corev1.ConditionFalse,
									},
								},
								Allocatable: corev1.ResourceList{
									gpuResourceName: resource.MustParse("8"),
								},
							},
						},
					},
				}
				client.Fake.PrependReactor("list", "nodes", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nodes, nil
				})

				civoClient := w.civoClient.(*FakeClient)
				instance := &civogo.Instance{
					ID: "instance-01",
				}
				civoClient.FindKubernetesClusterInstanceFunc = func(clusterID, search string) (*civogo.Instance, error) {
					return instance, nil
				}
				civoClient.HardRebootInstanceFunc = func(id string) (*civogo.SimpleResponse, error) {
					return new(civogo.SimpleResponse), nil
				}
			},
		},
		{
			name: "Returns an error when unable to list nodes",
			args: args{
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithCivoClient(&FakeClient{}),
				},
				nodeDesiredGPUCount: testNodeDesiredGPUCount,
				nodePoolID:          testNodePoolID,
			},
			beforeFunc: func(w *watcher) {
				t.Helper()
				client := w.client.(*fake.Clientset)

				client.Fake.PrependReactor("list", "nodes", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, &corev1.NodeList{}, errors.New("invalid error")
				})
			},
			wantErr: true,
		},

		{
			name: "Returns an error when finding the Kubernetes cluster instance fails during reboot",
			args: args{
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithCivoClient(&FakeClient{}),
				},
				nodeDesiredGPUCount: testNodeDesiredGPUCount,
				nodePoolID:          testNodePoolID,
			},
			beforeFunc: func(w *watcher) {
				t.Helper()
				client := w.client.(*fake.Clientset)

				nodes := &corev1.NodeList{
					Items: []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "node-01",
								Labels: map[string]string{
									nodePoolLabelKey: testNodePoolID,
								},
							},
							Status: corev1.NodeStatus{
								Conditions: []corev1.NodeCondition{
									{
										Type:   corev1.NodeReady,
										Status: corev1.ConditionFalse,
									},
								},
								Allocatable: corev1.ResourceList{
									gpuResourceName: resource.MustParse("8"),
								},
							},
						},
					},
				}
				client.Fake.PrependReactor("list", "nodes", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nodes, nil
				})

				civoClient := w.civoClient.(*FakeClient)
				civoClient.FindKubernetesClusterInstanceFunc = func(clusterID, search string) (*civogo.Instance, error) {
					return nil, errors.New("invalid error")
				}
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w, err := NewWatcher(t.Context(),
				testApiURL, testApiKey, testRegion, testClusterID, test.args.nodePoolID, test.args.nodeDesiredGPUCount, test.args.opts...)
			if err != nil {
				t.Fatal(err)
			}

			obj := w.(*watcher)
			if test.beforeFunc != nil {
				test.beforeFunc(obj)
			}

			err = obj.run(t.Context())
			if (err != nil) != test.wantErr {
				t.Errorf("error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestIsNodeReady(t *testing.T) {
	type test struct {
		name string
		node *corev1.Node
		want bool
	}

	tests := []test{
		{
			name: "Returns true when Node is ready state",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-01",
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionTrue,
						},
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "Returns false when Node is not ready state",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-01",
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
		},
		{
			name: "Returns false when no conditions for the node",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-01",
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isNodeReady(test.node)
			if got != test.want {
				t.Errorf("got = %v, want %v", got, test.want)
			}
		})
	}
}

func TestIsNodeDesiredGPU(t *testing.T) {
	type test struct {
		name    string
		node    *corev1.Node
		desired int
		want    bool
	}

	tests := []test{
		{
			name: "Returns true when GPU count matches desired value",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-01",
				},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						gpuResourceName: resource.MustParse("8"),
					},
				},
			},
			desired: 8,
			want:    true,
		},
		{
			name: "Returns false when GPU count is 0",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-01",
				},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						gpuResourceName: resource.MustParse("0"),
					},
				},
			},
			desired: 8,
			want:    false,
		},
		{
			name: "Returns false when GPU count is less than desired value",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-01",
				},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						gpuResourceName: resource.MustParse("7"),
					},
				},
			},
			desired: 8,
			want:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isNodeDesiredGPU(test.node, test.desired)
			if got != test.want {
				t.Errorf("got = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRebootNode(t *testing.T) {
	type args struct {
		nodeName string
		opts     []Option
	}
	type test struct {
		name       string
		args       args
		beforeFunc func(*testing.T, *watcher)
		wantErr    bool
	}

	tests := []test{
		{
			name: "Returns nil when there is no error finding and rebooting the instance",
			args: args{
				nodeName: "node-01",
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithCivoClient(&FakeClient{}),
				},
			},
			beforeFunc: func(t *testing.T, w *watcher) {
				t.Helper()
				client := w.civoClient.(*FakeClient)

				instance := &civogo.Instance{
					ID: "instance-01",
				}

				client.FindKubernetesClusterInstanceFunc = func(clusterID, search string) (*civogo.Instance, error) {
					return instance, nil
				}
				client.HardRebootInstanceFunc = func(id string) (*civogo.SimpleResponse, error) {
					if instance.ID != id {
						t.Errorf("instanceId dose not match. want: %s, but got: %s", instance.ID, id)
					}
					return new(civogo.SimpleResponse), nil
				}
			},
		},
		{
			name: "Returns an error when instance lookup fails",
			args: args{
				nodeName: "node-01",
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithCivoClient(&FakeClient{}),
				},
			},
			beforeFunc: func(t *testing.T, w *watcher) {
				t.Helper()
				client := w.civoClient.(*FakeClient)

				client.FindKubernetesClusterInstanceFunc = func(clusterID, search string) (*civogo.Instance, error) {
					return nil, errors.New("invalid error")
				}
			},
			wantErr: true,
		},
		{
			name: "Returns an error when instance reboot fails",
			args: args{
				nodeName: "node-01",
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithCivoClient(&FakeClient{}),
				},
			},
			beforeFunc: func(t *testing.T, w *watcher) {
				t.Helper()
				client := w.civoClient.(*FakeClient)

				instance := &civogo.Instance{
					ID: "instance-01",
				}

				client.FindKubernetesClusterInstanceFunc = func(clusterID, search string) (*civogo.Instance, error) {
					return instance, nil
				}
				client.HardRebootInstanceFunc = func(id string) (*civogo.SimpleResponse, error) {
					if instance.ID != id {
						t.Errorf("instanceId dose not match. want: %s, but got: %s", instance.ID, id)
					}
					return nil, errors.New("invalid error")
				}
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w, err := NewWatcher(t.Context(),
				testApiURL, testApiKey, testRegion, testClusterID, testNodePoolID, testNodeDesiredGPUCount, test.args.opts...)
			if err != nil {
				t.Fatal(err)
			}

			obj := w.(*watcher)
			if test.beforeFunc != nil {
				test.beforeFunc(t, obj)
			}

			err = obj.rebootNode(test.args.nodeName)
			if (err != nil) != test.wantErr {
				t.Errorf("error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

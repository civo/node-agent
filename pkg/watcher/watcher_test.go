package watcher

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/civo/civogo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var (
	testClusterID               = "test-cluster-123"
	testRegion                  = "lon1"
	testApiKey                  = "test-api-key"
	testApiURL                  = "https://test.civo.com"
	testNodePoolID              = "test-node-pool"
	testNodeDesiredGPUCount     = "8"
	testRebootTimeWindowMinutes = time.Duration(40)
)

func TestNew(t *testing.T) {
	type args struct {
		clusterID  string
		region     string
		apiKey     string
		apiURL     string
		nodePoolID string
		opts       []Option
	}
	type test struct {
		name      string
		args      args
		checkFunc func(*watcher) error
		wantErr   bool
	}

	tests := []test{
		{
			name: "Returns no error when given valid input",
			args: args{
				clusterID:  testClusterID,
				region:     testRegion,
				apiKey:     testApiKey,
				apiURL:     testApiURL,
				nodePoolID: testNodePoolID,
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithCivoClient(&FakeClient{}),
					WithDesiredGPUCount(testNodeDesiredGPUCount),
				},
			},
			checkFunc: func(w *watcher) error {
				if w.clusterID != testClusterID {
					return fmt.Errorf("clusterID mismatch: got %s, want %s", w.clusterID, testClusterID)
				}
				if w.region != testRegion {
					return fmt.Errorf("region mismatch: got %s, want %s", w.region, testRegion)
				}
				if w.apiKey != testApiKey {
					return fmt.Errorf("apiKey mismatch: got %s, want %s", w.apiKey, testApiKey)
				}
				if w.apiURL != testApiURL {
					return fmt.Errorf("apiURL mismatch: got %s, want %s", w.apiURL, testApiURL)
				}

				cnt, err := strconv.Atoi(testNodeDesiredGPUCount)
				if err != nil {
					return err
				}
				if w.nodeDesiredGPUCount != cnt {
					return fmt.Errorf("nodeDesiredGPUCount mismatch: got %d, want %s", w.nodeDesiredGPUCount, testNodeDesiredGPUCount)
				}
				if w.nodeSelector == nil || w.nodeSelector.MatchLabels[nodePoolLabelKey] != testNodePoolID {
					return fmt.Errorf("nodeSelector mismatch: got %v, want %s", w.nodeSelector, testNodePoolID)
				}
				if w.client == nil {
					return fmt.Errorf("client is nil")
				}
				if w.civoClient == nil {
					return fmt.Errorf("civoClient is nil")
				}
				if w.rebootTimeWindowMinutes != testRebootTimeWindowMinutes {
					return fmt.Errorf("w.rebootTimeWindowMinutes mismatch: got %v, want %s", w.nodeSelector, testNodePoolID)
				}
				return nil
			},
		},
		{
			name: "Returns no error when input is invalid, but default value is set",
			args: args{
				clusterID:  testClusterID,
				region:     testRegion,
				apiKey:     testApiKey,
				apiURL:     testApiURL,
				nodePoolID: testNodePoolID,
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithCivoClient(&FakeClient{}),
					WithDesiredGPUCount("invalid"),              // It is invalid, but the default count (0) will be used.
					WithDesiredGPUCount("-1"),                   // It is invalid, but the default count (0) will be used.
					WithRebootTimeWindowMinutes("invalid time"), // It is invalid, but the default time (40) will be used.
					WithRebootTimeWindowMinutes("0"),            // It is invalid, but the default time (40) will be used.
				},
			},
			checkFunc: func(w *watcher) error {
				if w.nodeDesiredGPUCount != 0 {
					return fmt.Errorf("w.nodeDesiredGPUCount mismatch: got %d, want %d", w.nodeDesiredGPUCount, 0)
				}
				if w.rebootTimeWindowMinutes != testRebootTimeWindowMinutes {
					return fmt.Errorf("w.rebootTimeWindowMinutes mismatch: got %v, want %s", w.nodeSelector, testNodePoolID)
				}
				return nil
			},
		},
		{
			name: "Returns no error when nodeDesiredGPUCount is 0",
			args: args{
				clusterID:  testClusterID,
				region:     testRegion,
				apiKey:     testApiKey,
				apiURL:     testApiURL,
				nodePoolID: testNodePoolID,
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithCivoClient(&FakeClient{}),
					WithDesiredGPUCount("0"),
				},
			},
			checkFunc: func(w *watcher) error {
				if w.nodeDesiredGPUCount != 0 {
					return fmt.Errorf("w.nodeDesiredGPUCount mismatch: got %d, want %d", w.nodeDesiredGPUCount, 0)
				}
				return nil
			},
		},
		{
			name: "Returns an error when clusterID is missing",
			args: args{
				region:     testRegion,
				apiKey:     testApiKey,
				apiURL:     testApiURL,
				nodePoolID: testNodePoolID,
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithCivoClient(&FakeClient{}),
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w, err := NewWatcher(t.Context(),
				test.args.apiURL,
				test.args.apiKey,
				test.args.region,
				test.args.clusterID,
				test.args.nodePoolID,
				test.args.opts...)
			if (err != nil) != test.wantErr {
				t.Errorf("error = %v, wantErr %v", err, test.wantErr)
			}

			if !test.wantErr {
				if w == nil {
					t.Errorf("expected non-nil object, but got nil")
					return
				}
				obj := w.(*watcher)
				if test.checkFunc != nil {
					if err := test.checkFunc(obj); err != nil {
						t.Errorf("checkFunc error: %v", err)
					}
				}
			}
		})
	}
}

func TestRun(t *testing.T) {
	type args struct {
		opts       []Option
		nodePoolID string
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
					WithDesiredGPUCount(testNodeDesiredGPUCount),
				},
				nodePoolID: testNodePoolID,
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
					WithDesiredGPUCount(testNodeDesiredGPUCount),
				},
				nodePoolID: testNodePoolID,
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
					WithDesiredGPUCount(testNodeDesiredGPUCount),
				},
				nodePoolID: testNodePoolID,
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
			name: "Returns nil and skips reboot when GPU count matches desired but node is not ready, and LastTransitionTime is more recent than thresholdTime",
			args: args{
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithCivoClient(&FakeClient{}),
					WithDesiredGPUCount(testNodeDesiredGPUCount),
				},
				nodePoolID: testNodePoolID,
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
										Type:               corev1.NodeReady,
										Status:             corev1.ConditionFalse,
										LastTransitionTime: metav1.NewTime(time.Now()),
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
			name: "Returns an error when unable to list nodes",
			args: args{
				opts: []Option{
					WithKubernetesClient(fake.NewSimpleClientset()),
					WithCivoClient(&FakeClient{}),
					WithDesiredGPUCount(testNodeDesiredGPUCount),
				},
				nodePoolID: testNodePoolID,
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
					WithDesiredGPUCount(testNodeDesiredGPUCount),
				},
				nodePoolID: testNodePoolID,
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
				testApiURL, testApiKey, testRegion, testClusterID, test.args.nodePoolID, test.args.opts...)
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

func TestIsReadyOrNotReadyStatusChangedAfter(t *testing.T) {
	type test struct {
		name          string
		node          *corev1.Node
		thresholdTime time.Time
		want          bool
	}

	tests := []test{
		{
			name: "Returns true when NodeReady condition is true (Ready) and last transition time is after threshold",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-01",
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:               corev1.NodeReady,
							Status:             corev1.ConditionTrue,
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
					},
				},
			},
			thresholdTime: time.Now().Add(-time.Hour),
			want:          true,
		},
		{
			name: "Returns true when NodeReady condition is false (NotReady) and last transition time is after threshold",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-01",
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:               corev1.NodeReady,
							Status:             corev1.ConditionFalse,
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
					},
				},
			},
			thresholdTime: time.Now().Add(-time.Hour),
			want:          true,
		},
		{
			name: "Returns false when the latest NodeReady condition is older than thresholdTime",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-01",
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:               corev1.NodeReady,
							Status:             corev1.ConditionFalse,
							LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour)),
						},
					},
				},
			},
			thresholdTime: time.Now(),
			want:          false,
		},
		{
			name: "Returns false when no conditions are present on the node",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-01",
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{},
				},
			},
			thresholdTime: time.Now().Add(-time.Hour),
			want:          false,
		},
		{
			name: "Returns false when there is only NodeDiskPressure condition",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-01",
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:              corev1.NodeDiskPressure,
							Status:            corev1.ConditionFalse,
							LastHeartbeatTime: metav1.NewTime(time.Now()),
						},
					},
				},
			},
			thresholdTime: time.Now().Add(-time.Hour),
			want:          false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isReadyOrNotReadyStatusChangedAfter(test.node, test.thresholdTime)
			if got != test.want {
				t.Errorf("got = %v, want %v", got, test.want)
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
			name: "Returns true when desired GPU count is, so count check is skipped",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-01",
				},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{},
				},
			},
			desired: 0,
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
					WithDesiredGPUCount(testNodeDesiredGPUCount),
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
					WithDesiredGPUCount(testNodeDesiredGPUCount),
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
					WithDesiredGPUCount(testNodeDesiredGPUCount),
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
				testApiURL, testApiKey, testRegion, testClusterID, testNodePoolID, test.args.opts...)
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

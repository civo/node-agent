package k8s

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func config(inCluster string) (*rest.Config, error) {
	if inCluster == "true" {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		return config, nil
	} else {
		KUBECONFIG := os.Getenv("KUBECONFIG")
		if len(KUBECONFIG) == 0 {
			return nil, fmt.Errorf("did not find kubeconfig")
		}
		config, err := clientcmd.BuildConfigFromFlags("", KUBECONFIG)
		if err != nil {
			return nil, err
		}
		return config, err
	}
}

func New() (kubernetes.Interface, error) {

	inCluster := os.Getenv("IN_CLUSTER")

	kcfg, err := config(inCluster)
	if err != nil {
		fmt.Errorf("failed to get k8s client: %w", err)
	}

	client, err := kubernetes.NewForConfig(kcfg)
	if err != nil {
		fmt.Errorf("Error creating k8s client: %w", err)
	}

	return client, nil

}

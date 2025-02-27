package k8s

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func WatchNodes(ctx context.Context, client kubernetes.Interface, nodes string) error {

	ticker := time.NewTicker(10 * time.Second)

	for {
		select {
		case <-ticker.C:
			listNodes(ctx, client)
		}
	}

}

func listNodes(ctx context.Context, client kubernetes.Interface) {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing nodes: %v\n", err)
		return
	}

	fmt.Println("\nNodes List:")
	for _, node := range nodes.Items {
		fmt.Printf("- %s (%s)\n", node.Name, getNodeCondition(node))
	}
}

func getNodeCondition(node v1.Node) string {
	for _, cond := range node.Status.Conditions {
		if cond.Type == v1.NodeReady {
			if cond.Status == v1.ConditionTrue {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}

package node

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (n *NodeClient) WatchNodes(ctx context.Context) error {

	ticker := time.NewTicker(10 * time.Second)

	for {
		select {
		case <-ticker.C:
			n.listNodes(ctx)
		}
	}

}

func (n *NodeClient) listNodes(ctx context.Context) {
	nodes, err := n.KClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing nodes: %v\n", err)
		return
	}
	var nodesMap []*v1.Node

	for _, node := range nodes.Items {
		condition := getNodeCondition(node)
		if condition != "Ready" {
			nodesMap = append(nodesMap, &node)
		}
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

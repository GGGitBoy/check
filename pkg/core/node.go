package core

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	"inspection-server/pkg/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func GetNodes(client *apis.Client, clusterNodeConfig *apis.ClusterNodeConfig, taskName string, nodeItem map[string][]*apis.Item) ([]*apis.Node, error) {
	logrus.Infof("[%s] Starting node inspection", taskName)
	nodeNodeArray := apis.NewNodes()

	if clusterNodeConfig.Enable {
		nodes := make(map[string][]*apis.CommandConfig)
		for _, n := range clusterNodeConfig.NodeConfig {
			listOptions := metav1.ListOptions{}
			if n.SelectorLabels != nil && len(n.SelectorLabels) > 0 {
				var set labels.Set
				set = n.SelectorLabels
				listOptions = metav1.ListOptions{LabelSelector: set.AsSelector().String()}
			}

			nodeList, err := client.Clientset.CoreV1().Nodes().List(context.TODO(), listOptions)
			if err != nil {
				return nil, fmt.Errorf("Failed to list nodes : %v\n", err)
			}

			for _, i := range nodeList.Items {
				nodes[i.Name] = append(nodes[i.Name], n.Commands...)
			}
		}

		podSet := labels.Set(map[string]string{"name": "inspection-agent"})
		podList, err := client.Clientset.CoreV1().Pods(common.InspectionNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: podSet.String()})
		if err != nil {
			return nil, fmt.Errorf("Error listing pods in namespace %s: %v\n", common.InspectionNamespace, err)
		}

		for _, pod := range podList.Items {
			node, err := client.Clientset.CoreV1().Nodes().Get(context.TODO(), pod.Spec.NodeName, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("Error getting node %s: %v\n", pod.Spec.NodeName, err)
			}

			podLimits := getResourceList(node.Annotations["management.cattle.io/pod-limits"])
			podRequests := getResourceList(node.Annotations["management.cattle.io/pod-requests"])

			limitsCPU := podLimits.Cpu().Value()
			limitsMemory := podLimits.Memory().Value()
			requestsCPU := podRequests.Cpu().Value()
			requestsMemory := podRequests.Memory().Value()
			requestsPods := podRequests.Pods().Value()
			allocatableCPU, _ := node.Status.Allocatable.Cpu().AsInt64()
			allocatableMemory, _ := node.Status.Allocatable.Memory().AsInt64()
			allocatablePods, _ := node.Status.Allocatable.Pods().AsInt64()

			highLimitsCPU := true
			highLimitsCPUMessage := ""
			if float64(limitsCPU)/float64(allocatableCPU) > 0.8 {
				highLimitsCPU = false
				highLimitsCPUMessage = fmt.Sprintf("节点 %s limits CPU 超过百分之 80", pod.Spec.NodeName)
				logrus.Infof("[%s] Node %s High Limits CPU: limits CPU %d, allocatable CPU %d", taskName, pod.Spec.NodeName, limitsCPU, allocatableCPU)
			}

			highLimitsMemory := true
			highLimitsMemoryMessage := ""
			if float64(limitsMemory)/float64(allocatableMemory) > 0.8 {
				highLimitsMemory = false
				highLimitsMemoryMessage = fmt.Sprintf("节点 %s limits Memory 超过百分之 80", pod.Spec.NodeName)
				logrus.Infof("[%s] Node %s High Limits Memory: limits Memory %d, allocatable Memory %d", taskName, pod.Spec.NodeName, limitsMemory, allocatableMemory)
			}

			highRequestsCPU := true
			highRequestsCPUMessage := ""
			if float64(requestsCPU)/float64(allocatableCPU) > 0.8 {
				highRequestsCPU = false
				highRequestsCPUMessage = fmt.Sprintf("节点 %s requests CPU 超过百分之 80", pod.Spec.NodeName)
				logrus.Infof("[%s] Node %s High Requests CPU: requests CPU %d, allocatable CPU %d", taskName, pod.Spec.NodeName, requestsCPU, allocatableCPU)
			}

			highRequestsMemory := true
			highRequestsMemoryMessage := ""
			if float64(requestsMemory)/float64(allocatableMemory) > 0.8 {
				highRequestsMemory = false
				highRequestsMemoryMessage = fmt.Sprintf("节点 %s requests Memory 超过百分之 80", pod.Spec.NodeName)
				logrus.Infof("[%s] Node %s High Requests Memory: requests Memory %d, allocatable Memory %d", taskName, pod.Spec.NodeName, requestsMemory, allocatableMemory)
			}

			highRequestsPods := true
			highRequestsPodsMessage := ""
			if float64(requestsPods)/float64(allocatablePods) > 0.8 {
				highRequestsPods = false
				highRequestsPodsMessage = fmt.Sprintf("节点 %s requests Pods 超过百分之 80", pod.Spec.NodeName)
				logrus.Infof("[%s] Node %s High Requests Pods: requests Pods %d, allocatable Pods %d", taskName, pod.Spec.NodeName, requestsPods, allocatablePods)
			}

			var commands []string
			commandConfigs, ok := nodes[pod.Status.HostIP]
			if ok {
				for _, c := range commandConfigs {
					commands = append(commands, c.Description+": "+c.Command)
				}
			}

			logrus.Debugf("Commands to execute on node %s: %v", pod.Spec.NodeName, commands)
			command := "/opt/inspection/inspection.sh"
			stdout, stderr, err := ExecToPodThroughAPI(client.Clientset, client.Config, command, commands, pod.Namespace, pod.Name, "inspection-agent-container", taskName)
			if err != nil {
				return nil, fmt.Errorf("Error executing command in pod %s: %v\n", pod.Name, err)
			}

			if stderr != "" {
				logrus.Errorf("Stderr from pod %s: %s", pod.Name, stderr)
			}

			var results []apis.CommandCheckResult
			err = json.Unmarshal([]byte(stdout), &results)
			if err != nil {
				return nil, fmt.Errorf("Error unmarshalling stdout for pod %s: %v\n", pod.Name, err)
			}

			items := []*apis.Item{
				{
					Name:    "Limits CPU 超过 80 %",
					Message: highLimitsCPUMessage,
					Pass:    highLimitsCPU,
					Level:   2,
				},
				{
					Name:    "Limits Memory 超过 80 %",
					Message: highLimitsMemoryMessage,
					Pass:    highLimitsMemory,
					Level:   2,
				},
				{
					Name:    "Requests CPU 超过 80 %",
					Message: highRequestsCPUMessage,
					Pass:    highRequestsCPU,
					Level:   2,
				},
				{
					Name:    "Requests Memory 超过 80 %",
					Message: highRequestsMemoryMessage,
					Pass:    highRequestsMemory,
					Level:   2,
				},
				{
					Name:    "Requests Pods 超过 80 %",
					Message: highRequestsPodsMessage,
					Pass:    highRequestsPods,
					Level:   2,
				},
			}

			for _, r := range results {
				commandCheck := true
				commandCheckMessage := ""
				if r.Error != "" {
					commandCheck = false
					commandCheckMessage = fmt.Sprintf("%s", r.Error)
					logrus.Errorf("Node %s inspection failed (%s): %s", pod.Spec.NodeName, r.Description, r.Error)
				}

				items = append(items, &apis.Item{
					Name:    r.Description,
					Message: commandCheckMessage,
					Pass:    commandCheck,
					Level:   2,
				})
			}

			grafanaItem, ok := nodeItem[pod.Status.HostIP]
			if ok {
				items = append(items, grafanaItem...)
			}

			itemsCount := GetItemsCount(items)

			nodeData := &apis.Node{
				Name:   pod.Spec.NodeName,
				HostIP: pod.Status.HostIP,
				//Resource: &apis.Resource{
				//	LimitsCPU:         limitsCPU,
				//	LimitsMemory:      limitsMemory,
				//	RequestsCPU:       requestsCPU,
				//	RequestsMemory:    requestsMemory,
				//	RequestsPods:      requestsPods,
				//	AllocatableCPU:    allocatableCPU,
				//	AllocatableMemory: allocatableMemory,
				//	AllocatablePods:   allocatablePods,
				//},
				//Commands: &apis.Command{
				//	Stdout: results,
				//	Stderr: stderr,
				//},
				Items:      items,
				ItemsCount: itemsCount,
			}

			nodeNodeArray = append(nodeNodeArray, nodeData)
		}
	}

	return nodeNodeArray, nil
}

func getResourceList(val string) corev1.ResourceList {
	if val == "" {
		return nil
	}
	result := corev1.ResourceList{}
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return corev1.ResourceList{}
	}
	return result
}

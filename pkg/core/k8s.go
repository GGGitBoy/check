package core

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	"inspection-server/pkg/common"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"os"
	"regexp"
	"strings"
	"sync"
)

var (
	Commands = []*apis.CommandConfig{
		{
			Description: "API Server Ready Check",
			Command:     "kubectl get --raw='/readyz'",
		},
		{
			Description: "API Server Live Check",
			Command:     "kubectl get --raw='/livez'",
		},
		{
			Description: "ETCD Ready Check",
			Command:     "kubectl get --raw='/readyz/etcd'",
		},
		{
			Description: "ETCD Live Check",
			Command:     "kubectl get --raw='/livez/etcd'",
		},
	}
)

func GetHealthCheck(client *apis.Client, clusterName, taskName string) (*apis.HealthCheck, []*apis.Inspection, error) {
	logrus.Infof("[%s] Starting health check inspection", taskName)
	healthCheck := apis.NewHealthCheck()
	coreInspections := apis.NewInspections()

	set := labels.Set(map[string]string{"name": "inspection-agent"})
	podList, err := client.Clientset.CoreV1().Pods(common.InspectionNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		return nil, nil, fmt.Errorf("Error listing pods in namespace %s: %v\n", common.InspectionNamespace, err)
	}

	if len(podList.Items) > 0 {
		var commands []string
		for _, c := range Commands {
			commands = append(commands, c.Description+": "+c.Command)
			logrus.Infof("[%s] add command description: %s, command: %s", taskName, c.Description, c.Command)
		}

		command := "/opt/inspection/inspection.sh"
		stdout, stderr, err := ExecToPodThroughAPI(client.Clientset, client.Config, command, commands, podList.Items[0].Namespace, podList.Items[0].Name, "inspection-agent-container", taskName)
		if err != nil {
			return nil, nil, fmt.Errorf("Error executing command in pod %s: %v\n", podList.Items[0].Name, err)
		}

		if stderr != "" {
			return nil, nil, fmt.Errorf("Stderr from pod %s: %s\n", podList.Items[0].Name, stderr)
		}

		var results []apis.CommandCheckResult
		err = json.Unmarshal([]byte(stdout), &results)
		if err != nil {
			return nil, nil, fmt.Errorf("Error unmarshalling stdout for pod %s: %v\n", podList.Items[0].Name, err)
		}

		for _, r := range results {
			if r.Error != "" {
				coreInspections = append(coreInspections, apis.NewInspection(fmt.Sprintf("cluster %s (%s) failed", clusterName, r.Description), fmt.Sprintf("%s", r.Error), 3))
			}

			switch r.Description {
			case "API Server Ready Check":
				healthCheck.APIServerReady = getCommandCheckResult(r)
			case "API Server Live Check":
				healthCheck.APIServerLive = getCommandCheckResult(r)
			case "ETCD Ready Check":
				healthCheck.EtcdReady = getCommandCheckResult(r)
			case "ETCD Live Check":
				healthCheck.EtcdLive = getCommandCheckResult(r)
			}
		}
	}

	return healthCheck, coreInspections, nil
}

func getCommandCheckResult(r apis.CommandCheckResult) *apis.CommandCheckResult {
	return &apis.CommandCheckResult{
		Description: r.Description,
		Command:     r.Command,
		Response:    r.Response,
		Error:       r.Error,
	}
}

type NodeTemplate struct {
	NodeTemplateConfig []NodeTemplateConfig `json:"node_template_config"`
}

type NodeTemplateConfig struct {
	Nodes map[string][]*apis.CommandConfig `json:"nodes"`
}

func GetNodes(client *apis.Client, nodesConfig []*apis.NodeConfig, taskName string) ([]*apis.Node, []*apis.Inspection, error) {
	logrus.Infof("[%s] Starting node inspection", taskName)
	nodeNodeArray := apis.NewNodes()
	nodeInspections := apis.NewInspections()

	var nodeTemplate NodeTemplate
	for _, n := range nodesConfig {
		var set labels.Set
		if n.SelectorLabels != nil {
			set = n.SelectorLabels
		}

		nodeList, err := client.Clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: set.AsSelector().String()})
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to list nodes : %v\n", err)
		}

		nodes := make(map[string][]*apis.CommandConfig)
		for _, i := range nodeList.Items {
			nodes[i.Name] = n.Commands
		}

		nodeTemplate.NodeTemplateConfig = append(nodeTemplate.NodeTemplateConfig, NodeTemplateConfig{
			Nodes: nodes,
		})
	}

	podSet := labels.Set(map[string]string{"name": "inspection-agent"})
	podList, err := client.Clientset.CoreV1().Pods(common.InspectionNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: podSet.String()})
	if err != nil {
		return nil, nil, fmt.Errorf("Error listing pods in namespace %s: %v\n", common.InspectionNamespace, err)
	}

	for _, pod := range podList.Items {
		node, err := client.Clientset.CoreV1().Nodes().Get(context.TODO(), pod.Spec.NodeName, metav1.GetOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("Error getting node %s: %v\n", pod.Spec.NodeName, err)
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
			nodeInspections = append(nodeInspections, apis.NewInspection(fmt.Sprintf("Node %s High Limits CPU", pod.Spec.NodeName), fmt.Sprintf("节点 %s limits CPU 超过百分之 80", pod.Spec.NodeName), 2))
			logrus.Infof("[%s] Node %s High Limits CPU: limits CPU %d, allocatable CPU %d", taskName, pod.Spec.NodeName, limitsCPU, allocatableCPU)
		}

		highLimitsMemory := true
		highLimitsMemoryMessage := ""
		if float64(limitsMemory)/float64(allocatableMemory) > 0.8 {
			highLimitsMemory = false
			highLimitsMemoryMessage = fmt.Sprintf("节点 %s limits Memory 超过百分之 80", pod.Spec.NodeName)
			nodeInspections = append(nodeInspections, apis.NewInspection(fmt.Sprintf("Node %s High Limits Memory", pod.Spec.NodeName), fmt.Sprintf("节点 %s limits Memory 超过百分之 80", pod.Spec.NodeName), 2))
			logrus.Infof("[%s] Node %s High Limits Memory: limits Memory %d, allocatable Memory %d", taskName, pod.Spec.NodeName, limitsMemory, allocatableMemory)
		}

		highRequestsCPU := true
		highRequestsCPUMessage := ""
		if float64(requestsCPU)/float64(allocatableCPU) > 0.8 {
			highRequestsCPU = false
			highRequestsCPUMessage = fmt.Sprintf("节点 %s requests CPU 超过百分之 80", pod.Spec.NodeName)
			nodeInspections = append(nodeInspections, apis.NewInspection(fmt.Sprintf("Node %s High Requests CPU", pod.Spec.NodeName), fmt.Sprintf("节点 %s requests CPU 超过百分之 80", pod.Spec.NodeName), 2))
			logrus.Infof("[%s] Node %s High Requests CPU: requests CPU %d, allocatable CPU %d", taskName, pod.Spec.NodeName, requestsCPU, allocatableCPU)
		}

		highRequestsMemory := true
		highRequestsMemoryMessage := ""
		if float64(requestsMemory)/float64(allocatableMemory) > 0.8 {
			highRequestsMemory = false
			highRequestsMemoryMessage = fmt.Sprintf("节点 %s requests Memory 超过百分之 80", pod.Spec.NodeName)
			nodeInspections = append(nodeInspections, apis.NewInspection(fmt.Sprintf("Node %s High Requests Memory", pod.Spec.NodeName), fmt.Sprintf("节点 %s requests Memory 超过百分之 80", pod.Spec.NodeName), 2))
			logrus.Infof("[%s] Node %s High Requests Memory: requests Memory %d, allocatable Memory %d", taskName, pod.Spec.NodeName, requestsMemory, allocatableMemory)
		}

		highRequestsPods := true
		highRequestsPodsMessage := ""
		if float64(requestsPods)/float64(allocatablePods) > 0.8 {
			highRequestsPods = false
			highRequestsPodsMessage = fmt.Sprintf("节点 %s requests Pods 超过百分之 80", pod.Spec.NodeName)
			nodeInspections = append(nodeInspections, apis.NewInspection(fmt.Sprintf("Node %s High Requests Pods", pod.Spec.NodeName), fmt.Sprintf("节点 %s requests Pods 超过百分之 80", pod.Spec.NodeName), 2))
			logrus.Infof("[%s] Node %s High Requests Pods: requests Pods %d, allocatable Pods %d", taskName, pod.Spec.NodeName, requestsPods, allocatablePods)
		}

		var commands []string
		for _, n := range nodeTemplate.NodeTemplateConfig {
			cs, ok := n.Nodes[pod.Spec.NodeName]
			if ok {
				for _, c := range cs {
					commands = append(commands, c.Description+": "+c.Command)
				}
			}
		}

		logrus.Debugf("Commands to execute on node %s: %v", pod.Spec.NodeName, commands)
		command := "/opt/inspection/inspection.sh"
		stdout, stderr, err := ExecToPodThroughAPI(client.Clientset, client.Config, command, commands, pod.Namespace, pod.Name, "inspection-agent-container", taskName)
		if err != nil {
			return nil, nil, fmt.Errorf("Error executing command in pod %s: %v\n", pod.Name, err)
		}

		if stderr != "" {
			logrus.Errorf("Stderr from pod %s: %s", pod.Name, stderr)
		}

		var results []apis.CommandCheckResult
		err = json.Unmarshal([]byte(stdout), &results)
		if err != nil {
			return nil, nil, fmt.Errorf("Error unmarshalling stdout for pod %s: %v\n", pod.Name, err)
		}

		var commandItems []*apis.Item
		for _, r := range results {
			commandCheck := true
			commandCheckMessage := ""
			if r.Error != "" {
				commandCheck = false
				commandCheckMessage = fmt.Sprintf("%s", r.Error)
				nodeInspections = append(nodeInspections, apis.NewInspection(fmt.Sprintf("Node %s (%s)", pod.Spec.NodeName, r.Description), fmt.Sprintf("%s", r.Error), 2))
				logrus.Errorf("Node %s inspection failed (%s): %s", pod.Spec.NodeName, r.Description, r.Error)
			}

			commandItems = append(commandItems, &apis.Item{
				Name:    r.Description,
				Message: commandCheckMessage,
				Pass:    commandCheck,
			})
		}

		nodeData := &apis.Node{
			Name:   pod.Spec.NodeName,
			HostIP: pod.Status.HostIP,
			Resource: &apis.Resource{
				LimitsCPU:         limitsCPU,
				LimitsMemory:      limitsMemory,
				RequestsCPU:       requestsCPU,
				RequestsMemory:    requestsMemory,
				RequestsPods:      requestsPods,
				AllocatableCPU:    allocatableCPU,
				AllocatableMemory: allocatableMemory,
				AllocatablePods:   allocatablePods,
			},
			Commands: &apis.Command{
				Stdout: results,
				Stderr: stderr,
			},
			Items: []*apis.Item{
				{
					Name:    "Limits CPU 超过 80 %",
					Message: highLimitsCPUMessage,
					Pass:    highLimitsCPU,
				},
				{
					Name:    "Limits Memory 超过 80 %",
					Message: highLimitsMemoryMessage,
					Pass:    highLimitsMemory,
				},
				{
					Name:    "Requests CPU 超过 80 %",
					Message: highRequestsCPUMessage,
					Pass:    highRequestsCPU,
				},
				{
					Name:    "Requests Memory 超过 80 %",
					Message: highRequestsMemoryMessage,
					Pass:    highRequestsMemory,
				},
				{
					Name:    "Requests Pods 超过 80 %",
					Message: highRequestsPodsMessage,
					Pass:    highRequestsPods,
				},
			},
		}

		nodeData.Items = append(nodeData.Items, commandItems...)
		nodeNodeArray = append(nodeNodeArray, nodeData)
	}

	return nodeNodeArray, nodeInspections, nil
}

func ExecToPodThroughAPI(clientset *kubernetes.Clientset, config *rest.Config, command string, commands []string, namespace, podName, containerName, taskName string) (string, string, error) {
	logrus.Infof("[%s] Starting exec to pod: %s, namespace: %s, container: %s", taskName, podName, namespace, containerName)
	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		Param("container", containerName).
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "false").
		Param("command", command)

	for _, c := range commands {
		req.Param("command", c)
	}
	logrus.Debugf("Executing command: %s with additional commands: %v", command, commands)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("Error creating SPDY executor: %v\n", err)
	}

	var stdout, stderr string
	stdoutWriter := &outputWriter{output: &stdout}
	stderrWriter := &outputWriter{output: &stderr}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: stdoutWriter,
		Stderr: stderrWriter,
		Tty:    false,
	})
	if err != nil {
		return stdout, stderr, fmt.Errorf("Error executing command: %v\n", err)
	}

	logrus.Debugf("Command execution completed. Stdout: %s, Stderr: %s", stdout, stderr)
	return stdout, stderr, nil
}

type outputWriter struct {
	output *string
}

func (w *outputWriter) Write(p []byte) (n int, err error) {
	*w.output += string(p)
	return len(p), nil
}

func GetWorkloads(client *apis.Client, workloadConfig *apis.WorkloadConfig, taskName string, clusterResourceItem *ClusterResourceItem) (*apis.Workload, []*apis.Inspection, error) {
	logrus.Infof("[%s] Starting workload inspection", taskName)

	ResourceWorkloadArray := apis.NewWorkload()
	resourceInspections := apis.NewInspections()

	if workloadConfig.Deployment.Enable {
		var set labels.Set
		if workloadConfig.Deployment.SelectorLabels != nil {
			set = workloadConfig.Deployment.SelectorLabels
		}

		deployments, err := client.Clientset.AppsV1().Deployments(workloadConfig.Deployment.SelectorNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: set.AsSelector().String()})
		if err != nil {
			return nil, nil, fmt.Errorf("Error list Deployment: %v\n", err)
		}

		for _, deploy := range deployments.Items {
			logrus.Debugf("[%s] Inspecting Deployment: %s in namespace %s", taskName, deploy.Name, deploy.Namespace)

			deployHealth := true
			deployHealthMessage := ""
			if !isDeploymentAvailable(&deploy) {
				deployHealth = false
				deployHealthMessage = fmt.Sprintf("命名空间 %s 下的 Deployment %s 处于非健康状态", deploy.Namespace, deploy.Name)
				resourceInspections = append(resourceInspections, apis.NewInspection(fmt.Sprintf("Deployment %s 警告", deploy.Name), fmt.Sprintf("命名空间 %s 下的 Deployment %s 处于非健康状态", deploy.Namespace, deploy.Name), defaultLevel(1)))
			}

			var condition []apis.Condition
			for _, c := range deploy.Status.Conditions {
				condition = append(condition, apis.Condition{
					Type:   string(c.Type),
					Status: string(c.Status),
					Reason: c.Reason,
				})
			}

			podset := labels.Set(deploy.Spec.Selector.MatchLabels)
			pods, err := GetPod("", deploy.Namespace, podset, client.Clientset, taskName)
			if err != nil {
				return nil, nil, fmt.Errorf("Error getting pods for Deployment %s in namespace %s: %v\n", deploy.Name, deploy.Namespace, err)
			}

			var items []*apis.Item
			items = append(items, &apis.Item{
				Name:    "健康状态",
				Message: deployHealthMessage,
				Pass:    deployHealth,
			})

			grafanaItem, ok := clusterResourceItem.DeploymentItem[deploy.Namespace+"/"+deploy.Name]
			if ok {
				items = append(items, grafanaItem...)
			}

			deploymentData := &apis.WorkloadData{
				Name:      deploy.Name,
				Namespace: deploy.Namespace,
				Pods:      pods,
				Status: &apis.Status{
					Condition: condition,
				},
				Items: items,
			}

			ResourceWorkloadArray.Deployment = append(ResourceWorkloadArray.Deployment, deploymentData)
		}
	}

	if workloadConfig.Daemonset.Enable {
		var set labels.Set
		if workloadConfig.Daemonset.SelectorLabels != nil {
			set = workloadConfig.Daemonset.SelectorLabels
		}

		daemonSets, err := client.Clientset.AppsV1().DaemonSets(workloadConfig.Daemonset.SelectorNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: set.AsSelector().String()})
		if err != nil {
			return nil, nil, fmt.Errorf("Error list Deployment: %v\n", err)
		}

		for _, ds := range daemonSets.Items {
			logrus.Debugf("[%s] Inspecting DaemonSet: %s in namespace %s", taskName, ds.Name, ds.Namespace)

			dsHealth := true
			dsHealthMessage := ""
			if !isDaemonSetAvailable(&ds) {
				dsHealth = false
				dsHealthMessage = fmt.Sprintf("命名空间 %s 下的 DaemonSet %s 处于非健康状态", ds.Namespace, ds.Name)
				resourceInspections = append(resourceInspections, apis.NewInspection(fmt.Sprintf("DaemonSet %s 警告", ds.Name), fmt.Sprintf("命名空间 %s 下的 DaemonSet %s 处于非健康状态", ds.Namespace, ds.Name), defaultLevel(1)))
			}

			var condition []apis.Condition
			for _, c := range ds.Status.Conditions {
				condition = append(condition, apis.Condition{
					Type:   string(c.Type),
					Status: string(c.Status),
					Reason: c.Reason,
				})
			}

			podset := labels.Set(ds.Spec.Selector.MatchLabels)
			pods, err := GetPod("", ds.Namespace, podset, client.Clientset, taskName)
			if err != nil {
				return nil, nil, fmt.Errorf("Error getting pods for DaemonSet %s in namespace %s: %v\n", ds.Name, ds.Namespace, err)
			}

			var items []*apis.Item
			items = append(items, &apis.Item{
				Name:    "健康状态",
				Message: dsHealthMessage,
				Pass:    dsHealth,
			})

			grafanaItem, ok := clusterResourceItem.DaemonsetItem[ds.Namespace+"/"+ds.Name]
			if ok {
				items = append(items, grafanaItem...)
			}

			daemonsetData := &apis.WorkloadData{
				Name:      ds.Name,
				Namespace: ds.Namespace,
				Pods:      pods,
				Status: &apis.Status{
					Condition: condition,
				},
				Items: items,
			}

			ResourceWorkloadArray.Daemonset = append(ResourceWorkloadArray.Daemonset, daemonsetData)
		}
	}

	if workloadConfig.Statefulset.Enable {
		var set labels.Set
		if workloadConfig.Statefulset.SelectorLabels != nil {
			set = workloadConfig.Statefulset.SelectorLabels
		}

		statefulsets, err := client.Clientset.AppsV1().StatefulSets(workloadConfig.Statefulset.SelectorNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: set.AsSelector().String()})
		if err != nil {
			return nil, nil, fmt.Errorf("Error list Deployment: %v\n", err)
		}

		for _, sts := range statefulsets.Items {
			logrus.Debugf("[%s] Inspecting Statefulset: %s in namespace %s", taskName, sts.Name, sts.Namespace)

			stsHealth := true
			stsHealthMessage := ""
			if !isStatefulSetAvailable(&sts) {
				stsHealth = false
				stsHealthMessage = fmt.Sprintf("命名空间 %s 下的 Statefulset %s 处于非健康状态", sts.Namespace, sts.Name)
				resourceInspections = append(resourceInspections, apis.NewInspection(fmt.Sprintf("Statefulset %s 警告", sts.Name), fmt.Sprintf("命名空间 %s 下的 Statefulset %s 处于非健康状态", sts.Namespace, sts.Name), defaultLevel(1)))
			}

			var condition []apis.Condition
			for _, c := range sts.Status.Conditions {
				condition = append(condition, apis.Condition{
					Type:   string(c.Type),
					Status: string(c.Status),
					Reason: c.Reason,
				})
			}

			podset := labels.Set(sts.Spec.Selector.MatchLabels)
			pods, err := GetPod("", sts.Namespace, podset, client.Clientset, taskName)
			if err != nil {
				return nil, nil, fmt.Errorf("Error getting pods for Statefulset %s in namespace %s: %v\n", sts.Name, sts.Namespace, err)
			}

			var items []*apis.Item
			items = append(items, &apis.Item{
				Name:    "健康状态",
				Message: stsHealthMessage,
				Pass:    stsHealth,
			})

			grafanaItem, ok := clusterResourceItem.StatefulsetItem[sts.Namespace+"/"+sts.Name]
			if ok {
				items = append(items, grafanaItem...)
			}

			statefulsetData := &apis.WorkloadData{
				Name:      sts.Name,
				Namespace: sts.Namespace,
				Pods:      pods,
				Status: &apis.Status{
					Condition: condition,
				},
				Items: items,
			}

			ResourceWorkloadArray.Statefulset = append(ResourceWorkloadArray.Statefulset, statefulsetData)
		}
	}

	if workloadConfig.Job.Enable {
		var set labels.Set
		if workloadConfig.Statefulset.SelectorLabels != nil {
			set = workloadConfig.Statefulset.SelectorLabels
		}

		jobs, err := client.Clientset.BatchV1().Jobs(workloadConfig.Job.SelectorNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: set.AsSelector().String()})
		if err != nil {
			return nil, nil, fmt.Errorf("Error list Job: %v\n", err)
		}

		for _, j := range jobs.Items {
			logrus.Debugf("[%s] Inspecting Job: %s in namespace %s", taskName, j.Name, j.Namespace)

			jHealth := true
			jHealthMessage := ""
			if !isJobCompleted(&j) {
				jHealth = false
				jHealthMessage = fmt.Sprintf("命名空间 %s 下的 Job %s 处于非健康状态", j.Namespace, j.Name)
				resourceInspections = append(resourceInspections, apis.NewInspection(fmt.Sprintf("Job %s 警告", j.Name), fmt.Sprintf("命名空间 %s 下的 Job %s 处于非健康状态", j.Namespace, j.Name), defaultLevel(1)))
			}

			var condition []apis.Condition
			for _, c := range j.Status.Conditions {
				condition = append(condition, apis.Condition{
					Type:   string(c.Type),
					Status: string(c.Status),
					Reason: c.Reason,
				})
			}

			podset := labels.Set(j.Spec.Selector.MatchLabels)
			pods, err := GetPod("", j.Namespace, podset, client.Clientset, taskName)
			if err != nil {
				return nil, nil, fmt.Errorf("Error getting pods for Job %s in namespace %s: %v\n", j.Name, j.Namespace, err)
			}

			var items []*apis.Item
			items = append(items, &apis.Item{
				Name:    "健康状态",
				Message: jHealthMessage,
				Pass:    jHealth,
			})

			grafanaItem, ok := clusterResourceItem.JobItem[j.Namespace+"/"+j.Name]
			if ok {
				items = append(items, grafanaItem...)
			}

			jobData := &apis.WorkloadData{
				Name:      j.Name,
				Namespace: j.Namespace,
				Pods:      pods,
				Status: &apis.Status{
					Condition: condition,
				},
				Items: items,
			}

			ResourceWorkloadArray.Job = append(ResourceWorkloadArray.Job, jobData)
		}
	}

	logrus.Infof("[%s] Workload inspection completed", taskName)
	return ResourceWorkloadArray, resourceInspections, nil
}

func GetPod(regexpString, namespace string, set labels.Set, clientset *kubernetes.Clientset, taskName string) ([]*apis.Pod, error) {
	logrus.Infof("[%s] Starting to get pods in namespace %s with labels %s", taskName, namespace, set.String())

	pods := apis.NewPods()

	podList, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		return nil, fmt.Errorf("Error listing pods in namespace %s: %v\n", namespace, err)
	}

	line := int64(50)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, pod := range podList.Items {
		wg.Add(1)
		go func(pod corev1.Pod) {
			defer wg.Done()
			logrus.Infof("[%s] Processing pod: %s", taskName, pod.Name)

			if len(pod.Spec.Containers) == 0 {
				logrus.Errorf("Error getting logs for pod %s: container is zero", pod.Name)
				return
			}

			getLog := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: pod.Spec.Containers[0].Name, TailLines: &line})
			podLogs, err := getLog.Stream(context.TODO())
			if err != nil {
				logrus.Errorf("Error getting logs for pod %s: %v", pod.Name, err)
				return
			}
			defer podLogs.Close()

			logs, err := io.ReadAll(podLogs)
			if err != nil {
				logrus.Errorf("Error reading logs for pod %s: %v", pod.Name, err)
				return
			}

			var str []string
			if regexpString == "" {
				regexpString = ".*"
			}

			re, err := regexp.Compile(regexpString)
			if err != nil {
				logrus.Errorf("Error compiling regex for pod %s: %v", pod.Name, err)
				return
			}

			str = re.FindAllString(string(logs), -1)
			if str == nil {
				str = []string{}
			}

			mu.Lock()
			pods = append(pods, &apis.Pod{
				Name: pod.Name,
				Log:  str,
			})
			mu.Unlock()
			logrus.Debugf("Processed pod: %s", pod.Name)
		}(pod)
	}
	wg.Wait()

	logrus.Infof("[%s] Completed pod retrieval in namespace %s", taskName, namespace)
	return pods, nil
}

func GetNamespaces(client *apis.Client, taskName string) ([]*apis.Namespace, []*apis.Inspection, error) {
	logrus.Infof("[%s] Starting namespaces inspection", taskName)

	resourceInspections := apis.NewInspections()
	namespaces := apis.NewNamespaces()

	namespaceList, err := client.Clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("Error listing namespaces: %v\n", err)
	}

	for _, n := range namespaceList.Items {
		logrus.Debugf("Processing namespace: %s", n.Name)

		emptyResourceQuota := true
		emptyResource := true
		emptyResourceQuotaMessage := ""
		emptyResourceMessage := ""

		podList, err := client.Clientset.CoreV1().Pods(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("Error listing pods in namespace %s: %v\n", n.Name, err)
		}

		serviceList, err := client.Clientset.CoreV1().Services(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("Error listing services in namespace %s: %v\n", n.Name, err)
		}

		deploymentList, err := client.Clientset.AppsV1().Deployments(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("Error listing deployments in namespace %s: %v\n", n.Name, err)
		}

		replicaSetList, err := client.Clientset.AppsV1().ReplicaSets(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("Error listing replica sets in namespace %s: %v\n", n.Name, err)
		}

		statefulSetList, err := client.Clientset.AppsV1().StatefulSets(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("Error listing stateful sets in namespace %s: %v\n", n.Name, err)
		}

		daemonSetList, err := client.Clientset.AppsV1().DaemonSets(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("Error listing daemon sets in namespace %s: %v\n", n.Name, err)
		}

		jobList, err := client.Clientset.BatchV1().Jobs(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("Error listing jobs in namespace %s: %v\n", n.Name, err)
		}

		secretList, err := client.Clientset.CoreV1().Secrets(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("Error listing secrets in namespace %s: %v\n", n.Name, err)
		}

		configMapList, err := client.Clientset.CoreV1().ConfigMaps(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("Error listing config maps in namespace %s: %v\n", n.Name, err)
		}

		resourceQuotaList, err := client.Clientset.CoreV1().ResourceQuotas(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("Error listing resource quotas in namespace %s: %v\n", n.Name, err)
		}

		if len(resourceQuotaList.Items) == 0 {
			emptyResourceQuota = false
			emptyResourceQuotaMessage = fmt.Sprintf("命名空间 %s 没有设置配额", n.Name)
			resourceInspections = append(resourceInspections, apis.NewInspection(
				fmt.Sprintf("命名空间 %s 没有设置配额", n.Name),
				"未设置资源配额",
				1,
			))
		}

		totalResources := len(podList.Items) + len(serviceList.Items) + len(deploymentList.Items) +
			len(replicaSetList.Items) + len(statefulSetList.Items) + len(daemonSetList.Items) +
			len(jobList.Items) + len(secretList.Items) + (len(configMapList.Items) - 1)

		if totalResources == 0 {
			emptyResource = false
			emptyResourceMessage = fmt.Sprintf("命名空间 %s 下资源为空", n.Name)
			resourceInspections = append(resourceInspections, apis.NewInspection(
				fmt.Sprintf("命名空间 %s 下资源为空", n.Name),
				"检查对象为 Pod、Service、Deployment、Replicaset、Statefulset、Daemonset、Job、Secret、ConfigMap",
				1,
			))
		}

		namespaces = append(namespaces, &apis.Namespace{
			Name:             n.Name,
			PodCount:         len(podList.Items),
			ServiceCount:     len(serviceList.Items),
			DeploymentCount:  len(deploymentList.Items),
			ReplicasetCount:  len(replicaSetList.Items),
			StatefulsetCount: len(statefulSetList.Items),
			DaemonsetCount:   len(daemonSetList.Items),
			JobCount:         len(jobList.Items),
			SecretCount:      len(secretList.Items),
			ConfigMapCount:   len(configMapList.Items) - 1,
			Items: []*apis.Item{
				{
					Name:    "有资源配置设置",
					Message: emptyResourceQuotaMessage,
					Pass:    emptyResourceQuota,
				},
				{
					Name:    "命名空间下资源非空",
					Message: emptyResourceMessage,
					Pass:    emptyResource,
				},
			},
		})

		logrus.Debugf("Processed namespace: %s", n.Name)
	}

	logrus.Infof("[%s] Completed namespace retrieval", taskName)
	return namespaces, resourceInspections, nil
}

func GetServices(client *apis.Client, taskName string, serviceItem map[string][]*apis.Item) ([]*apis.Service, []*apis.Inspection, error) {
	logrus.Infof("[%s] Starting services inspection", taskName)

	resourceInspections := apis.NewInspections()
	services := apis.NewServices()

	selectorNamespace := ""
	var set labels.Set
	set = map[string]string{}

	serviceList, err := client.Clientset.CoreV1().Services(selectorNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: set.AsSelector().String()})
	if err != nil {
		return nil, nil, fmt.Errorf("Error listing services: %v\n", err)
	}

	for _, s := range serviceList.Items {
		logrus.Debugf("Processing service: %s/%s", s.Namespace, s.Name)
		emptyEndpoints := true
		emptyEndpointsMessage := ""
		endpoints, err := client.Clientset.CoreV1().Endpoints(s.Namespace).Get(context.TODO(), s.Name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				emptyEndpoints = false
				emptyEndpointsMessage = fmt.Sprintf("命名空间 %s 下 Service %s 找不到对应 endpoint", s.Namespace, s.Name)
				logrus.Warnf("Service %s/%s does not have corresponding endpoints", s.Namespace, s.Name)
				resourceInspections = append(resourceInspections, apis.NewInspection(
					fmt.Sprintf("命名空间 %s 下 Service %s 找不到对应 endpoint", s.Namespace, s.Name),
					"对应的 Endpoints 未找到",
					1,
				))

				services = append(services, &apis.Service{
					Name:      s.Name,
					Namespace: s.Namespace,
					Items: []*apis.Item{
						{
							Name:    "存在对应 Endpoints 且 Subsets 非空",
							Message: emptyEndpointsMessage,
							Pass:    emptyEndpoints,
						},
					},
				})

				continue
			}
		}

		if len(endpoints.Subsets) == 0 {
			emptyEndpoints = false
			emptyEndpointsMessage = fmt.Sprintf("命名空间 %s 下 Service %s 对应 Endpoints 没有 Subsets", s.Namespace, s.Name)
			resourceInspections = append(resourceInspections, apis.NewInspection(
				fmt.Sprintf("命名空间 %s 下 Service %s 对应 Endpoints 没有 Subsets", s.Namespace, s.Name),
				"对应的 Endpoints 没有 Subsets",
				1,
			))
		}

		var items []*apis.Item
		items = append(items, &apis.Item{
			Name:    "存在对应 Endpoints 且 Subsets 非空",
			Message: emptyEndpointsMessage,
			Pass:    emptyEndpoints,
		})

		grafanaItem, ok := serviceItem[s.Namespace+"/"+s.Name]
		if ok {
			items = append(items, grafanaItem...)
		}

		services = append(services, &apis.Service{
			Name:      s.Name,
			Namespace: s.Namespace,
			Items:     items,
		})
	}

	logrus.Infof("[%s] Completed getting services", taskName)
	return services, resourceInspections, nil
}
func GetIngress(client *apis.Client, taskName string) ([]*apis.Ingress, []*apis.Inspection, error) {
	logrus.Infof("[%s] Starting ingresses inspection", taskName)

	resourceInspections := apis.NewInspections()
	ingress := apis.NewIngress()

	ingressList, err := client.Clientset.NetworkingV1().Ingresses("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("Error listing ingresses: %v\n", err)
	}

	ingressMap := make(map[string][]string)
	for _, i := range ingressList.Items {
		for _, rule := range i.Spec.Rules {
			host := rule.Host
			if rule.HTTP != nil {
				for _, path := range rule.HTTP.Paths {
					key := host + path.Path
					ingressMap[key] = append(ingressMap[key], fmt.Sprintf("%s/%s", i.Namespace, i.Name))
				}
			}
		}

		ingress = append(ingress, &apis.Ingress{
			Name:      i.Name,
			Namespace: i.Namespace,
			Items: []*apis.Item{
				{
					Name:    "不存在重复的 Path 路径",
					Message: "",
					Pass:    true,
				},
			},
		})
	}

	for _, ingressNames := range ingressMap {
		if len(ingressNames) > 1 {
			result := strings.Join(ingressNames, ",")
			for _, ingressName := range ingressNames {
				parts := strings.Split(ingressName, "/")
				for index, i := range ingress {
					if parts[0] == i.Namespace && parts[1] == i.Name {
						ingress[index] = &apis.Ingress{
							Name:      i.Name,
							Namespace: i.Namespace,
							Items: []*apis.Item{
								{
									Name:    "不存在重复的 Path 路径",
									Message: fmt.Sprintf("Ingress %s 存在重复的 Path 路径", result),
									Pass:    false,
								},
							},
						}
					}
				}
			}
		}
	}

	logrus.Infof("[%s] Completed getting ingresses", taskName)
	return ingress, resourceInspections, nil
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

func isDeploymentAvailable(deployment *appsv1.Deployment) bool {
	for _, condition := range deployment.Status.Conditions {
		if (condition.Type == "Failed" && condition.Status == "False") || condition.Reason == "Error" {
			return false
		}
	}
	return deployment.Status.AvailableReplicas >= *deployment.Spec.Replicas
}

func isDaemonSetAvailable(daemonset *appsv1.DaemonSet) bool {
	return daemonset.Status.NumberAvailable >= daemonset.Status.DesiredNumberScheduled
}

func isStatefulSetAvailable(statefulset *appsv1.StatefulSet) bool {
	return statefulset.Status.ReadyReplicas >= *statefulset.Spec.Replicas
}

func isJobCompleted(job *batchv1.Job) bool {
	return job.Status.Succeeded >= *job.Spec.Completions
}

func defaultLevel(level int) int {
	if level != 0 {
		return level
	}

	return 2
}

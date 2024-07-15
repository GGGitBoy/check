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
	"k8s.io/utils/strings/slices"
	"os"
	"regexp"
	"strings"
	"sync"
)

var (
	warning = "warning"
	success = "success"

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

func GetNodes(client *apis.Client, nodesConfig []*apis.NodeConfig, taskName string) ([]*apis.Node, []*apis.Inspection, error) {
	logrus.Infof("[%s] Starting node inspection", taskName)
	nodeNodeArray := apis.NewNodes()
	nodeInspections := apis.NewInspections()

	set := labels.Set(map[string]string{"name": "inspection-agent"})
	podList, err := client.Clientset.CoreV1().Pods(common.InspectionNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		return nil, nil, fmt.Errorf("Error listing pods in namespace %s: %v\n", common.InspectionNamespace, err)
	}

	for _, pod := range podList.Items {
		for _, n := range nodesConfig {
			if slices.Contains(n.Names, pod.Spec.NodeName) {
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

				if float64(limitsCPU)/float64(allocatableCPU) > 0.8 {
					nodeInspections = append(nodeInspections, apis.NewInspection(fmt.Sprintf("Node %s High Limits CPU", pod.Spec.NodeName), fmt.Sprintf("节点 %s limits CPU 超过百分之 80", pod.Spec.NodeName), 2))
					logrus.Infof("[%s] Node %s High Limits CPU: limits CPU %d, allocatable CPU %d", taskName, pod.Spec.NodeName, limitsCPU, allocatableCPU)
				}

				if float64(limitsMemory)/float64(allocatableMemory) > 0.8 {
					nodeInspections = append(nodeInspections, apis.NewInspection(fmt.Sprintf("Node %s High Limits Memory", pod.Spec.NodeName), fmt.Sprintf("节点 %s limits Memory 超过百分之 80", pod.Spec.NodeName), 2))
					logrus.Infof("[%s] Node %s High Limits Memory: limits Memory %d, allocatable Memory %d", taskName, pod.Spec.NodeName, limitsMemory, allocatableMemory)
				}

				if float64(requestsCPU)/float64(allocatableCPU) > 0.8 {
					nodeInspections = append(nodeInspections, apis.NewInspection(fmt.Sprintf("Node %s High Requests CPU", pod.Spec.NodeName), fmt.Sprintf("节点 %s requests CPU 超过百分之 80", pod.Spec.NodeName), 2))
					logrus.Infof("[%s] Node %s High Requests CPU: requests CPU %d, allocatable CPU %d", taskName, pod.Spec.NodeName, requestsCPU, allocatableCPU)
				}

				if float64(requestsMemory)/float64(allocatableMemory) > 0.8 {
					nodeInspections = append(nodeInspections, apis.NewInspection(fmt.Sprintf("Node %s High Requests Memory", pod.Spec.NodeName), fmt.Sprintf("节点 %s requests Memory 超过百分之 80", pod.Spec.NodeName), 2))
					logrus.Infof("[%s] Node %s High Requests Memory: requests Memory %d, allocatable Memory %d", taskName, pod.Spec.NodeName, requestsMemory, allocatableMemory)
				}

				if float64(requestsPods)/float64(allocatablePods) > 0.8 {
					nodeInspections = append(nodeInspections, apis.NewInspection(fmt.Sprintf("Node %s High Requests Pods", pod.Spec.NodeName), fmt.Sprintf("节点 %s requests Pods 超过百分之 80", pod.Spec.NodeName), 2))
					logrus.Infof("[%s] Node %s High Requests Pods: requests Pods %d, allocatable Pods %d", taskName, pod.Spec.NodeName, requestsPods, allocatablePods)
				}

				var commands []string
				for _, c := range n.Commands {
					commands = append(commands, c.Description+": "+c.Command)
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

				for _, r := range results {
					if r.Error != "" {
						nodeInspections = append(nodeInspections, apis.NewInspection(fmt.Sprintf("Node %s (%s)", pod.Spec.NodeName, r.Description), fmt.Sprintf("%s", r.Error), 2))
						logrus.Errorf("Node %s inspection failed (%s): %s", pod.Spec.NodeName, r.Description, r.Error)
					}
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
				}

				nodeNodeArray = append(nodeNodeArray, nodeData)
			}
		}
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

func GetWorkloads(client *apis.Client, workloadConfig *apis.WorkloadConfig, taskName string) (*apis.Workload, []*apis.Inspection, error) {
	logrus.Infof("[%s] Starting workload inspection", taskName)

	ResourceWorkloadArray := apis.NewWorkload()
	resourceInspections := apis.NewInspections()

	for _, deploy := range workloadConfig.Deployment {
		logrus.Debugf("[%s] Inspecting Deployment: %s in namespace %s", taskName, deploy.Name, deploy.Namespace)
		deployment, err := client.Clientset.AppsV1().Deployments(deploy.Namespace).Get(context.TODO(), deploy.Name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				logrus.Warnf("Deployment %s not found in namespace %s", deploy.Name, deploy.Namespace)
				continue
			}
			return nil, nil, fmt.Errorf("Error getting Deployment %s in namespace %s: %v\n", deploy.Name, deploy.Namespace, err)
		}

		deployState := warning
		if isDeploymentAvailable(deployment) {
			deployState = success
		}

		var condition []apis.Condition
		for _, c := range deployment.Status.Conditions {
			condition = append(condition, apis.Condition{
				Type:   string(c.Type),
				Status: string(c.Status),
				Reason: c.Reason,
			})
		}

		set := labels.Set(deployment.Spec.Selector.MatchLabels)
		pods, err := GetPod(deploy.Regexp, deployment.Namespace, set, client.Clientset, taskName)
		if err != nil {
			return nil, nil, fmt.Errorf("Error getting pods for Deployment %s in namespace %s: %v\n", deploy.Name, deploy.Namespace, err)
		}

		deploymentData := &apis.WorkloadData{
			Name:      deployment.Name,
			Namespace: deployment.Namespace,
			Pods:      pods,
			Status: &apis.Status{
				State:     deployState,
				Condition: condition,
			},
		}

		ResourceWorkloadArray.Deployment = append(ResourceWorkloadArray.Deployment, deploymentData)
		if deployState == warning {
			resourceInspections = append(resourceInspections, apis.NewInspection(fmt.Sprintf("Deployment %s 警告", deploymentData.Name), fmt.Sprintf("命名空间 %s 下的 Deployment %s 处于非健康状态", deploymentData.Namespace, deploymentData.Name), defaultLevel(deploy.Level)))
		}
	}

	for _, ds := range workloadConfig.Daemonset {
		logrus.Debugf("[%s] Inspecting DaemonSet: %s in namespace %s", taskName, ds.Name, ds.Namespace)
		daemonSet, err := client.Clientset.AppsV1().DaemonSets(ds.Namespace).Get(context.TODO(), ds.Name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				logrus.Warnf("DaemonSet %s not found in namespace %s", ds.Name, ds.Namespace)
				continue
			}
			return nil, nil, fmt.Errorf("Error getting DaemonSet %s in namespace %s: %v\n", ds.Name, ds.Namespace, err)
		}

		dsState := warning
		if isDaemonSetAvailable(daemonSet) {
			dsState = success
		}

		var condition []apis.Condition
		for _, c := range daemonSet.Status.Conditions {
			condition = append(condition, apis.Condition{
				Type:   string(c.Type),
				Status: string(c.Status),
				Reason: c.Reason,
			})
		}

		set := labels.Set(daemonSet.Spec.Selector.MatchLabels)
		pods, err := GetPod(ds.Regexp, daemonSet.Namespace, set, client.Clientset, taskName)
		if err != nil {
			return nil, nil, fmt.Errorf("Error getting pods for DaemonSet %s in namespace %s: %v\n", ds.Name, ds.Namespace, err)
		}

		daemonSetData := &apis.WorkloadData{
			Name:      daemonSet.Name,
			Namespace: daemonSet.Namespace,
			Pods:      pods,
			Status: &apis.Status{
				State:     dsState,
				Condition: condition,
			},
		}

		ResourceWorkloadArray.Daemonset = append(ResourceWorkloadArray.Daemonset, daemonSetData)
		if dsState == warning {
			resourceInspections = append(resourceInspections, apis.NewInspection(fmt.Sprintf("Daemonset %s 警告", daemonSetData.Name), fmt.Sprintf("命名空间 %s 下的 Daemonset %s 处于非健康状态", daemonSetData.Namespace, daemonSetData.Name), defaultLevel(ds.Level)))
		}
	}

	for _, sts := range workloadConfig.Statefulset {
		logrus.Debugf("[%s] Inspecting StatefulSet: %s in namespace %s", taskName, sts.Name, sts.Namespace)
		statefulset, err := client.Clientset.AppsV1().StatefulSets(sts.Namespace).Get(context.TODO(), sts.Name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				logrus.Warnf("StatefulSet %s not found in namespace %s", sts.Name, sts.Namespace)
				continue
			}
			return nil, nil, fmt.Errorf("Error getting StatefulSet %s in namespace %s: %v\n", sts.Name, sts.Namespace, err)
		}

		stsState := warning
		if isStatefulSetAvailable(statefulset) {
			stsState = success
		}

		var condition []apis.Condition
		for _, c := range statefulset.Status.Conditions {
			condition = append(condition, apis.Condition{
				Type:   string(c.Type),
				Status: string(c.Status),
				Reason: c.Reason,
			})
		}

		set := labels.Set(statefulset.Spec.Selector.MatchLabels)
		pods, err := GetPod(sts.Regexp, statefulset.Namespace, set, client.Clientset, taskName)
		if err != nil {
			return nil, nil, fmt.Errorf("Error getting pods for StatefulSet %s in namespace %s: %v\n", sts.Name, sts.Namespace, err)
		}

		statefulSetData := &apis.WorkloadData{
			Name:      statefulset.Name,
			Namespace: statefulset.Namespace,
			Pods:      pods,
			Status: &apis.Status{
				State:     stsState,
				Condition: condition,
			},
		}

		ResourceWorkloadArray.Statefulset = append(ResourceWorkloadArray.Statefulset, statefulSetData)
		if stsState == warning {
			resourceInspections = append(resourceInspections, apis.NewInspection(fmt.Sprintf("Statefulset %s 警告", statefulSetData.Name), fmt.Sprintf("命名空间 %s 下的 Statefulset %s 处于非健康状态", statefulSetData.Namespace, statefulSetData.Name), defaultLevel(sts.Level)))
		}
	}

	for _, j := range workloadConfig.Job {
		logrus.Debugf("[%s] Inspecting Job: %s in namespace %s", taskName, j.Name, j.Namespace)
		job, err := client.Clientset.BatchV1().Jobs(j.Namespace).Get(context.TODO(), j.Name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				logrus.Warnf("Job %s not found in namespace %s", j.Name, j.Namespace)
				continue
			}
			return nil, nil, fmt.Errorf("Error getting Job %s in namespace %s: %v\n", j.Name, j.Namespace, err)
		}

		jState := warning
		if isJobCompleted(job) {
			jState = success
		}

		var condition []apis.Condition
		for _, c := range job.Status.Conditions {
			condition = append(condition, apis.Condition{
				Type:   string(c.Type),
				Status: string(c.Status),
				Reason: c.Reason,
			})
		}

		set := labels.Set(job.Spec.Selector.MatchLabels)
		pods, err := GetPod(j.Regexp, j.Namespace, set, client.Clientset, taskName)
		if err != nil {
			return nil, nil, fmt.Errorf("Error getting pods for Job %s in namespace %s: %v\n", j.Name, j.Namespace, err)
		}

		jobData := &apis.WorkloadData{
			Name:      job.Name,
			Namespace: job.Namespace,
			Pods:      pods,
			Status: &apis.Status{
				State:     jState,
				Condition: condition,
			},
		}

		ResourceWorkloadArray.Job = append(ResourceWorkloadArray.Job, jobData)
		if jState == warning {
			resourceInspections = append(resourceInspections, apis.NewInspection(fmt.Sprintf("Job %s 警告", jobData.Name), fmt.Sprintf("命名空间 %s 下的 Job %s 处于非健康状态", jobData.Namespace, jobData.Name), defaultLevel(j.Level)))
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

		var emptyResourceQuota, emptyResource bool

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
			emptyResourceQuota = true
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
			emptyResource = true
			resourceInspections = append(resourceInspections, apis.NewInspection(
				fmt.Sprintf("命名空间 %s 下资源为空", n.Name),
				"检查对象为 Pod、Service、Deployment、Replicaset、Statefulset、Daemonset、Job、Secret、ConfigMap",
				1,
			))
		}

		namespaces = append(namespaces, &apis.Namespace{
			Name:               n.Name,
			EmptyResourceQuota: emptyResourceQuota,
			EmptyResource:      emptyResource,
			PodCount:           len(podList.Items),
			ServiceCount:       len(serviceList.Items),
			DeploymentCount:    len(deploymentList.Items),
			ReplicasetCount:    len(replicaSetList.Items),
			StatefulsetCount:   len(statefulSetList.Items),
			DaemonsetCount:     len(daemonSetList.Items),
			JobCount:           len(jobList.Items),
			SecretCount:        len(secretList.Items),
			ConfigMapCount:     len(configMapList.Items) - 1,
		})

		logrus.Debugf("Processed namespace: %s", n.Name)
	}

	logrus.Infof("[%s] Completed namespace retrieval", taskName)
	return namespaces, resourceInspections, nil
}

func GetServices(client *apis.Client, taskName string) ([]*apis.Service, []*apis.Inspection, error) {
	logrus.Infof("[%s] Starting services inspection", taskName)

	resourceInspections := apis.NewInspections()
	services := apis.NewServices()

	serviceList, err := client.Clientset.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("Error listing services: %v\n", err)
	}

	for _, s := range serviceList.Items {
		logrus.Debugf("Processing service: %s/%s", s.Namespace, s.Name)
		endpoints, err := client.Clientset.CoreV1().Endpoints(s.Namespace).Get(context.TODO(), s.Name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				logrus.Warnf("Service %s/%s does not have corresponding endpoints", s.Namespace, s.Name)
				resourceInspections = append(resourceInspections, apis.NewInspection(
					fmt.Sprintf("命名空间 %s 下 Service %s 找不到对应 endpoint", s.Namespace, s.Name),
					"对应的 Endpoints 未找到",
					1,
				))
				continue
			}
			return nil, nil, fmt.Errorf("Error getting endpoints for service %s/%s: %v\n", s.Namespace, s.Name, err)
		}

		var emptyEndpoints bool
		if len(endpoints.Subsets) == 0 {
			emptyEndpoints = true
			resourceInspections = append(resourceInspections, apis.NewInspection(
				fmt.Sprintf("命名空间 %s 下 Service %s 对应 Endpoints 没有 Subsets", s.Namespace, s.Name),
				"对应的 Endpoints 没有 Subsets",
				1,
			))
		}

		services = append(services, &apis.Service{
			Name:           s.Name,
			Namespace:      s.Namespace,
			EmptyEndpoints: emptyEndpoints,
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
			for _, path := range rule.HTTP.Paths {
				key := host + path.Path
				ingressMap[key] = append(ingressMap[key], fmt.Sprintf("%s/%s", i.Namespace, i.Name))
			}
		}

		ingress = append(ingress, &apis.Ingress{
			Name:          i.Name,
			Namespace:     i.Namespace,
			DuplicatePath: false,
		})
	}

	duplicateIngress := make(map[string]int)
	for key, ingressNames := range ingressMap {
		if len(ingressNames) > 1 {
			for _, ingressName := range ingressNames {
				duplicateIngress[ingressName] = 1
			}
			logrus.Warnf("Found duplicate ingress with same path: %s, Ingress list: %v", key, ingressNames)
		}
	}

	if len(duplicateIngress) > 0 {
		var result []string
		for namespaceName := range duplicateIngress {
			parts := strings.Split(namespaceName, "/")
			for index, i := range ingress {
				if parts[0] == i.Namespace && parts[1] == i.Name {
					ingress[index] = &apis.Ingress{
						Name:          i.Name,
						Namespace:     i.Namespace,
						DuplicatePath: true,
					}
				}
			}

			result = append(result, namespaceName)
		}

		resourceInspections = append(resourceInspections, apis.NewInspection(
			fmt.Sprintf("Ingress %s 存在重复的 Path", strings.Join(result, ", ")),
			"存在重复的路径",
			1,
		))
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

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"os"
	"regexp"
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
				coreInspections = append(coreInspections, apis.NewInspection(fmt.Sprintf("cluster %s (%s) failed", clusterName, r.Description), fmt.Sprintf("%s", r.Error), 3, []string{}))
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

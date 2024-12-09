package core

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	"io"
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

func GetItemsCount(items []*apis.Item) *apis.ItemsCount {
	totalCount := len(items)
	passCount := 0
	for _, i := range items {
		if i.Pass {
			passCount++
		}
	}

	return &apis.ItemsCount{
		PassCount:  passCount,
		TotalCount: totalCount,
	}
}

func Contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
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

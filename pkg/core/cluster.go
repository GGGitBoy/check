package core

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	"inspection-server/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"strings"
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

func GetClusterCore(client *apis.Client, clusterCoreConfig *apis.ClusterCoreConfig, clusterName, taskName string, clusterItem map[string][]*apis.Item) (*apis.ClusterCore, error) {
	logrus.Infof("[%s] Starting health check inspection", taskName)
	healthCheck := apis.NewHealthCheck()

	set := labels.Set(map[string]string{"name": "inspection-agent"})
	podList, err := client.Clientset.CoreV1().Pods(common.InspectionNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		return nil, fmt.Errorf("Error listing pods in namespace %s: %v\n", common.InspectionNamespace, err)
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
			return nil, fmt.Errorf("Error executing command in pod %s: %v\n", podList.Items[0].Name, err)
		}

		if stderr != "" {
			return nil, fmt.Errorf("Stderr from pod %s: %s\n", podList.Items[0].Name, stderr)
		}

		var results []apis.CommandCheckResult
		err = json.Unmarshal([]byte(stdout), &results)
		if err != nil {
			return nil, fmt.Errorf("Error unmarshalling stdout for pod %s: %v\n", podList.Items[0].Name, err)
		}

		for _, r := range results {
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

	apps, err := client.DynamicClient.Resource(common.AppRes).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list clusters: %v", err)
		return nil, fmt.Errorf("Failed to list apps: %v\n", err)
	}

	var items []*apis.Item
	if clusterCoreConfig.ChartVersionCheck.Enable {
		chartVersionCheck := true
		chartVersionCheckMessage := ""
		var sb strings.Builder
		if clusterCoreConfig.ChartVersionCheck != nil {
			for _, a := range apps.Items {
				excludedNamespaces := strings.Split(clusterCoreConfig.ChartVersionCheck.ExcludedNamespaces, ",")
				if Contains(excludedNamespaces, a.GetNamespace()) {
					continue
				}

				specMetadata, exists, err := unstructured.NestedMap(a.UnstructuredContent(), "spec", "chart", "metadata")
				if err != nil {
					logrus.Errorf("Failed to get app spec for %s: %v", a.GetName(), err)
					continue
				}

				if !exists {
					logrus.Errorf("Failed to get fields")
					continue
				}

				version, ok := specMetadata["version"].(string)
				if !ok {
					logrus.Warnf("version not found for app %s", a.GetName())
					continue
				}

				if !Contains(clusterCoreConfig.ChartVersionCheck.AllowVersion, version) {
					sb.WriteString(fmt.Sprintf("app %s / %s 的版本为 %s\n", a.GetNamespace(), a.GetName(), version))
				}
			}
		}

		if sb.Len() != 0 {
			chartVersionCheck = false
			chartVersionCheckMessage = sb.String()
		}

		items = append(items, &apis.Item{
			Name:    "是否为预期的 chart 版本",
			Message: chartVersionCheckMessage,
			Pass:    chartVersionCheck,
			Level:   1,
		})
	}

	grafanaItem, ok := clusterItem[clusterName]
	if ok {
		items = append(items, grafanaItem...)
	}

	itemsCount := GetItemsCount(items)

	return &apis.ClusterCore{
		HealthCheck: healthCheck,
		Items:       items,
		ItemsCount:  itemsCount,
	}, nil
}

func getCommandCheckResult(r apis.CommandCheckResult) *apis.CommandCheckResult {
	return &apis.CommandCheckResult{
		Description: r.Description,
		Command:     r.Command,
		Response:    r.Response,
		Error:       r.Error,
	}
}

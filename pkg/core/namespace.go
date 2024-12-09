package core

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"strings"
)

func GetNamespaces(client *apis.Client, nsConfig *apis.NamespaceConfig, taskName string, nsItem map[string][]*apis.Item) ([]*apis.Namespace, error) {
	logrus.Infof("[%s] Starting namespaces inspection", taskName)

	namespaces := apis.NewNamespaces()

	listOptions := metav1.ListOptions{}
	if nsConfig.SelectorLabels != nil && len(nsConfig.SelectorLabels) > 0 {
		var set labels.Set
		set = nsConfig.SelectorLabels
		listOptions = metav1.ListOptions{LabelSelector: set.AsSelector().String()}
	}

	namespaceList, err := client.Clientset.CoreV1().Namespaces().List(context.TODO(), listOptions)
	if err != nil {
		return nil, fmt.Errorf("Error listing namespaces: %v\n", err)
	}

	for _, n := range namespaceList.Items {
		logrus.Debugf("Processing namespace: %s", n.Name)

		emptyResourceQuota := true
		emptyResource := true
		emptyResourceQuotaMessage := ""
		emptyResourceMessage := ""

		podList, err := client.Clientset.CoreV1().Pods(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("Error listing pods in namespace %s: %v\n", n.Name, err)
		}

		serviceList, err := client.Clientset.CoreV1().Services(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("Error listing services in namespace %s: %v\n", n.Name, err)
		}

		deploymentList, err := client.Clientset.AppsV1().Deployments(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("Error listing deployments in namespace %s: %v\n", n.Name, err)
		}

		replicaSetList, err := client.Clientset.AppsV1().ReplicaSets(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("Error listing replica sets in namespace %s: %v\n", n.Name, err)
		}

		statefulSetList, err := client.Clientset.AppsV1().StatefulSets(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("Error listing stateful sets in namespace %s: %v\n", n.Name, err)
		}

		daemonSetList, err := client.Clientset.AppsV1().DaemonSets(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("Error listing daemon sets in namespace %s: %v\n", n.Name, err)
		}

		jobList, err := client.Clientset.BatchV1().Jobs(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("Error listing jobs in namespace %s: %v\n", n.Name, err)
		}

		secretList, err := client.Clientset.CoreV1().Secrets(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("Error listing secrets in namespace %s: %v\n", n.Name, err)
		}

		configMapList, err := client.Clientset.CoreV1().ConfigMaps(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("Error listing config maps in namespace %s: %v\n", n.Name, err)
		}

		resourceQuotaList, err := client.Clientset.CoreV1().ResourceQuotas(n.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("Error listing resource quotas in namespace %s: %v\n", n.Name, err)
		}

		if len(resourceQuotaList.Items) == 0 {
			emptyResourceQuota = false
			emptyResourceQuotaMessage = fmt.Sprintf("命名空间 %s 没有设置配额", n.Name)
		}

		totalResources := len(podList.Items) + len(serviceList.Items) + len(deploymentList.Items) +
			len(replicaSetList.Items) + len(statefulSetList.Items) + len(daemonSetList.Items) +
			len(jobList.Items) + len(secretList.Items) + (len(configMapList.Items) - 1)

		if totalResources == 0 {
			emptyResource = false
			emptyResourceMessage = fmt.Sprintf("命名空间 %s 下资源为空", n.Name)
		}

		items := []*apis.Item{
			{
				Name:    "有资源配置设置",
				Message: emptyResourceQuotaMessage,
				Pass:    emptyResourceQuota,
				Level:   1,
			},
			{
				Name:    "命名空间下资源非空",
				Message: emptyResourceMessage,
				Pass:    emptyResource,
				Level:   1,
			},
		}

		if nsConfig.NameCheck.IncludeName != "" {
			if nsConfig.NameCheck.ExcludedNamespace != "" {
				excludedNamespaces := strings.Split(nsConfig.NameCheck.ExcludedNamespace, ",")
				if !Contains(excludedNamespaces, n.Name) {
					nameCheck := strings.Contains(n.Name, nsConfig.NameCheck.IncludeName)
					nameCheckMessage := ""
					if !nameCheck {
						nameCheckMessage = fmt.Sprintf("未包含 %s 内容", nsConfig.NameCheck.IncludeName)
					}

					items = append(items, &apis.Item{
						Name:    "命名空间名称是否符合规范",
						Message: nameCheckMessage,
						Pass:    nameCheck,
					})
				}
			} else {
				nameCheck := strings.Contains(n.Name, nsConfig.NameCheck.IncludeName)
				nameCheckMessage := ""
				if !nameCheck {
					nameCheckMessage = fmt.Sprintf("未包含 %s 内容", nsConfig.NameCheck.IncludeName)
				}

				items = append(items, &apis.Item{
					Name:    "命名空间名称是否符合规范",
					Message: nameCheckMessage,
					Pass:    nameCheck,
				})
			}
		}

		grafanaItem, ok := nsItem[n.Name]
		if ok {
			items = append(items, grafanaItem...)
		}

		itemsCount := GetItemsCount(items)

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
			Items:            items,
			ItemsCount:       itemsCount,
		})

		logrus.Debugf("Processed namespace: %s", n.Name)
	}

	logrus.Infof("[%s] Completed namespace retrieval", taskName)
	return namespaces, nil
}

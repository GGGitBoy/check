package core

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"strings"
)

func GetWorkloads(client *apis.Client, workloadConfig *apis.WorkloadConfig, taskName string, clusterResourceItem *ClusterResourceItem) (*apis.Workload, error) {
	logrus.Infof("[%s] Starting workload inspection", taskName)

	ResourceWorkloadArray := apis.NewWorkload()

	if workloadConfig.Deployment.Enable {
		selectorNamespaces := []string{""}
		if workloadConfig.Deployment.SelectorNamespace != "" {
			selectorNamespaces = strings.Split(workloadConfig.Deployment.SelectorNamespace, ",")
		}

		listOptions := metav1.ListOptions{}
		if workloadConfig.Deployment.SelectorLabels != nil && len(workloadConfig.Deployment.SelectorLabels) > 0 {
			var set labels.Set
			set = workloadConfig.Deployment.SelectorLabels
			listOptions = metav1.ListOptions{LabelSelector: set.AsSelector().String()}
		}

		var allDeployment []appsv1.Deployment
		for _, ns := range selectorNamespaces {
			deploymentList, err := client.Clientset.AppsV1().Deployments(ns).List(context.TODO(), listOptions)
			if err != nil {
				return nil, fmt.Errorf("Error listing Deployment: %v\n", err)
			}

			allDeployment = append(allDeployment, deploymentList.Items...)
		}

		for _, deploy := range allDeployment {
			logrus.Debugf("[%s] Inspecting Deployment: %s in namespace %s", taskName, deploy.Name, deploy.Namespace)

			deployHealth := true
			deployHealthMessage := ""
			if !isDeploymentAvailable(&deploy) {
				deployHealth = false
				deployHealthMessage = fmt.Sprintf("命名空间 %s 下的 Deployment %s 处于非健康状态", deploy.Namespace, deploy.Name)
			}

			var condition []apis.Condition
			for _, c := range deploy.Status.Conditions {
				condition = append(condition, apis.Condition{
					Type:   string(c.Type),
					Status: string(c.Status),
					Reason: c.Reason,
				})
			}

			//podset := labels.Set(deploy.Spec.Selector.MatchLabels)
			//pods, err := GetPod("", deploy.Namespace, podset, client.Clientset, taskName)
			//if err != nil {
			//	return nil, nil, fmt.Errorf("Error getting pods for Deployment %s in namespace %s: %v\n", deploy.Name, deploy.Namespace, err)
			//}

			var items []*apis.Item
			items = append(items, &apis.Item{
				Name:    "健康状态",
				Message: deployHealthMessage,
				Pass:    deployHealth,
				Level:   1,
			})

			checkContainersProbeItem := CheckContainersProbe(deploy.Spec.Template.Spec.Containers)
			items = append(items, checkContainersProbeItem)

			grafanaItem, ok := clusterResourceItem.DeploymentItem[deploy.Namespace+"/"+deploy.Name]
			if ok {
				items = append(items, grafanaItem...)
			}

			itemsCount := GetItemsCount(items)

			deploymentData := &apis.WorkloadData{
				Name:      deploy.Name,
				Namespace: deploy.Namespace,
				//Pods:      pods,
				Status: &apis.Status{
					Condition: condition,
				},
				Items:      items,
				ItemsCount: itemsCount,
			}

			ResourceWorkloadArray.Deployment = append(ResourceWorkloadArray.Deployment, deploymentData)
		}
	}

	if workloadConfig.Daemonset.Enable {
		selectorNamespaces := []string{""}
		if workloadConfig.Daemonset.SelectorNamespace != "" {
			selectorNamespaces = strings.Split(workloadConfig.Daemonset.SelectorNamespace, ",")
		}

		listOptions := metav1.ListOptions{}
		if workloadConfig.Daemonset.SelectorLabels != nil && len(workloadConfig.Daemonset.SelectorLabels) > 0 {
			var set labels.Set
			set = workloadConfig.Daemonset.SelectorLabels
			listOptions = metav1.ListOptions{LabelSelector: set.AsSelector().String()}
		}

		var allDaemonSets []appsv1.DaemonSet
		for _, ns := range selectorNamespaces {
			daemonSetList, err := client.Clientset.AppsV1().DaemonSets(ns).List(context.TODO(), listOptions)
			if err != nil {
				return nil, fmt.Errorf("Error listing DaemonSets: %v\n", err)
			}

			allDaemonSets = append(allDaemonSets, daemonSetList.Items...)
		}

		for _, ds := range allDaemonSets {
			logrus.Debugf("[%s] Inspecting DaemonSet: %s in namespace %s", taskName, ds.Name, ds.Namespace)

			dsHealth := true
			dsHealthMessage := ""
			if !isDaemonSetAvailable(&ds) {
				dsHealth = false
				dsHealthMessage = fmt.Sprintf("命名空间 %s 下的 DaemonSet %s 处于非健康状态", ds.Namespace, ds.Name)
			}

			var condition []apis.Condition
			for _, c := range ds.Status.Conditions {
				condition = append(condition, apis.Condition{
					Type:   string(c.Type),
					Status: string(c.Status),
					Reason: c.Reason,
				})
			}

			//podset := labels.Set(ds.Spec.Selector.MatchLabels)
			//pods, err := GetPod("", ds.Namespace, podset, client.Clientset, taskName)
			//if err != nil {
			//	return nil, nil, fmt.Errorf("Error getting pods for DaemonSet %s in namespace %s: %v\n", ds.Name, ds.Namespace, err)
			//}

			var items []*apis.Item
			items = append(items, &apis.Item{
				Name:    "健康状态",
				Message: dsHealthMessage,
				Pass:    dsHealth,
				Level:   1,
			})

			checkContainersProbeItem := CheckContainersProbe(ds.Spec.Template.Spec.Containers)
			items = append(items, checkContainersProbeItem)

			grafanaItem, ok := clusterResourceItem.DaemonsetItem[ds.Namespace+"/"+ds.Name]
			if ok {
				items = append(items, grafanaItem...)
			}

			itemsCount := GetItemsCount(items)

			daemonsetData := &apis.WorkloadData{
				Name:      ds.Name,
				Namespace: ds.Namespace,
				//Pods:      pods,
				Status: &apis.Status{
					Condition: condition,
				},
				Items:      items,
				ItemsCount: itemsCount,
			}

			ResourceWorkloadArray.Daemonset = append(ResourceWorkloadArray.Daemonset, daemonsetData)
		}
	}

	if workloadConfig.Statefulset.Enable {
		selectorNamespaces := []string{""}
		if workloadConfig.Statefulset.SelectorNamespace != "" {
			selectorNamespaces = strings.Split(workloadConfig.Statefulset.SelectorNamespace, ",")
		}

		listOptions := metav1.ListOptions{}
		if workloadConfig.Statefulset.SelectorLabels != nil && len(workloadConfig.Statefulset.SelectorLabels) > 0 {
			var set labels.Set
			set = workloadConfig.Statefulset.SelectorLabels
			listOptions = metav1.ListOptions{LabelSelector: set.AsSelector().String()}
		}

		var allStatefulSets []appsv1.StatefulSet
		for _, ns := range selectorNamespaces {
			statefulsetList, err := client.Clientset.AppsV1().StatefulSets(ns).List(context.TODO(), listOptions)
			if err != nil {
				return nil, fmt.Errorf("Error listing StatefulSets: %v\n", err)
			}

			allStatefulSets = append(allStatefulSets, statefulsetList.Items...)
		}

		for _, sts := range allStatefulSets {
			logrus.Debugf("[%s] Inspecting Statefulset: %s in namespace %s", taskName, sts.Name, sts.Namespace)

			stsHealth := true
			stsHealthMessage := ""
			if !isStatefulSetAvailable(&sts) {
				stsHealth = false
				stsHealthMessage = fmt.Sprintf("命名空间 %s 下的 Statefulset %s 处于非健康状态", sts.Namespace, sts.Name)
			}

			var condition []apis.Condition
			for _, c := range sts.Status.Conditions {
				condition = append(condition, apis.Condition{
					Type:   string(c.Type),
					Status: string(c.Status),
					Reason: c.Reason,
				})
			}

			//podset := labels.Set(sts.Spec.Selector.MatchLabels)
			//pods, err := GetPod("", sts.Namespace, podset, client.Clientset, taskName)
			//if err != nil {
			//	return nil, nil, fmt.Errorf("Error getting pods for Statefulset %s in namespace %s: %v\n", sts.Name, sts.Namespace, err)
			//}

			var items []*apis.Item
			items = append(items, &apis.Item{
				Name:    "健康状态",
				Message: stsHealthMessage,
				Pass:    stsHealth,
				Level:   1,
			})

			checkContainersProbeItem := CheckContainersProbe(sts.Spec.Template.Spec.Containers)
			items = append(items, checkContainersProbeItem)

			grafanaItem, ok := clusterResourceItem.StatefulsetItem[sts.Namespace+"/"+sts.Name]
			if ok {
				items = append(items, grafanaItem...)
			}

			itemsCount := GetItemsCount(items)

			statefulsetData := &apis.WorkloadData{
				Name:      sts.Name,
				Namespace: sts.Namespace,
				//Pods:      pods,
				Status: &apis.Status{
					Condition: condition,
				},
				Items:      items,
				ItemsCount: itemsCount,
			}

			ResourceWorkloadArray.Statefulset = append(ResourceWorkloadArray.Statefulset, statefulsetData)
		}
	}

	if workloadConfig.Job.Enable {
		selectorNamespaces := []string{""}
		if workloadConfig.Job.SelectorNamespace != "" {
			selectorNamespaces = strings.Split(workloadConfig.Job.SelectorNamespace, ",")
		}

		listOptions := metav1.ListOptions{}
		if workloadConfig.Job.SelectorLabels != nil && len(workloadConfig.Job.SelectorLabels) > 0 {
			var set labels.Set
			set = workloadConfig.Job.SelectorLabels
			listOptions = metav1.ListOptions{LabelSelector: set.AsSelector().String()}
		}

		var allJob []batchv1.Job
		for _, ns := range selectorNamespaces {
			jobList, err := client.Clientset.BatchV1().Jobs(ns).List(context.TODO(), listOptions)
			if err != nil {
				return nil, fmt.Errorf("Error listing Jobs: %v\n", err)
			}

			allJob = append(allJob, jobList.Items...)
		}

		for _, j := range allJob {
			logrus.Debugf("[%s] Inspecting Job: %s in namespace %s", taskName, j.Name, j.Namespace)

			jHealth := true
			jHealthMessage := ""
			if !isJobCompleted(&j) {
				jHealth = false
				jHealthMessage = fmt.Sprintf("命名空间 %s 下的 Job %s 处于非健康状态", j.Namespace, j.Name)
			}

			var condition []apis.Condition
			for _, c := range j.Status.Conditions {
				condition = append(condition, apis.Condition{
					Type:   string(c.Type),
					Status: string(c.Status),
					Reason: c.Reason,
				})
			}

			//podset := labels.Set(j.Spec.Selector.MatchLabels)
			//pods, err := GetPod("", j.Namespace, podset, client.Clientset, taskName)
			//if err != nil {
			//	return nil, nil, fmt.Errorf("Error getting pods for Job %s in namespace %s: %v\n", j.Name, j.Namespace, err)
			//}

			var items []*apis.Item
			items = append(items, &apis.Item{
				Name:    "健康状态",
				Message: jHealthMessage,
				Pass:    jHealth,
				Level:   1,
			})

			checkContainersProbeItem := CheckContainersProbe(j.Spec.Template.Spec.Containers)
			items = append(items, checkContainersProbeItem)

			grafanaItem, ok := clusterResourceItem.JobItem[j.Namespace+"/"+j.Name]
			if ok {
				items = append(items, grafanaItem...)
			}

			itemsCount := GetItemsCount(items)

			jobData := &apis.WorkloadData{
				Name:      j.Name,
				Namespace: j.Namespace,
				//Pods:      pods,
				Status: &apis.Status{
					Condition: condition,
				},
				Items:      items,
				ItemsCount: itemsCount,
			}

			ResourceWorkloadArray.Job = append(ResourceWorkloadArray.Job, jobData)
		}
	}

	logrus.Infof("[%s] Workload inspection completed", taskName)
	return ResourceWorkloadArray, nil
}

func CheckContainersProbe(containers []corev1.Container) *apis.Item {
	pass := false
	var sb strings.Builder
	for _, container := range containers {
		if container.LivenessProbe == nil {
			sb.WriteString(fmt.Sprintf("容器 %s 没有设置 LivenessProbe\n", container.Name))
		}

		if container.ReadinessProbe == nil {
			sb.WriteString(fmt.Sprintf("容器 %s 没有设置 ReadinessProbe\n", container.Name))
		}
	}

	message := sb.String()
	if message == "" {
		pass = true
	}

	return &apis.Item{
		Name:    "健康检查设置",
		Message: message,
		Pass:    pass,
	}
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

package template

import (
	"context"
	"fmt"
	detector "github.com/rancher/kubernetes-provider-detector"
	detectorProviders "github.com/rancher/kubernetes-provider-detector/providers"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	"inspection-server/pkg/common"
	"inspection-server/pkg/db"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Register() error {
	logrus.Infof("[Template] Starting template registration process...")

	localKubernetesClient, err := common.GetKubernetesClient(common.LocalCluster)
	if err != nil {
		return fmt.Errorf("failed to get local Kubernetes client: %w", err)
	}

	clusters, err := localKubernetesClient.DynamicClient.Resource(common.ClusterRes).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	template := apis.NewTemplate()
	kubernetesConfig := apis.NewKubernetesConfig()

	for _, c := range clusters.Items {
		clusterName := c.GetName()
		logrus.Infof("[Template] Processing cluster: %s\n", clusterName)

		spec, _, err := unstructured.NestedMap(c.UnstructuredContent(), "spec")
		if err != nil {
			return fmt.Errorf("error getting spec for cluster %s: %w", clusterName, err)
		}

		clusterDisplayName, ok := spec["displayName"].(string)
		if !ok {
			return fmt.Errorf("invalid displayName format for cluster %s", clusterName)
		}

		if !common.IsClusterReady(c) {
			kubernetesConfig = append(kubernetesConfig, &apis.KubernetesConfig{
				Enable:      false,
				ClusterID:   clusterName,
				ClusterName: clusterDisplayName,
			})
			continue
		}

		kubernetesClient, err := common.GetKubernetesClient(clusterName)
		if err != nil {
			return fmt.Errorf("Failed to get Kubernetes client for cluster %s: %v\n", clusterName, err)
		}

		provider, err := detector.DetectProvider(context.TODO(), kubernetesClient.Clientset)
		if err != nil {
			return fmt.Errorf("Failed to detect provider for cluster %s: %v\n", clusterName, err)
		}

		workloadConfig := apis.NewWorkloadConfig()
		workloadConfig = getWorkloadConfigByProvider(provider)
		if c.GetName() == common.LocalCluster {
			logrus.Infof("[Template] %s cluster add rancher check\n", c.GetName())
			workloadConfig.Deployment = append(workloadConfig.Deployment, &apis.WorkloadDetailConfig{
				Name:      "rancher",
				Namespace: "cattle-system",
				Regexp:    "\\[(ERROR|WARNING)\\].*",
				Level:     3,
			})
		}

		nodeList, err := kubernetesClient.Clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("Failed to list nodes for cluster %s: %v\n", clusterName, err)
		}

		nodeConfigs := apis.NewNodeConfigs()
		nodeConfigs, err = generateNodeConfigs(nodeList, provider)
		if err != nil {
			return fmt.Errorf("Failed to generate node configs for cluster %s: %v\n", clusterName, err)
		}

		clusterCoreConfig := apis.NewClusterCoreConfig()
		clusterNodeConfig := apis.NewClusterNodeConfig()
		clusterResourceConfig := apis.NewClusterResourceConfig()

		clusterNodeConfig.NodeConfig = nodeConfigs
		clusterResourceConfig = &apis.ClusterResourceConfig{
			WorkloadConfig: workloadConfig,
			NamespaceConfig: &apis.NamespaceConfig{
				Enable: true,
			},
			ServiceConfig: &apis.ServiceConfig{
				Enable: true,
			},
			IngressConfig: &apis.IngressConfig{
				Enable: true,
			},
		}

		kubernetesConfig = append(kubernetesConfig, &apis.KubernetesConfig{
			Enable:                true,
			ClusterID:             clusterName,
			ClusterName:           clusterDisplayName,
			ClusterCoreConfig:     clusterCoreConfig,
			ClusterNodeConfig:     clusterNodeConfig,
			ClusterResourceConfig: clusterResourceConfig,
		})
	}

	template, err = db.GetTemplate("Default")
	if err != nil {
		logrus.Infof("[Template] Creating template in the database...")
		t := &apis.Template{
			ID:               "Default",
			Name:             "Default",
			KubernetesConfig: kubernetesConfig,
		}

		if err := db.CreateTemplate(t); err != nil {
			return fmt.Errorf("failed to create template: %w", err)
		}
	} else {
		logrus.Infof("[Template] Updating template in the database...")
		template.KubernetesConfig = kubernetesConfig
		if err := db.UpdateTemplate(template); err != nil {
			return fmt.Errorf("failed to update template: %w", err)
		}
	}

	logrus.Infof("[Template] Template registration completed successfully.")
	return nil
}

// 获取工作负载配置
func getWorkloadConfigByProvider(provider string) *apis.WorkloadConfig {
	switch provider {
	case detectorProviders.RKE:
		return NewRKE1WorkloadConfig()
	case detectorProviders.RKE2:
		return NewRKE2WorkloadConfig()
	case detectorProviders.K3s:
		return NewK3SWorkloadConfig()
	default:
		return &apis.WorkloadConfig{}
	}
}

// 生成节点配置
func generateNodeConfigs(nodeList *v1.NodeList, provider string) ([]*apis.NodeConfig, error) {
	var nodeConfigs []*apis.NodeConfig
	var workerNames, otherNodeNames []string

	for _, n := range nodeList.Items {
		if isWorkerNode(n) || isMasterNode(n) {
			workerNames = append(workerNames, n.GetName())
		} else {
			otherNodeNames = append(otherNodeNames, n.GetName())
		}
	}

	if len(workerNames) > 0 {
		workerCommands := generateWorkerCommands(provider)
		nodeConfigs = append(nodeConfigs, &apis.NodeConfig{
			Names:    workerNames,
			Commands: workerCommands,
		})
	}

	if len(otherNodeNames) > 0 {
		otherCommands := generateOtherCommands(provider)
		nodeConfigs = append(nodeConfigs, &apis.NodeConfig{
			Names:    otherNodeNames,
			Commands: otherCommands,
		})
	}

	return nodeConfigs, nil
}

// 生成工作节点命令
func generateWorkerCommands(provider string) []*apis.CommandConfig {
	commands := []*apis.CommandConfig{
		{
			Description: "Kubelet Health Check",
			Command:     "curl -sS http://localhost:10248/healthz",
		},
		{
			Description: "KubeProxy Health Check",
			Command:     "curl -sS http://localhost:10256/healthz > /dev/null 2>&1 && echo ok || { curl -sS http://localhost:10256/healthz; }",
		},
	}

	switch provider {
	case detectorProviders.RKE:
		commands = append(commands, &apis.CommandConfig{
			Description: "Docker Health Check",
			Command:     "docker ps > /dev/null 2>&1 && echo ok || { docker ps; }",
		})
	case detectorProviders.RKE2, detectorProviders.K3s:
		commands = append(commands, &apis.CommandConfig{
			Description: "Containerd Health Check",
			Command:     "crictl pods > /dev/null 2>&1 && echo ok || { crictl pods; }",
		})
	}

	return commands
}

// 生成其他节点命令
func generateOtherCommands(provider string) []*apis.CommandConfig {
	var commands []*apis.CommandConfig

	switch provider {
	case detectorProviders.RKE:
		commands = append(commands, &apis.CommandConfig{
			Description: "Docker Health Check",
			Command:     "docker ps > /dev/null 2>&1 && echo ok || { docker ps; }",
		})
	case detectorProviders.RKE2, detectorProviders.K3s:
		commands = append(commands, &apis.CommandConfig{
			Description: "Containerd Health Check",
			Command:     "crictl pods > /dev/null 2>&1 && echo ok || { crictl pods; }",
		})
	}

	return commands
}

// 检查节点是否为工作节点
func isWorkerNode(node v1.Node) bool {
	isWorker, ok := node.Labels["node-role.kubernetes.io/worker"]
	return ok && isWorker == "true"
}

// 检查节点是否为主节点
func isMasterNode(node v1.Node) bool {
	isMaster, ok := node.Labels["node-role.kubernetes.io/master"]
	return ok && isMaster == "true"
}

func NewRKE1WorkloadConfig() *apis.WorkloadConfig {
	return &apis.WorkloadConfig{
		Deployment: []*apis.WorkloadDetailConfig{
			{
				Name:      "cattle-cluster-agent",
				Namespace: "cattle-system",
			},
			{
				Name:      "rancher-webhook",
				Namespace: "cattle-system",
			},
			{
				Name:      "calico-kube-controllers",
				Namespace: "kube-system",
			},
			{
				Name:      "coredns",
				Namespace: "kube-system",
				Regexp:    "",
			},
			{
				Name:      "coredns-autoscaler",
				Namespace: "kube-system",
			},
			{
				Name:      "metrics-server",
				Namespace: "kube-system",
			},
		},
		Daemonset: []*apis.WorkloadDetailConfig{
			{
				Name:      "inspection-agent",
				Namespace: "cattle-system",
			},
			{
				Name:      "kube-api-auth",
				Namespace: "cattle-system",
			},
			{
				Name:      "cattle-node-agent",
				Namespace: "cattle-system",
			},
			{
				Name:      "nginx-ingress-controller",
				Namespace: "ingress-nginx",
			},
			{
				Name:      "canal",
				Namespace: "kube-system",
			},
		},
	}
}

func NewRKE2WorkloadConfig() *apis.WorkloadConfig {
	return &apis.WorkloadConfig{
		Deployment: []*apis.WorkloadDetailConfig{
			{
				Name:      "cattle-cluster-agent",
				Namespace: "cattle-system",
			},
			{
				Name:      "rancher-webhook",
				Namespace: "cattle-system",
			},
			{
				Name:      "system-upgrade-controller",
				Namespace: "cattle-system",
			},
			{
				Name:      "rke2-coredns-rke2-coredns",
				Namespace: "kube-system",
			},
			{
				Name:      "rke2-coredns-rke2-coredns-autoscaler",
				Namespace: "kube-system",
			},
			{
				Name:      "rke2-metrics-server",
				Namespace: "kube-system",
			},
			{
				Name:      "rke2-snapshot-controller",
				Namespace: "kube-system",
			},
			{
				Name:      "rke2-snapshot-validation-webhook",
				Namespace: "kube-system",
			},
			{
				Name:      "calico-kube-controllers",
				Namespace: "calico-system",
			},
			{
				Name:      "calico-typha",
				Namespace: "calico-system",
			},
		},
		Daemonset: []*apis.WorkloadDetailConfig{
			{
				Name:      "inspection-agent",
				Namespace: "cattle-system",
			},
			{
				Name:      "rke2-ingress-nginx-controller",
				Namespace: "kube-system",
			},
			{
				Name:      "calico-node",
				Namespace: "calico-system",
			},
		},
	}
}

func NewK3SWorkloadConfig() *apis.WorkloadConfig {
	return &apis.WorkloadConfig{
		Deployment: []*apis.WorkloadDetailConfig{
			{
				Name:      "cattle-cluster-agent",
				Namespace: "cattle-system",
			},
			{
				Name:      "rancher-webhook",
				Namespace: "cattle-system",
			},
			{
				Name:      "coredns",
				Namespace: "kube-system",
			},
		},
		Daemonset: []*apis.WorkloadDetailConfig{
			{
				Name:      "inspection-agent",
				Namespace: "cattle-system",
			},
		},
	}
}

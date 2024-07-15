package agent

import (
	"bytes"
	"context"
	"fmt"
	detector "github.com/rancher/kubernetes-provider-detector"
	detectorProviders "github.com/rancher/kubernetes-provider-detector/providers"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyappsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applyrbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"k8s.io/client-go/kubernetes"
	"os"
	"sigs.k8s.io/yaml"
	"text/template"
)

var image = "cnrancher/inspection-agent:v1.0.0"

func Register() error {
	logrus.Infof("[Agent] Starting registration of inspection agents")

	localKubernetesClient, err := common.GetKubernetesClient(common.LocalCluster)
	if err != nil {
		logrus.Errorf("Failed to get Kubernetes client for local cluster: %v", err)
		return err
	}

	clusters, err := localKubernetesClient.DynamicClient.Resource(common.ClusterRes).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list clusters: %v", err)
		return err
	}

	for _, c := range clusters.Items {
		logrus.Infof("[Agent] Processing cluster: %s", c.GetName())

		if !common.IsClusterReady(c) {
			logrus.Errorf("cluster %s is not ready", c.GetName())
			continue
		}

		kubernetesClient, err := common.GetKubernetesClient(c.GetName())
		if err != nil {
			logrus.Errorf("Failed to get Kubernetes client for cluster %s: %v", c.GetName(), err)
			continue
		}

		err = CreateAgent(kubernetesClient.Clientset)
		if err != nil {
			logrus.Errorf("Failed to create agent for cluster %s: %v", c.GetName(), err)
			continue
		}

		logrus.Infof("[Agent] Successfully registered inspection agent for cluster: %s", c.GetName())
	}

	return nil
}

func CreateAgent(clientset *kubernetes.Clientset) error {
	logrus.Infof("[Agent] Creating inspection agent resources")

	if err := ApplyNamespace(clientset); err != nil {
		logrus.Errorf("Failed to apply namespace: %v", err)
		return err
	}

	if err := ApplyServiceAccount(clientset); err != nil {
		logrus.Errorf("Failed to apply service account: %v", err)
		return err
	}

	if err := ApplyClusterRoleBinding(clientset); err != nil {
		logrus.Errorf("Failed to apply cluster role binding: %v", err)
		return err
	}

	if err := ApplyConfigMap(clientset); err != nil {
		logrus.Errorf("Failed to apply config map: %v", err)
		return err
	}

	if err := ApplyDaemonSet(clientset); err != nil {
		logrus.Errorf("Failed to apply daemon set: %v", err)
		return err
	}

	logrus.Infof("[Agent] Successfully created all inspection agent resources")
	return nil
}

func ApplyNamespace(clientset *kubernetes.Clientset) error {
	logrus.Infof("[Agent] Applying namespace configuration")

	yamlFile, err := os.ReadFile(common.AgentYamlPath + "namespace.yaml")
	if err != nil {
		logrus.Errorf("Failed to read namespace YAML file: %v", err)
		return err
	}

	var namespace *applycorev1.NamespaceApplyConfiguration
	if err := yaml.Unmarshal(yamlFile, &namespace); err != nil {
		logrus.Errorf("Failed to unmarshal namespace YAML: %v", err)
		return err
	}

	if _, err := clientset.CoreV1().Namespaces().Apply(context.TODO(), namespace, metav1.ApplyOptions{Force: true, FieldManager: "application/apply-patch"}); err != nil {
		logrus.Errorf("Failed to apply namespace: %v", err)
		return err
	}

	logrus.Infof("[Agent] Namespace configuration applied successfully")
	return nil
}

func ApplyServiceAccount(clientset *kubernetes.Clientset) error {
	logrus.Infof("[Agent] Applying service account configuration")

	yamlFile, err := os.ReadFile(common.AgentYamlPath + "serviceaccount.yaml")
	if err != nil {
		logrus.Errorf("Failed to read service account YAML file: %v", err)
		return err
	}

	var serviceAccount *applycorev1.ServiceAccountApplyConfiguration
	if err := yaml.Unmarshal(yamlFile, &serviceAccount); err != nil {
		logrus.Errorf("Failed to unmarshal service account YAML: %v", err)
		return err
	}

	if _, err := clientset.CoreV1().ServiceAccounts(common.InspectionNamespace).Apply(context.TODO(), serviceAccount, metav1.ApplyOptions{Force: true, FieldManager: "application/apply-patch"}); err != nil {
		logrus.Errorf("Failed to apply service account: %v", err)
		return err
	}

	logrus.Infof("[Agent] Service account configuration applied successfully")
	return nil
}

func ApplyClusterRoleBinding(clientset *kubernetes.Clientset) error {
	logrus.Infof("[Agent] Applying cluster role binding configuration")

	yamlFile, err := os.ReadFile(common.AgentYamlPath + "clusterrolebinding.yaml")
	if err != nil {
		logrus.Errorf("Failed to read cluster role binding YAML file: %v", err)
		return err
	}

	var clusterRoleBinding *applyrbacv1.ClusterRoleBindingApplyConfiguration
	if err := yaml.Unmarshal(yamlFile, &clusterRoleBinding); err != nil {
		logrus.Errorf("Failed to unmarshal cluster role binding YAML: %v", err)
		return err
	}

	if _, err := clientset.RbacV1().ClusterRoleBindings().Apply(context.TODO(), clusterRoleBinding, metav1.ApplyOptions{Force: true, FieldManager: "application/apply-patch"}); err != nil {
		logrus.Errorf("Failed to apply cluster role binding: %v", err)
		return err
	}

	logrus.Infof("[Agent] Cluster role binding configuration applied successfully")
	return nil
}

func ApplyConfigMap(clientset *kubernetes.Clientset) error {
	logrus.Infof("[Agent] Applying config map configuration")

	yamlFile, err := os.ReadFile(common.AgentYamlPath + "configmap.yaml")
	if err != nil {
		logrus.Errorf("Failed to read config map YAML file: %v", err)
		return err
	}

	var configMap *applycorev1.ConfigMapApplyConfiguration
	if err := yaml.Unmarshal(yamlFile, &configMap); err != nil {
		logrus.Errorf("Failed to unmarshal config map YAML: %v", err)
		return err
	}

	if _, err := clientset.CoreV1().ConfigMaps(common.InspectionNamespace).Apply(context.TODO(), configMap, metav1.ApplyOptions{Force: true, FieldManager: "application/apply-patch"}); err != nil {
		logrus.Errorf("Failed to apply config map: %v", err)
		return err
	}

	logrus.Infof("[Agent] Config map configuration applied successfully")
	return nil
}

func ApplyDaemonSet(clientset *kubernetes.Clientset) error {
	logrus.Infof("[Agent] Applying daemon set configuration")

	provider, err := detector.DetectProvider(context.TODO(), clientset)
	if err != nil {
		logrus.Errorf("Failed to detect Kubernetes provider: %v", err)
		return err
	}

	var setDocker, setContainerd bool
	switch provider {
	case detectorProviders.RKE:
		setDocker = true
	case detectorProviders.RKE2, detectorProviders.K3s:
		setContainerd = true
	default:
		logrus.Warnf("Unknown provider detected: %s", provider)
	}

	tmpl, err := template.ParseFiles(common.AgentYamlPath + "daemonset.yaml")
	if err != nil {
		logrus.Errorf("Failed to parse daemon set template: %v", err)
		return err
	}

	if common.SystemDefaultRegistry != "" {
		image = fmt.Sprintf("%s/%s", common.SystemDefaultRegistry, image)
	}

	var rendered bytes.Buffer
	err = tmpl.Execute(&rendered, map[string]interface{}{
		"Values": Values{
			SetDocker:     setDocker,
			SetContainerd: setContainerd,
			Provider:      provider,
			Image:         image,
		},
	})
	if err != nil {
		return err
	}

	var daemonSet *applyappsv1.DaemonSetApplyConfiguration
	if err := yaml.Unmarshal(rendered.Bytes(), &daemonSet); err != nil {
		logrus.Errorf("Failed to unmarshal daemon set YAML: %v", err)
		return err
	}

	if _, err := clientset.AppsV1().DaemonSets(common.InspectionNamespace).Apply(context.TODO(), daemonSet, metav1.ApplyOptions{Force: true, FieldManager: "application/apply-patch"}); err != nil {
		logrus.Errorf("Failed to apply daemon set: %v", err)
		return err
	}

	logrus.Infof("[Agent] Daemon set configuration applied successfully")
	return nil
}

type Values struct {
	SetDocker     bool
	SetContainerd bool
	Provider      string
	Image         string
}

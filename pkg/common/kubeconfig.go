package common

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"time"
)

var (
	ClusterRes = schema.GroupVersionResource{
		Group:    "management.cattle.io",
		Version:  "v3",
		Resource: "clusters",
	}
)

type KubeConfig struct {
	BaseType string `json:"baseType"`
	Config   string `json:"config"`
	Type     string `json:"apis"`
}

func getClient() http.Client {
	return http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 20 * time.Second,
	}
}

func getAuthHeader() http.Header {
	authHeader := http.Header{}
	authHeader.Add("Authorization", "Bearer "+BearerToken)
	return authHeader
}

// GenerateKubeconfig retrieves kubeconfigs for all clusters and stores them.
func GenerateKubeconfig(clients map[string]*apis.Client) error {
	localKubernetesClien, err := GetKubernetesClient(LocalCluster)
	if err != nil {
		logrus.Errorf("Failed to get Kubernetes client for local cluster: %v", err)
		return err
	}

	clusters, err := localKubernetesClien.DynamicClient.Resource(ClusterRes).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list clusters: %v", err)
		return err
	}

	for _, c := range clusters.Items {
		clusterName := c.GetName()
		logrus.Infof("[Kubeconfig] Processing cluster: %s", clusterName)

		if !IsClusterReady(c) {
			logrus.Errorf("cluster %s is not ready", c.GetName())
			continue
		}

		kubernetesClient, err := GetKubernetesClient(clusterName)
		if err != nil {
			logrus.Errorf("Failed to get Kubernetes client for cluster %s: %v", clusterName, err)
			continue
		}

		clients[clusterName] = &apis.Client{
			DynamicClient: kubernetesClient.DynamicClient,
			Clientset:     kubernetesClient.Clientset,
			Config:        kubernetesClient.Config,
		}
	}

	return nil
}

// GetKubernetesClient retrieves a Kubernetes client for a given cluster name.
func GetKubernetesClient(name string) (*apis.Client, error) {
	err := WriteKubeconfig(name)
	if err != nil {
		logrus.Errorf("Failed to write kubeconfig for cluster %s: %v", name, err)
		return nil, err
	}

	kubernetesClient, err := GetClient(name)
	if err != nil {
		logrus.Errorf("Failed to get Kubernetes client for cluster %s: %v", name, err)
		return nil, err
	}

	return kubernetesClient, nil
}

// WriteKubeconfig generates and writes a kubeconfig for a given cluster ID.
func WriteKubeconfig(clusterID string) error {
	kubeconfigPath := WriteKubeconfigPath + clusterID
	if FileExists(kubeconfigPath) {
		logrus.Debugf("Cluster %s kubeconfig already exists at path: %s", clusterID, kubeconfigPath)
		return nil
	}

	client := getClient()
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/v3/clusters/%s?action=generateKubeconfig", ServerURL, clusterID), nil)
	if err != nil {
		logrus.Errorf("Failed to create HTTP request to generate kubeconfig for cluster %s: %v", clusterID, err)
		return err
	}
	req.Header = getAuthHeader()

	resp, err := client.Do(req)
	if err != nil {
		logrus.Errorf("Failed to execute HTTP request to generate kubeconfig for cluster %s: %v", clusterID, err)
		return err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			logrus.Errorf("Failed to close response body for cluster %s: %v", clusterID, cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Expected status 200 for kubeconfig generation request for cluster %s, got %d", clusterID, resp.StatusCode)
		logrus.Errorf(errMsg)
		return fmt.Errorf(errMsg)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("Failed to read response body for cluster %s: %v", clusterID, err)
		return err
	}

	var kubeConfig KubeConfig
	err = json.Unmarshal(body, &kubeConfig)
	if err != nil {
		logrus.Errorf("Failed to unmarshal kubeconfig for cluster %s: %v", clusterID, err)
		return err
	}

	err = WriteFile(kubeconfigPath, []byte(kubeConfig.Config))
	if err != nil {
		logrus.Errorf("Failed to write kubeconfig file for cluster %s: %v", clusterID, err)
		return err
	}

	logrus.Infof("[Kubeconfig] Successfully wrote kubeconfig for cluster %s to path: %s", clusterID, kubeconfigPath)
	return nil
}

// GetClient creates a Kubernetes client from a kubeconfig file.
func GetClient(kubeconfigPath string) (*apis.Client, error) {
	configPath := WriteKubeconfigPath + kubeconfigPath
	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		logrus.Errorf("Failed to build Kubernetes config from path %s: %v", configPath, err)
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logrus.Errorf("Failed to create dynamic client from config: %v", err)
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Errorf("Failed to create clientset from config: %v", err)
		return nil, err
	}

	logrus.Infof("[Kubeconfig] Successfully created Kubernetes client from path: %s", configPath)
	return &apis.Client{
		DynamicClient: dynamicClient,
		Clientset:     clientset,
		Config:        config,
	}, nil
}

func IsClusterReady(cluster unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(cluster.Object, "status", "conditions")
	if err != nil {
		logrus.Errorf("Error getting conditions: %v\n", err)
		return false
	}

	if !found {
		logrus.Errorf("No conditions found in the object.")
		return false
	}

	for _, condition := range conditions {
		if condMap, ok := condition.(map[string]interface{}); ok {
			if condMap["type"].(string) == "Ready" && condMap["status"].(string) == "True" {
				return true
			}
		}
	}

	return false
}

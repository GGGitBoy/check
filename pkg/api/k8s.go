package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/common"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"net/http"
)

type Resource struct {
	Nodes        []string `json:"nodes"`
	Deployments  []*Data  `json:"deployments"`
	Statefulsets []*Data  `json:"statefulsets"`
	Daemonsets   []*Data  `json:"daemonsets"`
	Jobs         []*Data  `json:"jobs"`
	Cronjobs     []*Data  `json:"cronjobs"`
}

type Data struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type Cluster struct {
	ClusterID   string `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
	Ready       bool   `json:"ready"`
}

func NewResource() *Resource {
	return &Resource{
		Nodes:        []string{},
		Deployments:  []*Data{},
		Statefulsets: []*Data{},
		Daemonsets:   []*Data{},
		Jobs:         []*Data{},
		Cronjobs:     []*Data{},
	}
}

func NewClusters() []*Cluster {
	return []*Cluster{}
}

func GetClusters() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		clusters := NewClusters()
		localKubernetesClient, err := common.GetKubernetesClient(common.LocalCluster)
		if err != nil {
			logrus.Errorf("Failed to get local Kubernetes client: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		clusterList, err := localKubernetesClient.DynamicClient.Resource(common.ClusterRes).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to list clusters: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		for _, c := range clusterList.Items {
			spec, _, err := unstructured.NestedMap(c.UnstructuredContent(), "spec")
			if err != nil {
				logrus.Errorf("Failed to get cluster spec for %s: %v", c.GetName(), err)
				continue
			}

			displayName, ok := spec["displayName"].(string)
			if !ok {
				logrus.Warnf("displayName not found for cluster %s", c.GetName())
				continue
			}

			clusters = append(clusters, &Cluster{
				ClusterID:   c.GetName(),
				ClusterName: displayName,
				Ready:       common.IsClusterReady(c),
			})
		}

		jsonData, err := json.MarshalIndent(clusters, "", "\t")
		if err != nil {
			logrus.Errorf("Failed to marshal clusters response: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		if _, err := rw.Write(jsonData); err != nil {
			logrus.Errorf("Failed to write clusters response: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
		}
	})
}

func GetResource() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		resource := NewResource()
		vars := mux.Vars(req)
		clusterID := vars["id"]

		kubernetesClient, err := common.GetKubernetesClient(clusterID)
		if err != nil {
			logrus.Errorf("Failed to get Kubernetes client for cluster %s: %v", clusterID, err)
			common.HandleError(rw, http.StatusInternalServerError, fmt.Errorf("Failed to get Kubernetes client for cluster %s: %v\n", clusterID, err))
			return
		}

		nodes, err := kubernetesClient.Clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to list nodes for cluster %s: %v", clusterID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		for _, r := range nodes.Items {
			resource.Nodes = append(resource.Nodes, r.GetName())
		}

		deployments, err := kubernetesClient.Clientset.AppsV1().Deployments("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to list deployments for cluster %s: %v", clusterID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		for _, r := range deployments.Items {
			resource.Deployments = append(resource.Deployments, &Data{
				Name:      r.GetName(),
				Namespace: r.GetNamespace(),
			})
		}

		daemonSets, err := kubernetesClient.Clientset.AppsV1().DaemonSets("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to list daemonsets for cluster %s: %v", clusterID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		for _, r := range daemonSets.Items {
			resource.Daemonsets = append(resource.Daemonsets, &Data{
				Name:      r.GetName(),
				Namespace: r.GetNamespace(),
			})
		}

		statefulSets, err := kubernetesClient.Clientset.AppsV1().StatefulSets("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to list statefulsets for cluster %s: %v", clusterID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		for _, r := range statefulSets.Items {
			resource.Statefulsets = append(resource.Statefulsets, &Data{
				Name:      r.GetName(),
				Namespace: r.GetNamespace(),
			})
		}

		jobs, err := kubernetesClient.Clientset.BatchV1().Jobs("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to list jobs for cluster %s: %v", clusterID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		for _, r := range jobs.Items {
			resource.Jobs = append(resource.Jobs, &Data{
				Name:      r.GetName(),
				Namespace: r.GetNamespace(),
			})
		}

		cronJobs, err := kubernetesClient.Clientset.BatchV1beta1().CronJobs("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to list cronjobs for cluster %s: %v", clusterID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		for _, r := range cronJobs.Items {
			resource.Cronjobs = append(resource.Cronjobs, &Data{
				Name:      r.GetName(),
				Namespace: r.GetNamespace(),
			})
		}

		jsonData, err := json.MarshalIndent(resource, "", "\t")
		if err != nil {
			logrus.Errorf("Failed to marshal resource response: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		if _, err := rw.Write(jsonData); err != nil {
			logrus.Errorf("Failed to write resource response: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
		}
	})
}

type GrafanaService struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	ClusterIP string `json:"cluster_ip"`
}

func GetGrafanaClusterIP() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		localKubernetesClient, err := common.GetKubernetesClient(common.LocalCluster)
		if err != nil {
			logrus.Errorf("Failed to get local Kubernetes client: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		service, err := localKubernetesClient.Clientset.CoreV1().Services("cattle-global-monitoring").Get(context.TODO(), "access-grafana", metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				logrus.Warnf("Grafana service not found: %v", err)
			} else {
				logrus.Errorf("Failed to get Grafana service: %v", err)
				common.HandleError(rw, http.StatusInternalServerError, err)
			}
			return
		}

		grafanaService := &GrafanaService{
			Name:      service.Name,
			Namespace: service.Namespace,
			ClusterIP: service.Spec.ClusterIP,
		}

		jsonData, err := json.MarshalIndent(grafanaService, "", "\t")
		if err != nil {
			logrus.Errorf("Failed to marshal Grafana service response: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		if _, err := rw.Write(jsonData); err != nil {
			logrus.Errorf("Failed to write Grafana service response: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
		}
	})
}

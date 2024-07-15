package api

import (
	"context"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/common"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
)

func ListAgent() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		logrus.Infof("[API] Received request to list agents")

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

		var listAgent []string
		for _, c := range clusterList.Items {
			logrus.Infof("[API] Checking agent for cluster: %s", c.GetName())

			if !common.IsClusterReady(c) {
				logrus.Errorf("cluster %s is not ready", c.GetName())
				continue
			}

			kubernetesClient, err := common.GetKubernetesClient(c.GetName())
			if err != nil {
				logrus.Errorf("Failed to get Kubernetes client for cluster %s: %v", c.GetName(), err)
				continue
			}

			_, err = kubernetesClient.Clientset.AppsV1().DaemonSets(common.InspectionNamespace).Get(context.TODO(), common.AgentName, metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					logrus.Infof("[API] Agent not found in cluster %s", c.GetName())
				} else {
					logrus.Errorf("Failed to get DaemonSet for cluster %s: %v", c.GetName(), err)
				}
				continue
			}

			logrus.Infof("[API] Agent found in cluster: %s", c.GetName())
			listAgent = append(listAgent, c.GetName())
		}

		jsonData, err := json.MarshalIndent(listAgent, "", "\t")
		if err != nil {
			logrus.Errorf("Failed to marshal agent list to JSON: %v", err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		logrus.Infof("[API] Returning agent list")
		rw.Header().Set("Content-Type", "application/json")
		rw.Write(jsonData)
	})
}

func DeleteAgent() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		clusterID := vars["id"]

		logrus.Infof("[API] Received request to delete agent for cluster: %s", clusterID)

		kubernetesClient, err := common.GetKubernetesClient(clusterID)
		if err != nil {
			logrus.Errorf("Failed to get Kubernetes client for cluster %s: %v", clusterID, err)
			common.HandleError(rw, http.StatusInternalServerError, err)
			return
		}

		err = kubernetesClient.Clientset.AppsV1().DaemonSets(common.InspectionNamespace).Delete(context.TODO(), common.AgentName, metav1.DeleteOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				logrus.Infof("[API] DaemonSet not found in cluster %s", clusterID)
			} else {
				logrus.Errorf("Failed to delete DaemonSet for cluster %s: %v", clusterID, err)
				common.HandleError(rw, http.StatusInternalServerError, err)
				return
			}
		}

		err = kubernetesClient.Clientset.CoreV1().ConfigMaps(common.InspectionNamespace).Delete(context.TODO(), common.AgentScriptName, metav1.DeleteOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				logrus.Infof("[API] ConfigMap not found in cluster %s", clusterID)
			} else {
				logrus.Errorf("Failed to delete ConfigMap for cluster %s: %v", clusterID, err)
				common.HandleError(rw, http.StatusInternalServerError, err)
				return
			}
		}

		err = kubernetesClient.Clientset.RbacV1().ClusterRoleBindings().Delete(context.TODO(), common.AgentName, metav1.DeleteOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				logrus.Infof("[API] ClusterRoleBinding not found in cluster %s", clusterID)
			} else {
				logrus.Errorf("Failed to delete ClusterRoleBinding for cluster %s: %v", clusterID, err)
				common.HandleError(rw, http.StatusInternalServerError, err)
				return
			}
		}

		err = kubernetesClient.Clientset.CoreV1().ServiceAccounts(common.InspectionNamespace).Delete(context.TODO(), common.AgentName, metav1.DeleteOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				logrus.Infof("[API] ServiceAccount not found in cluster %s", clusterID)
			} else {
				logrus.Errorf("Failed to delete ServiceAccount for cluster %s: %v", clusterID, err)
				common.HandleError(rw, http.StatusInternalServerError, err)
				return
			}
		}

		logrus.Infof("[API] Successfully deleted agent for cluster: %s", clusterID)
		rw.Write([]byte("删除完成"))
	})
}

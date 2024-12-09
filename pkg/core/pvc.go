package core

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"strings"
)

func GetPVC(client *apis.Client, pvcConfig *apis.PVCConfig, taskName string, pvcItem map[string][]*apis.Item) ([]*apis.PVC, error) {
	logrus.Infof("[%s] Starting pvc inspection", taskName)

	pvcs := apis.NewPVCs()

	selectorNamespaces := []string{""}
	if pvcConfig.SelectorNamespace != "" {
		selectorNamespaces = strings.Split(pvcConfig.SelectorNamespace, ",")
	}

	listOptions := metav1.ListOptions{}
	if pvcConfig.SelectorLabels != nil && len(pvcConfig.SelectorLabels) > 0 {
		var set labels.Set
		set = pvcConfig.SelectorLabels
		listOptions = metav1.ListOptions{LabelSelector: set.AsSelector().String()}
	}

	var allPVC []corev1.PersistentVolumeClaim
	for _, ns := range selectorNamespaces {
		pvcList, err := client.Clientset.CoreV1().PersistentVolumeClaims(ns).List(context.TODO(), listOptions)
		if err != nil {
			return nil, fmt.Errorf("Error listing pvc: %v\n", err)
		}

		allPVC = append(allPVC, pvcList.Items...)
	}

	for _, p := range allPVC {
		var items []*apis.Item
		grafanaItem, ok := pvcItem[p.Namespace+"/"+p.Name]
		if ok {
			items = append(items, grafanaItem...)
		}

		itemsCount := GetItemsCount(items)

		pvcs = append(pvcs, &apis.PVC{
			Name:       p.Name,
			Namespace:  p.Namespace,
			Items:      items,
			ItemsCount: itemsCount,
		})
	}

	logrus.Infof("[%s] Completed getting pvc", taskName)
	return pvcs, nil
}

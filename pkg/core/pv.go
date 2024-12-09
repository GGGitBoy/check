package core

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func GetPV(client *apis.Client, pvConfig *apis.PVConfig, taskName string, pvItem map[string][]*apis.Item) ([]*apis.PV, error) {
	logrus.Infof("[%s] Starting pv inspection", taskName)

	pvs := apis.NewPVs()

	listOptions := metav1.ListOptions{}
	if pvConfig.SelectorLabels != nil && len(pvConfig.SelectorLabels) > 0 {
		var set labels.Set
		set = pvConfig.SelectorLabels
		listOptions = metav1.ListOptions{LabelSelector: set.AsSelector().String()}
	}

	pvList, err := client.Clientset.CoreV1().PersistentVolumes().List(context.TODO(), listOptions)
	if err != nil {
		return nil, fmt.Errorf("Error listing pv: %v\n", err)
	}

	for _, p := range pvList.Items {
		var items []*apis.Item
		grafanaItem, ok := pvItem[p.Name]
		if ok {
			items = append(items, grafanaItem...)
		}

		itemsCount := GetItemsCount(items)

		pvs = append(pvs, &apis.PV{
			Name:       p.Name,
			Items:      items,
			ItemsCount: itemsCount,
		})
	}

	logrus.Infof("[%s] Completed getting pv", taskName)
	return pvs, nil
}

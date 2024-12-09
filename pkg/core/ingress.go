package core

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"strings"
)

func GetIngress(client *apis.Client, ingressConfig *apis.IngressConfig, taskName string, ingressItem map[string][]*apis.Item) ([]*apis.Ingress, error) {
	logrus.Infof("[%s] Starting ingresses inspection", taskName)

	ingress := apis.NewIngress()

	selectorNamespaces := []string{""}
	if ingressConfig.SelectorNamespace != "" {
		selectorNamespaces = strings.Split(ingressConfig.SelectorNamespace, ",")
	}

	listOptions := metav1.ListOptions{}
	if ingressConfig.SelectorLabels != nil && len(ingressConfig.SelectorLabels) > 0 {
		var set labels.Set
		set = ingressConfig.SelectorLabels
		listOptions = metav1.ListOptions{LabelSelector: set.AsSelector().String()}
	}

	var allIngress []networkingv1.Ingress
	for _, ns := range selectorNamespaces {
		ingressList, err := client.Clientset.NetworkingV1().Ingresses(ns).List(context.TODO(), listOptions)
		if err != nil {
			return nil, fmt.Errorf("Error listing pvc: %v\n", err)
		}

		allIngress = append(allIngress, ingressList.Items...)
	}

	ingressMap := make(map[string][]string)
	for _, i := range allIngress {
		for _, rule := range i.Spec.Rules {
			host := rule.Host
			if rule.HTTP != nil {
				for _, path := range rule.HTTP.Paths {
					key := host + path.Path
					ingressMap[key] = append(ingressMap[key], fmt.Sprintf("%s/%s", i.Namespace, i.Name))
				}
			}
		}

		ingressItem[fmt.Sprintf("%s/%s", i.Namespace, i.Name)] = append(ingressItem[fmt.Sprintf("%s/%s", i.Namespace, i.Name)], &apis.Item{
			Name:    "不存在重复的 Path 路径",
			Pass:    true,
			Message: "",
		})
	}

	for _, ingressNames := range ingressMap {
		if len(ingressNames) > 1 {
			result := strings.Join(ingressNames, ",")
			for _, ingressName := range ingressNames {
				ingressItem[ingressName][0] = &apis.Item{
					Name:    "不存在重复的 Path 路径",
					Pass:    false,
					Message: fmt.Sprintf("Ingress %s 存在重复的 Path 路径", result),
				}
			}
		}
	}

	for _, i := range allIngress {
		var items []*apis.Item
		grafanaItem, ok := ingressItem[i.Namespace+"/"+i.Name]
		if ok {
			items = append(items, grafanaItem...)
		}

		itemsCount := GetItemsCount(items)

		ingress = append(ingress, &apis.Ingress{
			Name:       i.Name,
			Namespace:  i.Namespace,
			Items:      items,
			ItemsCount: itemsCount,
		})
	}

	logrus.Infof("[%s] Completed getting ingresses", taskName)
	return ingress, nil
}

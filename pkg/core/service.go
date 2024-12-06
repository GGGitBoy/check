package core

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"strings"
)

func GetServices(client *apis.Client, serviceConfig *apis.ServiceConfig, taskName string, serviceItem map[string][]*apis.Item) ([]*apis.Service, []*apis.Inspection, error) {
	logrus.Infof("[%s] Starting services inspection", taskName)

	resourceInspections := apis.NewInspections()
	services := apis.NewServices()

	selectorNamespaces := []string{""}
	if serviceConfig.SelectorNamespace != "" {
		selectorNamespaces = strings.Split(serviceConfig.SelectorNamespace, ",")
	}

	listOptions := metav1.ListOptions{}
	if serviceConfig.SelectorLabels != nil && len(serviceConfig.SelectorLabels) > 0 {
		var set labels.Set
		set = serviceConfig.SelectorLabels
		listOptions = metav1.ListOptions{LabelSelector: set.AsSelector().String()}
	}

	var allService []corev1.Service
	for _, ns := range selectorNamespaces {
		serviceList, err := client.Clientset.CoreV1().Services(ns).List(context.TODO(), listOptions)
		if err != nil {
			return nil, nil, fmt.Errorf("Error listing pvc: %v\n", err)
		}

		allService = append(allService, serviceList.Items...)
	}

	for _, s := range allService {
		logrus.Debugf("Processing service: %s/%s", s.Namespace, s.Name)
		emptyEndpoints := true
		emptyEndpointsMessage := ""
		endpoints, err := client.Clientset.CoreV1().Endpoints(s.Namespace).Get(context.TODO(), s.Name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				emptyEndpoints = false
				emptyEndpointsMessage = fmt.Sprintf("命名空间 %s 下 Service %s 找不到对应 endpoint", s.Namespace, s.Name)
				logrus.Warnf("Service %s/%s does not have corresponding endpoints", s.Namespace, s.Name)
				resourceInspections = append(resourceInspections, apis.NewInspection(
					fmt.Sprintf("命名空间 %s 下 Service %s 找不到对应 endpoint", s.Namespace, s.Name),
					"对应的 Endpoints 未找到",
					1,
					[]string{},
				))

				services = append(services, &apis.Service{
					Name:      s.Name,
					Namespace: s.Namespace,
					Items: []*apis.Item{
						{
							Name:    "存在对应 Endpoints 且 Subsets 非空",
							Message: emptyEndpointsMessage,
							Pass:    emptyEndpoints,
						},
					},
				})

				continue
			}
		}

		if len(endpoints.Subsets) == 0 {
			emptyEndpoints = false
			emptyEndpointsMessage = fmt.Sprintf("命名空间 %s 下 Service %s 对应 Endpoints 没有 Subsets", s.Namespace, s.Name)
			resourceInspections = append(resourceInspections, apis.NewInspection(
				fmt.Sprintf("命名空间 %s 下 Service %s 对应 Endpoints 没有 Subsets", s.Namespace, s.Name),
				"对应的 Endpoints 没有 Subsets",
				1,
				[]string{},
			))
		}

		items := []*apis.Item{
			{
				Name:    "存在对应 Endpoints 且 Subsets 非空",
				Message: emptyEndpointsMessage,
				Pass:    emptyEndpoints,
			},
		}

		grafanaItem, ok := serviceItem[s.Namespace+"/"+s.Name]
		if ok {
			items = append(items, grafanaItem...)
		}

		services = append(services, &apis.Service{
			Name:      s.Name,
			Namespace: s.Namespace,
			Items:     items,
		})
	}

	logrus.Infof("[%s] Completed getting services", taskName)
	return services, resourceInspections, nil
}

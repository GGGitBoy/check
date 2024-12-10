package core

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	"inspection-server/pkg/common"
	"inspection-server/pkg/db"
	pdfPrint "inspection-server/pkg/print"
	"inspection-server/pkg/send"
	"strings"
	"time"
)

var (
	inspectionFailed = "巡检失败"
	printFailed      = "打印失败"
	notifyFailed     = "通知失败"
)

func Inspection(task *apis.Task) (*apis.Task, string, error) {
	task.State = "巡检中"
	logrus.Infof("[%s] Starting inspection for task ID: %s", task.Name, task.ID)

	err := db.UpdateTask(task)
	if err != nil {
		return task, inspectionFailed, fmt.Errorf("Failed to update task state to '巡检中' for task ID %s: %v\n", task.ID, err)
	}

	template, err := db.GetTemplate(task.TemplateID)
	if err != nil {
		return task, inspectionFailed, fmt.Errorf("Failed to get template for task ID %s: %v\n", task.ID, err)
	}

	clients := apis.NewClients()
	err = common.GenerateKubeconfig(clients)
	if err != nil {
		return task, inspectionFailed, fmt.Errorf("Failed to generate kubeconfig: %v\n", err)
	}

	report := apis.NewReport()
	kubernetes := apis.NewKubernetes()

	allGrafanaItems, err := GetAllGrafanaItems(task.Name)
	if err != nil {
		logrus.Errorf("Failed to get all Grafana items: %v", err)
	}

	level := 0
	var sendMessageDetail []string
	for _, k := range template.KubernetesConfig {
		if k.Enable {
			client, ok := clients[k.ClusterID]

			clusterCore := apis.NewClusterCore()
			clusterNode := apis.NewClusterNode()
			clusterResource := apis.NewClusterResource()

			allCoreInspections := apis.NewInspections()
			allNodeInspections := apis.NewInspections()
			allResourceInspections := apis.NewInspections()

			grafanaItems := NewGrafanaItem()
			if allGrafanaItems != nil && allGrafanaItems[k.ClusterName] != nil {
				grafanaItems = allGrafanaItems[k.ClusterName]
			}

			if ok {
				sendMessageDetail = append(sendMessageDetail, fmt.Sprintf("集群 %s 巡检警告：", k.ClusterName))
				logrus.Infof("[%s] Processing inspections for cluster: %s", task.Name, k.ClusterName)

				// cluster
				clusterCore, err = GetClusterCore(client, k.ClusterCoreConfig, k.ClusterName, task.Name, grafanaItems.ClusterCoreItem.ClusterItem)
				if err != nil {
					return task, inspectionFailed, fmt.Errorf("Failed to get health check for cluster %s: %v\n", k.ClusterID, err)
				}

				//coreInspections = append(coreInspections, coreInspectionArray...)

				// node
				NodeNodeArray, err := GetNodes(client, k.ClusterNodeConfig, task.Name, grafanaItems.ClusterNodeItem.NodeItem)
				if err != nil {
					return task, inspectionFailed, fmt.Errorf("Failed to get nodes for cluster %s: %v\n", k.ClusterID, err)
				}
				//nodeInspections = append(nodeInspections, nodeInspectionArray...)

				// workload
				ResourceWorkloadArray, err := GetWorkloads(client, k.ClusterResourceConfig.WorkloadConfig, task.Name, grafanaItems.ClusterResourceItem)
				if err != nil {
					return task, inspectionFailed, fmt.Errorf("Failed to get workloads for cluster %s: %v\n", k.ClusterID, err)
				}
				//resourceInspections = append(resourceInspections, resourceInspectionArray...)

				// namespace
				if k.ClusterResourceConfig.NamespaceConfig.Enable {
					ResourceNamespaceArray, err := GetNamespaces(client, k.ClusterResourceConfig.NamespaceConfig, task.Name, grafanaItems.ClusterResourceItem.NamespaceItem)
					if err != nil {
						return task, inspectionFailed, fmt.Errorf("Failed to get namespaces for cluster %s: %v\n", k.ClusterID, err)
					}

					clusterResource.Namespace = ResourceNamespaceArray
					//resourceInspections = append(resourceInspections, resourceInspectionArray...)
				}

				// service
				if k.ClusterResourceConfig.ServiceConfig.Enable {
					ResourceServiceArray, err := GetServices(client, k.ClusterResourceConfig.ServiceConfig, task.Name, grafanaItems.ClusterResourceItem.ServiceItems)
					if err != nil {
						return task, inspectionFailed, fmt.Errorf("Failed to get services for cluster %s: %v\n", k.ClusterID, err)
					}

					clusterResource.Service = ResourceServiceArray
					//resourceInspections = append(resourceInspections, resourceInspectionArray...)
				}

				// ingress
				if k.ClusterResourceConfig.IngressConfig.Enable {
					ResourceIngressArray, err := GetIngress(client, k.ClusterResourceConfig.IngressConfig, task.Name, grafanaItems.ClusterResourceItem.IngressItems)
					if err != nil {
						return task, inspectionFailed, fmt.Errorf("Failed to get ingress for cluster %s: %v\n", k.ClusterID, err)
					}

					clusterResource.Ingress = ResourceIngressArray
					//resourceInspections = append(resourceInspections, resourceInspectionArray...)
				}

				// pvc
				if k.ClusterResourceConfig.PVCConfig.Enable {
					ResourcePVCArray, err := GetPVC(client, k.ClusterResourceConfig.PVCConfig, task.Name, grafanaItems.ClusterResourceItem.PVCItems)
					if err != nil {
						return task, inspectionFailed, fmt.Errorf("Failed to get pvc for cluster %s: %v\n", k.ClusterID, err)
					}

					clusterResource.PVC = ResourcePVCArray
					//resourceInspections = append(resourceInspections, resourceInspectionArray...)
				}

				// pv
				if k.ClusterResourceConfig.PVConfig.Enable {
					ResourcePVArray, err := GetPV(client, k.ClusterResourceConfig.PVConfig, task.Name, grafanaItems.ClusterResourceItem.PVItems)
					if err != nil {
						return task, inspectionFailed, fmt.Errorf("Failed to get pv for cluster %s: %v\n", k.ClusterID, err)
					}

					clusterResource.PV = ResourcePVArray
					//resourceInspections = append(resourceInspections, resourceInspectionArray...)
				}

				clusterNode.Nodes = NodeNodeArray
				clusterResource.Workloads = ResourceWorkloadArray
			} else {
				allCoreInspections = append(allCoreInspections, apis.NewInspection(fmt.Sprintf("cluster %s is not ready", k.ClusterID), 3, []string{}))
			}

			//if allGrafanaInspections != nil && allGrafanaInspections[k.ClusterName] != nil {
			//	if len(allGrafanaInspections[k.ClusterName].ClusterCoreInspection) > 0 {
			//		coreInspections = append(coreInspections, allGrafanaInspections[k.ClusterName].ClusterCoreInspection...)
			//	}
			//
			//	if len(allGrafanaInspections[k.ClusterName].ClusterNodeInspection) > 0 {
			//		nodeInspections = append(nodeInspections, allGrafanaInspections[k.ClusterName].ClusterNodeInspection...)
			//	}
			//
			//	if len(allGrafanaInspections[k.ClusterName].ClusterResourceInspection) > 0 {
			//		resourceInspections = append(resourceInspections, allGrafanaInspections[k.ClusterName].ClusterResourceInspection...)
			//	}
			//}

			coreInspections := GetCoreInspections(clusterCore.Items)
			allCoreInspections = append(allCoreInspections, coreInspections...)

			nodeInspections := GetNodeInspections(clusterNode.Nodes)
			allNodeInspections = append(allResourceInspections, nodeInspections...)

			workloadInspections := GetWorkloadInspections(clusterResource.Workloads)
			serviceInspections := GetServiceInspections(clusterResource.Service)
			ingressInspections := GetIngressInspections(clusterResource.Ingress)
			pvcInspections := GetPVCInspections(clusterResource.PVC)
			NamespaceInspections := GetNamespaceInspections(clusterResource.Namespace)
			pvInspections := GetPVInspections(clusterResource.PV)
			allResourceInspections = append(allResourceInspections, workloadInspections...)
			allResourceInspections = append(allResourceInspections, serviceInspections...)
			allResourceInspections = append(allResourceInspections, ingressInspections...)
			allResourceInspections = append(allResourceInspections, pvcInspections...)
			allResourceInspections = append(allResourceInspections, NamespaceInspections...)
			allResourceInspections = append(allResourceInspections, pvInspections...)

			clusterCore.Inspections = allCoreInspections
			clusterNode.Inspections = allNodeInspections
			clusterResource.Inspections = allResourceInspections

			for _, c := range allCoreInspections {
				if c.Level > level {
					level = c.Level
				}
				if c.Level >= 2 {
					sendMessageDetail = append(sendMessageDetail, fmt.Sprintf("%s", c.Title))
				}
			}
			for _, n := range allNodeInspections {
				if n.Level > level {
					level = n.Level
				}
				if n.Level >= 2 {
					sendMessageDetail = append(sendMessageDetail, fmt.Sprintf("%s", n.Title))
				}
			}
			for _, r := range allResourceInspections {
				if r.Level > level {
					level = r.Level
				}
				if r.Level >= 2 {
					sendMessageDetail = append(sendMessageDetail, fmt.Sprintf("%s", r.Title))
				}
			}

			kubernetes = append(kubernetes, &apis.Kubernetes{
				ClusterID:       k.ClusterID,
				ClusterName:     k.ClusterName,
				ClusterCore:     clusterCore,
				ClusterNode:     clusterNode,
				ClusterResource: clusterResource,
			})
		}
	}

	var rating string
	switch level {
	case 0:
		rating = "优"
	case 1:
		rating = "高"
	case 2:
		rating = "中"
	case 3:
		rating = "低"
	default:
		rating = "未知"
	}

	report = &apis.Report{
		ID: common.GetUUID(),
		Global: &apis.Global{
			Name:       task.Name,
			Rating:     rating,
			ReportTime: time.Now().Format("2006-01-02 15:04:05"),
		},
		Kubernetes: kubernetes,
	}

	err = db.CreateReport(report)
	if err != nil {
		return task, inspectionFailed, fmt.Errorf("Failed to create report: %v\n", err)
	}
	task.Rating = report.Global.Rating
	task.ReportID = report.ID

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("该巡检报告的健康等级为: %s\n", report.Global.Rating))

	for _, s := range sendMessageDetail {
		sb.WriteString(fmt.Sprintf("%s\n", s))
	}

	p := pdfPrint.NewPrint()
	p.URL = "http://127.0.0.1/#/inspection/result-pdf-view/" + report.ID
	p.ReportTime = report.Global.ReportTime

	if common.PrintPDFEnable == "true" {
		err = pdfPrint.FullScreenshot(p, task.Name)
		if err != nil {
			return task, printFailed, fmt.Errorf("Failed to take screenshot for report ID %s: %v\n", report.ID, err)
		}
	}

	if task.NotifyID != "" {
		notify, err := db.GetNotify(task.NotifyID)
		if err != nil {
			return task, notifyFailed, fmt.Errorf("Failed to get notification details for NotifyID %s: %v\n", task.NotifyID, err)
		}

		sb.WriteString(fmt.Sprintf("该巡检报告的访问地址为: %s/api/v1/namespaces/cattle-inspection-system/services/http:access-inspection:80/proxy/#/inspection/result/%s\n", common.ServerURL, report.ID))
		if notify.WebhookURL != "" && notify.Secret != "" {
			err = send.Webhook(notify.WebhookURL, notify.Secret, sb.String(), task.Name)
			if err != nil {
				return task, notifyFailed, fmt.Errorf("Failed to send notification: %v\n", err)
			}
		} else if notify.AppID != "" && notify.AppSecret != "" && (notify.Mobiles != "" || notify.Emails != "") {
			err = send.NotifyUser(notify.AppID, notify.AppSecret, notify.Mobiles, notify.Emails, sb.String(), task.Name)
			if err != nil {
				return task, notifyFailed, fmt.Errorf("Failed to send notification: %v\n", err)
			}
		} else {
			err = send.NotifyChat(notify.AppID, notify.AppSecret, common.GetReportFileName(p.ReportTime), common.PrintPDFPath+common.GetReportFileName(p.ReportTime), sb.String(), task.Name)
			if err != nil {
				return task, notifyFailed, fmt.Errorf("Failed to send notification: %v\n", err)
			}
		}
	}

	task.EndTime = time.Now().Format("2006-01-02 15:04:05")
	task.State = "巡检完成"
	logrus.Infof("[%s] Inspection completed for task ID: %s", task.Name, task.ID)
	return task, "巡检完成", nil
}

func GetWorkloadInspections(workloadDatas *apis.Workload) []*apis.Inspection {
	inspectionMap := make(map[string]*apis.Inspection)

	for _, s := range workloadDatas.Deployment {
		for _, i := range s.Items {
			if !i.Pass {
				if inspectionMap[i.Name] == nil {
					inspectionMap[i.Name] = &apis.Inspection{
						Title: i.Name,
						Level: i.Level,
					}
				}

				inspectionMap[i.Name].Names = append(inspectionMap[i.Name].Names, fmt.Sprintf("Deployment: %s / %s\n", s.Namespace, s.Name))
			}
		}
	}

	for _, s := range workloadDatas.Statefulset {
		for _, i := range s.Items {
			if !i.Pass {
				if inspectionMap[i.Name] == nil {
					inspectionMap[i.Name] = &apis.Inspection{
						Title: i.Name,
						Level: i.Level,
					}
				}

				inspectionMap[i.Name].Names = append(inspectionMap[i.Name].Names, fmt.Sprintf("Statefulset: %s / %s\n", s.Namespace, s.Name))
			}
		}
	}

	for _, s := range workloadDatas.Deployment {
		for _, i := range s.Items {
			if !i.Pass {
				if inspectionMap[i.Name] == nil {
					inspectionMap[i.Name] = &apis.Inspection{
						Title: i.Name,
						Level: i.Level,
					}
				}

				inspectionMap[i.Name].Names = append(inspectionMap[i.Name].Names, fmt.Sprintf("Daemonset: %s / %s\n", s.Namespace, s.Name))
			}
		}
	}

	for _, s := range workloadDatas.Job {
		for _, i := range s.Items {
			if !i.Pass {
				if inspectionMap[i.Name] == nil {
					inspectionMap[i.Name] = &apis.Inspection{
						Title: i.Name,
						Level: i.Level,
					}
				}

				inspectionMap[i.Name].Names = append(inspectionMap[i.Name].Names, fmt.Sprintf("Job: %s / %s\n", s.Namespace, s.Name))
			}
		}
	}

	for _, s := range workloadDatas.Cronjob {
		for _, i := range s.Items {
			if !i.Pass {
				if inspectionMap[i.Name] == nil {
					inspectionMap[i.Name] = &apis.Inspection{
						Title: i.Name,
						Level: i.Level,
					}
				}

				inspectionMap[i.Name].Names = append(inspectionMap[i.Name].Names, fmt.Sprintf("Cronjob: %s / %s\n", s.Namespace, s.Name))
			}
		}
	}

	var inspections []*apis.Inspection
	for _, i := range inspectionMap {
		inspections = append(inspections, i)
	}

	return inspections
}

func GetServiceInspections(datas []*apis.Service) []*apis.Inspection {
	inspectionMap := make(map[string]*apis.Inspection)
	for _, d := range datas {
		for _, i := range d.Items {
			if !i.Pass {
				if inspectionMap[i.Name] == nil {
					inspectionMap[i.Name] = &apis.Inspection{
						Title: i.Name,
						Level: i.Level,
					}
				}

				inspectionMap[i.Name].Names = append(inspectionMap[i.Name].Names, fmt.Sprintf("%s / %s\n", d.Namespace, d.Name))
			}
		}
	}

	var inspections []*apis.Inspection
	for _, i := range inspectionMap {
		inspections = append(inspections, i)
	}

	return inspections
}

func GetIngressInspections(datas []*apis.Ingress) []*apis.Inspection {
	inspectionMap := make(map[string]*apis.Inspection)
	for _, d := range datas {
		for _, i := range d.Items {
			if !i.Pass {
				if inspectionMap[i.Name] == nil {
					inspectionMap[i.Name] = &apis.Inspection{
						Title: i.Name,
						Level: i.Level,
					}
				}

				inspectionMap[i.Name].Names = append(inspectionMap[i.Name].Names, fmt.Sprintf("%s / %s\n", d.Namespace, d.Name))
			}
		}
	}

	var inspections []*apis.Inspection
	for _, i := range inspectionMap {
		inspections = append(inspections, i)
	}

	return inspections
}

func GetPVCInspections(datas []*apis.PVC) []*apis.Inspection {
	inspectionMap := make(map[string]*apis.Inspection)
	for _, d := range datas {
		for _, i := range d.Items {
			if !i.Pass {
				if inspectionMap[i.Name] == nil {
					inspectionMap[i.Name] = &apis.Inspection{
						Title: i.Name,
						Level: i.Level,
					}
				}

				inspectionMap[i.Name].Names = append(inspectionMap[i.Name].Names, fmt.Sprintf("%s / %s\n", d.Namespace, d.Name))
			}
		}
	}

	var inspections []*apis.Inspection
	for _, i := range inspectionMap {
		inspections = append(inspections, i)
	}

	return inspections
}

func GetPVInspections(datas []*apis.PV) []*apis.Inspection {
	inspectionMap := make(map[string]*apis.Inspection)
	for _, d := range datas {
		for _, i := range d.Items {
			if !i.Pass {
				if inspectionMap[i.Name] == nil {
					inspectionMap[i.Name] = &apis.Inspection{
						Title: i.Name,
						Level: i.Level,
					}
				}

				inspectionMap[i.Name].Names = append(inspectionMap[i.Name].Names, fmt.Sprintf("%s\n", d.Name))
			}
		}
	}

	var inspections []*apis.Inspection
	for _, i := range inspectionMap {
		inspections = append(inspections, i)
	}

	return inspections
}

func GetNamespaceInspections(datas []*apis.Namespace) []*apis.Inspection {
	inspectionMap := make(map[string]*apis.Inspection)
	for _, d := range datas {
		for _, i := range d.Items {
			if !i.Pass {
				if inspectionMap[i.Name] == nil {
					inspectionMap[i.Name] = &apis.Inspection{
						Title: i.Name,
						Level: i.Level,
					}
				}

				inspectionMap[i.Name].Names = append(inspectionMap[i.Name].Names, fmt.Sprintf("%s\n", d.Name))
			}
		}
	}

	var inspections []*apis.Inspection
	for _, i := range inspectionMap {
		inspections = append(inspections, i)
	}

	return inspections
}

func GetNodeInspections(datas []*apis.Node) []*apis.Inspection {
	inspectionMap := make(map[string]*apis.Inspection)
	for _, d := range datas {
		for _, i := range d.Items {
			if !i.Pass {
				if inspectionMap[i.Name] == nil {
					inspectionMap[i.Name] = &apis.Inspection{
						Title: i.Name,
						Level: i.Level,
					}
				}

				inspectionMap[i.Name].Names = append(inspectionMap[i.Name].Names, fmt.Sprintf("%s / %s\n", d.Name, d.HostIP))
			}
		}
	}

	var inspections []*apis.Inspection
	for _, i := range inspectionMap {
		inspections = append(inspections, i)
	}

	return inspections
}

func GetCoreInspections(items []*apis.Item) []*apis.Inspection {
	inspectionMap := make(map[string]*apis.Inspection)
	for _, i := range items {
		if !i.Pass {
			if inspectionMap[i.Name] == nil {
				inspectionMap[i.Name] = &apis.Inspection{
					Title: i.Name,
					Level: i.Level,
				}
			}
		}
	}

	var inspections []*apis.Inspection
	for _, i := range inspectionMap {
		inspections = append(inspections, i)
	}

	return inspections
}

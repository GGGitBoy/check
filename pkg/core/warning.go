package core

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"inspection-server/pkg/apis"
	"inspection-server/pkg/common"
	"io"
	"net/http"
	"strings"
	"time"
)

type Alerting struct {
	Data *Data `json:"data"`
}

type Data struct {
	RuleGroups []RuleGroup      `json:"groups"`
	Totals     map[string]int64 `json:"totals,omitempty"`
}

// swagger:model
type RuleGroup struct {
	// required: true
	Name string `json:"name"`
	// required: true
	File string `json:"file"`
	// In order to preserve rule ordering, while exposing type (alerting or tasking)
	// specific properties, both alerting and tasking rules are exposed in the
	// same array.
	// required: true
	Rules  []AlertingRule   `json:"rules"`
	Totals map[string]int64 `json:"totals"`
	// required: true
	Interval       float64   `json:"interval"`
	LastEvaluation time.Time `json:"lastEvaluation"`
	EvaluationTime float64   `json:"evaluationTime"`
}

type AlertingRule struct {
	// State can be "pending", "firing", "inactive".
	// required: true
	State string `json:"state,omitempty"`
	// required: true
	Name string `json:"name,omitempty"`
	// required: true
	Query    string  `json:"query,omitempty"`
	Duration float64 `json:"duration,omitempty"`
	// required: true
	Annotations map[string]string `json:"annotations,omitempty"`

	// required: true
	ActiveAt       *time.Time       `json:"activeAt,omitempty"`
	Alerts         []Alert          `json:"alerts,omitempty"`
	Totals         map[string]int64 `json:"totals,omitempty"`
	TotalsFiltered map[string]int64 `json:"totalsFiltered,omitempty"`
	Rule
}

type Alert struct {
	// required: true
	Labels map[string]string `json:"labels"`
	// required: true
	Annotations map[string]string `json:"annotations"`
	// required: true
	State    string     `json:"state"`
	ActiveAt *time.Time `json:"activeAt"`
	// required: true
	Value string `json:"value"`
}

type Rule struct {
	// required: true
	Name string `json:"name"`
	// required: true
	Query  string            `json:"query"`
	Labels map[string]string `json:"labels,omitempty"`
	// required: true
	Health    string `json:"health"`
	LastError string `json:"lastError,omitempty"`
	// required: true
	Type           string    `json:"type"`
	LastEvaluation time.Time `json:"lastEvaluation"`
	EvaluationTime float64   `json:"evaluationTime"`
}

type AllGrafanaInspection struct {
	GrafanaInspections map[string]*GrafanaInspection `json:"grafana_inspections"`
}

type GrafanaInspection struct {
	ClusterCoreInspection     []*apis.Inspection `json:"cluster_core_inspection"`
	ClusterNodeInspection     []*apis.Inspection `json:"cluster_node_inspection"`
	ClusterResourceInspection []*apis.Inspection `json:"cluster_resource_inspection"`
}

type AllGrafanaItems struct {
	GrafanaItems map[string]*GrafanaItem `json:"grafana_items"`
}

type GrafanaItem struct {
	ClusterCoreItem     *ClusterCoreItem     `json:"cluster_core_item"`
	ClusterNodeItem     *ClusterNodeItem     `json:"cluster_node_item"`
	ClusterResourceItem *ClusterResourceItem `json:"cluster_resource_item"`
}

type ClusterCoreItem struct {
	ClusterItem map[string][]*apis.Item `json:"cluster_item"`
}

type ClusterNodeItem struct {
	NodeItem map[string][]*apis.Item `json:"node_item"`
}

type ClusterResourceItem struct {
	DeploymentItem  map[string][]*apis.Item `json:"deployment_item"`
	StatefulsetItem map[string][]*apis.Item `json:"statefulset_item"`
	DaemonsetItem   map[string][]*apis.Item `json:"daemonset_item"`
	JobItem         map[string][]*apis.Item `json:"job_item"`
	CronjobItem     map[string][]*apis.Item `json:"cronjob_item"`
	NamespaceItem   map[string][]*apis.Item `json:"namespace_item"`
	ServiceItems    map[string][]*apis.Item `json:"service_items"`
	IngressItems    map[string][]*apis.Item `json:"ingress_items"`
	PVCItems        map[string][]*apis.Item `json:"pvc_items"`
	PVItems         map[string][]*apis.Item `json:"pv_items"`
}

type Item struct {
	Name    string `json:"name"`
	Pass    string `json:"pass"`
	Message string `json:"message"`
}

func NewAllGrafanaItems() map[string]*GrafanaItem {
	return make(map[string]*GrafanaItem)
}

func NewGrafanaItem() *GrafanaItem {
	return &GrafanaItem{
		ClusterCoreItem: &ClusterCoreItem{
			ClusterItem: make(map[string][]*apis.Item),
		},
		ClusterNodeItem: &ClusterNodeItem{
			NodeItem: make(map[string][]*apis.Item),
		},
		ClusterResourceItem: &ClusterResourceItem{
			DeploymentItem:  make(map[string][]*apis.Item),
			StatefulsetItem: make(map[string][]*apis.Item),
			DaemonsetItem:   make(map[string][]*apis.Item),
			JobItem:         make(map[string][]*apis.Item),
			CronjobItem:     make(map[string][]*apis.Item),
			NamespaceItem:   make(map[string][]*apis.Item),
			ServiceItems:    make(map[string][]*apis.Item),
			IngressItems:    make(map[string][]*apis.Item),
			PVCItems:        make(map[string][]*apis.Item),
			PVItems:         make(map[string][]*apis.Item),
		},
	}
}

func NewAllGrafanaInspection() map[string]*GrafanaInspection {
	return make(map[string]*GrafanaInspection)
}

func NewGrafanaInspection() *GrafanaInspection {
	return &GrafanaInspection{
		ClusterCoreInspection:     []*apis.Inspection{},
		ClusterNodeInspection:     []*apis.Inspection{},
		ClusterResourceInspection: []*apis.Inspection{},
	}
}

func NewAlerting() *Alerting {
	return &Alerting{}
}

func GetAllGrafanaItems(taskName string) (map[string]*GrafanaItem, error) {
	logrus.Infof("[%s] Starting to get all Grafana items", taskName)

	allGrafanaItems := NewAllGrafanaItems()
	alerting, err := GetAlerting()
	if err != nil {
		return nil, fmt.Errorf("Error getting alerting data: %v\n", err)
	}

	if alerting == nil || alerting.Data == nil || len(alerting.Data.RuleGroups) == 0 {
		return nil, fmt.Errorf("alerting rule is empty: %v", err)
	}

	for _, group := range alerting.Data.RuleGroups {
		for _, rule := range group.Rules {
			for _, alert := range rule.Alerts {
				prometheusFrom, ok := alert.Labels["prometheus_from"]
				if !ok {
					logrus.Errorf("Alert %s missing 'prometheus_from' label", rule.Name)
					continue
				}

				alertname, ok := alert.Labels["alertname"]
				if !ok {
					logrus.Errorf("Alert %s missing 'alertname' label", rule.Name)
					continue
				}

				summary, ok := alert.Annotations["summary"]
				if !ok {
					logrus.Errorf("Alert %s missing 'summary' annotation", rule.Name)
					continue
				}

				var pass bool
				var message string
				if alert.State == "Normal" {
					pass = true
				} else if alert.State == "Alerting" || alert.State == "Pending" {
					pass = false
					message = summary
				} else if alert.State == "NoData" || alert.State == "Error" {
					continue
				}

				kind, ok := alert.Annotations["kind"]
				if !ok {
					logrus.Errorf("Alert %s missing 'kind' annotation", rule.Name)
					continue
				}

				if allGrafanaItems[prometheusFrom] == nil {
					allGrafanaItems[prometheusFrom] = NewGrafanaItem()
				}

				if group.Name == "inspection-cluster" {
					if kind == "cluster" {
						allGrafanaItems[prometheusFrom].ClusterCoreItem.ClusterItem[prometheusFrom] = append(allGrafanaItems[prometheusFrom].ClusterCoreItem.ClusterItem[prometheusFrom], apis.NewItem(alertname, message, pass))
					}
				} else if group.Name == "inspection-node" {
					if kind == "node" {
						instance, ok := alert.Labels["instance"]
						if !ok {
							logrus.Errorf("Alert %s missing 'instance' label", rule.Name)
							continue
						}
						nodeIP := strings.Split(instance, ":")[0]

						allGrafanaItems[prometheusFrom].ClusterNodeItem.NodeItem[nodeIP] = append(allGrafanaItems[prometheusFrom].ClusterNodeItem.NodeItem[nodeIP], apis.NewItem(alertname, message, pass))
					}
				} else if group.Name == "inspection-resource" {
					if kind == "workload" {
						createdByKind, ok := alert.Labels["created_by_kind"]
						if !ok {
							logrus.Errorf("Alert %s missing 'created_by_kind' label", rule.Name)
							continue
						}

						createdByName, ok := alert.Labels["created_by_name"]
						if !ok {
							logrus.Errorf("Alert %s missing 'created_by_name' label", rule.Name)
							continue
						}

						namespace, ok := alert.Labels["namespace"]
						if !ok {
							logrus.Errorf("Alert %s missing 'namespace' label", rule.Name)
							continue
						}

						if createdByKind == "ReplicaSet" {
							index := strings.LastIndex(createdByName, "-")
							workloadName := createdByName[:index]
							workloadNamespaceName := namespace + "/" + workloadName

							allGrafanaItems[prometheusFrom].ClusterResourceItem.DeploymentItem[workloadNamespaceName] = append(allGrafanaItems[prometheusFrom].ClusterResourceItem.DeploymentItem[workloadNamespaceName], apis.NewItem(alertname, message, pass))
						} else if createdByKind == "StatefulSet" {
							workloadNamespaceName := namespace + "/" + createdByName

							allGrafanaItems[prometheusFrom].ClusterResourceItem.StatefulsetItem[workloadNamespaceName] = append(allGrafanaItems[prometheusFrom].ClusterResourceItem.StatefulsetItem[workloadNamespaceName], apis.NewItem(alertname, message, pass))
						} else if createdByKind == "DaemonSet" {
							workloadNamespaceName := namespace + "/" + createdByName

							allGrafanaItems[prometheusFrom].ClusterResourceItem.DaemonsetItem[workloadNamespaceName] = append(allGrafanaItems[prometheusFrom].ClusterResourceItem.DaemonsetItem[workloadNamespaceName], apis.NewItem(alertname, message, pass))
						}
					} else if kind == "pvc" {
						namespace, ok := alert.Labels["namespace"]
						if !ok {
							logrus.Errorf("Alert %s missing 'namespace' label", rule.Name)
							continue
						}

						persistentvolumeclaim, ok := alert.Labels["persistentvolumeclaim"]
						if !ok {
							logrus.Errorf("Alert %s missing 'persistentvolumeclaim' label", rule.Name)
							continue
						}
						pvcNamespaceName := namespace + "/" + persistentvolumeclaim

						allGrafanaItems[prometheusFrom].ClusterResourceItem.PVCItems[pvcNamespaceName] = append(allGrafanaItems[prometheusFrom].ClusterResourceItem.PVCItems[pvcNamespaceName], apis.NewItem(alertname, message, pass))
					} else if kind == "pv" {
						persistentvolume, ok := alert.Labels["persistentvolume"]
						if !ok {
							logrus.Errorf("Alert %s missing 'persistentvolume' label", rule.Name)
							continue
						}

						allGrafanaItems[prometheusFrom].ClusterResourceItem.PVItems[persistentvolume] = append(allGrafanaItems[prometheusFrom].ClusterResourceItem.PVItems[persistentvolume], apis.NewItem(alertname, message, pass))
					} else if kind == "namespace" {
						namespace, ok := alert.Labels["namespace"]
						if !ok {
							logrus.Errorf("Alert %s missing 'namespace' label", rule.Name)
							continue
						}

						resource, ok := alert.Labels["resource"]
						if !ok {
							logrus.Errorf("Alert %s missing 'resource' label", rule.Name)
							continue
						}

						allGrafanaItems[prometheusFrom].ClusterResourceItem.NamespaceItem[namespace] = append(allGrafanaItems[prometheusFrom].ClusterResourceItem.NamespaceItem[namespace], apis.NewItem(alertname+" - "+resource, message, pass))
					}
				}
			}
		}
	}

	jsonData, err := json.MarshalIndent(allGrafanaItems, "", "\t")
	fmt.Println(string(jsonData))

	logrus.Infof("[%s] Completed getting all Grafana items", taskName)

	return allGrafanaItems, nil
}

func GetAllGrafanaInspections(taskName string) (map[string]*GrafanaInspection, error) {
	logrus.Infof("[%s] Starting to get all Grafana inspections", taskName)

	allGrafanaInspection := NewAllGrafanaInspection()

	alerting, err := GetAlerting()
	if err != nil {
		return nil, fmt.Errorf("Error getting alerting data: %v\n", err)
	}

	if alerting == nil || alerting.Data == nil || len(alerting.Data.RuleGroups) == 0 {
		return nil, fmt.Errorf("alerting rule is empty: %v", err)
	}

	for _, group := range alerting.Data.RuleGroups {
		for _, rule := range group.Rules {
			if rule.State == "firing" || rule.State == "pending" {
				for _, alert := range rule.Alerts {
					if alert.State == "Alerting" || alert.State == "pending" {
						prometheusFrom, ok := alert.Labels["prometheus_from"]
						if !ok {
							logrus.Errorf("Alert %s missing 'prometheus_from' label", rule.Name)
							continue
						}

						alertname, ok := alert.Labels["alertname"]
						if !ok {
							logrus.Errorf("Alert %s missing 'alertname' label", rule.Name)
							continue
						}

						summary, ok := alert.Annotations["summary"]
						if !ok {
							logrus.Errorf("Alert %s missing 'summary' annotation", rule.Name)
							continue
						}

						if allGrafanaInspection[prometheusFrom] == nil {
							allGrafanaInspection[prometheusFrom] = NewGrafanaInspection()
						}

						if group.Name == "inspection-cluster" {
							allGrafanaInspection[prometheusFrom].ClusterCoreInspection = append(allGrafanaInspection[prometheusFrom].ClusterCoreInspection, apis.NewInspection(fmt.Sprintf("%s : %s", alertname, prometheusFrom), fmt.Sprintf("%s %s", prometheusFrom, summary), 2))
						} else if group.Name == "inspection-node" {
							instance, ok := alert.Labels["instance"]
							if !ok {
								logrus.Errorf("Alert %s missing 'instance' label", rule.Name)
								continue
							}
							result := strings.Split(instance, ":")[0]

							allGrafanaInspection[prometheusFrom].ClusterNodeInspection = append(allGrafanaInspection[prometheusFrom].ClusterNodeInspection, apis.NewInspection(fmt.Sprintf("%s : %s : %s", alertname, prometheusFrom, result), fmt.Sprintf("%s %s %s", prometheusFrom, result, summary), 2))
						} else if group.Name == "inspection-resource" {
							allGrafanaInspection[prometheusFrom].ClusterResourceInspection = append(allGrafanaInspection[prometheusFrom].ClusterResourceInspection, apis.NewInspection(fmt.Sprintf("%s : %s", alertname, prometheusFrom), fmt.Sprintf("%s %s", prometheusFrom, summary), 2))
						}
					}
				}
			}
		}
	}

	logrus.Infof("[%s] Completed getting all Grafana inspections", taskName)
	return allGrafanaInspection, nil
}

func GetAlerting() (*Alerting, error) {
	url := common.ServerURL + "/api/v1/namespaces/cattle-global-monitoring/services/http:access-grafana:80/proxy/api/prometheus/grafana/api/v1/rules"
	logrus.Debugf("Fetching alerting data from URL: %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating request: %v\n", err)
	}

	req.Header.Set("Authorization", "Bearer "+common.BearerToken)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error executing request: %v\n", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {

		return nil, fmt.Errorf("Error reading response body: %v\n", err)
	}

	alerting := NewAlerting()
	err = json.Unmarshal(body, alerting)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling alerting data: %v\n", err)
	}

	return alerting, nil
}

func GetGrafanaAlerting() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		alerting, err := GetAlerting()
		if err != nil {
			logrus.Errorf("Error getting alerting data: %v", err)
			http.Error(rw, "Failed to get alerting data", http.StatusInternalServerError)
			return
		}

		jsonData, err := json.MarshalIndent(alerting, "", "\t")
		if err != nil {
			logrus.Errorf("Error marshalling alerting data: %v", err)
			http.Error(rw, "Failed to marshal alerting data", http.StatusInternalServerError)
			return
		}

		rw.Header().Set("Content-Type", "application/json")
		_, err = rw.Write(jsonData)
		if err != nil {
			logrus.Errorf("Error writing response: %v", err)
		}
	})
}

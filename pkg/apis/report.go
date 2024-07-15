package apis

type Report struct {
	ID         string        `json:"id"`
	Global     *Global       `json:"global"`
	Kubernetes []*Kubernetes `json:"kubernetes"`
}

type Global struct {
	Name       string `json:"name"`
	Rating     string `json:"rating"`
	ReportTime string `json:"report_time"`
}

type ClusterCore struct {
	HealthCheck *HealthCheck  `json:"health_check"`
	Inspections []*Inspection `json:"inspections"`
}

type HealthCheck struct {
	APIServerReady *CommandCheckResult `json:"api_server_ready"`
	APIServerLive  *CommandCheckResult `json:"api_server_live"`
	EtcdReady      *CommandCheckResult `json:"etcd_ready"`
	EtcdLive       *CommandCheckResult `json:"etcd_live"`
}

type ClusterNode struct {
	Nodes       []*Node       `json:"nodes"`
	Inspections []*Inspection `json:"inspections"`
}

type ClusterResource struct {
	Workloads   *Workload     `json:"workloads"`
	Namespace   []*Namespace  `json:"namespace"`
	Service     []*Service    `json:"service"`
	Ingress     []*Ingress    `json:"ingress"`
	Inspections []*Inspection `json:"inspections"`
}

type Inspection struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	Level   int    `json:"level"`
}

type Kubernetes struct {
	ClusterID       string           `json:"cluster_id"`
	ClusterName     string           `json:"cluster_name"`
	ClusterCore     *ClusterCore     `json:"cluster_core"`
	ClusterNode     *ClusterNode     `json:"cluster_node"`
	ClusterResource *ClusterResource `json:"cluster_resource"`
}

type Workload struct {
	Deployment  []*WorkloadData `json:"deployment"`
	Statefulset []*WorkloadData `json:"statefulset"`
	Daemonset   []*WorkloadData `json:"daemonset"`
	Job         []*WorkloadData `json:"job"`
	Cronjob     []*WorkloadData `json:"cronjob"`
}

type WorkloadData struct {
	Name      string  `json:"name"`
	Namespace string  `json:"namespace"`
	Pods      []*Pod  `json:"pods"`
	Status    *Status `json:"status"`
}

type Node struct {
	Name     string    `json:"name"`
	HostIP   string    `json:"host_ip"`
	Resource *Resource `json:"resource"`
	Commands *Command  `json:"commands"`
}

type Resource struct {
	LimitsCPU         int64 `json:"limits_cpu"`
	LimitsMemory      int64 `json:"limits_memory"`
	RequestsCPU       int64 `json:"requests_cpu"`
	RequestsMemory    int64 `json:"requests_memory"`
	RequestsPods      int64 `json:"requests_pods"`
	AllocatableCPU    int64 `json:"allocatable_cpu"`
	AllocatableMemory int64 `json:"allocatable_memory"`
	AllocatablePods   int64 `json:"allocatable_pods"`
}

type Namespace struct {
	Name               string `json:"name"`
	EmptyResourceQuota bool   `json:"empty_resource_quota"`
	EmptyResource      bool   `json:"empty_resource"`
	PodCount           int    `json:"pod_count"`
	ServiceCount       int    `json:"service_count"`
	DeploymentCount    int    `json:"deployment_count"`
	ReplicasetCount    int    `json:"replicaset_count"`
	StatefulsetCount   int    `json:"statefulset_count"`
	DaemonsetCount     int    `json:"daemonset_count"`
	JobCount           int    `json:"job_count"`
	SecretCount        int    `json:"secret_count"`
	ConfigMapCount     int    `json:"config_map_count"`
}

type PersistentVolumeClaim struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type Service struct {
	Name           string `json:"name"`
	Namespace      string `json:"namespace"`
	EmptyEndpoints bool   `json:"empty_endpoints"`
}

type Ingress struct {
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	DuplicatePath bool   `json:"duplicate_path"`
}

type Pod struct {
	Name string   `json:"name"`
	Log  []string `json:"log"`
}

type Status struct {
	State     string      `json:"state"`
	Condition []Condition `json:"condition"`
}

type Condition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type Command struct {
	Stdout []CommandCheckResult `json:"stdout"`
	Stderr string               `json:"stderr"`
}

type CommandCheckResult struct {
	Description string `json:"description"`
	Command     string `json:"command"`
	Response    string `json:"response"`
	Error       string `json:"error"`
}

func NewReport() *Report {
	return &Report{
		Global:     &Global{},
		Kubernetes: []*Kubernetes{},
	}
}

func NewKubernetes() []*Kubernetes {
	return []*Kubernetes{}
}

func NewClusterCore() *ClusterCore {
	return &ClusterCore{
		Inspections: []*Inspection{},
	}
}

func NewClusterNode() *ClusterNode {
	return &ClusterNode{
		Nodes:       []*Node{},
		Inspections: []*Inspection{},
	}
}

func NewClusterResource() *ClusterResource {
	return &ClusterResource{
		Workloads: &Workload{
			Deployment:  []*WorkloadData{},
			Statefulset: []*WorkloadData{},
			Daemonset:   []*WorkloadData{},
			Job:         []*WorkloadData{},
			Cronjob:     []*WorkloadData{},
		},
		Namespace:   []*Namespace{},
		Service:     []*Service{},
		Ingress:     []*Ingress{},
		Inspections: []*Inspection{},
	}
}

func NewPods() []*Pod {
	return []*Pod{}
}

func NewNodes() []*Node {
	return []*Node{}
}

func NewHealthCheck() *HealthCheck {
	return &HealthCheck{}
}

func NewWorkload() *Workload {
	return &Workload{
		Deployment:  []*WorkloadData{},
		Statefulset: []*WorkloadData{},
		Daemonset:   []*WorkloadData{},
		Job:         []*WorkloadData{},
		Cronjob:     []*WorkloadData{},
	}
}

func NewWorkloadDatas() []*WorkloadData {
	return []*WorkloadData{}
}

func NewNamespaces() []*Namespace {
	return []*Namespace{}
}

func NewPersistentVolumeClaims() []*PersistentVolumeClaim {
	return []*PersistentVolumeClaim{}
}

func NewServices() []*Service {
	return []*Service{}
}

func NewIngress() []*Ingress {
	return []*Ingress{}
}

func NewInspections() []*Inspection {
	return []*Inspection{}
}

func NewInspection(title, message string, level int) *Inspection {
	return &Inspection{
		Title:   title,
		Message: message,
		Level:   level,
	}
}

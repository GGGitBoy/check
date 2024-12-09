package apis

type Template struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	KubernetesConfig []*KubernetesConfig `json:"kubernetes"`
}

type KubernetesConfig struct {
	Enable                bool                   `json:"enable"`
	ClusterID             string                 `json:"cluster_id"`
	ClusterName           string                 `json:"cluster_name"`
	ClusterCoreConfig     *ClusterCoreConfig     `json:"cluster_core_config"`
	ClusterNodeConfig     *ClusterNodeConfig     `json:"cluster_node_config"`
	ClusterResourceConfig *ClusterResourceConfig `json:"cluster_resource_config"`
}

type ClusterCoreConfig struct {
	ChartVersionCheck *ChartVersionCheck `json:"chart_version_check"`
	//EtcdHealthCheck
	//APIServerHealthCheck
}

type ChartVersionCheck struct {
	Enable             bool     `json:"enable"`
	AllowVersion       []string `json:"allow_version"`
	ExcludedNamespaces string   `json:"excluded_namespace"`
}

type ClusterNodeConfig struct {
	Enable     bool          `json:"enable"`
	NodeConfig []*NodeConfig `json:"node_config"`
}

type ClusterResourceConfig struct {
	WorkloadConfig  *WorkloadConfig  `json:"workload_config"`
	NamespaceConfig *NamespaceConfig `json:"namespace_config"`
	ServiceConfig   *ServiceConfig   `json:"service_config"`
	IngressConfig   *IngressConfig   `json:"ingress_config"`
	PVCConfig       *PVCConfig       `json:"pvc_config"`
	PVConfig        *PVConfig        `json:"pv_config"`
}

type NamespaceConfig struct {
	Enable         bool              `json:"enable"`
	SelectorLabels map[string]string `json:"selector_labels"`
	NameCheck      *NameCheck        `json:"name_check"`
}

type NameCheck struct {
	IncludeName       string `json:"include_name"`
	ExcludedNamespace string `json:"excluded_namespace"`
}

type ServiceConfig struct {
	Enable            bool              `json:"enable"`
	SelectorNamespace string            `json:"selector_namespace"`
	SelectorLabels    map[string]string `json:"selector_labels"`
}

type PVCConfig struct {
	Enable            bool              `json:"enable"`
	SelectorNamespace string            `json:"selector_namespace"`
	SelectorLabels    map[string]string `json:"selector_labels"`
}

type PVConfig struct {
	Enable         bool              `json:"enable"`
	SelectorLabels map[string]string `json:"selector_labels"`
}

type IngressConfig struct {
	Enable            bool              `json:"enable"`
	SelectorNamespace string            `json:"selector_namespace"`
	SelectorLabels    map[string]string `json:"selector_labels"`
}

type WorkloadConfig struct {
	Deployment  *WorkloadDetailConfig `json:"deployment"`
	Statefulset *WorkloadDetailConfig `json:"statefulset"`
	Daemonset   *WorkloadDetailConfig `json:"daemonset"`
	Job         *WorkloadDetailConfig `json:"job"`
	Cronjob     *WorkloadDetailConfig `json:"cronjob"`
}

type WorkloadDetailConfig struct {
	Enable            bool              `json:"enable"`
	SelectorNamespace string            `json:"selector_namespace"`
	SelectorLabels    map[string]string `json:"selector_labels"`
}

type NodeConfig struct {
	SelectorLabels map[string]string `json:"selector_labels"`
	Commands       []*CommandConfig  `json:"commands"`
}

type CommandConfig struct {
	Description string `json:"description"`
	Command     string `json:"command"`
	Level       int    `json:"level"`
}

func NewTemplate() *Template {
	return &Template{
		KubernetesConfig: []*KubernetesConfig{},
	}
}

func NewTemplates() []*Template {
	return []*Template{}
}

func NewKubernetesConfig() []*KubernetesConfig {
	return []*KubernetesConfig{}
}

func NewClusterCoreConfig() *ClusterCoreConfig {
	return &ClusterCoreConfig{
		ChartVersionCheck: &ChartVersionCheck{},
	}
}

func NewClusterNodeConfig() *ClusterNodeConfig {
	return &ClusterNodeConfig{
		NodeConfig: []*NodeConfig{},
	}
}

func NewNodeConfigs() []*NodeConfig {
	return []*NodeConfig{}
}

func NewClusterResourceConfig() *ClusterResourceConfig {
	return &ClusterResourceConfig{
		WorkloadConfig:  &WorkloadConfig{},
		NamespaceConfig: &NamespaceConfig{},
		ServiceConfig:   &ServiceConfig{},
		IngressConfig:   &IngressConfig{},
		PVCConfig:       &PVCConfig{},
		PVConfig:        &PVConfig{},
	}
}

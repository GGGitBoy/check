package apis

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

type Client struct {
	DynamicClient dynamic.Interface     `json:"dynamic_client"`
	Clientset     *kubernetes.Clientset `json:"clientset"`
	Config        *restclient.Config    `json:"config"`
}

func NewClients() map[string]*Client {
	return make(map[string]*Client)
}

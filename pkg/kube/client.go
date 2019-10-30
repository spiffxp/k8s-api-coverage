package kube

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// BuildKubeClient returns a kubernetes client loaded via the following rules:
// - first try the in-cluster config
// - next try the default rules (KUBECONFIG, .kube/config)
func BuildKubeClient() (kubernetes.Interface, error) {
	clientConfig, err := rest.InClusterConfig()
	if err != nil {
		config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
		if err != nil {
			return nil, fmt.Errorf("could not load credentials from config: %v", err)
		}
		clientConfig, err = clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("could not load client configuration: %v", err)
		}
	}
	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to make client from config: %v", err)
	}
	return client, nil
}

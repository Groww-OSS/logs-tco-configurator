package kubernetes

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sClient manages Kubernetes secrets
type K8sClient struct {
	clientset kubernetes.Interface
}

// NewK8sClient creates a new Kubernetes client using provided kubeconfig path.
// If local kubeconfig fails, it falls back to in-cluster configuration.
func New(kubeconfig string) (*K8sClient, error) {
	log.Debug().Msg("creating new K8sClient")

	var config *rest.Config
	var err error

	// Try local kubeconfig first
	if kubeconfig != "" {
		log.Debug().Str("kubeconfig", kubeconfig).Msg("attempting to use local kubeconfig")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Debug().Err(err).Msg("failed to use local kubeconfig")
		}
	}

	// Fall back to in-cluster config if local config fails or isn't provided
	if config == nil {
		log.Debug().Msg("attempting to use in-cluster config")
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	log.Debug().Msg("successfully created kubernetes clientset")
	return &K8sClient{clientset: clientset}, nil
}

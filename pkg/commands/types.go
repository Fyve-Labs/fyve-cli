package commands

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/client/pkg/k8s"
	"os"

	clientdynamic "knative.dev/client/pkg/dynamic"
	knerrors "knative.dev/client/pkg/errors"
	clientservingv1 "knative.dev/client/pkg/serving/v1"
	clientservingv1beta1 "knative.dev/client/pkg/serving/v1beta1"
	servingv1client "knative.dev/serving/pkg/client/clientset/versioned/typed/serving/v1"
	servingv1beta1client "knative.dev/serving/pkg/client/clientset/versioned/typed/serving/v1beta1"
)

type FyveParams struct {
	k8s.Params

	// Memorizes the loaded config
	clientcmd.ClientConfig

	NewKubeClient           func() (kubernetes.Interface, error)
	NewDynamicClient        func(namespace string) (clientdynamic.KnDynamicClient, error)
	NewServingClient        func(namespace string) (clientservingv1.KnServingClient, error)
	NewServingV1beta1Client func(namespace string) (clientservingv1beta1.KnServingClient, error)
}

func (params *FyveParams) Initialize() {
	if params.NewKubeClient == nil {
		params.NewKubeClient = params.newKubeClient
	}

	if params.NewDynamicClient == nil {
		params.NewDynamicClient = params.newDynamicClient
	}

	if params.NewServingClient == nil {
		params.NewServingClient = params.newServingClient
	}

	if params.NewServingV1beta1Client == nil {
		params.NewServingV1beta1Client = params.newServingClientV1beta1
	}
}

func (params *FyveParams) newKubeClient() (kubernetes.Interface, error) {
	restConfig, err := params.RestConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (params *FyveParams) newServingClient(namespace string) (clientservingv1.KnServingClient, error) {
	restConfig, err := params.RestConfig()
	if err != nil {
		return nil, err
	}

	client, err := servingv1client.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return clientservingv1.NewKnServingClient(client, namespace), nil
}

func (params *FyveParams) newServingClientV1beta1(namespace string) (clientservingv1beta1.KnServingClient, error) {
	restConfig, err := params.RestConfig()
	if err != nil {
		return nil, err
	}

	client, err := servingv1beta1client.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return clientservingv1beta1.NewKnServingClient(client, namespace), nil
}

func (params *FyveParams) newDynamicClient(namespace string) (clientdynamic.KnDynamicClient, error) {
	restConfig, err := params.RestConfig()
	if err != nil {
		return nil, err
	}

	client, _ := dynamic.NewForConfig(restConfig)
	return clientdynamic.NewKnDynamicClient(client, namespace), nil
}

// RestConfig returns REST config, which can be to use to create specific clientset
func (params *FyveParams) RestConfig() (*rest.Config, error) {
	var err error

	if params.ClientConfig == nil {
		params.ClientConfig, err = params.GetClientConfig()
		if err != nil {
			return nil, knerrors.GetError(err)
		}
	}

	config, err := params.ClientConfig.ClientConfig()
	if err != nil {
		return nil, knerrors.GetError(err)
	}

	// Override client-go's warning handler to give us nicely printed warnings.
	config.WarningHandler = rest.NewWarningWriter(os.Stderr, rest.WarningWriterOptions{
		// only print a given warning the first time we receive it
		Deduplicate: true,
	})

	return config, nil
}

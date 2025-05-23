package app

import (
	"errors"
	"fmt"
	"github.com/fyve-labs/fyve-cli/pkg/commands"
	"github.com/fyve-labs/fyve-cli/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	knerrors "knative.dev/client/pkg/errors"
	clientv1beta1 "knative.dev/client/pkg/serving/v1beta1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"sigs.k8s.io/external-dns/endpoint"
	"strings"
)

func NewPublishCommand(p *commands.Params) *cobra.Command {
	var domain string

	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish application deployed to Fyve App Platform",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			_ = viper.BindPFlag("app", cmd.Flags().Lookup("name"))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			appConfig, err := config.LoadAppConfig()
			if err != nil {
				return err
			}

			baseDomain := viper.GetString("domain")
			if domain == "" {
				domain = appConfig.App + "." + baseDomain
			}

			if !strings.HasSuffix(domain, baseDomain) {
				return errors.New(fmt.Sprintf("domain '%s' must end with '%s'", domain, baseDomain))
			}

			namespace := "default"

			// 1. Create DomainMapping
			reference := &duckv1.KReference{
				Kind:       "Service",
				APIVersion: "serving.knative.dev/v1",
				Name:       appConfig.App,
				Namespace:  namespace,
			}

			domainmapping := clientv1beta1.NewDomainMappingBuilder(domain).
				Namespace(namespace).
				Reference(*reference).
				Build()

			client, err := p.NewServingV1beta1Client(namespace)
			if err != nil {
				return err
			}

			err = client.CreateDomainMapping(cmd.Context(), domainmapping)
			if err != nil {
				return knerrors.GetError(err)
			}

			// 2. Create DNSEndpoint
			dclient, _ := p.NewDynamicClient(namespace)
			cname := endpoint.NewEndpoint(domain, endpoint.RecordTypeCNAME, config.DefaultCnameTarget)
			cname.RecordTTL = endpoint.TTL(viper.GetInt64("dns.ttl"))

			object := EndpointToUnstructured(namespace, *cname)
			kubeClient := dclient.RawClient()
			created, err := kubeClient.
				Resource(DNSEndpointResource()).
				Namespace(namespace).
				Create(cmd.Context(), object, metav1.CreateOptions{})
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully published %s\n", created.GetName())
			return nil
		},
	}

	cmd.Flags().String("name", "", "App name")
	cmd.Flags().StringVar(&domain, "domain", "", "Domain to publish to (default is {app}.fyve.dev)")

	return cmd
}

// DNSEndpointResource returns the GroupVersionResource for DNSEndpoints
func DNSEndpointResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "externaldns.k8s.io",
		Version:  "v1alpha1",
		Resource: "dnsendpoints",
	}
}

func EndpointToUnstructured(namespace string, endpoint endpoint.Endpoint) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "externaldns.k8s.io/v1alpha1",
			"kind":       "DNSEndpoint",
			"metadata": map[string]interface{}{
				"name":      endpoint.DNSName,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"endpoints": []interface{}{
					map[string]interface{}{
						"dnsName":    endpoint.DNSName,
						"recordTTL":  endpoint.RecordTTL,
						"recordType": endpoint.RecordType,
						"targets":    endpoint.Targets,
					},
				},
			},
		},
	}
}

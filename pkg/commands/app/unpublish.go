package app

import (
	"context"
	"fmt"
	"github.com/fyve-labs/fyve-cli/pkg/commands"
	"github.com/fyve-labs/fyve-cli/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knerrors "knative.dev/client/pkg/errors"
	clientservingv1beta1 "knative.dev/client/pkg/serving/v1beta1"
	"knative.dev/serving/pkg/apis/serving/v1beta1"
)

func NewUnPublishCommand(p *commands.Params) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unpublish",
		Short: "Un-publish all associated domains of the app",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			_ = viper.BindPFlag("app", cmd.Flags().Lookup("name"))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			appConfig, err := config.LoadAppConfig()
			if err != nil {
				return err
			}

			namespace := "default"

			// Get client for domainmappings
			client, err := p.NewServingV1beta1Client(namespace)
			if err != nil {
				return err
			}

			// Get client for DNSEndpoints
			dclient, err := p.NewDynamicClient(namespace)
			if err != nil {
				return err
			}
			kubeClient := dclient.RawClient()

			// Get all domainmappings in the namespace
			mappings, err := listDomainMappingsForApp(cmd.Context(), client, appConfig.App)
			if err != nil {
				return fmt.Errorf("failed to list domain mappings: %w", err)
			}

			if len(mappings) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No published domains found for app '%s'\n", appConfig.App)
				return nil
			}

			var failedDomains []string
			var succeededDomains []string

			// Delete each domainmapping and its corresponding DNSEndpoint
			for _, mapping := range mappings {
				domainName := mapping.Name

				// 1. Delete DomainMapping
				err = client.DeleteDomainMapping(cmd.Context(), domainName)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStderr(), "Warning: Failed to unpublish domainmapping %s: %v\n", domainName, err)
					failedDomains = append(failedDomains, domainName)
					continue
				}

				// 2. Delete DNSEndpoint
				err = kubeClient.
					Resource(DNSEndpointResource()).
					Namespace(namespace).
					Delete(cmd.Context(), domainName, metav1.DeleteOptions{})
				if err != nil {
					fmt.Fprintf(cmd.OutOrStderr(), "Warning: Failed to delete DNSEndpoint for %s: %v\n", domainName, err)
					// Don't add to failedDomains since the domainmapping was deleted successfully
				}

				succeededDomains = append(succeededDomains, domainName)
			}

			// Report results
			if len(succeededDomains) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Successfully unpublished domains: %v\n", succeededDomains)
			}

			if len(failedDomains) > 0 {
				return fmt.Errorf("failed to unpublish some domains: %v", failedDomains)
			}

			return nil
		},
	}

	cmd.Flags().String("name", "", "App name")

	return cmd
}

// listDomainMappingsForApp retrieves all domainmappings that reference the specified app
func listDomainMappingsForApp(ctx context.Context, client clientservingv1beta1.KnServingClient, appName string) ([]v1beta1.DomainMapping, error) {
	// Get all domainmappings in the namespace
	allMappings, err := client.ListDomainMappings(ctx)
	if err != nil {
		return nil, knerrors.GetError(err)
	}

	// Filter mappings that reference the specified app
	var appMappings []v1beta1.DomainMapping
	for _, mapping := range allMappings.Items {
		ref := mapping.Spec.Ref
		if ref.Kind == "Service" && ref.Name == appName {
			appMappings = append(appMappings, mapping)
		}
	}

	return appMappings, nil
}

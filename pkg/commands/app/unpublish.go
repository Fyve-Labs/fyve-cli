package app

import (
	"fmt"
	"github.com/fyve-labs/fyve-cli/pkg/commands"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knerrors "knative.dev/client/pkg/errors"
)

func NewUnPublishCommand(p *commands.Params) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unpublish",
		Short: "Un-publish application",
		RunE: func(cmd *cobra.Command, args []string) error {
			app := viper.GetString("app")
			if app == "" {
				return errors.New("missing app name, set app name in fyve.yaml or use FYVE_APP environment variable")
			}

			domain := app + "." + viper.GetString("domain")
			namespace := "default"

			client, err := p.NewServingV1beta1Client(namespace)
			if err != nil {
				return err
			}

			err = client.DeleteDomainMapping(cmd.Context(), domain)
			if err != nil {
				return knerrors.GetError(err)
			}

			// 2. Delete DNSEndpoint
			dclient, _ := p.NewDynamicClient(namespace)
			kubeClient := dclient.RawClient()
			err = kubeClient.
				Resource(DNSEndpointResource()).
				Namespace(namespace).
				Delete(cmd.Context(), domain, metav1.DeleteOptions{})
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Unpublished %s\n", domain)
			return nil
		},
	}

	return cmd
}

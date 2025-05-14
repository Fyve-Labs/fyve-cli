package commands

import (
	"fmt"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

func NewKubeconfigCommand(p *Params) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Get kubeconfig for use with kubectl.",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientconfig, err := p.Params.GetClientConfig()
			if err != nil {
				return err
			}

			apiconfig, _ := clientconfig.RawConfig()
			bytes, err := clientcmd.Write(apiconfig)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), string(bytes))

			return nil
		},
	}

	return cmd
}

package commands

import (
	"context"
	"fmt"
	"github.com/fyve-labs/fyve-cli/pkg/docker"
	"github.com/spf13/cobra"
	"time"
)

func NewUpdateCmd() *cobra.Command {
	var (
		imageTag   string
		dockerHost string
	)

	cmd := &cobra.Command{
		Use:   "update [app-name]",
		Short: "Update Cosmos application",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var appName string
			if len(args) > 0 && args[0] != "" {
				appName = args[0]
			}

			if appName == "" {
				return fmt.Errorf("app name is required")
			}

			if imageTag == "" {
				return fmt.Errorf("image tag is required")
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
			defer cancel()

			containerName := appName
			containerService, err := docker.NewContainerService(dockerHost)
			if err != nil {
				return err
			}

			_, err = containerService.ReCreate(ctx, containerName, true, imageTag)

			return err
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&imageTag, "tag", "t", "", "New image tag")
	cmd.Flags().StringVarP(&dockerHost, "docker-host", "d", "tcp://10.100.26.239:2375", "Remote Docker host URL")

	return cmd
}

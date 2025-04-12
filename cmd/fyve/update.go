package fyve

import (
	"context"
	"fmt"
	"github.com/fyve-labs/fyve-cli/pkg/config"
	"github.com/fyve-labs/fyve-cli/pkg/docker"
	"github.com/spf13/cobra"
	"time"
)

func init() {
	rootCmd.AddCommand(UpdateCmd())
}

func UpdateCmd() *cobra.Command {
	var (
		imageTag   string
		configFile string
		dockerHost string
	)

	cmd := &cobra.Command{
		Use:   "update [app-name]",
		Short: "Update Cosmos application",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg, err := config.Load(configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Override app name if provided via args
			appName := cfg.App
			if len(args) > 0 && args[0] != "" {
				appName = args[0]
				cfg.OverrideAppName(appName)
			}

			if appName == "" {
				return fmt.Errorf("app name must be specified in config or as an argument")
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
	cmd.Flags().StringVarP(&configFile, "config", "c", "fyve.yaml", "Path to configuration file")
	cmd.Flags().StringVarP(&dockerHost, "docker-host", "d", "tcp://10.100.26.239:2375", "Remote Docker host URL")

	return cmd
}

package commands

import (
	"context"
	"fmt"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/fyve-labs/fyve-cli/pkg/builder"
	"github.com/fyve-labs/fyve-cli/pkg/config"
	"github.com/fyve-labs/fyve-cli/pkg/deployer"
	"github.com/fyve-labs/fyve-cli/pkg/secrets"
	"github.com/fyve-labs/fyve-cli/pkg/service"
	"github.com/spf13/cobra"
	"os"
)

const (
	// DefaultDockerHost is the default Docker host to connect to
	DefaultDockerHost = "tcp://socket-proxy:2375"
)

// NewDeployCmd returns the deploy command
func NewDeployCmd(p *Params) *cobra.Command {
	var (
		image        string
		port         int32
		deployDocker bool
		dockerHost   string
	)

	cmd := &cobra.Command{
		Use:   "deploy [app-name]",
		Short: "Deploy a NextJS application",
		Long:  `Build and deploy a NextJS application to a remote docker host or Fyve App Platform.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			environment := "production"
			projectDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			// LoadAppConfig configuration
			cfg, err := config.LoadAppConfig()
			if err != nil {
				return fmt.Errorf("error load app config: %w", err)
			}

			ctx := context.Background()
			awsConfig, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(config.GlobalConfig.Region()))
			if err != nil {
				return fmt.Errorf("AWS credentials: %w", err)
			}

			ecrClient := ecr.NewFromConfig(awsConfig)
			ssmClient := ssm.NewFromConfig(awsConfig)
			buildConfig := cfg.BuildConfig()

			// Create SSM manager using AWS SDK v2
			secretManager, err := secrets.NewSSMManager(ssmClient)
			if err != nil {
				return fmt.Errorf("failed to initialize secrets manager: %w", err)
			}

			// Process environment variables and resolve any secret references
			resolvedEnv, err := secretManager.ProcessSecretRefs(cfg.Env, environment)
			if err != nil {
				return fmt.Errorf("failed to process secrets: %w", err)
			}

			if image == "" {
				err = buildConfig.EnsureECRRepositoryExists(ctx, ecrClient)
				if err != nil {
					return err
				}

				// Set up builder
				b, err := builder.NewNextJSBuilder(projectDir, cfg.App, environment, buildConfig)
				if err != nil {
					return fmt.Errorf("failed to initialize builder: %w", err)
				}

				// Build the NextJS application
				if err := b.Build(); err != nil {
					return fmt.Errorf("build failed: %w", err)
				}

				err = buildConfig.ECRLogin(ctx, ecrClient)
				if err != nil {
					return fmt.Errorf("ECRLogin: %w", err)
				}
				defer buildConfig.ECRLogout()

				// Push to ECR
				if err := b.PushToECR(); err != nil {
					return fmt.Errorf("failed to push to ECR: %w", err)
				}

				image = buildConfig.GetImage()
			}

			// Deploy to a remote Docker host
			if deployDocker {
				d, err := deployer.NewDockerDeployer(cfg.App, buildConfig, dockerHost, resolvedEnv)
				if err != nil {
					return fmt.Errorf("failed to create deployer: %w", err)
				}

				if err := d.Deploy(environment, port); err != nil {
					return fmt.Errorf("deployment failed: %w", err)
				}

				fmt.Printf("Successfully deployed %s to %s environment\n", cfg.App, environment)
				return nil
			}

			// Deploy to Kubernetes
			namespace := "default"
			client, err := p.NewServingClient(namespace)
			if err != nil {
				return err
			}

			err = service.CreateService(ctx, client, namespace, cfg.App, image, port, resolvedEnv, true, cmd.OutOrStdout())
			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&image, "image", "", "Image to deploy. if not specified, the image will be built from the current directory.")
	cmd.Flags().Int32Var(&port, "port", 3000, "Port to expose the application on (default: 3000)")
	cmd.Flags().BoolVar(&deployDocker, "docker", false, "Deploy to docker instead of Kubernetes")
	cmd.Flags().StringVarP(&dockerHost, "docker-host", "d", DefaultDockerHost, "Remote Docker host URL to deploy to")

	return cmd
}

package app

import (
	"context"
	"fmt"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/fyve-labs/fyve-cli/pkg/builder"
	"github.com/fyve-labs/fyve-cli/pkg/commands"
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

var deploy_example = `
  # Deploy using configuration from fyve.yaml
  fyve deploy

  # Deploy without configuration file
  fyve deploy --name whoami --image ghcr.io/traefik/whoami:latest --port 80

  # Custom scale down delay
  fyve deploy --scale-down-delay 10m

  # Deploy to a remote Docker host
  fyve deploy --docker`

// NewDeployCmd returns the deploy command
func NewDeployCmd(p *commands.Params) *cobra.Command {
	var (
		deployDocker bool
		dockerHost   string
	)

	cmd := &cobra.Command{
		Use:     "deploy",
		Short:   "Build and deploy Docker based application",
		Example: deploy_example,
		Args:    cobra.MaximumNArgs(1),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			BindAppFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			environment := "prod"
			projectDir, _ := os.Getwd()

			// LoadAppConfig configuration
			appConfig, err := config.LoadAppConfig()
			if err != nil {
				return err
			}

			ctx := context.Background()
			awsConfig, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(appConfig.Region))
			if err != nil {
				return fmt.Errorf("AWS credentials: %w", err)
			}

			ecrClient := ecr.NewFromConfig(awsConfig)
			ssmClient := ssm.NewFromConfig(awsConfig)
			buildConfig := appConfig.BuildConfig()

			// Create SSM Manager Client
			secretManager, err := secrets.NewSSMManager(ssmClient)
			if err != nil {
				return fmt.Errorf("failed to initialize secrets manager: %w", err)
			}

			// Process environment variables and resolve any secret references
			resolvedEnv, err := secretManager.ProcessSecretRefs(appConfig.Env, environment)
			if err != nil {
				return fmt.Errorf("failed to process secrets: %w", err)
			}

			if !appConfig.SkipBuild() {
				err = buildConfig.EnsureECRRepositoryExists(ctx, ecrClient)
				if err != nil {
					return err
				}

				// Set up builder
				b, err := builder.NewNextJSBuilder(projectDir, appConfig.App, environment, buildConfig)
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

				appConfig.Image = buildConfig.GetImage()
			}

			// Deploy to a remote Docker host
			if deployDocker {
				d, err := deployer.NewDockerDeployer(appConfig.App, buildConfig, dockerHost, resolvedEnv)
				if err != nil {
					return fmt.Errorf("failed to create deployer: %w", err)
				}

				if err := d.Deploy(environment, appConfig.Port); err != nil {
					return fmt.Errorf("deployment failed: %w", err)
				}

				fmt.Printf("Successfully deployed %s to %s environment\n", appConfig.App, environment)
				return nil
			}

			// Deploy to Kubernetes
			namespace := "default"
			client, err := p.NewServingClient(namespace)
			if err != nil {
				return err
			}

			return service.CreateService(ctx, client, namespace, appConfig, resolvedEnv, true, cmd.OutOrStdout())
		},
	}

	cmd.Flags().BoolVar(&deployDocker, "docker", false, "Deploy to docker instead of Kubernetes")
	cmd.Flags().StringVarP(&dockerHost, "docker-host", "d", DefaultDockerHost, "Remote Docker host URL to deploy to")
	SetAppFlags(cmd.Flags())

	return cmd
}

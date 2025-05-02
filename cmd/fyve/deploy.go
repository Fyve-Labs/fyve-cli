package fyve

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
	"github.com/spf13/cobra"
	"os"
)

const (
	// DefaultDockerHost is the default Docker host to connect to
	DefaultDockerHost = "tcp://socket-proxy:2375"
)

func init() {
	rootCmd.AddCommand(DeployCmd())
}

// DeployCmd returns the deploy command
func DeployCmd() *cobra.Command {
	var (
		awsRegion   string
		environment string
		configFile  string
		dockerHost  string
	)

	cmd := &cobra.Command{
		Use:   "deploy [app-name]",
		Short: "Deploy a NextJS application",
		Long:  `Build and deploy a NextJS application to a remote docker host.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get current working directory as project directory
			projectDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

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

			ctx := context.Background()
			awsConfig, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(awsRegion))
			if err != nil {
				return fmt.Errorf("AWS credentials: %w", err)
			}

			ecrClient := ecr.NewFromConfig(awsConfig)
			ssmClient := ssm.NewFromConfig(awsConfig)
			buildConfig := cfg.BuildConfig(environment)

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

			err = buildConfig.EnsureECRRepositoryExists(ctx, ecrClient)
			if err != nil {
				return err
			}

			// Set up builder
			b, err := builder.NewNextJSBuilder(projectDir, appName, environment, buildConfig)
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

			// Deploy to remote Docker host
			d, err := deployer.NewDockerDeployer(appName, buildConfig, dockerHost, resolvedEnv)
			if err != nil {
				return fmt.Errorf("failed to create deployer: %w", err)
			}

			if err := d.Deploy(environment); err != nil {
				return fmt.Errorf("deployment failed: %w", err)
			}

			fmt.Printf("Successfully deployed %s to %s environment\n", appName, environment)
			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&environment, "environment", "e", "prod", "Deployment environment (prod, staging, dev, test, preview)")
	cmd.Flags().StringVarP(&configFile, "config", "c", "fyve.yaml", "Path to configuration file")
	cmd.Flags().StringVarP(&awsRegion, "region", "", "us-east-1", "AWS region")
	cmd.Flags().StringVarP(&dockerHost, "docker-host", "d", DefaultDockerHost, "Remote Docker host URL")

	return cmd
}

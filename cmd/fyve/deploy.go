package fyve

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fyve-labs/fyve-cli/pkg/builder"
	"github.com/fyve-labs/fyve-cli/pkg/config"
	"github.com/fyve-labs/fyve-cli/pkg/deployer"
	"github.com/fyve-labs/fyve-cli/pkg/secrets"
	"github.com/spf13/cobra"
)

// Environment variables
const (
	DefaultRegion = "us-east-1"

	// DefaultECRRegistry is the default ECR registry URL template
	DefaultECRRegistry = "{aws_account_id}.dkr.ecr.{region}.amazonaws.com"

	// DefaultDockerHost is the default Docker host to connect to
	DefaultDockerHost = "" // Empty string means use local Docker daemon

	// DefaultPort is the default port to expose
	DefaultPort = "3000"
)

// DeployCmd returns the deploy command
func DeployCmd() *cobra.Command {
	var (
		awsRegion   string
		environment string
		configFile  string
		registry    string
		dockerHost  string
		imagePrefix string
		platform    string
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

			// Replace templated values in registry URL
			registry = strings.Replace(registry, "{region}", awsRegion, 1)
			registry, err = deployer.ReplaceAWSAccountID(context.Background(), registry, awsRegion)
			if err != nil {
				return fmt.Errorf("failed to replace AWS account ID: %w", err)
			}

			// Create SSM manager using AWS SDK v2
			secretManager, err := secrets.NewSSMManager(awsRegion)
			if err != nil {
				return fmt.Errorf("failed to initialize secrets manager: %w", err)
			}

			// Process environment variables and resolve any secret references
			resolvedEnv, err := secretManager.ProcessSecretRefs(cfg.Env, environment)
			if err != nil {
				return fmt.Errorf("failed to process secrets: %w", err)
			}

			// Set up builder
			builder, err := builder.NewNextJSBuilder(projectDir, appName, registry, imagePrefix, platform, environment, awsRegion)
			if err != nil {
				return fmt.Errorf("failed to initialize builder: %w", err)
			}

			// Build the NextJS application
			if err := builder.Build(); err != nil {
				return fmt.Errorf("build failed: %w", err)
			}

			// Push to ECR
			if err := builder.PushToECR(); err != nil {
				return fmt.Errorf("failed to push to ECR: %w", err)
			}

			// Deploy to remote Docker host
			deployer, err := deployer.NewDockerDeployer(appName, registry, dockerHost, awsRegion, resolvedEnv, imagePrefix)
			if err != nil {
				return fmt.Errorf("failed to create deployer: %w", err)
			}

			if err := deployer.Deploy(environment); err != nil {
				return fmt.Errorf("deployment failed: %w", err)
			}

			fmt.Printf("Successfully deployed %s to %s environment\n", appName, environment)
			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&environment, "environment", "e", "prod", "Deployment environment (prod, staging, dev, test, preview)")
	cmd.Flags().StringVarP(&configFile, "config", "c", "fyve.yaml", "Path to configuration file")
	cmd.Flags().StringVarP(&registry, "registry", "r", DefaultECRRegistry, "ECR registry URL (format: {aws_account_id}.dkr.ecr.{region}.amazonaws.com)")
	cmd.Flags().StringVarP(&awsRegion, "region", "", DefaultRegion, "AWS region")
	cmd.Flags().StringVarP(&dockerHost, "docker-host", "d", DefaultDockerHost, "Remote Docker host URL")
	cmd.Flags().StringVarP(&imagePrefix, "image-prefix", "i", "fyve-", "Prefix for Docker image names")
	cmd.Flags().StringVar(&platform, "platform", "linux/amd64", "Target platform for Docker build (e.g., linux/amd64, linux/arm64)")

	return cmd
}

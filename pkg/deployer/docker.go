package deployer

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// DockerDeployer handles deploying to a remote Docker host
type DockerDeployer struct {
	registry    string
	appName     string
	env         map[string]string
	remoteHost  string
	imagePrefix string
	awsRegion   string
}

// NewDockerDeployer creates a new Docker deployer
func NewDockerDeployer(appName, registry, remoteHost, port string, env map[string]string, imagePrefix string) (*DockerDeployer, error) {
	// Default prefix to "fyve-" if not provided
	if imagePrefix == "" {
		imagePrefix = "fyve-"
	}

	// Extract region from registry URL (format: account.dkr.ecr.region.amazonaws.com)
	awsRegion := "us-east-1" // Default region
	if region, ok := env["AWS_REGION"]; ok {
		awsRegion = region
	} else if strings.Contains(registry, ".amazonaws.com") {
		parts := strings.Split(registry, ".")
		if len(parts) >= 4 {
			awsRegion = parts[3]
		}
	}

	// Replace region template
	registry = strings.Replace(registry, "{region}", awsRegion, 1)

	// Load AWS config to get account ID if needed
	if strings.Contains(registry, "{aws_account_id}") || strings.Contains(registry, "aws_account_id") {
		ctx := context.Background()
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegion))
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config for region %s: %w", awsRegion, err)
		}

		// Get AWS account ID
		stsClient := sts.NewFromConfig(cfg)
		identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			return nil, fmt.Errorf("failed to get AWS account ID using region %s: %w", awsRegion, err)
		}

		accountID := *identity.Account
		// Replace both formats
		registry = strings.Replace(registry, "{aws_account_id}", accountID, 1)
		registry = strings.Replace(registry, "aws_account_id", accountID, 1)
	}

	return &DockerDeployer{
		registry:    registry,
		appName:     appName,
		env:         env,
		remoteHost:  remoteHost,
		imagePrefix: imagePrefix,
		awsRegion:   awsRegion,
	}, nil
}

// authenticateECR authenticates with ECR to get valid credentials for pulling images
func (d *DockerDeployer) authenticateECR() error {
	fmt.Println("Authenticating with ECR before pulling image...")

	ctx := context.Background()

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(d.awsRegion),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create ECR client
	ecrClient := ecr.NewFromConfig(cfg)

	// Get authorization token
	output, err := ecrClient.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return fmt.Errorf("failed to get ECR authorization token: %w", err)
	}

	if len(output.AuthorizationData) == 0 {
		return fmt.Errorf("no authorization data returned from ECR")
	}

	// Get auth data
	authData := output.AuthorizationData[0]
	authToken := *authData.AuthorizationToken

	// Decode the token
	decodedToken, err := base64.StdEncoding.DecodeString(authToken)
	if err != nil {
		return fmt.Errorf("failed to decode authorization token: %w", err)
	}

	// Format is "username:password"
	tokenParts := strings.Split(string(decodedToken), ":")
	if len(tokenParts) != 2 {
		return fmt.Errorf("invalid token format")
	}

	// username := tokenParts[0]
	password := tokenParts[1]

	// Login to Docker with the credentials
	// Use AWS CLI directly to ensure authentication
	// awsLoginCmd := exec.Command("aws", "ecr", "get-login-password", "--region", d.awsRegion)
	// awsPassword, err := awsLoginCmd.Output()
	// if err != nil {
	// 	return fmt.Errorf("failed to get ECR login password via AWS CLI: %w", err)
	// }

	// Login with docker
	dockerLoginCmd := exec.Command("docker", "login", "--username", "AWS", "--password-stdin", d.registry)
	dockerLoginCmd.Stdin = strings.NewReader(string(password))
	dockerLoginCmd.Stdout = os.Stdout
	dockerLoginCmd.Stderr = os.Stderr

	if err := dockerLoginCmd.Run(); err != nil {
		return fmt.Errorf("failed to login to ECR with all methods: %w", err)
	}

	fmt.Println("Successfully authenticated with ECR")
	return nil
}

// Deploy deploys the application to the remote Docker host
func (d *DockerDeployer) Deploy(environment string) error {
	// Use "latest" tag for production, environment name for other environments
	imageTag := environment
	if environment == "prod" {
		imageTag = "latest"
	}

	// Include the image prefix in the image name
	imageName := fmt.Sprintf("%s/%s%s:%s", d.registry, d.imagePrefix, d.appName, imageTag)
	containerName := fmt.Sprintf("%s-%s", d.appName, environment)

	fmt.Printf("Deploying image %s to %s environment\n", imageName, environment)

	// Authenticate with ECR before pulling
	if err := d.authenticateECR(); err != nil {
		return fmt.Errorf("failed to authenticate with ECR: %w", err)
	}

	// Prepare docker command with remote host if specified
	dockerCmd := "docker"
	if d.remoteHost != "" {
		dockerCmd = fmt.Sprintf("docker -H %s", d.remoteHost)
		fmt.Printf("Using remote Docker host: %s\n", d.remoteHost)
	} else {
		fmt.Println("Using local Docker daemon")
	}

	// Pull the image
	fmt.Println("Pulling image...")
	pullCmd := exec.Command("sh", "-c", fmt.Sprintf("%s pull %s", dockerCmd, imageName))
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr
	if err := pullCmd.Run(); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// Check if container already exists and remove it
	inspectCmd := exec.Command("sh", "-c", fmt.Sprintf("%s inspect %s", dockerCmd, containerName))
	if inspectCmd.Run() == nil {
		fmt.Printf("Container %s already exists, removing...\n", containerName)

		// Stop the container
		stopCmd := exec.Command("sh", "-c", fmt.Sprintf("%s stop %s", dockerCmd, containerName))
		stopCmd.Stdout = os.Stdout
		stopCmd.Stderr = os.Stderr
		if err := stopCmd.Run(); err != nil {
			return fmt.Errorf("failed to stop container: %w", err)
		}

		// Remove the container
		rmCmd := exec.Command("sh", "-c", fmt.Sprintf("%s rm %s", dockerCmd, containerName))
		rmCmd.Stdout = os.Stdout
		rmCmd.Stderr = os.Stderr
		if err := rmCmd.Run(); err != nil {
			return fmt.Errorf("failed to remove container: %w", err)
		}
	}

	// Create the base docker run command
	runCmdArgs := []string{}

	// Add host flag if remote host is specified
	if d.remoteHost != "" {
		runCmdArgs = append(runCmdArgs, "-H", d.remoteHost)
	}

	// Add run command and basic options
	runCmdArgs = append(runCmdArgs, "run", "-d", "--name", containerName)

	// Add environment variables
	for key, val := range d.env {
		runCmdArgs = append(runCmdArgs, "-e", fmt.Sprintf("%s=%s", key, val))
	}

	// Add NODE_ENV
	runCmdArgs = append(runCmdArgs, "-e", fmt.Sprintf("NODE_ENV=%s", environment))

	// Add Traefik labels for production environment
	if environment == "prod" {
		fmt.Println("Adding Traefik labels for production deployment...")

		domainName := fmt.Sprintf("%s.fyve.dev", d.appName)

		// Add labels with proper escaping
		runCmdArgs = append(runCmdArgs, "--label", "traefik.enable=true")
		runCmdArgs = append(runCmdArgs, "--label", fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", d.appName, domainName))
		runCmdArgs = append(runCmdArgs, "--label", fmt.Sprintf("traefik.http.routers.%s.tls.certresolver=default", d.appName))
		runCmdArgs = append(runCmdArgs, "--label", fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=3000", d.appName))
		runCmdArgs = append(runCmdArgs, "--label", fmt.Sprintf("traefik.http.routers.%s.entrypoints=websecure", d.appName))

		// Attach to "public" network for production deployments
		fmt.Println("Attaching container to 'public' network...")
		runCmdArgs = append(runCmdArgs, "--network", "public")
	}

	// Add restart policy and image name
	runCmdArgs = append(runCmdArgs, "--restart", "always", imageName)

	// Run the docker command
	fmt.Println("Starting container...")
	runCmd := exec.Command("docker", runCmdArgs...)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr

	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	fmt.Printf("Successfully deployed %s to %s environment\n", d.appName, environment)
	return nil
}

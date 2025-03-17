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
	port        string
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
	if strings.Contains(registry, ".amazonaws.com") {
		parts := strings.Split(registry, ".")
		if len(parts) >= 4 {
			awsRegion = parts[3]
		}
	}

	return &DockerDeployer{
		registry:    registry,
		appName:     appName,
		env:         env,
		port:        port,
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

	// Get AWS account ID if needed
	registry := d.registry
	if strings.Contains(registry, "aws_account_id") {
		stsClient := sts.NewFromConfig(cfg)
		identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			return fmt.Errorf("failed to get AWS account ID: %w", err)
		}
		accountID := *identity.Account
		registry = strings.Replace(registry, "aws_account_id", accountID, 1)
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
	dockerLoginCmd := exec.Command("docker", "login", "--username", "AWS", "--password-stdin", registry)
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
	// Include the image prefix in the image name
	imageName := fmt.Sprintf("%s/%s%s:%s", d.registry, d.imagePrefix, d.appName, "latest")
	containerName := fmt.Sprintf("%s-%s", d.appName, environment)

	fmt.Printf("Deploying image %s to %s environment\n", imageName, environment)

	// Authenticate with ECR before pulling
	if err := d.authenticateECR(); err != nil {
		return fmt.Errorf("failed to authenticate with ECR: %w", err)
	}

	// Prepare docker command with remote host
	dockerCmd := fmt.Sprintf("docker -H %s", d.remoteHost)

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
	runCmdArgs := []string{
		"-H", d.remoteHost,
		"run", "-d",
		"--name", containerName,
	}

	// Add port mapping if specified
	if d.port != "" {
		runCmdArgs = append(runCmdArgs, "-p", fmt.Sprintf("%s:3000", d.port))
	}

	// Add environment variables
	for key, val := range d.env {
		runCmdArgs = append(runCmdArgs, "-e", fmt.Sprintf("%s=%s", key, val))
	}

	// Add FYVE_ENV
	runCmdArgs = append(runCmdArgs, "-e", fmt.Sprintf("FYVE_ENV=%s", environment))

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

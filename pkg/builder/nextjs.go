package builder

import (
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

//go:embed nextjs.Dockerfile
var nextjsDockerfileFS embed.FS

// NextJSBuilder handles building NextJS applications using Docker
type NextJSBuilder struct {
	ProjectDir  string
	AppName     string
	ImageName   string
	Registry    string
	ImagePrefix string
	ecrClient   *ecr.Client
	awsRegion   string
	accountID   string
	ctx         context.Context
	Platform    string // Target platform for Docker build
	Environment string // Deployment environment
}

// NewNextJSBuilder creates a new NextJS builder
func NewNextJSBuilder(projectDir, appName, registry string, imagePrefix string, platform string, environment string) (*NextJSBuilder, error) {
	// Default prefix to "fyve-" if not provided
	if imagePrefix == "" {
		imagePrefix = "fyve-"
	}

	// Default platform to linux/amd64 if not provided
	if platform == "" {
		platform = "linux/amd64"
	}

	// Default environment to prod if not provided
	if environment == "" {
		environment = "prod"
	}

	// Use "latest" tag for production, environment name for other environments
	imageTag := environment
	if environment == "prod" {
		imageTag = "latest"
	}

	ctx := context.Background()

	// Extract region from registry URL (format: account.dkr.ecr.region.amazonaws.com)
	awsRegion := "us-east-1" // Default region
	if strings.Contains(registry, ".amazonaws.com") {
		parts := strings.Split(registry, ".")
		if len(parts) >= 4 {
			awsRegion = parts[3]
		}
	}

	// Replace region template with the extracted region
	registry = strings.Replace(registry, "{region}", awsRegion, 1)

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(awsRegion),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for region %s: %w", awsRegion, err)
	}

	// Create ECR client
	ecrClient := ecr.NewFromConfig(cfg)

	// Get AWS account ID if needed
	accountID := ""
	if strings.Contains(registry, "{aws_account_id}") || strings.Contains(registry, "aws_account_id") {
		stsClient := sts.NewFromConfig(cfg)
		identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			return nil, fmt.Errorf("failed to get AWS account ID using region %s. Make sure you have valid AWS credentials configured: %w", awsRegion, err)
		}
		accountID = *identity.Account
		// Replace both possible formats
		registry = strings.Replace(registry, "{aws_account_id}", accountID, 1)
		registry = strings.Replace(registry, "aws_account_id", accountID, 1)
	}

	return &NextJSBuilder{
		ProjectDir:  projectDir,
		AppName:     appName,
		ImageName:   fmt.Sprintf("%s%s:%s", imagePrefix, appName, imageTag),
		Registry:    registry,
		ImagePrefix: imagePrefix,
		ecrClient:   ecrClient,
		awsRegion:   awsRegion,
		accountID:   accountID,
		ctx:         ctx,
		Platform:    platform,
		Environment: environment,
	}, nil
}

// Build creates a Docker image for the NextJS application
func (b *NextJSBuilder) Build() error {
	fmt.Printf("Building Docker image for NextJS application (platform: %s)...\n", b.Platform)

	// Track temporary files to clean up
	var tempFiles []string
	defer func() {
		// Clean up any temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}
	}()

	// Check if Dockerfile exists, or use default one
	dockerfile := filepath.Join(b.ProjectDir, "Dockerfile")
	dockerfilePath := dockerfile

	if _, err := os.Stat(dockerfile); os.IsNotExist(err) {
		fmt.Println("No Dockerfile found, using default NextJS Dockerfile")

		// Create temporary Dockerfile in the project directory
		defaultDockerfileContent, err := nextjsDockerfileFS.ReadFile("nextjs.Dockerfile")
		if err != nil {
			return fmt.Errorf("failed to read default NextJS Dockerfile: %w", err)
		}

		// Write the default Dockerfile to the project directory temporarily
		tempDockerfile := filepath.Join(b.ProjectDir, "Dockerfile.fyve.tmp")
		if err := os.WriteFile(tempDockerfile, defaultDockerfileContent, 0644); err != nil {
			return fmt.Errorf("failed to write temporary Dockerfile: %w", err)
		}

		// Set the dockerfile path to the temporary one
		dockerfilePath = tempDockerfile
		tempFiles = append(tempFiles, tempDockerfile)
	}

	// Check if .dockerignore exists, or use default one
	dockerignore := filepath.Join(b.ProjectDir, ".dockerignore")
	if _, err := os.Stat(dockerignore); os.IsNotExist(err) {

		dockerignoreContent := []byte(`# Dependencies
node_modules
npm-debug.log
yarn-debug.log
yarn-error.log

.dockerignore

# Testing
coverage
.nyc_output

# Build
.next
out
build
dist

# Misc
.DS_Store

# Editor directories and files
.idea
.vscode
*.suo
*.ntvs*
*.njsproj
*.sln
*.sw?
`)

		if err := os.WriteFile(dockerignore, dockerignoreContent, 0644); err != nil {
			fmt.Printf("Warning: Failed to create .dockerignore file: %v\n", err)
			// Continue anyway, since we've added a safeguard in the Dockerfile
		} else {
			// Add to the list of files to clean up after building
			tempFiles = append(tempFiles, dockerignore)
		}
	}

	// Build Docker image with platform specified
	cmd := exec.Command("docker", "build",
		"--platform", b.Platform,
		"-f", dockerfilePath,
		"-t", b.ImageName,
		b.ProjectDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// GetECRRepositoryName returns the ECR repository name with the correct prefix
func (b *NextJSBuilder) GetECRRepositoryName() string {
	return fmt.Sprintf("%s%s", b.ImagePrefix, b.AppName)
}

// EnsureECRRepositoryExists creates the ECR repository if it doesn't exist
func (b *NextJSBuilder) EnsureECRRepositoryExists() error {
	fmt.Println("Ensuring ECR repository exists...")

	repositoryName := b.GetECRRepositoryName()

	// Check if repository exists
	_, err := b.ecrClient.DescribeRepositories(b.ctx, &ecr.DescribeRepositoriesInput{
		RepositoryNames: []string{repositoryName},
	})

	if err != nil {
		// Repository doesn't exist, create it
		fmt.Printf("Creating ECR repository '%s'...\n", repositoryName)

		_, err = b.ecrClient.CreateRepository(b.ctx, &ecr.CreateRepositoryInput{
			RepositoryName:     aws.String(repositoryName),
			ImageTagMutability: types.ImageTagMutabilityMutable,
			ImageScanningConfiguration: &types.ImageScanningConfiguration{
				ScanOnPush: true,
			},
		})

		if err != nil {
			return fmt.Errorf("failed to create ECR repository: %w", err)
		}

		fmt.Printf("ECR repository '%s' created successfully\n", repositoryName)
	} else {
		fmt.Printf("ECR repository '%s' already exists\n", repositoryName)
	}

	return nil
}

// GetECRAuthToken gets authorization token for ECR
func (b *NextJSBuilder) GetECRAuthToken() (username string, password string, endpoint string, err error) {
	output, err := b.ecrClient.GetAuthorizationToken(b.ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get ECR authorization token: %w", err)
	}

	if len(output.AuthorizationData) == 0 {
		return "", "", "", fmt.Errorf("no authorization data returned")
	}

	authData := output.AuthorizationData[0]
	authToken := *authData.AuthorizationToken
	endpoint = *authData.ProxyEndpoint

	// Decode the token
	decodedToken, err := base64.StdEncoding.DecodeString(authToken)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to decode authorization token: %w", err)
	}

	// Format is "username:password"
	tokenParts := strings.Split(string(decodedToken), ":")
	if len(tokenParts) != 2 {
		return "", "", "", fmt.Errorf("invalid token format")
	}

	return tokenParts[0], tokenParts[1], endpoint, nil
}

// GetECRRegistryURL returns the ECR registry URL
func (b *NextJSBuilder) GetECRRegistryURL() string {
	registry := b.Registry

	// Replace account ID placeholder if needed
	if strings.Contains(registry, "aws_account_id") && b.accountID != "" {
		registry = strings.Replace(registry, "aws_account_id", b.accountID, 1)
	}

	return registry
}

// PushToECR uploads the built image to AWS ECR
func (b *NextJSBuilder) PushToECR() error {
	// Get tag from image name
	imageParts := strings.Split(b.ImageName, ":")
	imageTag := "latest"
	if len(imageParts) > 1 {
		imageTag = imageParts[len(imageParts)-1]
	}

	fmt.Printf("Pushing Docker image to AWS ECR with tag '%s'...\n", imageTag)

	// Ensure ECR repository exists
	if err := b.EnsureECRRepositoryExists(); err != nil {
		return err
	}

	// Get ECR registry URL
	registry := b.GetECRRegistryURL()

	// Skip Docker login with AWS SDK and use AWS CLI directly
	// This avoids Docker credential storage issues
	fmt.Println("Authenticating with AWS ECR...")

	// Use AWS CLI to get the login command
	getLoginCmd := exec.Command("aws", "ecr", "get-login", "--no-include-email", "--region", b.awsRegion)
	loginOutput, err := getLoginCmd.Output()
	if err != nil {
		// If aws ecr get-login fails, try alternative AWS CLI command
		fmt.Println("Trying alternative ECR login method...")

		// Use AWS CLI to get the login password and pipe it to docker login
		passwordCmd := exec.Command("aws", "ecr", "get-login-password", "--region", b.awsRegion)
		passwordBytes, err := passwordCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get ECR login password: %w", err)
		}

		password := strings.TrimSpace(string(passwordBytes))
		loginCmd := exec.Command("docker", "login",
			"--username", "AWS",
			"--password-stdin",
			registry)

		loginCmd.Stdin = strings.NewReader(password)
		loginCmd.Stdout = os.Stdout
		loginCmd.Stderr = os.Stderr

		if err := loginCmd.Run(); err != nil {
			// One more fallback option - bypass Docker credential store completely
			fmt.Println("Trying last fallback ECR login method...")

			// Create a temporary file with the Docker config containing the auth token
			tempConfigDir, err := os.MkdirTemp("", "docker-config")
			if err != nil {
				return fmt.Errorf("failed to create temp directory: %w", err)
			}
			defer os.RemoveAll(tempConfigDir)

			// Create Docker config structure with ECR auth
			auth := base64.StdEncoding.EncodeToString([]byte("AWS:" + password))
			dockerConfig := fmt.Sprintf(`{
				"auths": {
					"%s": {
						"auth": "%s"
					}
				}
			}`, registry, auth)

			configPath := filepath.Join(tempConfigDir, "config.json")
			if err := os.WriteFile(configPath, []byte(dockerConfig), 0600); err != nil {
				return fmt.Errorf("failed to write Docker config: %w", err)
			}

			// Set DOCKER_CONFIG environment variable for subsequent Docker commands
			os.Setenv("DOCKER_CONFIG", tempConfigDir)

			fmt.Println("Using temporary Docker credentials configuration")
		}
	} else {
		// Execute the login command directly
		loginCmd := exec.Command("sh", "-c", string(loginOutput))
		loginCmd.Stdout = os.Stdout
		loginCmd.Stderr = os.Stderr

		if err := loginCmd.Run(); err != nil {
			return fmt.Errorf("failed to login to ECR: %w", err)
		}
	}

	// Tag the image for ECR
	ecrImage := fmt.Sprintf("%s/%s", registry, b.ImageName)
	tagCmd := exec.Command("docker", "tag", b.ImageName, ecrImage)
	tagCmd.Stdout = os.Stdout
	tagCmd.Stderr = os.Stderr
	if err := tagCmd.Run(); err != nil {
		return fmt.Errorf("failed to tag docker image: %w", err)
	}

	// Push to ECR
	pushCmd := exec.Command("docker", "push", ecrImage)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr

	return pushCmd.Run()
}

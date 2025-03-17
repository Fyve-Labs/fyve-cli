package builder

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
}

// NewNextJSBuilder creates a new NextJS builder
func NewNextJSBuilder(projectDir, appName, registry string, imagePrefix string) *NextJSBuilder {
	// Default prefix to "fyve-" if not provided
	if imagePrefix == "" {
		imagePrefix = "fyve-"
	}

	return &NextJSBuilder{
		ProjectDir:  projectDir,
		AppName:     appName,
		ImageName:   fmt.Sprintf("%s%s:%s", imagePrefix, appName, "latest"),
		Registry:    registry,
		ImagePrefix: imagePrefix,
	}
}

// Build creates a Docker image for the NextJS application
func (b *NextJSBuilder) Build() error {
	fmt.Println("Building Docker image for NextJS application...")

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

		// Ensure we clean up the temporary file when done
		defer os.Remove(tempDockerfile)
	}

	// Build Docker image
	cmd := exec.Command("docker", "build", "-f", dockerfilePath, "-t", b.ImageName, b.ProjectDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// EnsureECRRepositoryExists creates the ECR repository if it doesn't exist
func (b *NextJSBuilder) EnsureECRRepositoryExists() error {
	fmt.Println("Ensuring ECR repository exists...")

	// Extract repository name from registry URL (assuming format: account.dkr.ecr.region.amazonaws.com)
	repositoryName := fmt.Sprintf("%s-%s", b.ImagePrefix, b.AppName)

	// Check if repository exists using AWS CLI
	checkCmd := exec.Command("aws", "ecr", "describe-repositories", "--repository-names", repositoryName)

	// Redirect stderr to avoid printing errors when repository doesn't exist
	checkCmd.Stderr = nil

	if err := checkCmd.Run(); err != nil {
		// Repository doesn't exist, create it
		fmt.Printf("Creating ECR repository '%s'...\n", repositoryName)
		createCmd := exec.Command("aws", "ecr", "create-repository", "--repository-name", repositoryName)
		createCmd.Stdout = os.Stdout
		createCmd.Stderr = os.Stderr

		if err := createCmd.Run(); err != nil {
			return fmt.Errorf("failed to create ECR repository: %w", err)
		}

		fmt.Printf("ECR repository '%s' created successfully\n", repositoryName)
	} else {
		fmt.Printf("ECR repository '%s' already exists\n", repositoryName)
	}

	return nil
}

// PushToECR uploads the built image to AWS ECR
func (b *NextJSBuilder) PushToECR() error {
	fmt.Println("Pushing Docker image to AWS ECR...")

	// Ensure ECR repository exists
	if err := b.EnsureECRRepositoryExists(); err != nil {
		return err
	}

	// Get AWS account ID from registry URL if needed
	registry := b.Registry
	if strings.Contains(registry, "aws_account_id") {
		// We need to get the actual AWS account ID
		cmd := exec.Command("aws", "sts", "get-caller-identity", "--query", "Account", "--output", "text")
		accountIDBytes, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get AWS account ID: %w", err)
		}

		accountID := strings.TrimSpace(string(accountIDBytes))
		registry = strings.Replace(registry, "aws_account_id", accountID, 1)
	}

	// Login to ECR
	awsRegion := ""
	if strings.Contains(registry, ".amazonaws.com") {
		// Extract region from registry URL (format: account.dkr.ecr.region.amazonaws.com)
		parts := strings.Split(registry, ".")
		if len(parts) >= 4 {
			awsRegion = parts[3]
		}
	}

	if awsRegion == "" {
		awsRegion = "us-east-1" // Default region
	}

	loginCmd := exec.Command("aws", "ecr", "get-login-password", "--region", awsRegion)
	loginPassword, err := loginCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get ECR login password: %w", err)
	}

	dockerLoginCmd := exec.Command("docker", "login", "--username", "AWS", "--password-stdin", registry)
	dockerLoginCmd.Stdin = strings.NewReader(string(loginPassword))
	dockerLoginCmd.Stdout = os.Stdout
	dockerLoginCmd.Stderr = os.Stderr

	if err := dockerLoginCmd.Run(); err != nil {
		return fmt.Errorf("failed to login to ECR: %w", err)
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

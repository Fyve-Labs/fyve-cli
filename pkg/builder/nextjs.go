package builder

import (
	"context"
	"embed"
	"fmt"
	"github.com/fyve-labs/fyve-cli/pkg/config"
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
	config      *config.Build
	ImagePrefix string
	ctx         context.Context
	Environment string // Deployment environment
}

// NewNextJSBuilder creates a new NextJS builder
func NewNextJSBuilder(projectDir, appName, environment string, config *config.Build) (*NextJSBuilder, error) {
	return &NextJSBuilder{
		ProjectDir:  projectDir,
		AppName:     appName,
		Environment: environment,
		config:      config,
	}, nil
}

// Build creates a Docker image for the NextJS application
func (b *NextJSBuilder) Build() error {
	// Track temporary files to clean up
	var tempFiles []string
	defer func() {
		// Clean up any temporary files
		for _, file := range tempFiles {
			_ = os.Remove(file)
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

	for _, platform := range dockerBuildPlatforms() {
		cmd := exec.Command("docker", "build",
			"--platform", platform,
			"-f", dockerfilePath,
			"-t", platformImage(b.config.GetImage(), platform),
			b.ProjectDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to build image for %s: %w", platform, err)
		}
	}

	return nil
}

// PushToECR uploads the built image to AWS ECR
func (b *NextJSBuilder) PushToECR() error {
	taggedImage := b.config.GetImage()
	platforms := dockerBuildPlatforms()
	platformImages := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		image := platformImage(taggedImage, platform)
		if err := dockerPush(image); err != nil {
			return err
		}
		platformImages = append(platformImages, image)
	}

	if err := dockerManifestPush(taggedImage, platformImages); err != nil {
		return err
	}

	tagSeparator := strings.LastIndex(taggedImage, ":")
	if tagSeparator == -1 {
		return fmt.Errorf("PushToECR: invalid image format")
	}

	imageURL := taggedImage[:tagSeparator]
	imageTag := taggedImage[tagSeparator+1:]

	if imageTag != "latest" {
		latestImage := imageURL + ":latest"
		return dockerManifestPush(latestImage, platformImages)
	}

	return nil
}

func dockerBuildPlatforms() []string {
	platforms := "linux/amd64,linux/arm64"
	if val := os.Getenv("DOCKER_BUILD_PLATFORM"); val != "" {
		platforms = val
	}

	return strings.FieldsFunc(platforms, func(r rune) bool {
		return r == ',' || r == ' '
	})
}

func platformImage(image, platform string) string {
	return image + "-" + strings.ReplaceAll(platform, "/", "-")
}

func dockerManifestPush(image string, platformImages []string) error {
	createArgs := append([]string{"manifest", "create", image}, platformImages...)
	createCmd := exec.Command("docker", createArgs...)
	createCmd.Stdout = os.Stdout
	createCmd.Stderr = os.Stderr
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create manifest %s: %w", image, err)
	}

	pushCmd := exec.Command("docker", "manifest", "push", "--purge", image)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("failed to push manifest %s: %w", image, err)
	}

	return nil
}

func dockerPush(image string) error {
	pushCmd := exec.Command("docker", "push", image)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr

	return pushCmd.Run()
}

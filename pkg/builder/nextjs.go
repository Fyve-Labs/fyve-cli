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

// BuildAndPushToECR builds and publishes the NextJS application image.
func (b *NextJSBuilder) BuildAndPushToECR() error {
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

	args, err := dockerBuildArgs(b.ProjectDir, dockerfilePath, b.config.GetImage())
	if err != nil {
		return err
	}

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build and push image: %w", err)
	}

	return nil
}

func dockerBuildArgs(projectDir, dockerfilePath, image string) ([]string, error) {
	platforms := dockerBuildPlatforms()
	if len(platforms) == 0 {
		return nil, fmt.Errorf("DOCKER_BUILD_PLATFORM must contain at least one platform")
	}

	repository, imageTag, err := splitImageTag(image)
	if err != nil {
		return nil, err
	}
	if imageTag == "buildcache" {
		return nil, fmt.Errorf("image tag %q is reserved for the BuildKit cache", imageTag)
	}
	cacheImage := repository + ":buildcache"

	args := []string{
		"buildx", "build",
		"--platform", strings.Join(platforms, ","),
		"--file", dockerfilePath,
		"--tag", image,
	}
	if imageTag != "latest" {
		args = append(args, "--tag", repository+":latest")
	}
	args = append(args,
		"--cache-from", "type=registry,ref="+cacheImage,
		"--cache-to", "type=registry,ref="+cacheImage+",mode=max,image-manifest=true,oci-mediatypes=true",
		"--push",
		projectDir,
	)

	return args, nil
}

func splitImageTag(image string) (string, string, error) {
	tagSeparator := strings.LastIndex(image, ":")
	if tagSeparator <= strings.LastIndex(image, "/") || tagSeparator == len(image)-1 {
		return "", "", fmt.Errorf("invalid tagged image %q", image)
	}

	return image[:tagSeparator], image[tagSeparator+1:], nil
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

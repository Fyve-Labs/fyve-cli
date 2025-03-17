package deployer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// DockerDeployer handles deploying to a remote Docker host
type DockerDeployer struct {
	registry   string
	appName    string
	env        map[string]string
	port       string
	remoteHost string
}

// NewDockerDeployer creates a new Docker deployer
func NewDockerDeployer(appName, registry, remoteHost, port string, env map[string]string) (*DockerDeployer, error) {
	return &DockerDeployer{
		registry:   registry,
		appName:    appName,
		env:        env,
		port:       port,
		remoteHost: remoteHost,
	}, nil
}

// Deploy deploys the application to the remote Docker host
func (d *DockerDeployer) Deploy(environment string) error {
	imageName := fmt.Sprintf("%s/%s:%s", d.registry, d.appName, "latest")
	containerName := fmt.Sprintf("%s-%s", d.appName, environment)

	fmt.Printf("Deploying image %s to %s environment\n", imageName, environment)

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

	// Prepare environment variables
	envFlags := []string{}
	for key, val := range d.env {
		envFlags = append(envFlags, fmt.Sprintf("-e %s=%s", key, val))
	}

	// Add environment name
	envFlags = append(envFlags, fmt.Sprintf("-e NODE_ENV=%s", environment))
	envString := strings.Join(envFlags, " ")

	// Port mapping
	portMapping := ""
	if d.port != "" {
		portMapping = fmt.Sprintf("-p %s:3000", d.port)
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

	// Create and start the container
	runCmd := exec.Command("sh", "-c", fmt.Sprintf(
		"%s run -d --name %s %s %s --restart always %s",
		dockerCmd, containerName, portMapping, envString, imageName,
	))
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr

	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	fmt.Printf("Successfully deployed %s to %s environment\n", d.appName, environment)
	return nil
}

package deployer

import (
	"fmt"
	"github.com/fyve-labs/fyve-cli/pkg/config"
	"os"
	"os/exec"
)

// DockerDeployer handles deploying to a remote Docker host
type DockerDeployer struct {
	appName     string
	buildConfig *config.Build
	env         map[string]string
	appHost     string
	remoteHost  string
}

// NewDockerDeployer creates a new Docker deployer
func NewDockerDeployer(appName string, buildConfig *config.Build, remoteHost string, env map[string]string) (*DockerDeployer, error) {
	appHost := fmt.Sprintf("%s.%s", appName, "fyve.dev")
	if customAppHost := os.Getenv("CUSTOM_APP_HOST"); customAppHost != "" {
		appHost = customAppHost
	}

	return &DockerDeployer{
		appName:     appName,
		appHost:     appHost,
		buildConfig: buildConfig,
		env:         env,
		remoteHost:  remoteHost,
	}, nil
}

// Deploy deploys the application to the remote Docker host
func (d *DockerDeployer) Deploy(environment string) error {
	imageName := d.buildConfig.GetImage()
	containerName := fmt.Sprintf("%s-%s", d.appName, environment)
	fmt.Printf("Deploying image %s to %s environment\n", imageName, environment)

	// Prepare docker command with remote host if specified
	dockerCmd := "docker"
	if d.remoteHost != "" {
		dockerCmd = fmt.Sprintf("docker -H %s", d.remoteHost)
	}

	// Pull the image
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
	runCmdArgs = append(runCmdArgs, "-e", fmt.Sprintf("FYVE_ENV=%s", environment))

	routeName := fmt.Sprintf("%s-%s", d.appName, environment)
	// Add Traefik labels for production environment
	if environment == "prod" {
		fmt.Println("Adding Traefik labels for production deployment...")

		// Add labels with proper escaping
		runCmdArgs = append(runCmdArgs, "--label", "traefik.enable=true")
		runCmdArgs = append(runCmdArgs, "--label", fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", routeName, d.appHost))
		runCmdArgs = append(runCmdArgs, "--label", fmt.Sprintf("traefik.http.routers.%s.tls.certresolver=default", routeName))
		runCmdArgs = append(runCmdArgs, "--label", fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=3000", routeName))
		runCmdArgs = append(runCmdArgs, "--label", fmt.Sprintf("traefik.http.routers.%s.entrypoints=websecure", routeName))

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

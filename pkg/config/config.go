package config

import (
	"fmt"
	"os"

	"github.com/fyve-labs/fyve-cli/pkg/secrets"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	App string            `yaml:"app"`
	Env map[string]string `yaml:"env"`
}

// Load reads configuration from a YAML file
func Load(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	return &config, nil
}

// OverrideAppName allows overriding the app name from command line arguments
func (c *Config) OverrideAppName(appName string) {
	if appName != "" {
		c.App = appName
	}
}

// ProcessSecrets handles any secret references in configuration
func (c *Config) ProcessSecrets() error {
	// Get AWS region from environment variables or use default
	awsRegion := "us-east-1"
	if region, ok := c.Env["AWS_REGION"]; ok {
		awsRegion = region
	}

	// Create SSM manager
	secretManager, err := secrets.NewSSMManager(awsRegion)
	if err != nil {
		return fmt.Errorf("failed to initialize secrets manager: %w", err)
	}

	// Resolve secrets in environment variables
	resolvedEnv, err := secretManager.ProcessSecretRefs(c.Env)
	if err != nil {
		return fmt.Errorf("failed to process secrets: %w", err)
	}

	// Update environment variables with resolved secrets
	c.Env = resolvedEnv

	return nil
}

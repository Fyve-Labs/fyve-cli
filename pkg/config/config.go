package config

import (
	"fmt"
	"os"

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

func (c *Config) BuildConfig(environment string) *Build {
	return &Build{
		appName:     c.App,
		environment: environment,
	}
}

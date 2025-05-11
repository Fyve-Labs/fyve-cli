package config

import (
	"fmt"
	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultDomain      = "fyve.dev"
	defaultConfigFile  = "fyve.yaml"
	DefaultCnameTarget = "app-ingress.fyve.dev"
	defaultRegion      = "us-east-1"
	defaultRecordTTL   = 3600
)

// AppConfig represents the application configuration
type AppConfig struct {
	App string            `yaml:"app"`
	Env map[string]string `yaml:"env"`
}

// Load reads configuration from a YAML file
func Load(filePath string) (*AppConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config AppConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	return &config, nil
}

// OverrideAppName allows overriding the app name from command line arguments
func (c *AppConfig) OverrideAppName(appName string) {
	if appName != "" {
		c.App = appName
	}
}

func (c *AppConfig) BuildConfig(environment string) *Build {
	return &Build{
		appName:     c.App,
		environment: environment,
	}
}

type Config interface {
	// ConfigFile returns the location of the configuration file
	ConfigFile() string
	Region() string
}

type config struct {
	// configFile is the config file location
	configFile string
	region     string
}

func (c *config) ConfigFile() string {
	if c.configFile != "" {
		if !filepath.IsAbs(c.configFile) {
			pwd, _ := os.Getwd()
			c.configFile = filepath.Join(pwd, c.configFile)
		}

		return c.configFile
	}

	return defaultConfigFile
}

func (c *config) Region() string {
	if c.region != "" {
		return c.region
	}

	if val := os.Getenv("AWS_REGION"); val != "" {
		return val
	}

	if val := os.Getenv("AWS_DEFAULT_REGION"); val != "" {
		return val
	}

	return defaultRegion
}

// Config used for flag binding
var globalConfig = config{}

// GlobalConfig is the global configuration available for every sub-command
var GlobalConfig Config = &globalConfig

func BootstrapConfig() error {
	// Create a new FlagSet for the bootstrap flags and parse those. This will
	// initialize the config file to use (obtained via GlobalConfig.ConfigFile())
	bootstrapFlagSet := flag.NewFlagSet("fyve", flag.ContinueOnError)
	AddBootstrapFlags(bootstrapFlagSet)
	bootstrapFlagSet.ParseErrorsWhitelist = flag.ParseErrorsWhitelist{UnknownFlags: true}
	bootstrapFlagSet.Usage = func() {}
	err := bootstrapFlagSet.Parse(os.Args)
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		return err
	}

	viper.SetConfigFile(GlobalConfig.ConfigFile())
	viper.AutomaticEnv() // read in environment variables that match
	viper.SetEnvPrefix("FYVE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		// Config file not found; ignore error if desired
	}

	viper.SetDefault("domain", defaultDomain)
	viper.SetDefault("dns.ttl", defaultRecordTTL)
	return nil
}

func AddBootstrapFlags(flags *flag.FlagSet) {
	flags.StringVarP(&globalConfig.configFile, "config", "c", "", fmt.Sprintf("fyve configuration file (default: %s)", defaultConfigFile))
	flags.StringVar(&globalConfig.region, "region", "", "AWS region")
}

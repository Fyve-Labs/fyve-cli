package config

import (
	"errors"
	"fmt"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultDomain      = "fyve.dev"
	defaultConfigFile  = "fyve.yaml"
	DefaultCnameTarget = "app-ingress.fyve.dev"
	DefaultRegion      = "us-east-1"
	defaultRecordTTL   = 3600
)

// Config used for flag binding
var globalConfig = config{}

// GlobalConfig is the global configuration available for every sub-command
var GlobalConfig Config = &globalConfig

type Autoscaling struct {
	ScaledownDelay string `yaml:"delay"`
}

// AppConfig represents the application configuration
type AppConfig struct {
	App         string            `yaml:"app"`
	Region      string            `yaml:"region,omitempty"`
	Image       string            `yaml:"image"`
	Port        int32             `yaml:"port,omitempty"`
	Env         map[string]string `yaml:"env"`
	Autoscaling Autoscaling       `yaml:"autoscaling"`
}

func (c *AppConfig) Validate() error {
	if c.App == "" {
		return errors.New("missing app name")
	}

	if c.Port == 0 {
		c.Port = 80
	}

	if c.Autoscaling.ScaledownDelay == "" {
		c.Autoscaling.ScaledownDelay = "15m"
	}

	_, err := time.ParseDuration(c.Autoscaling.ScaledownDelay)

	return err
}

func (c *AppConfig) SkipBuild() bool {
	return len(c.Image) > 0
}

// LoadAppConfig reads configuration from a YAML file
func LoadAppConfig() (*AppConfig, error) {
	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			// Config file not found; ignore error if desired
		}
	}

	var config AppConfig

	err := viper.Unmarshal(&config)
	if err != nil {
		return nil, err
	}

	// viper makes all envvar keys lowercase, so we need to convert them back to uppercase
	config.Env = convertMapKeysToUppercase(config.Env)

	var c = &config
	return &config, c.Validate()
}

func (c *AppConfig) BuildConfig() *Build {
	return &Build{
		appName: c.App,
	}
}

type Config interface {
	// ConfigFile returns the location of the configuration file
	ConfigFile() string
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

	viper.SetDefault("domain", defaultDomain)
	viper.SetDefault("dns.ttl", defaultRecordTTL)
	viper.SetDefault("oidc.issuer.url", "https://dex.fyve.dev")

	return nil
}

func AddBootstrapFlags(flags *flag.FlagSet) {
	flags.StringVarP(&globalConfig.configFile, "config", "c", "", fmt.Sprintf("fyve configuration file (default: %s)", defaultConfigFile))
}

func convertMapKeysToUppercase(source map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range source {
		result[strings.ToUpper(k)] = v
	}

	return result
}

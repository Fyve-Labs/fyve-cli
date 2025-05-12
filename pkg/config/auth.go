package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// AuthConfig represents the authentication configuration
type AuthConfig struct {
	IDToken      string    `json:"id_token"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
}

// SaveAuthConfig saves the authentication configuration to ~/.fyve/config.json
func SaveAuthConfig(authConfig AuthConfig) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	fyveDir := filepath.Join(homeDir, ".fyve")
	if err := os.MkdirAll(fyveDir, 0700); err != nil {
		return err
	}

	configPath := filepath.Join(fyveDir, "config.json")

	// Marshal the auth config to YAML
	jsonData, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}

	// Write to the file
	if err := os.WriteFile(configPath, jsonData, 0600); err != nil {
		return err
	}

	return nil
}

func LoadAuthConfig() (*AuthConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	fyveDir := filepath.Join(homeDir, ".fyve")
	if err := os.MkdirAll(fyveDir, 0700); err != nil {
		return nil, err
	}

	configPath := filepath.Join(fyveDir, "config.json")

	bytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var authConfig AuthConfig
	err = json.Unmarshal(bytes, &authConfig)
	if err != nil {
		return nil, err
	}

	return &authConfig, nil
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Skyiesac/welp/providers"
)

// loadConfig loads the configuration from file
func loadConfig() *providers.Config {
	configPath := getConfigPath()
	if configPath == "" {
		return &providers.Config{Providers: make(map[string]string)}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		// File doesn't exist or can't be read, return empty config
		return &providers.Config{Providers: make(map[string]string)}
	}

	var config providers.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return &providers.Config{Providers: make(map[string]string)}
	}

	return &config
}

// saveConfig saves the configuration to file
func saveConfig(config *providers.Config) error {
	configPath := getConfigPath()
	if configPath == "" {
		return fmt.Errorf("could not determine config path")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Save with restricted permissions for security
	return os.WriteFile(configPath, data, 0600)
}

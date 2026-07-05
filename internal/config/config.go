package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DbURL           string `json:"db_url"`
	CurrentUsername string `json:"current_user_name"`
}

func getConfigFilePath() (string, error) {
	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error finding home directory of signed in user: %w", err)
	}
	absoluteConfigPath := filepath.Join(homeDirectory, ".gatorconfig.json")
	return absoluteConfigPath, nil
}

func Read() (Config, error) {
	var config Config
	absoluteConfigPath, err := getConfigFilePath()
	if err != nil {
		return config, err
	}
	configBytes, err := os.ReadFile(absoluteConfigPath)
	if err != nil {
		return config, fmt.Errorf("error reading file from file path: %w", err)
	}
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return config, fmt.Errorf("error parsing json to provided Config struct: %w", err)
	}
	return config, nil
}

func (cfg *Config) SetUser(username string) error {
	cfg.CurrentUsername = username
	updatedConfigBytes, err := json.MarshalIndent(cfg, "", " ")
	if err != nil {
		return fmt.Errorf("error converting config struct to bytes array: %w", err)
	}
	absoluteConfigPath, err := getConfigFilePath()
	if err != nil {
		return err
	}
	err = os.WriteFile(absoluteConfigPath, updatedConfigBytes, 0644)
	if err != nil {
		return fmt.Errorf("error writing to config json file: %w", err)
	}
	return nil
}

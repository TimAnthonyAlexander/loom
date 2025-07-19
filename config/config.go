package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config represents the loom configuration
type Config struct {
	Model       string `json:"model"`
	EnableShell bool   `json:"enable_shell"`
	MaxFileSize int64  `json:"max_file_size"` // Maximum file size to index in bytes
	APIKey      string `json:"api_key"`       // API key for LLM providers
	BaseURL     string `json:"base_url"`      // Base URL for LLM providers (optional)
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	return &Config{
		Model:       "openai:gpt-4o",
		EnableShell: false,
		MaxFileSize: 500 * 1024, // 500 KB default
	}
}

// LoadConfig loads configuration from global and local sources
func LoadConfig(workspacePath string) (*Config, error) {
	// Start with default config
	cfg := DefaultConfig()

	// Load global config
	globalCfg, err := loadGlobalConfig()
	if err == nil {
		mergeCfg(cfg, globalCfg)
	}

	// Load local config (takes precedence)
	localCfg, err := loadLocalConfig(workspacePath)
	if err == nil {
		mergeCfg(cfg, localCfg)
	}

	return cfg, nil
}

// Get retrieves a configuration value by key
func (c *Config) Get(key string) (interface{}, error) {
	switch key {
	case "model":
		return c.Model, nil
	case "enable_shell":
		return c.EnableShell, nil
	case "max_file_size":
		return c.MaxFileSize, nil
	case "api_key":
		return c.APIKey, nil
	case "base_url":
		return c.BaseURL, nil
	default:
		return nil, fmt.Errorf("unknown config key: %s", key)
	}
}

// Set updates a configuration value by key
func (c *Config) Set(key string, value interface{}) error {
	// Convert value to string (CLI input is always string)
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string value for %s", key)
	}

	switch key {
	case "model":
		c.Model = str
		return nil
	case "enable_shell":
		switch str {
		case "true":
			c.EnableShell = true
		case "false":
			c.EnableShell = false
		default:
			return fmt.Errorf("expected 'true' or 'false' for enable_shell, got: %s", str)
		}
		return nil
	case "max_file_size":
		val, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return fmt.Errorf("expected numeric value for max_file_size, got: %s", str)
		}
		c.MaxFileSize = val
		return nil
	case "api_key":
		c.APIKey = str
		return nil
	case "base_url":
		c.BaseURL = str
		return nil
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
}

// loadGlobalConfig loads configuration from ~/.loom/config.json
func loadGlobalConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".loom", "config.json")
	return loadConfigFromFile(configPath)
}

// loadLocalConfig loads configuration from <workspace>/.loom/config.json
func loadLocalConfig(workspacePath string) (*Config, error) {
	configPath := filepath.Join(workspacePath, ".loom", "config.json")
	return loadConfigFromFile(configPath)
}

// loadConfigFromFile loads configuration from a specific file
func loadConfigFromFile(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

// SaveLocalConfig saves configuration to <workspace>/.loom/config.json
func SaveLocalConfig(workspacePath string, cfg *Config) error {
	configPath := filepath.Join(workspacePath, ".loom", "config.json")

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// mergeCfg merges source config into destination config
func mergeCfg(dst, src *Config) {
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.APIKey != "" {
		dst.APIKey = src.APIKey
	}
	if src.BaseURL != "" {
		dst.BaseURL = src.BaseURL
	}
	// EnableShell is a bool, so we need to check if it was explicitly set
	// For simplicity, we'll always take the source value if the source exists
	dst.EnableShell = src.EnableShell
}

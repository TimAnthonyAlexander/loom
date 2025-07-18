package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
)

// Config represents the loom configuration
type Config struct {
	Model       string `json:"model"`
	EnableShell bool   `json:"enable_shell"`
	MaxFileSize int64  `json:"max_file_size"` // Maximum file size to index in bytes
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
	v := reflect.ValueOf(c).Elem()
	t := reflect.TypeOf(c).Elem()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == key {
			return v.Field(i).Interface(), nil
		}
	}

	return nil, fmt.Errorf("unknown config key: %s", key)
}

// Set updates a configuration value by key
func (c *Config) Set(key string, value interface{}) error {
	v := reflect.ValueOf(c).Elem()
	t := reflect.TypeOf(c).Elem()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == key {
			fieldValue := v.Field(i)
			switch fieldValue.Kind() {
			case reflect.String:
				if str, ok := value.(string); ok {
					fieldValue.SetString(str)
					return nil
				}
				return fmt.Errorf("expected string value for %s", key)
			case reflect.Bool:
				if str, ok := value.(string); ok {
					if str == "true" {
						fieldValue.SetBool(true)
						return nil
					} else if str == "false" {
						fieldValue.SetBool(false)
						return nil
					}
				}
				return fmt.Errorf("expected 'true' or 'false' for %s", key)
			case reflect.Int64:
				if str, ok := value.(string); ok {
					// Parse string to int64
					if val, err := strconv.ParseInt(str, 10, 64); err == nil {
						fieldValue.SetInt(val)
						return nil
					}
				}
				return fmt.Errorf("expected numeric value for %s", key)
			default:
				return fmt.Errorf("unsupported field type for %s", key)
			}
		}
	}

	return fmt.Errorf("unknown config key: %s", key)
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
	// EnableShell is a bool, so we need to check if it was explicitly set
	// For simplicity, we'll always take the source value if the source exists
	dst.EnableShell = src.EnableShell
}

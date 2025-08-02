package config

import (
	"encoding/json"
	"fmt"
	"loom/paths"
	"os"
	"strconv"
)

// ValidationConfig holds validation-specific configuration
type ValidationConfig struct {
	EnableLSP             bool     `json:"enable_lsp"`
	EnableVerification    bool     `json:"enable_verification"`
	ContextLines          int      `json:"context_lines"`
	RollbackOnSyntaxError bool     `json:"rollback_on_syntax_error"`
	LSPTimeoutSeconds     int      `json:"lsp_timeout_seconds"`
	SupportedLanguages    []string `json:"supported_languages"`
}

// Config represents the loom configuration
type Config struct {
	Model            string           `json:"model"`
	EnableShell      bool             `json:"enable_shell"`
	MaxFileSize      int64            `json:"max_file_size"`      // Maximum file size to index in bytes
	APIKey           string           `json:"api_key"`            // API key for LLM providers
	BaseURL          string           `json:"base_url"`           // Base URL for LLM providers (optional)
	LLMTimeout       int              `json:"llm_timeout"`        // LLM request timeout in seconds (default: 120)
	LLMStreamTimeout int              `json:"llm_stream_timeout"` // LLM streaming timeout in seconds (default: 300)
	LLMMaxRetries    int              `json:"llm_max_retries"`    // Maximum number of retries for failed LLM requests (default: 3)
	Validation       ValidationConfig `json:"validation"`         // Validation configuration
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	return &Config{
		Model:            "openai:gpt-4o",
		EnableShell:      false,
		MaxFileSize:      500 * 1024, // 500 KB default
		LLMTimeout:       120,        // 2 minutes default
		LLMStreamTimeout: 300,        // 5 minutes default for streaming
		LLMMaxRetries:    3,          // 3 retries default
		Validation: ValidationConfig{
			EnableLSP:             false, // Start disabled until LSP integration is complete
			EnableVerification:    true,  // Context extraction is always useful
			ContextLines:          8,
			RollbackOnSyntaxError: false, // Conservative default
			LSPTimeoutSeconds:     5,
			SupportedLanguages:    []string{"go", "typescript", "javascript", "python", "rust", "java"},
		},
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
	case "llm_timeout":
		return c.LLMTimeout, nil
	case "llm_stream_timeout":
		return c.LLMStreamTimeout, nil
	case "llm_max_retries":
		return c.LLMMaxRetries, nil
	case "validation.enable_lsp":
		return c.Validation.EnableLSP, nil
	case "validation.enable_verification":
		return c.Validation.EnableVerification, nil
	case "validation.context_lines":
		return c.Validation.ContextLines, nil
	case "validation.rollback_on_syntax_error":
		return c.Validation.RollbackOnSyntaxError, nil
	case "validation.lsp_timeout_seconds":
		return c.Validation.LSPTimeoutSeconds, nil
	case "validation.supported_languages":
		return c.Validation.SupportedLanguages, nil
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
	case "llm_timeout":
		val, err := strconv.Atoi(str)
		if err != nil {
			return fmt.Errorf("expected numeric value for llm_timeout, got: %s", str)
		}
		if val <= 0 {
			return fmt.Errorf("llm_timeout must be positive, got: %d", val)
		}
		c.LLMTimeout = val
		return nil
	case "llm_stream_timeout":
		val, err := strconv.Atoi(str)
		if err != nil {
			return fmt.Errorf("expected numeric value for llm_stream_timeout, got: %s", str)
		}
		if val <= 0 {
			return fmt.Errorf("llm_stream_timeout must be positive, got: %d", val)
		}
		c.LLMStreamTimeout = val
		return nil
	case "llm_max_retries":
		val, err := strconv.Atoi(str)
		if err != nil {
			return fmt.Errorf("expected numeric value for llm_max_retries, got: %s", str)
		}
		if val < 0 {
			return fmt.Errorf("llm_max_retries must be non-negative, got: %d", val)
		}
		c.LLMMaxRetries = val
		return nil
	case "validation.enable_lsp":
		switch str {
		case "true":
			c.Validation.EnableLSP = true
		case "false":
			c.Validation.EnableLSP = false
		default:
			return fmt.Errorf("expected 'true' or 'false' for validation.enable_lsp, got: %s", str)
		}
		return nil
	case "validation.enable_verification":
		switch str {
		case "true":
			c.Validation.EnableVerification = true
		case "false":
			c.Validation.EnableVerification = false
		default:
			return fmt.Errorf("expected 'true' or 'false' for validation.enable_verification, got: %s", str)
		}
		return nil
	case "validation.context_lines":
		val, err := strconv.Atoi(str)
		if err != nil {
			return fmt.Errorf("expected numeric value for validation.context_lines, got: %s", str)
		}
		if val < 0 {
			return fmt.Errorf("validation.context_lines must be non-negative, got: %d", val)
		}
		c.Validation.ContextLines = val
		return nil
	case "validation.rollback_on_syntax_error":
		switch str {
		case "true":
			c.Validation.RollbackOnSyntaxError = true
		case "false":
			c.Validation.RollbackOnSyntaxError = false
		default:
			return fmt.Errorf("expected 'true' or 'false' for validation.rollback_on_syntax_error, got: %s", str)
		}
		return nil
	case "validation.lsp_timeout_seconds":
		val, err := strconv.Atoi(str)
		if err != nil {
			return fmt.Errorf("expected numeric value for validation.lsp_timeout_seconds, got: %s", str)
		}
		if val <= 0 {
			return fmt.Errorf("validation.lsp_timeout_seconds must be positive, got: %d", val)
		}
		c.Validation.LSPTimeoutSeconds = val
		return nil
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
}

// loadGlobalConfig loads configuration from ~/.loom/config.json
func loadGlobalConfig() (*Config, error) {
	configPath, err := paths.GetGlobalConfigPath()
	if err != nil {
		return nil, err
	}

	return loadConfigFromFile(configPath)
}

// loadLocalConfig loads configuration from user loom directory for the project
func loadLocalConfig(workspacePath string) (*Config, error) {
	projectPaths, err := paths.NewProjectPaths(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create project paths: %w", err)
	}

	configPath := projectPaths.ConfigPath()
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

// SaveLocalConfig saves configuration to user loom directory for the project
func SaveLocalConfig(workspacePath string, cfg *Config) error {
	projectPaths, err := paths.NewProjectPaths(workspacePath)
	if err != nil {
		return fmt.Errorf("failed to create project paths: %w", err)
	}

	// Ensure project directories exist
	if err := projectPaths.EnsureProjectDir(); err != nil {
		return fmt.Errorf("failed to create project directories: %w", err)
	}

	configPath := projectPaths.ConfigPath()

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

	// Merge timeout settings if they're non-zero (explicitly set)
	if src.LLMTimeout > 0 {
		dst.LLMTimeout = src.LLMTimeout
	}
	if src.LLMStreamTimeout > 0 {
		dst.LLMStreamTimeout = src.LLMStreamTimeout
	}
	if src.LLMMaxRetries >= 0 {
		dst.LLMMaxRetries = src.LLMMaxRetries
	}

	// Merge validation configuration
	// Note: For boolean fields, we'll always take the source value if source config exists
	dst.Validation.EnableLSP = src.Validation.EnableLSP
	dst.Validation.EnableVerification = src.Validation.EnableVerification
	dst.Validation.RollbackOnSyntaxError = src.Validation.RollbackOnSyntaxError

	// Merge numeric validation settings if they're non-zero (explicitly set)
	if src.Validation.ContextLines > 0 {
		dst.Validation.ContextLines = src.Validation.ContextLines
	}
	if src.Validation.LSPTimeoutSeconds > 0 {
		dst.Validation.LSPTimeoutSeconds = src.Validation.LSPTimeoutSeconds
	}

	// Merge supported languages if explicitly set
	if len(src.Validation.SupportedLanguages) > 0 {
		dst.Validation.SupportedLanguages = src.Validation.SupportedLanguages
	}
}

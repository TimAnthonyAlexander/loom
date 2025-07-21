package llm

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// ConfigInterface represents the configuration interface that the factory needs
type ConfigInterface interface {
	Get(key string) (interface{}, error)
}

// CreateAdapterFromConfig creates an LLM adapter based on a configuration struct
func CreateAdapterFromConfig(cfg ConfigInterface) (LLMAdapter, error) {
	// Get model string
	modelInterface, err := cfg.Get("model")
	if err != nil {
		return nil, fmt.Errorf("failed to get model from config: %w", err)
	}
	modelStr, ok := modelInterface.(string)
	if !ok {
		return nil, fmt.Errorf("model config value is not a string")
	}

	// Get API key
	var apiKey string
	if apiKeyInterface, err := cfg.Get("api_key"); err == nil {
		if apiKeyStr, ok := apiKeyInterface.(string); ok {
			apiKey = apiKeyStr
		}
	}

	// Get base URL
	var baseURL string
	if baseURLInterface, err := cfg.Get("base_url"); err == nil {
		if baseURLStr, ok := baseURLInterface.(string); ok {
			baseURL = baseURLStr
		}
	}

	// Get timeout settings with defaults
	llmTimeout := 120 // Default 2 minutes
	if timeoutInterface, err := cfg.Get("llm_timeout"); err == nil {
		if timeoutInt, ok := timeoutInterface.(int); ok && timeoutInt > 0 {
			llmTimeout = timeoutInt
		}
	}

	llmStreamTimeout := 300 // Default 5 minutes
	if streamTimeoutInterface, err := cfg.Get("llm_stream_timeout"); err == nil {
		if streamTimeoutInt, ok := streamTimeoutInterface.(int); ok && streamTimeoutInt > 0 {
			llmStreamTimeout = streamTimeoutInt
		}
	}

	llmMaxRetries := 3 // Default 3 retries
	if maxRetriesInterface, err := cfg.Get("llm_max_retries"); err == nil {
		if maxRetriesInt, ok := maxRetriesInterface.(int); ok && maxRetriesInt >= 0 {
			llmMaxRetries = maxRetriesInt
		}
	}

	return createAdapterWithTimeouts(modelStr, apiKey, baseURL, 
		time.Duration(llmTimeout)*time.Second,
		time.Duration(llmStreamTimeout)*time.Second,
		llmMaxRetries)
}

// CreateAdapter creates an LLM adapter based on the model configuration (legacy version)
func CreateAdapter(modelStr, apiKey, baseURL string) (LLMAdapter, error) {
	return createAdapterWithTimeouts(modelStr, apiKey, baseURL, DefaultTimeout, DefaultStreamTimeout, DefaultMaxRetries)
}

// createAdapterWithTimeouts is the internal function that creates adapters with specific timeout settings
func createAdapterWithTimeouts(modelStr, apiKey, baseURL string, timeout, streamTimeout time.Duration, maxRetries int) (LLMAdapter, error) {
	parts := strings.SplitN(modelStr, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid model format: %s (expected provider:model)", modelStr)
	}

	provider := parts[0]
	model := parts[1]

	config := AdapterConfig{
		Model:          model,
		APIKey:         apiKey,
		BaseURL:        baseURL,
		Timeout:        timeout,
		StreamTimeout:  streamTimeout,
		MaxRetries:     maxRetries,
		RetryDelayBase: DefaultRetryDelayBase,
	}

	switch provider {
	case "openai":
		if config.APIKey == "" {
			// Try to get from environment
			config.APIKey = os.Getenv("OPENAI_API_KEY")
		}
		if config.APIKey == "" {
			return nil, fmt.Errorf("OpenAI API key not provided (set OPENAI_API_KEY environment variable or configure via loom config)")
		}
		return NewOpenAIAdapter(config), nil

	case "claude":
		if config.APIKey == "" {
			// Try to get from environment
			config.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if config.APIKey == "" {
			return nil, fmt.Errorf("Claude API key not provided (set ANTHROPIC_API_KEY environment variable or configure via loom config)")
		}
		return NewClaudeAdapter(config), nil

	case "ollama":
		return NewOllamaAdapter(config), nil

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s (supported: openai, claude, ollama)", provider)
	}
}

// GetProviderFromModel extracts the provider from a model string
func GetProviderFromModel(modelStr string) string {
	parts := strings.SplitN(modelStr, ":", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return "unknown"
}

// GetModelFromModel extracts the model name from a model string
func GetModelFromModel(modelStr string) string {
	parts := strings.SplitN(modelStr, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return modelStr
}

package llm

import (
	"fmt"
	"os"
	"strings"
)

// CreateAdapter creates an LLM adapter based on the model configuration
func CreateAdapter(modelStr, apiKey, baseURL string) (LLMAdapter, error) {
	parts := strings.SplitN(modelStr, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid model format: %s (expected provider:model)", modelStr)
	}

	provider := parts[0]
	model := parts[1]

	config := AdapterConfig{
		Model:   model,
		APIKey:  apiKey,
		BaseURL: baseURL,
		Timeout: DefaultTimeout,
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

	case "ollama":
		return NewOllamaAdapter(config), nil

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s (supported: openai, ollama)", provider)
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

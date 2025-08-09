package adapter

import (
	"errors"
	"os"
	"strings"

	"github.com/loom/loom/internal/adapter/anthropic"
	"github.com/loom/loom/internal/adapter/ollama"
	"github.com/loom/loom/internal/adapter/openai"
	responses "github.com/loom/loom/internal/adapter/openai/responses"
	"github.com/loom/loom/internal/engine"
)

// Provider represents the type of LLM provider.
type Provider string

const (
	// OpenAI provider (e.g., GPT-4o)
	ProviderOpenAI Provider = "openai"

	// Anthropic provider (e.g., Claude)
	ProviderAnthropic Provider = "anthropic"

	// Ollama provider for local models
	ProviderOllama Provider = "ollama"
)

// Config holds configuration for an LLM adapter.
type Config struct {
	Provider Provider
	Model    string
	APIKey   string
	Endpoint string // For custom endpoints (e.g., Azure OpenAI or Ollama)
}

// DefaultConfig returns a configuration based on environment variables.
func DefaultConfig() Config {
	// Default to OpenAI
	config := Config{
		Provider: ProviderOpenAI,
		Model:    "gpt-4o",
	}

	// Check for model selection with provider prefix (e.g., "claude:claude-opus-4-20250514")
	if model := os.Getenv("LOOM_MODEL"); model != "" {
		if strings.Contains(model, ":") {
			provider, modelID, err := GetProviderFromModel(model)
			if err == nil {
				config.Provider = provider
				config.Model = modelID
			}
		} else {
			// Backward compatibility for model without provider prefix
			config.Model = model
		}
	}

	// Check for explicit provider selection (overrides model-based provider)
	if provider := os.Getenv("LOOM_PROVIDER"); provider != "" {
		config.Provider = Provider(provider)
	}

	// Get API key based on provider
	switch config.Provider {
	case ProviderOpenAI:
		config.APIKey = os.Getenv("OPENAI_API_KEY")
	case ProviderAnthropic:
		config.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	case ProviderOllama:
		if endpoint := os.Getenv("OLLAMA_ENDPOINT"); endpoint != "" {
			config.Endpoint = endpoint
		} else {
			config.Endpoint = "http://localhost:11434"
		}
	}

	// Check for custom endpoint
	if endpoint := os.Getenv("LOOM_ENDPOINT"); endpoint != "" {
		config.Endpoint = endpoint
	}

	return config
}

// New creates a new LLM adapter based on configuration.
func New(config Config) (engine.LLM, error) {
	switch config.Provider {
	case ProviderOpenAI:
		if config.APIKey == "" {
			return nil, errors.New("OpenAI API key not set. Set the OPENAI_API_KEY environment variable")
		}
		// Prefer Responses API for o-series and gpt-5 models, or when explicitly requested
		useResponses := false
		m := strings.ToLower(strings.TrimSpace(config.Model))
		if strings.HasPrefix(m, "o3") || strings.HasPrefix(m, "o4") || strings.HasPrefix(m, "gpt-5") {
			useResponses = true
		}
		if v := os.Getenv("OPENAI_USE_RESPONSES"); v != "" {
			if vv := strings.ToLower(strings.TrimSpace(v)); vv == "1" || vv == "true" || vv == "yes" || vv == "on" {
				useResponses = true
			}
		}
		if useResponses {
			return responses.New(config.APIKey, config.Model), nil
		}
		return openai.New(config.APIKey, config.Model), nil

	case ProviderAnthropic:
		if config.APIKey == "" {
			return nil, errors.New("Anthropic API key not set. Set the ANTHROPIC_API_KEY environment variable")
		}
		return anthropic.New(config.APIKey, config.Model), nil

	case ProviderOllama:
		baseURL := config.Endpoint
		if baseURL == "" {
			baseURL = "http://localhost:11434/v1/chat/completions"
		}
		return ollama.New(baseURL, config.Model), nil

	default:
		return nil, errors.New("unknown LLM provider")
	}
}

// Removed safeSubstring helper and debug prints

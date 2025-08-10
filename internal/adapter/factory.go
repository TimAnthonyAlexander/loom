package adapter

import (
	"errors"
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

// DefaultConfig returns a conservative default configuration.
func DefaultConfig() Config {
	// Default to OpenAI
	config := Config{
		Provider: ProviderOpenAI,
		Model:    "gpt-4o",
	}

	return config
}

// New creates a new LLM adapter based on configuration.
func New(config Config) (engine.LLM, error) {
	switch config.Provider {
	case ProviderOpenAI:
		if config.APIKey == "" {
			return nil, errors.New("OpenAI API key not set. Set it in Settings.")
		}
		// Prefer Responses API for o-series and gpt-5 models, or when explicitly requested
		useResponses := false
		m := strings.ToLower(strings.TrimSpace(config.Model))
		if strings.HasPrefix(m, "o3") || strings.HasPrefix(m, "o4") || strings.HasPrefix(m, "gpt-5") {
			useResponses = true
		}
		// Environment toggle removed; rely on model prefix to choose responses client
		if useResponses {
			return responses.New(config.APIKey, config.Model), nil
		}
		return openai.New(config.APIKey, config.Model), nil

	case ProviderAnthropic:
		if config.APIKey == "" {
			return nil, errors.New("Anthropic API key not set. Set it in Settings.")
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

package adapter

import (
	"fmt"
	"strings"
)

// Model represents an LLM model with its provider prefix and ID
type Model struct {
	ProviderPrefix string // e.g., "claude", "openai", "ollama"
	ID             string // e.g., "claude-opus-4-20250514", "gpt-4.1", "llama3.1:8b"
}

// String returns the formatted model string with provider prefix
func (m Model) String() string {
	return fmt.Sprintf("%s:%s", m.ProviderPrefix, m.ID)
}

// ParseModel parses a model string in the format "provider:model_id"
func ParseModel(modelString string) (Model, error) {
	parts := strings.SplitN(modelString, ":", 2)
	if len(parts) != 2 {
		return Model{}, fmt.Errorf("invalid model format: %s (expected provider:model_id)", modelString)
	}

	providerPrefix := strings.TrimSpace(parts[0])
	modelID := strings.TrimSpace(parts[1])

	if providerPrefix == "" || modelID == "" {
		return Model{}, fmt.Errorf("both provider and model ID must be specified")
	}

	return Model{
		ProviderPrefix: providerPrefix,
		ID:             modelID,
	}, nil
}

// AvailableModels returns the list of supported models
func AvailableModels() []Model {
	// List of supported models with provider prefixes
	models := []string{
		"claude:claude-opus-4-20250514",
		"claude:claude-sonnet-4-20250514",
		"openai:gpt-4.1",
		"openai:o4-mini",
		"openai:o3",
		"ollama:llama3.1:8b",
		"ollama:llama3:8b",
		"ollama:gpt-oss:20b",
		"ollama:qwen3:8b",
		"ollama:gemma3:12b",
		"ollama:mistral:7b",
		"ollama:deepseek-r1:70b",
	}

	result := make([]Model, 0, len(models))

	for _, modelString := range models {
		model, err := ParseModel(modelString)
		if err == nil {
			result = append(result, model)
		}
	}

	return result
}

// GetProviderFromModel determines the provider type from a model string
func GetProviderFromModel(modelString string) (Provider, string, error) {
	model, err := ParseModel(modelString)
	if err != nil {
		return "", "", err
	}

	var provider Provider
	switch strings.ToLower(model.ProviderPrefix) {
	case "openai":
		provider = ProviderOpenAI
	case "claude":
		provider = ProviderAnthropic
	case "ollama":
		provider = ProviderOllama
	default:
		return "", "", fmt.Errorf("unknown provider: %s", model.ProviderPrefix)
	}

	return provider, model.ID, nil
}

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Settings represents persisted application settings such as API keys and endpoints.
type Settings struct {
	OpenAIAPIKey    string `json:"openai_api_key"`
	AnthropicAPIKey string `json:"anthropic_api_key"`
	OllamaEndpoint  string `json:"ollama_endpoint,omitempty"`
	LastWorkspace   string `json:"last_workspace,omitempty"`
	// Feature flags
	AutoApproveShell bool `json:"auto_approve_shell,omitempty"`
	AutoApproveEdits bool `json:"auto_approve_edits,omitempty"`
}

// settingsFilePath returns the absolute path to the settings JSON file
// using the OS-specific user config directory, under "loom/settings.json".
func settingsFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user config dir: %w", err)
	}
	loomDir := filepath.Join(configDir, "loom")
	return filepath.Join(loomDir, "settings.json"), nil
}

// ensureDir ensures the parent directory for the provided file path exists.
func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return errors.New("invalid directory for settings file")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil { // restrict permissions
		return fmt.Errorf("failed to create settings directory: %w", err)
	}
	return nil
}

// Load reads settings from disk. If the settings file doesn't exist, it returns an empty Settings.
func Load() (Settings, error) {
	path, err := settingsFilePath()
	if err != nil {
		return Settings{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Settings{}, nil
		}
		return Settings{}, fmt.Errorf("failed to read settings: %w", err)
	}
	var s Settings
	if len(data) == 0 {
		return Settings{}, nil
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, fmt.Errorf("failed to parse settings: %w", err)
	}
	return s, nil
}

// Save writes settings to disk (overwriting any previous file) with restricted permissions.
func Save(s Settings) error {
	path, err := settingsFilePath()
	if err != nil {
		return err
	}
	if err := ensureDir(path); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode settings: %w", err)
	}
	// Use 0600 permissions to keep secrets private
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}
	return nil
}

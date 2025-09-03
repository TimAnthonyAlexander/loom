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
	OpenAIAPIKey     string `json:"openai_api_key"`
	AnthropicAPIKey  string `json:"anthropic_api_key"`
	OpenRouterAPIKey string `json:"openrouter_api_key"`
	OllamaEndpoint   string `json:"ollama_endpoint,omitempty"`
	LastWorkspace    string `json:"last_workspace,omitempty"`
	// Last selected model in the format "provider:model_id"
	LastModel string `json:"last_model,omitempty"`
	// Feature flags
	AutoApproveShell bool `json:"auto_approve_shell,omitempty"`
	AutoApproveEdits bool `json:"auto_approve_edits,omitempty"`
	// UI preferences
	Theme string `json:"theme,omitempty"`
	// Recent workspaces (max 10, ordered from most recent)
	RecentWorkspaces []string `json:"recent_workspaces,omitempty"`
}

// settingsFilePath returns the absolute path to the settings JSON file
// under the user's home directory at ~/.loom/settings.json
func settingsFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve HOME: %w", err)
	}
	loomDir := filepath.Join(home, ".loom")
	return filepath.Join(loomDir, "settings.json"), nil
}

// legacySettingsFilePath returns the old settings location under the OS config dir
// e.g. macOS: ~/Library/Application Support/loom/settings.json
func legacySettingsFilePath() (string, error) {
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
	// Prefer new location
	newPath, err := settingsFilePath()
	if err != nil {
		return Settings{}, err
	}
	data, err := os.ReadFile(newPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Try legacy location
			legacyPath, lerr := legacySettingsFilePath()
			if lerr == nil {
				if legacyData, rerr := os.ReadFile(legacyPath); rerr == nil && len(legacyData) > 0 {
					var legacy Settings
					if jerr := json.Unmarshal(legacyData, &legacy); jerr == nil {
						// Migrate by saving to new path
						_ = Save(legacy)
						return legacy, nil
					}
				}
			}
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

// AddRecentWorkspace adds a workspace path to the recent workspaces list.
// If the path already exists, it moves it to the front. If the list exceeds maxRecent,
// it trims the oldest entries.
func (s *Settings) AddRecentWorkspace(path string, maxRecent int) {
	if path == "" {
		return
	}

	// Normalize the path
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	// Remove existing entry if present
	for i, existing := range s.RecentWorkspaces {
		if existing == path {
			s.RecentWorkspaces = append(s.RecentWorkspaces[:i], s.RecentWorkspaces[i+1:]...)
			break
		}
	}

	// Add to front
	s.RecentWorkspaces = append([]string{path}, s.RecentWorkspaces...)

	// Trim to max length
	if len(s.RecentWorkspaces) > maxRecent {
		s.RecentWorkspaces = s.RecentWorkspaces[:maxRecent]
	}
}

// RemoveRecentWorkspace removes a workspace path from the recent workspaces list.
func (s *Settings) RemoveRecentWorkspace(path string) {
	if path == "" {
		return
	}

	// Normalize the path for comparison
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	for i, existing := range s.RecentWorkspaces {
		if existing == path {
			s.RecentWorkspaces = append(s.RecentWorkspaces[:i], s.RecentWorkspaces[i+1:]...)
			break
		}
	}
}

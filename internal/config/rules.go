package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadRules reads both user and project rules.
// User rules live at $HOME/.loom/rules.json
// Project rules live at <workspace>/.loom/rules.json
func LoadRules(workspacePath string) (userRules []string, projectRules []string, _ error) {
	// User rules
	u, err := loadUserRules()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("failed to load user rules: %w", err)
	}

	// Project rules
	p, err := loadProjectRules(workspacePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("failed to load project rules: %w", err)
	}

	// Normalize: trim whitespace and drop empty
	userRules = normalizeRules(u)
	projectRules = normalizeRules(p)
	return userRules, projectRules, nil
}

// SaveUserRules writes user rules to $HOME/.loom/rules.json
func SaveUserRules(rules []string) error {
	path, err := userRulesPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create rules directory: %w", err)
	}
	normalized := normalizeRules(rules)
	if err := writeRulesFile(path, normalized); err != nil {
		return err
	}
	return nil
}

// SaveProjectRules writes project rules to <workspace>/.loom/rules.json
func SaveProjectRules(workspacePath string, rules []string) error {
	if workspacePath == "" {
		return errors.New("workspace path is empty")
	}
	// Normalize workspace path to avoid writing to literal "~" paths
	workspacePath = expandUserHome(workspacePath)
	if abs, err := filepath.Abs(workspacePath); err == nil {
		workspacePath = abs
	}
	path := filepath.Join(workspacePath, ".loom", "rules.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create project rules directory: %w", err)
	}
	normalized := normalizeRules(rules)
	if err := writeRulesFile(path, normalized); err != nil {
		return err
	}
	return nil
}

func loadUserRules() ([]string, error) {
	path, err := userRulesPath()
	if err != nil {
		return nil, err
	}
	return readRulesFile(path)
}

func userRulesPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve HOME: %w", err)
	}
	return filepath.Join(home, ".loom", "rules.json"), nil
}

func loadProjectRules(workspacePath string) ([]string, error) {
	if workspacePath == "" {
		return nil, errors.New("workspace path is empty")
	}
	// Expand ~ and normalize abs path for robustness
	workspacePath = expandUserHome(workspacePath)
	if abs, err := filepath.Abs(workspacePath); err == nil {
		workspacePath = abs
	}
	path := filepath.Join(workspacePath, ".loom", "rules.json")
	return readRulesFile(path)
}

func readRulesFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, os.ErrNotExist
		}
		return nil, err
	}
	// Expect a simple JSON array of strings
	var rules []string
	if len(data) == 0 {
		return []string{}, nil
	}
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("failed to parse rules file '%s': %w", path, err)
	}
	return rules, nil
}

func writeRulesFile(path string, rules []string) error {
	b, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode rules: %w", err)
	}
	// Restrictive perms for user rules; project rules inherit umask
	mode := os.FileMode(0o600)
	if strings.Contains(path, string(os.PathSeparator)+".loom"+string(os.PathSeparator)) && !strings.HasPrefix(path, os.TempDir()) {
		// For project files, 0644 is fine
		mode = 0o644
	}
	if err := os.WriteFile(path, b, mode); err != nil {
		return fmt.Errorf("failed to write rules: %w", err)
	}
	return nil
}

func normalizeRules(rules []string) []string {
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		trimmed := strings.TrimSpace(r)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

// expandUserHome expands a leading ~ or ~/ to the user's home directory.
func expandUserHome(p string) string {
	if p == "" {
		return p
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

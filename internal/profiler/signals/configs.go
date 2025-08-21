package signals

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/loom/loom/internal/profiler/shared"
)

// ConfigsExtractor extracts information from configuration files
type ConfigsExtractor struct {
	root string
}

// NewConfigsExtractor creates a new configs extractor
func NewConfigsExtractor(root string) *ConfigsExtractor {
	return &ConfigsExtractor{root: root}
}

// Extract processes configuration files and returns signals
func (c *ConfigsExtractor) Extract(files []*shared.FileInfo, existing *shared.SignalData) {
	for _, file := range files {
		if file.IsConfig {
			c.processConfigFile(file, existing)
		}
	}
}

// processConfigFile processes a single configuration file
func (c *ConfigsExtractor) processConfigFile(file *shared.FileInfo, signals *shared.SignalData) {
	configFile := shared.ConfigFile{
		Tool: c.detectConfigTool(file.Basename, file.Path),
		Path: file.Path,
	}

	// Add to configs if not already present
	found := false
	for _, existing := range signals.Configs {
		if existing.Path == configFile.Path {
			found = true
			break
		}
	}

	if !found {
		signals.Configs = append(signals.Configs, configFile)
	}

	// Extract additional information based on config type
	switch configFile.Tool {
	case "docker", "docker-compose":
		c.extractDockerConfig(file.Path, signals)
	case "vite", "webpack", "rollup":
		c.extractBundlerConfig(file.Path, signals)
	case "eslint", "prettier", "babel":
		c.extractLinterConfig(file.Path, signals)
	case "jest", "vitest":
		c.extractTestConfig(file.Path, signals)
	case "typescript":
		c.extractTypeScriptConfig(file.Path, signals)
	}
}

// detectConfigTool determines the tool for a config file
func (c *ConfigsExtractor) detectConfigTool(basename, path string) string {
	lower := strings.ToLower(basename)
	lowerPath := strings.ToLower(path)

	// Direct tool mapping
	toolMap := map[string]string{
		"package.json":         "npm",
		"package-lock.json":    "npm",
		"yarn.lock":            "yarn",
		"pnpm-lock.yaml":       "pnpm",
		"composer.json":        "composer",
		"composer.lock":        "composer",
		"go.mod":               "go",
		"go.sum":               "go",
		"cargo.toml":           "cargo",
		"cargo.lock":           "cargo",
		"pyproject.toml":       "python",
		"requirements.txt":     "pip",
		"poetry.lock":          "poetry",
		"gemfile":              "ruby",
		"gemfile.lock":         "ruby",
		"wails.json":           "wails",
		"wails.toml":           "wails",
		"dockerfile":           "docker",
		"docker-compose.yml":   "docker-compose",
		"docker-compose.yaml":  "docker-compose",
		"makefile":             "make",
		"justfile":             "just",
		"taskfile.yml":         "task",
		"taskfile.yaml":        "task",
		"procfile":             "heroku",
		"vercel.json":          "vercel",
		"netlify.toml":         "netlify",
		"vite.config.ts":       "vite",
		"vite.config.js":       "vite",
		"vite.config.mjs":      "vite",
		"webpack.config.js":    "webpack",
		"webpack.config.ts":    "webpack",
		"rollup.config.js":     "rollup",
		"rollup.config.ts":     "rollup",
		"tsconfig.json":        "typescript",
		"tsconfig.base.json":   "typescript",
		"jsconfig.json":        "javascript",
		"babel.config.js":      "babel",
		"babel.config.json":    "babel",
		".babelrc":             "babel",
		".babelrc.js":          "babel",
		".babelrc.json":        "babel",
		"jest.config.js":       "jest",
		"jest.config.ts":       "jest",
		"jest.config.json":     "jest",
		"vitest.config.ts":     "vitest",
		"vitest.config.js":     "vitest",
		"playwright.config.ts": "playwright",
		"cypress.config.js":    "cypress",
		"phpunit.xml":          "phpunit",
		"phpstan.neon":         "phpstan",
		"pint.json":            "pint",
		".editorconfig":        "editorconfig",
		".gitignore":           "git",
		".dockerignore":        "docker",
		".env":                 "env",
		".env.local":           "env",
		".env.example":         "env",
	}

	if tool, exists := toolMap[lower]; exists {
		return tool
	}

	// Pattern matching for config files
	if strings.Contains(lower, "eslint") {
		return "eslint"
	}
	if strings.Contains(lower, "prettier") {
		return "prettier"
	}
	if strings.Contains(lower, "tsconfig") {
		return "typescript"
	}
	if strings.Contains(lower, "webpack") {
		return "webpack"
	}
	if strings.Contains(lower, "rollup") {
		return "rollup"
	}
	if strings.Contains(lower, "vite") {
		return "vite"
	}
	if strings.Contains(lower, "jest") {
		return "jest"
	}
	if strings.Contains(lower, "vitest") {
		return "vitest"
	}
	if strings.Contains(lower, "cypress") {
		return "cypress"
	}
	if strings.Contains(lower, "playwright") {
		return "playwright"
	}
	if strings.Contains(lowerPath, "docker") {
		return "docker"
	}

	// Check for specific config patterns
	if strings.HasSuffix(lower, ".config.js") || strings.HasSuffix(lower, ".config.ts") {
		return "config"
	}
	if strings.HasPrefix(lower, ".") && (strings.Contains(lower, "rc") || strings.Contains(lower, "config")) {
		return "config"
	}

	return "unknown"
}

// extractDockerConfig extracts information from Docker configurations
func (c *ConfigsExtractor) extractDockerConfig(path string, signals *shared.SignalData) {
	// Mark this as an infrastructure entrypoint
	entrypoint := shared.EntryPoint{
		Path:  path,
		Kind:  "infra",
		Hints: []string{"docker"},
	}

	// Check if entrypoint already exists
	found := false
	for _, existing := range signals.Entrypoints {
		if existing.Path == entrypoint.Path {
			found = true
			break
		}
	}

	if !found {
		signals.Entrypoints = append(signals.Entrypoints, entrypoint)
	}
}

// extractBundlerConfig extracts information from bundler configurations
func (c *ConfigsExtractor) extractBundlerConfig(path string, signals *shared.SignalData) {
	// Try to read the config file to extract entry points
	data, err := os.ReadFile(filepath.Join(c.root, path))
	if err != nil {
		return
	}

	content := string(data)

	// Look for common entry patterns in bundler configs
	entryPatterns := []string{
		"entry:", "entry =", "input:", "input =",
		"main:", "main =", "index:", "index =",
	}

	for _, pattern := range entryPatterns {
		if strings.Contains(content, pattern) {
			// This could be enhanced to actually parse the config
			// For now, just mark it as a frontend entrypoint
			entrypoint := shared.EntryPoint{
				Path:  path,
				Kind:  "frontend",
				Hints: []string{c.detectConfigTool(filepath.Base(path), path)},
			}

			found := false
			for _, existing := range signals.Entrypoints {
				if existing.Path == entrypoint.Path {
					found = true
					break
				}
			}

			if !found {
				signals.Entrypoints = append(signals.Entrypoints, entrypoint)
			}
			break
		}
	}
}

// extractLinterConfig extracts information from linter configurations
func (c *ConfigsExtractor) extractLinterConfig(path string, signals *shared.SignalData) {
	// Linter configs don't typically need special processing for the profiler
	// but we could extract patterns about what files they lint
}

// extractTestConfig extracts information from test configurations
func (c *ConfigsExtractor) extractTestConfig(path string, signals *shared.SignalData) {
	// Test configs could be used to find test file patterns
	// but for now we'll keep it simple
}

// extractTypeScriptConfig extracts information from TypeScript configurations
func (c *ConfigsExtractor) extractTypeScriptConfig(path string, signals *shared.SignalData) {
	data, err := os.ReadFile(filepath.Join(c.root, path))
	if err != nil {
		return
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return
	}

	// Store the TypeScript config for use in language graph building
	if signals.TSConfig == nil {
		signals.TSConfig = config
	}
}

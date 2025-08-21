package signals

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/loom/loom/internal/profiler/shared"
)

// ManifestExtractor extracts information from various manifest files
type ManifestExtractor struct {
	root string
}

// NewManifestExtractor creates a new manifest extractor
func NewManifestExtractor(root string) *ManifestExtractor {
	return &ManifestExtractor{root: root}
}

// Extract processes manifest files and returns signals
func (m *ManifestExtractor) Extract(files []*shared.FileInfo) *shared.SignalData {
	signals := &shared.SignalData{
		TSFiles:        make([]string, 0),
		GoFiles:        make([]string, 0),
		PHPFiles:       make([]string, 0),
		TSConfig:       make(map[string]interface{}),
		ComposerPSR:    make(map[string]string),
		ScriptRefs:     make(map[string][]string),
		CIRefs:         make(map[string][]string),
		DocRefs:        make([]string, 0),
		Manifests:      make([]string, 0),
		Entrypoints:    make([]shared.EntryPoint, 0),
		Scripts:        make([]shared.Script, 0),
		CI:             make([]shared.CIConfig, 0),
		Configs:        make([]shared.ConfigFile, 0),
		Codegen:        make([]shared.CodegenSpec, 0),
		RoutesServices: make([]shared.RouteOrService, 0),
	}

	for _, file := range files {
		switch strings.ToLower(file.Basename) {
		case "package.json":
			m.extractPackageJSON(file.Path, signals)
		case "composer.json":
			m.extractComposerJSON(file.Path, signals)
		case "go.mod":
			m.extractGoMod(file.Path, signals)
		case "cargo.toml":
			m.extractCargoToml(file.Path, signals)
		case "pyproject.toml":
			m.extractPyProjectToml(file.Path, signals)
		case "wails.json":
			m.extractWailsJSON(file.Path, signals)
		}

		// Collect TypeScript files
		if m.isTypeScriptFile(file.Extension) {
			signals.TSFiles = append(signals.TSFiles, file.Path)
		}

		// Collect Go files
		if file.Extension == ".go" {
			signals.GoFiles = append(signals.GoFiles, file.Path)
		}

		// Collect PHP files
		if file.Extension == ".php" {
			signals.PHPFiles = append(signals.PHPFiles, file.Path)
		}

		// Collect config files
		if file.IsConfig {
			signals.Configs = append(signals.Configs, shared.ConfigFile{
				Tool: m.detectConfigTool(file.Basename),
				Path: file.Path,
			})
		}
	}

	// Extract TypeScript config
	m.extractTSConfig(files, signals)

	return signals
}

// isTypeScriptFile checks if file extension indicates TypeScript/JavaScript
func (m *ManifestExtractor) isTypeScriptFile(ext string) bool {
	tsExts := map[string]bool{
		".ts":  true,
		".tsx": true,
		".js":  true,
		".jsx": true,
		".mjs": true,
		".cjs": true,
	}
	return tsExts[ext]
}

// detectConfigTool determines the tool for a config file
func (m *ManifestExtractor) detectConfigTool(basename string) string {
	lower := strings.ToLower(basename)

	toolMap := map[string]string{
		"package.json":       "npm",
		"composer.json":      "composer",
		"go.mod":             "go",
		"cargo.toml":         "cargo",
		"wails.json":         "wails",
		"vite.config.ts":     "vite",
		"vite.config.js":     "vite",
		"webpack.config.js":  "webpack",
		"rollup.config.js":   "rollup",
		"tsconfig.json":      "typescript",
		"jsconfig.json":      "javascript",
		"babel.config.js":    "babel",
		"jest.config.js":     "jest",
		"vitest.config.ts":   "vitest",
		"phpunit.xml":        "phpunit",
		"phpstan.neon":       "phpstan",
		"pint.json":          "pint",
		"dockerfile":         "docker",
		"docker-compose.yml": "docker-compose",
		"makefile":           "make",
		"justfile":           "just",
	}

	if tool, exists := toolMap[lower]; exists {
		return tool
	}

	// Pattern matching
	if strings.Contains(lower, "eslint") {
		return "eslint"
	}
	if strings.Contains(lower, "prettier") {
		return "prettier"
	}
	if strings.Contains(lower, "config") {
		return "config"
	}

	return "unknown"
}

// extractPackageJSON processes package.json files
func (m *ManifestExtractor) extractPackageJSON(path string, signals *shared.SignalData) {
	signals.Manifests = append(signals.Manifests, path)

	data, err := os.ReadFile(filepath.Join(m.root, path))
	if err != nil {
		return
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return
	}

	// Extract scripts
	if scripts, ok := pkg["scripts"].(map[string]interface{}); ok {
		for name, cmd := range scripts {
			if cmdStr, ok := cmd.(string); ok {
				script := shared.Script{
					Name:   name,
					Source: "package.json",
					Cmd:    cmdStr,
					Paths:  m.extractPathsFromScript(cmdStr),
				}
				signals.Scripts = append(signals.Scripts, script)
			}
		}
	}

	// Detect entrypoints
	entrypoints := m.detectJSEntrypoints(pkg, path)
	signals.Entrypoints = append(signals.Entrypoints, entrypoints...)
}

// extractComposerJSON processes composer.json files
func (m *ManifestExtractor) extractComposerJSON(path string, signals *shared.SignalData) {
	signals.Manifests = append(signals.Manifests, path)

	data, err := os.ReadFile(filepath.Join(m.root, path))
	if err != nil {
		return
	}

	var composer map[string]interface{}
	if err := json.Unmarshal(data, &composer); err != nil {
		return
	}

	// Extract PSR-4 autoloading
	if autoload, ok := composer["autoload"].(map[string]interface{}); ok {
		if psr4, ok := autoload["psr-4"].(map[string]interface{}); ok {
			for namespace, pathVal := range psr4 {
				if pathStr, ok := pathVal.(string); ok {
					signals.ComposerPSR[namespace] = pathStr
				}
			}
		}
	}

	// Extract dev PSR-4 autoloading
	if autoloadDev, ok := composer["autoload-dev"].(map[string]interface{}); ok {
		if psr4, ok := autoloadDev["psr-4"].(map[string]interface{}); ok {
			for namespace, pathVal := range psr4 {
				if pathStr, ok := pathVal.(string); ok {
					signals.ComposerPSR[namespace] = pathStr
				}
			}
		}
	}

	// Extract scripts
	if scripts, ok := composer["scripts"].(map[string]interface{}); ok {
		for name, cmd := range scripts {
			if cmdStr, ok := cmd.(string); ok {
				script := shared.Script{
					Name:   name,
					Source: "composer",
					Cmd:    cmdStr,
					Paths:  m.extractPathsFromScript(cmdStr),
				}
				signals.Scripts = append(signals.Scripts, script)
			}
		}
	}

	// Detect Laravel entrypoints
	entrypoints := m.detectLaravelEntrypoints(path)
	signals.Entrypoints = append(signals.Entrypoints, entrypoints...)
}

// extractGoMod processes go.mod files
func (m *ManifestExtractor) extractGoMod(path string, signals *shared.SignalData) {
	signals.Manifests = append(signals.Manifests, path)

	// Detect Go entrypoints
	entrypoints := m.detectGoEntrypoints(path)
	signals.Entrypoints = append(signals.Entrypoints, entrypoints...)
}

// extractCargoToml processes Cargo.toml files
func (m *ManifestExtractor) extractCargoToml(path string, signals *shared.SignalData) {
	signals.Manifests = append(signals.Manifests, path)
	// Could add Rust support in the future
}

// extractPyProjectToml processes pyproject.toml files
func (m *ManifestExtractor) extractPyProjectToml(path string, signals *shared.SignalData) {
	signals.Manifests = append(signals.Manifests, path)
	// Could add Python support in the future
}

// extractWailsJSON processes wails.json files
func (m *ManifestExtractor) extractWailsJSON(path string, signals *shared.SignalData) {
	signals.Manifests = append(signals.Manifests, path)

	data, err := os.ReadFile(filepath.Join(m.root, path))
	if err != nil {
		return
	}

	var wails map[string]interface{}
	if err := json.Unmarshal(data, &wails); err != nil {
		return
	}

	// Detect Wails entrypoints
	entrypoint := shared.EntryPoint{
		Path:  path,
		Kind:  "backend",
		Hints: []string{"wails"},
	}
	signals.Entrypoints = append(signals.Entrypoints, entrypoint)
}

// extractTSConfig processes TypeScript configuration
func (m *ManifestExtractor) extractTSConfig(files []*shared.FileInfo, signals *shared.SignalData) {
	for _, file := range files {
		if strings.Contains(strings.ToLower(file.Basename), "tsconfig") {
			data, err := os.ReadFile(filepath.Join(m.root, file.Path))
			if err != nil {
				continue
			}

			var config map[string]interface{}
			if err := json.Unmarshal(data, &config); err != nil {
				continue
			}

			signals.TSConfig = config
			break // Use first found tsconfig
		}
	}
}

// detectJSEntrypoints detects JavaScript/TypeScript entrypoints from package.json
func (m *ManifestExtractor) detectJSEntrypoints(pkg map[string]interface{}, packagePath string) []shared.EntryPoint {
	var entrypoints []shared.EntryPoint

	// Check main field
	if main, ok := pkg["main"].(string); ok {
		dir := filepath.Dir(packagePath)
		entryPath := filepath.Join(dir, main)
		entrypoints = append(entrypoints, shared.EntryPoint{
			Path:  entryPath,
			Kind:  m.guessJSEntrypointKind(main, pkg),
			Hints: m.guessJSHints(pkg),
		})
	}

	// Check scripts for dev servers
	if scripts, ok := pkg["scripts"].(map[string]interface{}); ok {
		for name, cmdInterface := range scripts {
			if cmd, ok := cmdInterface.(string); ok {
				if strings.Contains(name, "dev") || strings.Contains(name, "start") {
					if strings.Contains(cmd, "vite") {
						hints := []string{"vite"}
						entrypoints = append(entrypoints, shared.EntryPoint{
							Path:  filepath.Join(filepath.Dir(packagePath), "src/main.ts"),
							Kind:  "frontend",
							Hints: hints,
						})
					}
				}
			}
		}
	}

	return entrypoints
}

// detectGoEntrypoints detects Go entrypoints
func (m *ManifestExtractor) detectGoEntrypoints(goModPath string) []shared.EntryPoint {
	var entrypoints []shared.EntryPoint

	// Check for main.go in project root
	dir := filepath.Dir(goModPath)
	mainPath := filepath.Join(dir, "main.go")
	if _, err := os.Stat(filepath.Join(m.root, mainPath)); err == nil {
		entrypoints = append(entrypoints, shared.EntryPoint{
			Path:  mainPath,
			Kind:  "backend",
			Hints: []string{"go-main"},
		})
	}

	// Check for cmd directory pattern
	cmdDir := filepath.Join(dir, "cmd")
	if entries, err := os.ReadDir(filepath.Join(m.root, cmdDir)); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				cmdMainPath := filepath.Join(cmdDir, entry.Name(), "main.go")
				if _, err := os.Stat(filepath.Join(m.root, cmdMainPath)); err == nil {
					entrypoints = append(entrypoints, shared.EntryPoint{
						Path:  cmdMainPath,
						Kind:  "cli",
						Hints: []string{"go-cmd"},
					})
				}
			}
		}
	}

	return entrypoints
}

// detectLaravelEntrypoints detects Laravel PHP entrypoints
func (m *ManifestExtractor) detectLaravelEntrypoints(composerPath string) []shared.EntryPoint {
	var entrypoints []shared.EntryPoint

	dir := filepath.Dir(composerPath)

	// Check for Laravel artisan
	artisanPath := filepath.Join(dir, "artisan")
	if _, err := os.Stat(filepath.Join(m.root, artisanPath)); err == nil {
		entrypoints = append(entrypoints, shared.EntryPoint{
			Path:  artisanPath,
			Kind:  "cli",
			Hints: []string{"laravel", "artisan"},
		})
	}

	// Check for Laravel public/index.php
	indexPath := filepath.Join(dir, "public/index.php")
	if _, err := os.Stat(filepath.Join(m.root, indexPath)); err == nil {
		entrypoints = append(entrypoints, shared.EntryPoint{
			Path:  indexPath,
			Kind:  "backend",
			Hints: []string{"laravel", "web"},
		})
	}

	return entrypoints
}

// guessJSEntrypointKind determines the kind of JS entrypoint
func (m *ManifestExtractor) guessJSEntrypointKind(main string, pkg map[string]interface{}) string {
	// Check dependencies for clues
	if deps, ok := pkg["dependencies"].(map[string]interface{}); ok {
		if _, hasReact := deps["react"]; hasReact {
			return "frontend"
		}
		if _, hasVue := deps["vue"]; hasVue {
			return "frontend"
		}
		if _, hasExpress := deps["express"]; hasExpress {
			return "backend"
		}
		if _, hasNext := deps["next"]; hasNext {
			return "frontend"
		}
	}

	// Guess from file path
	if strings.Contains(main, "server") || strings.Contains(main, "backend") {
		return "backend"
	}
	if strings.Contains(main, "frontend") || strings.Contains(main, "client") {
		return "frontend"
	}

	return "backend" // Default
}

// guessJSHints determines hints for JS projects
func (m *ManifestExtractor) guessJSHints(pkg map[string]interface{}) []string {
	var hints []string

	if deps, ok := pkg["dependencies"].(map[string]interface{}); ok {
		if _, hasReact := deps["react"]; hasReact {
			hints = append(hints, "react")
		}
		if _, hasVue := deps["vue"]; hasVue {
			hints = append(hints, "vue")
		}
		if _, hasNext := deps["next"]; hasNext {
			hints = append(hints, "next")
		}
		if _, hasExpress := deps["express"]; hasExpress {
			hints = append(hints, "express")
		}
	}

	if devDeps, ok := pkg["devDependencies"].(map[string]interface{}); ok {
		if _, hasVite := devDeps["vite"]; hasVite {
			hints = append(hints, "vite")
		}
		if _, hasWebpack := devDeps["webpack"]; hasWebpack {
			hints = append(hints, "webpack")
		}
	}

	return hints
}

// extractPathsFromScript extracts file paths mentioned in script commands
func (m *ManifestExtractor) extractPathsFromScript(script string) []string {
	var paths []string

	// Simple regex-like extraction for common path patterns
	words := strings.Fields(script)
	for _, word := range words {
		// Look for path-like patterns
		if strings.Contains(word, "/") &&
			(strings.Contains(word, "src/") ||
				strings.Contains(word, "app/") ||
				strings.Contains(word, "cmd/") ||
				strings.Contains(word, "internal/") ||
				strings.Contains(word, "ui/")) {
			// Clean up common script artifacts
			cleaned := strings.Trim(word, "\"'`);&|")
			if cleaned != "" {
				paths = append(paths, cleaned)
			}
		}
	}

	return paths
}

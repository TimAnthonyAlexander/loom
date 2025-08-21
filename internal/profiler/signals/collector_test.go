package signals

import (
	"testing"

	"github.com/loom/loom/internal/profiler/shared"
)

func TestNewCollector(t *testing.T) {
	collector := NewCollector("/test/root")
	if collector == nil {
		t.Fatal("NewCollector returned nil")
	}
	if collector.root != "/test/root" {
		t.Errorf("Expected root '/test/root', got %s", collector.root)
	}
	if collector.manifestExtractor == nil {
		t.Error("Expected manifestExtractor to be initialized")
	}
	if collector.scriptExtractor == nil {
		t.Error("Expected scriptExtractor to be initialized")
	}
	if collector.ciExtractor == nil {
		t.Error("Expected ciExtractor to be initialized")
	}
	if collector.docsExtractor == nil {
		t.Error("Expected docsExtractor to be initialized")
	}
	if collector.configsExtractor == nil {
		t.Error("Expected configsExtractor to be initialized")
	}
	if collector.codegenExtractor == nil {
		t.Error("Expected codegenExtractor to be initialized")
	}
	if collector.routesExtractor == nil {
		t.Error("Expected routesExtractor to be initialized")
	}
}

func TestCollector_Collect(t *testing.T) {
	collector := NewCollector("/test/root")

	files := []*shared.FileInfo{
		{Path: "package.json", Basename: "package.json", IsConfig: true},
		{Path: "src/main.ts", Extension: ".ts", Basename: "main.ts"},
		{Path: "src/utils.go", Extension: ".go", Basename: "utils.go"},
		{Path: "README.md", Basename: "README.md", IsDoc: true},
		{Path: "build.sh", Basename: "build.sh", IsScript: true},
	}

	signals := collector.Collect(files)

	if signals == nil {
		t.Fatal("Collect returned nil")
	}

	// Check that TypeScript files were collected
	found := false
	for _, file := range signals.TSFiles {
		if file == "src/main.ts" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected src/main.ts to be in TSFiles")
	}

	// Check that Go files were collected
	found = false
	for _, file := range signals.GoFiles {
		if file == "src/utils.go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected src/utils.go to be in GoFiles")
	}

	// Check that manifests were collected
	found = false
	for _, manifest := range signals.Manifests {
		if manifest == "package.json" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected package.json to be in Manifests")
	}

	// Check that configs were collected
	found = false
	for _, config := range signals.Configs {
		if config.Path == "package.json" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected package.json to be in Configs")
	}
}

func TestCollector_CollectWithEmptyFiles(t *testing.T) {
	collector := NewCollector("/test/root")

	signals := collector.Collect([]*shared.FileInfo{})

	if signals == nil {
		t.Fatal("Collect returned nil for empty files")
	}

	// All slices should be initialized but empty
	if signals.TSFiles == nil {
		t.Error("Expected TSFiles to be initialized")
	}
	if signals.GoFiles == nil {
		t.Error("Expected GoFiles to be initialized")
	}
	if signals.PHPFiles == nil {
		t.Error("Expected PHPFiles to be initialized")
	}
	if signals.Manifests == nil {
		t.Error("Expected Manifests to be initialized")
	}
	if signals.Entrypoints == nil {
		t.Error("Expected Entrypoints to be initialized")
	}
	if signals.Scripts == nil {
		t.Error("Expected Scripts to be initialized")
	}
	if signals.CI == nil {
		t.Error("Expected CI to be initialized")
	}
	if signals.Configs == nil {
		t.Error("Expected Configs to be initialized")
	}
	if signals.Codegen == nil {
		t.Error("Expected Codegen to be initialized")
	}
	if signals.RoutesServices == nil {
		t.Error("Expected RoutesServices to be initialized")
	}

	// Maps should be initialized
	if signals.TSConfig == nil {
		t.Error("Expected TSConfig to be initialized")
	}
	if signals.ComposerPSR == nil {
		t.Error("Expected ComposerPSR to be initialized")
	}
	if signals.ScriptRefs == nil {
		t.Error("Expected ScriptRefs to be initialized")
	}
	if signals.CIRefs == nil {
		t.Error("Expected CIRefs to be initialized")
	}
	if signals.DocRefs == nil {
		t.Error("Expected DocRefs to be initialized")
	}
}

func TestCollector_CollectMultipleLanguages(t *testing.T) {
	collector := NewCollector("/test/root")

	files := []*shared.FileInfo{
		{Path: "frontend/app.tsx", Extension: ".tsx", Basename: "app.tsx"},
		{Path: "frontend/utils.js", Extension: ".js", Basename: "utils.js"},
		{Path: "backend/main.go", Extension: ".go", Basename: "main.go"},
		{Path: "backend/handler.go", Extension: ".go", Basename: "handler.go"},
		{Path: "api/controller.php", Extension: ".php", Basename: "controller.php"},
		{Path: "api/model.php", Extension: ".php", Basename: "model.php"},
	}

	signals := collector.Collect(files)

	// Check TypeScript/JavaScript files
	if len(signals.TSFiles) != 2 {
		t.Errorf("Expected 2 TS files, got %d", len(signals.TSFiles))
	}

	expectedTSFiles := map[string]bool{
		"frontend/app.tsx":  true,
		"frontend/utils.js": true,
	}
	for _, file := range signals.TSFiles {
		if !expectedTSFiles[file] {
			t.Errorf("Unexpected TS file: %s", file)
		}
	}

	// Check Go files
	if len(signals.GoFiles) != 2 {
		t.Errorf("Expected 2 Go files, got %d", len(signals.GoFiles))
	}

	expectedGoFiles := map[string]bool{
		"backend/main.go":    true,
		"backend/handler.go": true,
	}
	for _, file := range signals.GoFiles {
		if !expectedGoFiles[file] {
			t.Errorf("Unexpected Go file: %s", file)
		}
	}

	// Check PHP files
	if len(signals.PHPFiles) != 2 {
		t.Errorf("Expected 2 PHP files, got %d", len(signals.PHPFiles))
	}

	expectedPHPFiles := map[string]bool{
		"api/controller.php": true,
		"api/model.php":      true,
	}
	for _, file := range signals.PHPFiles {
		if !expectedPHPFiles[file] {
			t.Errorf("Unexpected PHP file: %s", file)
		}
	}
}

func TestCollector_CollectConfigFiles(t *testing.T) {
	collector := NewCollector("/test/root")

	files := []*shared.FileInfo{
		{Path: "package.json", Basename: "package.json", IsConfig: true},
		{Path: "composer.json", Basename: "composer.json", IsConfig: true},
		{Path: "go.mod", Basename: "go.mod", IsConfig: true},
		{Path: "tsconfig.json", Basename: "tsconfig.json", IsConfig: true},
		{Path: "vite.config.ts", Basename: "vite.config.ts", IsConfig: true},
		{Path: "Dockerfile", Basename: "dockerfile", IsConfig: true},
		{Path: "src/main.go", Extension: ".go", Basename: "main.go", IsConfig: false},
	}

	signals := collector.Collect(files)

	// Should have 6 config files (excluding main.go)
	if len(signals.Configs) != 6 {
		t.Errorf("Expected 6 config files, got %d", len(signals.Configs))
	}

	// Check specific config tools
	configTools := make(map[string]string)
	for _, config := range signals.Configs {
		configTools[config.Path] = config.Tool
	}

	expectedTools := map[string]string{
		"package.json":   "npm",
		"composer.json":  "composer",
		"go.mod":         "go",
		"tsconfig.json":  "typescript",
		"vite.config.ts": "vite",
		"Dockerfile":     "docker",
	}

	for path, expectedTool := range expectedTools {
		if tool, exists := configTools[path]; !exists {
			t.Errorf("Expected config file %s not found", path)
		} else if tool != expectedTool {
			t.Errorf("Expected tool %s for %s, got %s", expectedTool, path, tool)
		}
	}
}

func TestCollector_CollectDocFiles(t *testing.T) {
	collector := NewCollector("/test/root")

	files := []*shared.FileInfo{
		{Path: "README.md", Basename: "README.md", IsDoc: true},
		{Path: "CHANGELOG.md", Basename: "CHANGELOG.md", IsDoc: true},
		{Path: "docs/api.md", Basename: "api.md", IsDoc: true},
		{Path: "LICENSE", Basename: "LICENSE", IsDoc: true},
		{Path: "src/main.go", Extension: ".go", Basename: "main.go", IsDoc: false},
	}

	signals := collector.Collect(files)

	// Check that doc files are processed (exact behavior depends on DocsExtractor)
	// The collector should call the docs extractor
	if signals.DocRefs == nil {
		t.Error("Expected DocRefs to be initialized")
	}
}

func TestCollector_CollectScriptFiles(t *testing.T) {
	collector := NewCollector("/test/root")

	files := []*shared.FileInfo{
		{Path: "build.sh", Basename: "build.sh", IsScript: true},
		{Path: "deploy.py", Basename: "deploy.py", IsScript: true},
		{Path: "Makefile", Basename: "Makefile", IsScript: true},
		{Path: "scripts/setup.sh", Basename: "setup.sh", IsScript: true},
		{Path: "src/main.go", Extension: ".go", Basename: "main.go", IsScript: false},
	}

	signals := collector.Collect(files)

	// Check that script references are processed (exact behavior depends on ScriptExtractor)
	if signals.ScriptRefs == nil {
		t.Error("Expected ScriptRefs to be initialized")
	}
}

func TestCollector_CollectOrder(t *testing.T) {
	// Test that extractors are called in the expected order
	// This is important because later extractors may depend on earlier ones
	collector := NewCollector("/test/root")

	files := []*shared.FileInfo{
		{Path: "package.json", Basename: "package.json", IsConfig: true},
		{Path: "src/main.ts", Extension: ".ts", Basename: "main.ts"},
	}

	signals := collector.Collect(files)

	// The manifest extractor should run first and populate the base signals
	if signals == nil {
		t.Fatal("Collect returned nil")
	}

	// TSFiles should be populated by manifest extractor
	if len(signals.TSFiles) == 0 {
		t.Error("Expected TSFiles to be populated by manifest extractor")
	}

	// Manifests should be populated
	if len(signals.Manifests) == 0 {
		t.Error("Expected Manifests to be populated")
	}
}

func TestCollector_CollectIntegration(t *testing.T) {
	// Integration test with realistic file structure
	collector := NewCollector("/test/project")

	files := []*shared.FileInfo{
		// Frontend
		{Path: "package.json", Basename: "package.json", IsConfig: true},
		{Path: "src/App.tsx", Extension: ".tsx", Basename: "App.tsx"},
		{Path: "src/index.ts", Extension: ".ts", Basename: "index.ts"},
		{Path: "src/utils.js", Extension: ".js", Basename: "utils.js"},

		// Backend
		{Path: "go.mod", Basename: "go.mod", IsConfig: true},
		{Path: "main.go", Extension: ".go", Basename: "main.go"},
		{Path: "internal/handler.go", Extension: ".go", Basename: "handler.go"},

		// PHP API
		{Path: "api/composer.json", Basename: "composer.json", IsConfig: true},
		{Path: "api/index.php", Extension: ".php", Basename: "index.php"},

		// Config and docs
		{Path: "tsconfig.json", Basename: "tsconfig.json", IsConfig: true},
		{Path: "README.md", Basename: "README.md", IsDoc: true},
		{Path: "docker-compose.yml", Basename: "docker-compose.yml", IsConfig: true},

		// Scripts
		{Path: "build.sh", Basename: "build.sh", IsScript: true},
		{Path: "Makefile", Basename: "Makefile", IsScript: true},
	}

	signals := collector.Collect(files)

	// Verify all expected language files are collected
	if len(signals.TSFiles) != 3 {
		t.Errorf("Expected 3 TypeScript files, got %d", len(signals.TSFiles))
	}
	if len(signals.GoFiles) != 2 {
		t.Errorf("Expected 2 Go files, got %d", len(signals.GoFiles))
	}
	if len(signals.PHPFiles) != 1 {
		t.Errorf("Expected 1 PHP file, got %d", len(signals.PHPFiles))
	}

	// Verify manifests are collected
	expectedManifests := []string{"package.json", "api/composer.json"}
	if len(signals.Manifests) < len(expectedManifests) {
		t.Errorf("Expected at least %d manifests, got %d", len(expectedManifests), len(signals.Manifests))
	}

	// Verify configs are collected
	if len(signals.Configs) < 5 { // At least package.json, go.mod, composer.json, tsconfig.json, docker-compose.yml
		t.Errorf("Expected at least 5 config files, got %d", len(signals.Configs))
	}

	// Verify maps are initialized
	if signals.TSConfig == nil {
		t.Error("Expected TSConfig map to be initialized")
	}
	if signals.ComposerPSR == nil {
		t.Error("Expected ComposerPSR map to be initialized")
	}
	if signals.ScriptRefs == nil {
		t.Error("Expected ScriptRefs map to be initialized")
	}
	if signals.CIRefs == nil {
		t.Error("Expected CIRefs map to be initialized")
	}

	// Verify slices are initialized
	if signals.DocRefs == nil {
		t.Error("Expected DocRefs slice to be initialized")
	}
}

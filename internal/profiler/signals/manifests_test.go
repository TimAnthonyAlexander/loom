package signals

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/loom/loom/internal/profiler/shared"
)

func TestNewManifestExtractor(t *testing.T) {
	extractor := NewManifestExtractor("/test/root")
	if extractor == nil {
		t.Fatal("NewManifestExtractor returned nil")
	}
	if extractor.root != "/test/root" {
		t.Errorf("Expected root '/test/root', got %s", extractor.root)
	}
}

func TestManifestExtractor_isTypeScriptFile(t *testing.T) {
	extractor := NewManifestExtractor("/test")

	tests := []struct {
		ext      string
		expected bool
	}{
		{".ts", true},
		{".tsx", true},
		{".js", true},
		{".jsx", true},
		{".mjs", true},
		{".cjs", true},
		{".go", false},
		{".php", false},
		{".py", false},
		{".rb", false},
		{"", false},
	}

	for _, tt := range tests {
		result := extractor.isTypeScriptFile(tt.ext)
		if result != tt.expected {
			t.Errorf("isTypeScriptFile(%q) = %v, want %v", tt.ext, result, tt.expected)
		}
	}
}

func TestManifestExtractor_detectConfigTool(t *testing.T) {
	extractor := NewManifestExtractor("/test")

	tests := []struct {
		basename string
		expected string
	}{
		{"package.json", "npm"},
		{"composer.json", "composer"},
		{"go.mod", "go"},
		{"Cargo.toml", "cargo"},
		{"wails.json", "wails"},
		{"vite.config.ts", "vite"},
		{"webpack.config.js", "webpack"},
		{"tsconfig.json", "typescript"},
		{"jest.config.js", "jest"},
		{"Dockerfile", "docker"},
		{"docker-compose.yml", "docker-compose"},
		{"Makefile", "make"},
		{".eslintrc.js", "eslint"},
		{".prettierrc", "prettier"},
		{"my.config.js", "config"},
		{"unknown.txt", "unknown"},
	}

	for _, tt := range tests {
		result := extractor.detectConfigTool(tt.basename)
		if result != tt.expected {
			t.Errorf("detectConfigTool(%q) = %q, want %q", tt.basename, result, tt.expected)
		}
	}
}

func TestManifestExtractor_Extract_FileCollection(t *testing.T) {
	extractor := NewManifestExtractor("/test")

	files := []*shared.FileInfo{
		{Path: "src/main.ts", Extension: ".ts", Basename: "main.ts"},
		{Path: "src/utils.tsx", Extension: ".tsx", Basename: "utils.tsx"},
		{Path: "src/app.js", Extension: ".js", Basename: "app.js"},
		{Path: "backend/main.go", Extension: ".go", Basename: "main.go"},
		{Path: "backend/handler.go", Extension: ".go", Basename: "handler.go"},
		{Path: "api/controller.php", Extension: ".php", Basename: "controller.php"},
		{Path: "config/settings.py", Extension: ".py", Basename: "settings.py"},
		{Path: "package.json", Basename: "package.json", IsConfig: true},
		{Path: "tsconfig.json", Basename: "tsconfig.json", IsConfig: true},
	}

	signals := extractor.Extract(files)

	// Check TypeScript files
	expectedTSFiles := []string{"src/main.ts", "src/utils.tsx", "src/app.js"}
	if len(signals.TSFiles) != len(expectedTSFiles) {
		t.Errorf("Expected %d TS files, got %d", len(expectedTSFiles), len(signals.TSFiles))
	}

	tsFileSet := make(map[string]bool)
	for _, file := range signals.TSFiles {
		tsFileSet[file] = true
	}
	for _, expected := range expectedTSFiles {
		if !tsFileSet[expected] {
			t.Errorf("Expected TS file %s not found", expected)
		}
	}

	// Check Go files
	expectedGoFiles := []string{"backend/main.go", "backend/handler.go"}
	if len(signals.GoFiles) != len(expectedGoFiles) {
		t.Errorf("Expected %d Go files, got %d", len(expectedGoFiles), len(signals.GoFiles))
	}

	// Check PHP files
	expectedPHPFiles := []string{"api/controller.php"}
	if len(signals.PHPFiles) != len(expectedPHPFiles) {
		t.Errorf("Expected %d PHP files, got %d", len(expectedPHPFiles), len(signals.PHPFiles))
	}

	// Check config files
	if len(signals.Configs) != 2 {
		t.Errorf("Expected 2 config files, got %d", len(signals.Configs))
	}

	// Check specific configs
	configPaths := make(map[string]string)
	for _, config := range signals.Configs {
		configPaths[config.Path] = config.Tool
	}

	if tool, exists := configPaths["package.json"]; !exists {
		t.Error("Expected package.json in configs")
	} else if tool != "npm" {
		t.Errorf("Expected npm tool for package.json, got %s", tool)
	}

	if tool, exists := configPaths["tsconfig.json"]; !exists {
		t.Error("Expected tsconfig.json in configs")
	} else if tool != "typescript" {
		t.Errorf("Expected typescript tool for tsconfig.json, got %s", tool)
	}
}

func TestManifestExtractor_extractPackageJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manifest_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create package.json
	packageJSONContent := `{
		"name": "test-project",
		"version": "1.0.0",
		"main": "dist/index.js",
		"scripts": {
			"build": "tsc",
			"test": "jest",
			"dev": "vite"
		},
		"dependencies": {
			"react": "^18.0.0",
			"express": "^4.18.0"
		},
		"devDependencies": {
			"vite": "^4.0.0",
			"typescript": "^4.9.0"
		}
	}`

	packageJSONPath := filepath.Join(tmpDir, "package.json")
	err = os.WriteFile(packageJSONPath, []byte(packageJSONContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	extractor := NewManifestExtractor(tmpDir)
	signals := &shared.SignalData{
		Scripts:     make([]shared.Script, 0),
		Entrypoints: make([]shared.EntryPoint, 0),
		Manifests:   make([]string, 0),
	}

	extractor.extractPackageJSON("package.json", signals)

	// Check that manifest was added
	if len(signals.Manifests) != 1 || signals.Manifests[0] != "package.json" {
		t.Error("Expected package.json to be added to manifests")
	}

	// Check scripts
	if len(signals.Scripts) != 3 {
		t.Errorf("Expected 3 scripts, got %d", len(signals.Scripts))
	}

	scriptNames := make(map[string]string)
	for _, script := range signals.Scripts {
		scriptNames[script.Name] = script.Cmd
	}

	expectedScripts := map[string]string{
		"build": "tsc",
		"test":  "jest",
		"dev":   "vite",
	}

	for name, expectedCmd := range expectedScripts {
		if cmd, exists := scriptNames[name]; !exists {
			t.Errorf("Expected script %s not found", name)
		} else if cmd != expectedCmd {
			t.Errorf("Expected script %s command %s, got %s", name, expectedCmd, cmd)
		}
	}

	// Check entrypoints
	if len(signals.Entrypoints) == 0 {
		t.Error("Expected entrypoints to be detected")
	}

	// Should detect main entrypoint
	foundMain := false
	for _, ep := range signals.Entrypoints {
		if ep.Path == "dist/index.js" {
			foundMain = true
			if ep.Kind != "frontend" { // Should be frontend due to React dependency
				t.Errorf("Expected frontend kind for main entrypoint, got %s", ep.Kind)
			}
		}
	}
	if !foundMain {
		t.Error("Expected main entrypoint to be detected")
	}

	// Should detect Vite entrypoint from dev script
	foundVite := false
	for _, ep := range signals.Entrypoints {
		if ep.Path == "src/main.ts" {
			foundVite = true
			if ep.Kind != "frontend" {
				t.Errorf("Expected frontend kind for Vite entrypoint, got %s", ep.Kind)
			}
			// Check for vite hint
			hasViteHint := false
			for _, hint := range ep.Hints {
				if hint == "vite" {
					hasViteHint = true
					break
				}
			}
			if !hasViteHint {
				t.Error("Expected vite hint for Vite entrypoint")
			}
		}
	}
	if !foundVite {
		t.Error("Expected Vite entrypoint to be detected")
	}
}

func TestManifestExtractor_extractComposerJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manifest_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create composer.json
	composerJSONContent := `{
		"name": "test/project",
		"autoload": {
			"psr-4": {
				"App\\": "src/",
				"Tests\\": "tests/"
			}
		},
		"autoload-dev": {
			"psr-4": {
				"DevTools\\": "dev-tools/"
			}
		},
		"scripts": {
			"test": "phpunit",
			"format": "php-cs-fixer fix"
		}
	}`

	composerJSONPath := filepath.Join(tmpDir, "composer.json")
	err = os.WriteFile(composerJSONPath, []byte(composerJSONContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	extractor := NewManifestExtractor(tmpDir)
	signals := &shared.SignalData{
		ComposerPSR: make(map[string]string),
		Scripts:     make([]shared.Script, 0),
		Entrypoints: make([]shared.EntryPoint, 0),
		Manifests:   make([]string, 0),
	}

	extractor.extractComposerJSON("composer.json", signals)

	// Check PSR-4 mappings
	expectedPSR := map[string]string{
		"App\\":      "src/",
		"Tests\\":    "tests/",
		"DevTools\\": "dev-tools/",
	}

	if len(signals.ComposerPSR) != len(expectedPSR) {
		t.Errorf("Expected %d PSR-4 mappings, got %d", len(expectedPSR), len(signals.ComposerPSR))
	}

	for namespace, expectedPath := range expectedPSR {
		if path, exists := signals.ComposerPSR[namespace]; !exists {
			t.Errorf("Expected PSR-4 mapping for %s not found", namespace)
		} else if path != expectedPath {
			t.Errorf("Expected PSR-4 path %s for %s, got %s", expectedPath, namespace, path)
		}
	}

	// Check scripts
	if len(signals.Scripts) != 2 {
		t.Errorf("Expected 2 scripts, got %d", len(signals.Scripts))
	}

	scriptNames := make(map[string]string)
	for _, script := range signals.Scripts {
		scriptNames[script.Name] = script.Cmd
		if script.Source != "composer" {
			t.Errorf("Expected script source 'composer', got %s", script.Source)
		}
	}

	expectedScripts := map[string]string{
		"test":   "phpunit",
		"format": "php-cs-fixer fix",
	}

	for name, expectedCmd := range expectedScripts {
		if cmd, exists := scriptNames[name]; !exists {
			t.Errorf("Expected script %s not found", name)
		} else if cmd != expectedCmd {
			t.Errorf("Expected script %s command %s, got %s", name, expectedCmd, cmd)
		}
	}
}

func TestManifestExtractor_extractGoMod(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manifest_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create go.mod
	goModContent := `module github.com/test/project

go 1.19

require (
	github.com/gin-gonic/gin v1.9.0
	github.com/spf13/cobra v1.6.1
)`

	goModPath := filepath.Join(tmpDir, "go.mod")
	err = os.WriteFile(goModPath, []byte(goModContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create main.go for entrypoint detection
	mainGoPath := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(mainGoPath, []byte("package main\n\nfunc main() {}"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create cmd structure
	cmdDir := filepath.Join(tmpDir, "cmd", "server")
	err = os.MkdirAll(cmdDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	cmdMainPath := filepath.Join(cmdDir, "main.go")
	err = os.WriteFile(cmdMainPath, []byte("package main\n\nfunc main() {}"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	extractor := NewManifestExtractor(tmpDir)
	signals := &shared.SignalData{
		Entrypoints: make([]shared.EntryPoint, 0),
		Manifests:   make([]string, 0),
	}

	extractor.extractGoMod("go.mod", signals)

	// Check that manifest was added
	if len(signals.Manifests) != 1 || signals.Manifests[0] != "go.mod" {
		t.Error("Expected go.mod to be added to manifests")
	}

	// Check entrypoints
	if len(signals.Entrypoints) != 2 {
		t.Errorf("Expected 2 entrypoints, got %d", len(signals.Entrypoints))
	}

	// Check main.go entrypoint
	foundMain := false
	foundCmd := false
	for _, ep := range signals.Entrypoints {
		if ep.Path == "main.go" {
			foundMain = true
			if ep.Kind != "backend" {
				t.Errorf("Expected backend kind for main.go, got %s", ep.Kind)
			}
			// Check hints
			hasGoHint := false
			for _, hint := range ep.Hints {
				if hint == "go-main" {
					hasGoHint = true
					break
				}
			}
			if !hasGoHint {
				t.Error("Expected go-main hint for main.go")
			}
		} else if ep.Path == "cmd/server/main.go" {
			foundCmd = true
			if ep.Kind != "cli" {
				t.Errorf("Expected cli kind for cmd entrypoint, got %s", ep.Kind)
			}
		}
	}

	if !foundMain {
		t.Error("Expected main.go entrypoint to be detected")
	}
	if !foundCmd {
		t.Error("Expected cmd/server/main.go entrypoint to be detected")
	}
}

func TestManifestExtractor_extractWailsJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manifest_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create wails.json
	wailsJSONContent := `{
		"$schema": "https://wails.io/schemas/config.v2.json",
		"name": "test-app",
		"outputfilename": "test-app",
		"frontend:install": "npm install",
		"frontend:build": "npm run build"
	}`

	wailsJSONPath := filepath.Join(tmpDir, "wails.json")
	err = os.WriteFile(wailsJSONPath, []byte(wailsJSONContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	extractor := NewManifestExtractor(tmpDir)
	signals := &shared.SignalData{
		Entrypoints: make([]shared.EntryPoint, 0),
		Manifests:   make([]string, 0),
	}

	extractor.extractWailsJSON("wails.json", signals)

	// Check that manifest was added
	if len(signals.Manifests) != 1 || signals.Manifests[0] != "wails.json" {
		t.Error("Expected wails.json to be added to manifests")
	}

	// Check entrypoint
	if len(signals.Entrypoints) != 1 {
		t.Errorf("Expected 1 entrypoint, got %d", len(signals.Entrypoints))
	}

	ep := signals.Entrypoints[0]
	if ep.Path != "wails.json" {
		t.Errorf("Expected entrypoint path 'wails.json', got %s", ep.Path)
	}
	if ep.Kind != "backend" {
		t.Errorf("Expected backend kind, got %s", ep.Kind)
	}

	// Check hints
	hasWailsHint := false
	for _, hint := range ep.Hints {
		if hint == "wails" {
			hasWailsHint = true
			break
		}
	}
	if !hasWailsHint {
		t.Error("Expected wails hint")
	}
}

func TestManifestExtractor_guessJSEntrypointKind(t *testing.T) {
	extractor := NewManifestExtractor("/test")

	tests := []struct {
		name     string
		main     string
		deps     map[string]interface{}
		expected string
	}{
		{
			name: "React app",
			main: "src/index.js",
			deps: map[string]interface{}{
				"react": "^18.0.0",
			},
			expected: "frontend",
		},
		{
			name: "Vue app",
			main: "src/main.js",
			deps: map[string]interface{}{
				"vue": "^3.0.0",
			},
			expected: "frontend",
		},
		{
			name: "Express server",
			main: "server.js",
			deps: map[string]interface{}{
				"express": "^4.18.0",
			},
			expected: "backend",
		},
		{
			name: "Next.js app",
			main: "pages/index.js",
			deps: map[string]interface{}{
				"next": "^13.0.0",
			},
			expected: "frontend",
		},
		{
			name:     "Server file",
			main:     "server.js",
			deps:     map[string]interface{}{},
			expected: "backend",
		},
		{
			name:     "Frontend file",
			main:     "frontend/app.js",
			deps:     map[string]interface{}{},
			expected: "frontend",
		},
		{
			name:     "Default",
			main:     "index.js",
			deps:     map[string]interface{}{},
			expected: "backend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := map[string]interface{}{
				"dependencies": tt.deps,
			}
			result := extractor.guessJSEntrypointKind(tt.main, pkg)
			if result != tt.expected {
				t.Errorf("guessJSEntrypointKind(%q, deps=%v) = %q, want %q",
					tt.main, tt.deps, result, tt.expected)
			}
		})
	}
}

func TestManifestExtractor_guessJSHints(t *testing.T) {
	extractor := NewManifestExtractor("/test")

	pkg := map[string]interface{}{
		"dependencies": map[string]interface{}{
			"react":   "^18.0.0",
			"express": "^4.18.0",
		},
		"devDependencies": map[string]interface{}{
			"vite":    "^4.0.0",
			"webpack": "^5.0.0",
		},
	}

	hints := extractor.guessJSHints(pkg)

	expectedHints := []string{"react", "express", "vite", "webpack"}
	if len(hints) != len(expectedHints) {
		t.Errorf("Expected %d hints, got %d", len(expectedHints), len(hints))
	}

	hintSet := make(map[string]bool)
	for _, hint := range hints {
		hintSet[hint] = true
	}

	for _, expected := range expectedHints {
		if !hintSet[expected] {
			t.Errorf("Expected hint %s not found", expected)
		}
	}
}

func TestManifestExtractor_extractPathsFromScript(t *testing.T) {
	extractor := NewManifestExtractor("/test")

	tests := []struct {
		script   string
		expected []string
	}{
		{
			script:   "go build -o bin/app cmd/main.go",
			expected: []string{"cmd/main.go"},
		},
		{
			script:   "tsc src/main.ts --outDir dist/",
			expected: []string{"src/main.ts"},
		},
		{
			script:   "webpack src/app.js --output-path dist/",
			expected: []string{"src/app.js"},
		},
		{
			script:   "cp src/config.json app/config.json",
			expected: []string{"src/config.json", "app/config.json"},
		},
		{
			script:   "echo 'Hello World'",
			expected: []string{},
		},
		{
			script:   "node build.js && rm -rf dist/",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		result := extractor.extractPathsFromScript(tt.script)
		if len(result) != len(tt.expected) {
			t.Errorf("extractPathsFromScript(%q) returned %d paths, want %d",
				tt.script, len(result), len(tt.expected))
			continue
		}

		resultSet := make(map[string]bool)
		for _, path := range result {
			resultSet[path] = true
		}

		for _, expected := range tt.expected {
			if !resultSet[expected] {
				t.Errorf("extractPathsFromScript(%q) missing expected path %q",
					tt.script, expected)
			}
		}
	}
}

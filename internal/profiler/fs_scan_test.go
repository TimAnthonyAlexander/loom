package profiler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFSScan(t *testing.T) {
	scanner := NewFSScan("/test/path")
	if scanner == nil {
		t.Fatal("NewFSScan returned nil")
	}
	if scanner.root != "/test/path" {
		t.Errorf("Expected root to be '/test/path', got %s", scanner.root)
	}
	if len(scanner.ignoreSet) == 0 {
		t.Error("Expected ignore patterns to be initialized")
	}
}

func TestFSScan_shouldIgnore(t *testing.T) {
	scanner := NewFSScan("/test")

	// Test ignore directories
	info := &mockFileInfo{isDir: true, size: 1024}
	if !scanner.shouldIgnore("node_modules", info) {
		t.Error("Expected node_modules to be ignored")
	}

	if !scanner.shouldIgnore(".git", info) {
		t.Error("Expected .git to be ignored")
	}

	// Test ignore files
	info = &mockFileInfo{isDir: false, size: 1024}
	if !scanner.shouldIgnore("test.min.js", info) {
		t.Error("Expected minified files to be ignored")
	}

	if !scanner.shouldIgnore("file.map", info) {
		t.Error("Expected source map files to be ignored")
	}

	// Test important files that should not be ignored
	if scanner.shouldIgnore(".gitignore", info) {
		t.Error("Expected .gitignore not to be ignored")
	}

	if scanner.shouldIgnore("package.json", info) {
		t.Error("Expected package.json not to be ignored")
	}

	// Test large files
	largeInfo := &mockFileInfo{isDir: false, size: 6 * 1024 * 1024} // 6MB
	if !scanner.shouldIgnore("large.txt", largeInfo) {
		t.Error("Expected very large files to be ignored")
	}

	// Test config files should not be ignored regardless of size
	largeConfigInfo := &mockFileInfo{isDir: false, size: 6 * 1024 * 1024}
	largeConfigInfo.name = "package.json" // Set name for config detection
	if scanner.shouldIgnore("package.json", largeConfigInfo) {
		t.Error("Expected large config files not to be ignored")
	}
}

func TestFSScan_categorizeFile(t *testing.T) {
	scanner := NewFSScan("/test")

	// Test regular source file
	info := &mockFileInfo{isDir: false, size: 1024}
	fileInfo := scanner.categorizeFile("src/main.go", info)

	if fileInfo.Path != "src/main.go" {
		t.Errorf("Expected path 'src/main.go', got %s", fileInfo.Path)
	}
	if fileInfo.Extension != ".go" {
		t.Errorf("Expected extension '.go', got %s", fileInfo.Extension)
	}
	if fileInfo.Basename != "main.go" {
		t.Errorf("Expected basename 'main.go', got %s", fileInfo.Basename)
	}

	// Test config file
	fileInfo = scanner.categorizeFile("package.json", info)
	if !fileInfo.IsConfig {
		t.Error("Expected package.json to be marked as config")
	}

	// Test doc file
	fileInfo = scanner.categorizeFile("README.md", info)
	if !fileInfo.IsDoc {
		t.Error("Expected README.md to be marked as doc")
	}

	// Test script file
	fileInfo = scanner.categorizeFile("build.sh", info)
	if !fileInfo.IsScript {
		t.Error("Expected build.sh to be marked as script")
	}

	// Test generated file
	fileInfo = scanner.categorizeFile("/generated/types.pb.go", info)
	if !fileInfo.IsGenerated {
		t.Error("Expected generated file to be marked as generated")
	}

	// Test vendored file
	fileInfo = scanner.categorizeFile("/vendor/package/file.go", info)
	if !fileInfo.IsVendored {
		t.Error("Expected vendored file to be marked as vendored")
	}
}

func TestIsConfigFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		ext      string
		expected bool
	}{
		{"package.json", "package.json", ".json", true},
		{"composer.json", "composer.json", ".json", true},
		{"go.mod", "go.mod", "", true},
		{"Dockerfile", "dockerfile", "", true},
		{"vite.config.ts", "vite.config.ts", ".ts", true},
		{"tsconfig.json", "tsconfig.json", ".json", true},
		{"regular file", "main.go", ".go", false},
		{"config in name", "app.config.js", ".js", true},
		{"eslint config", ".eslintrc.js", ".js", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isConfigFile(tt.filename, tt.ext)
			if result != tt.expected {
				t.Errorf("isConfigFile(%q, %q) = %v, want %v", tt.filename, tt.ext, result, tt.expected)
			}
		})
	}
}

func TestIsDocFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		ext      string
		expected bool
	}{
		{"README.md", "readme.md", ".md", true},
		{"CHANGELOG", "changelog", "", true},
		{"LICENSE", "license", "", true},
		{"markdown file", "guide.md", ".md", true},
		{"text file", "notes.txt", ".txt", true},
		{"source file", "main.go", ".go", false},
		{"config file", "package.json", ".json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDocFile(tt.filename, tt.ext)
			if result != tt.expected {
				t.Errorf("isDocFile(%q, %q) = %v, want %v", tt.filename, tt.ext, result, tt.expected)
			}
		})
	}
}

func TestIsScriptFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		ext      string
		expected bool
	}{
		{"shell script", "build.sh", ".sh", true},
		{"python script", "setup.py", ".py", true},
		{"Makefile", "makefile", "", true},
		{"Dockerfile", "dockerfile", "", true},
		{"batch file", "build.bat", ".bat", true},
		{"source file", "main.go", ".go", false},
		{"markdown", "README.md", ".md", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isScriptFile(tt.filename, tt.ext)
			if result != tt.expected {
				t.Errorf("isScriptFile(%q, %q) = %v, want %v", tt.filename, tt.ext, result, tt.expected)
			}
		})
	}
}

func TestIsGeneratedFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		path     string
		expected bool
	}{
		{"protobuf", "types.pb.go", "proto/types.pb.go", true},
		{"generated suffix", "api.generated.ts", "src/api.generated.ts", true},
		{"minified", "bundle.min.js", "dist/bundle.min.js", true},
		{"in build dir", "main.js", "/build/main.js", true},
		{"in dist dir", "app.css", "/dist/app.css", true},
		{"regular file", "main.go", "src/main.go", false},
		{"config file", "package.json", "package.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isGeneratedFile(tt.filename, tt.path)
			if result != tt.expected {
				t.Errorf("isGeneratedFile(%q, %q) = %v, want %v", tt.filename, tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsVendoredFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"vendor dir", "/vendor/package/file.go", true},
		{"node_modules", "/node_modules/package/index.js", true},
		{"third_party", "/third_party/lib/code.c", true},
		{"venv", "/.venv/lib/python/file.py", true},
		{"regular file", "src/main.go", false},
		{"project root", "main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isVendoredFile(tt.path)
			if result != tt.expected {
				t.Errorf("isVendoredFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsImportantLargeFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"package-lock.json", "package-lock.json", true},
		{"yarn.lock", "yarn.lock", true},
		{"composer.lock", "composer.lock", true},
		{"go.sum", "go.sum", true},
		{"regular large file", "large.txt", false},
		{"source file", "main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isImportantLargeFile(tt.filename)
			if result != tt.expected {
				t.Errorf("isImportantLargeFile(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestFSScan_Scan(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fs_scan_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create test directory structure
	srcDir := filepath.Join(tmpDir, "src")
	err = os.MkdirAll(srcDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create test files
	files := map[string]string{
		"package.json":     `{"name": "test"}`,
		"README.md":        "# Test Project",
		"src/main.go":      "package main\n\nfunc main() {}",
		"src/utils.ts":     "export function test() {}",
		"build.sh":         "#!/bin/bash\necho 'building'",
		".gitignore":       "node_modules/",
		"node_modules/pkg": "ignored",
	}

	for filePath, content := range files {
		fullPath := filepath.Join(tmpDir, filePath)
		dir := filepath.Dir(fullPath)

		if dir != tmpDir {
			err = os.MkdirAll(dir, 0755)
			if err != nil {
				t.Fatal(err)
			}
		}

		err = os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	scanner := NewFSScan(tmpDir)
	ctx := context.Background()

	scannedFiles, extensions, basenames := scanner.Scan(ctx)

	// Check that files were found
	if len(scannedFiles) == 0 {
		t.Error("Expected files to be found")
	}

	// Check that node_modules is ignored
	for _, file := range scannedFiles {
		if file.Path == "node_modules/pkg" {
			t.Error("Expected node_modules files to be ignored")
		}
	}

	// Check extensions mapping
	if len(extensions[".go"]) == 0 {
		t.Error("Expected Go files to be indexed by extension")
	}

	// Check basenames mapping
	if len(basenames["package.json"]) == 0 {
		t.Error("Expected package.json to be indexed by basename")
	}

	// Check file categorization
	var foundConfig, foundDoc, foundScript bool
	for _, file := range scannedFiles {
		if file.Basename == "package.json" && file.IsConfig {
			foundConfig = true
		}
		if file.Basename == "README.md" && file.IsDoc {
			foundDoc = true
		}
		if file.Basename == "build.sh" && file.IsScript {
			foundScript = true
		}
	}

	if !foundConfig {
		t.Error("Expected package.json to be categorized as config")
	}
	if !foundDoc {
		t.Error("Expected README.md to be categorized as doc")
	}
	if !foundScript {
		t.Error("Expected build.sh to be categorized as script")
	}
}

func TestFSScan_calculateEntropy(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "entropy_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	scanner := NewFSScan(tmpDir)

	// Create a text file (low entropy)
	textFile := filepath.Join(tmpDir, "text.txt")
	textContent := "This is a regular text file with repeated patterns. " +
		"This is a regular text file with repeated patterns. " +
		"This is a regular text file with repeated patterns."
	err = os.WriteFile(textFile, []byte(textContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	entropy := scanner.calculateEntropy("text.txt")
	if entropy > 6.0 {
		t.Errorf("Expected low entropy for text file, got %f", entropy)
	}

	// Create a binary-like file (high entropy)
	binaryFile := filepath.Join(tmpDir, "binary.bin")
	binaryContent := make([]byte, 1024)
	for i := range binaryContent {
		binaryContent[i] = byte(i % 256)
	}
	err = os.WriteFile(binaryFile, binaryContent, 0644)
	if err != nil {
		t.Fatal(err)
	}

	entropy = scanner.calculateEntropy("binary.bin")
	if entropy < 7.0 {
		t.Errorf("Expected high entropy for binary file, got %f", entropy)
	}
}

func TestFSScan_isBinaryFile(t *testing.T) {
	scanner := NewFSScan("/test")

	// Text files should not be binary
	textInfo := &mockFileInfo{isDir: false, size: 2048}
	if scanner.isBinaryFile("test.txt", textInfo) {
		t.Error("Expected .txt file not to be detected as binary")
	}

	if scanner.isBinaryFile("main.go", textInfo) {
		t.Error("Expected .go file not to be detected as binary")
	}

	// Small files should not be checked for entropy
	smallInfo := &mockFileInfo{isDir: false, size: 512}
	if scanner.isBinaryFile("unknown.bin", smallInfo) {
		t.Error("Expected small files not to be detected as binary")
	}
}

// Mock implementation of os.FileInfo for testing
type mockFileInfo struct {
	isDir bool
	size  int64
	name  string
}

func (m *mockFileInfo) Name() string {
	if m.name != "" {
		return m.name
	}
	return "test"
}
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

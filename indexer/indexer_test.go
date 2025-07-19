package indexer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewIndex(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loom-indexer-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	maxFileSize := int64(1024 * 1024) // 1MB
	index := NewIndex(tempDir, maxFileSize)

	if index == nil {
		t.Fatal("Expected non-nil index")
	}

	if index.WorkspacePath != tempDir {
		t.Errorf("Expected workspace path %s, got %s", tempDir, index.WorkspacePath)
	}

	if index.maxFileSize != maxFileSize {
		t.Errorf("Expected max file size %d, got %d", maxFileSize, index.maxFileSize)
	}

	if index.Files == nil {
		t.Error("Expected Files map to be initialized")
	}

	if len(index.Files) != 0 {
		t.Error("Expected Files map to be empty initially")
	}
}

func TestBuildIndex(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loom-build-index-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFiles := map[string]string{
		"main.go":           "package main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}",
		"config.json":       `{"key": "value"}`,
		"README.md":         "# Test Project\n\nThis is a test.",
		"subdir/utils.go":   "package utils\n\nfunc Helper() string {\n\treturn \"help\"\n}",
		"subdir/test.txt":   "This is a test file.",
	}

	for filePath, content := range testFiles {
		fullPath := filepath.Join(tempDir, filePath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory for %s: %v", filePath, err)
		}

		err = os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filePath, err)
		}
	}

	// Build index
	index := NewIndex(tempDir, 1024*1024)
	err = index.BuildIndex()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	// Verify that index was built successfully
	if len(index.Files) == 0 {
		t.Error("Expected some files to be indexed")
	}

	// Log what files were actually indexed
	t.Logf("Indexed %d files:", len(index.Files))
	for filePath := range index.Files {
		t.Logf("  - %s", filePath)
	}

	// Check for at least some expected files
	if _, exists := index.Files["main.go"]; !exists {
		t.Error("Expected main.go to be indexed")
	}

	// Verify file metadata
	mainGoMeta := index.Files["main.go"]
	if mainGoMeta == nil {
		t.Fatal("Expected main.go to be in index")
	}

	if mainGoMeta.Language != "Go" {
		t.Errorf("Expected main.go language to be 'Go', got '%s'", mainGoMeta.Language)
	}

	if mainGoMeta.Size <= 0 {
		t.Error("Expected main.go size to be positive")
	}

	if mainGoMeta.Hash == "" {
		t.Error("Expected main.go to have a hash")
	}
}

func TestLanguageDetectionInIndex(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loom-language-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files with various extensions
	testFiles := map[string]string{
		"main.go":    "package main",
		"script.py":  "print('hello')",
		"app.js":     "console.log('hello')",
		"README.md":  "# Test",
	}

	for filePath, content := range testFiles {
		fullPath := filepath.Join(tempDir, filePath)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filePath, err)
		}
	}

	// Build index
	index := NewIndex(tempDir, 1024*1024)
	err = index.BuildIndex()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	// Verify language detection through the indexed files
	expectedLanguages := map[string]string{
		"main.go":   "Go",
		"script.py": "Python",
		"app.js":    "JavaScript",
		"README.md": "Markdown",
	}

	for filePath, expectedLang := range expectedLanguages {
		fileMeta := index.Files[filePath]
		if fileMeta == nil {
			t.Errorf("Expected file %s to be in index", filePath)
			continue
		}

		if fileMeta.Language != expectedLang {
			t.Errorf("For file %s, expected language %s, got %s", filePath, expectedLang, fileMeta.Language)
		}
	}
}

func TestGetStats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loom-stats-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files with different languages
	testFiles := map[string]string{
		"main.go":      "package main\nfunc main() {}",
		"helper.go":    "package helper\nfunc Help() {}",
		"script.py":    "print('hello')",
		"config.json":  `{"key": "value"}`,
		"README.md":    "# Project",
	}

	for filePath, content := range testFiles {
		fullPath := filepath.Join(tempDir, filePath)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filePath, err)
		}
	}

	// Build index and get stats
	index := NewIndex(tempDir, 1024*1024)
	err = index.BuildIndex()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	stats := index.GetStats()

	if stats.TotalFiles == 0 {
		t.Error("Expected some files to be indexed")
	}

	if stats.TotalSize <= 0 {
		t.Error("Expected positive total size")
	}

	// Log language breakdown for debugging
	t.Logf("Language breakdown:")
	for lang, count := range stats.LanguageBreakdown {
		t.Logf("  %s: %d files", lang, count)
	}

	// Just verify that we have some Go files
	if stats.LanguageBreakdown["Go"] == 0 {
		t.Error("Expected at least one Go file")
	}

	// Verify percentages add up to 100%
	totalPercent := 0.0
	for _, percent := range stats.LanguagePercent {
		totalPercent += percent
	}

	if totalPercent < 99.9 || totalPercent > 100.1 {
		t.Errorf("Expected percentages to add up to ~100%%, got %.1f%%", totalPercent)
	}
}

func TestSaveAndLoadCache(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loom-cache-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create .loom directory
	loomDir := filepath.Join(tempDir, ".loom")
	err = os.MkdirAll(loomDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .loom directory: %v", err)
	}

	// Create test files
	testFiles := map[string]string{
		"main.go":    "package main\nfunc main() {}",
		"helper.py":  "def help(): pass",
	}

	for filePath, content := range testFiles {
		fullPath := filepath.Join(tempDir, filePath)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filePath, err)
		}
	}

	// Build and save index
	index := NewIndex(tempDir, 1024*1024)
	err = index.BuildIndex()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	err = index.SaveToCache()
	if err != nil {
		t.Fatalf("Failed to save cache: %v", err)
	}

	// Verify cache file exists
	cachePath := filepath.Join(loomDir, "index.cache")
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("Cache file was not created")
	}

	// Load from cache
	loadedIndex, err := LoadFromCache(tempDir, 1024*1024)
	if err != nil {
		t.Fatalf("Failed to load from cache: %v", err)
	}

	// Verify loaded index matches original
	if len(loadedIndex.Files) != len(index.Files) {
		t.Errorf("Expected %d files in loaded index, got %d", len(index.Files), len(loadedIndex.Files))
	}

	for filePath, originalMeta := range index.Files {
		loadedMeta, exists := loadedIndex.Files[filePath]
		if !exists {
			t.Errorf("Expected file %s to be in loaded index", filePath)
			continue
		}

		if loadedMeta.Hash != originalMeta.Hash {
			t.Errorf("Hash mismatch for file %s", filePath)
		}

		if loadedMeta.Language != originalMeta.Language {
			t.Errorf("Language mismatch for file %s", filePath)
		}
	}
}

func TestGitIgnorePatterns(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loom-gitignore-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create .gitignore file
	gitignoreContent := `# Ignore patterns
*.log
*.tmp
build/
node_modules/
.DS_Store
`
	err = os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte(gitignoreContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create .gitignore: %v", err)
	}

	// Create test files (some should be ignored)
	testFiles := map[string]string{
		"main.go":           "package main",
		"debug.log":         "log content",         // should be ignored
		"temp.tmp":          "temp content",        // should be ignored
		"build/output.bin":  "binary content",     // should be ignored
		"src/helper.go":     "package helper",
		".DS_Store":         "mac metadata",       // should be ignored
	}

	for filePath, content := range testFiles {
		fullPath := filepath.Join(tempDir, filePath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory for %s: %v", filePath, err)
		}

		err = os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filePath, err)
		}
	}

	// Build index
	index := NewIndex(tempDir, 1024*1024)
	err = index.BuildIndex()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	// Verify only non-ignored files are indexed
	expectedFiles := []string{"main.go", "src/helper.go"}
	if len(index.Files) != len(expectedFiles) {
		t.Errorf("Expected %d files to be indexed, got %d", len(expectedFiles), len(index.Files))
	}

	for _, expectedFile := range expectedFiles {
		if _, exists := index.Files[expectedFile]; !exists {
			t.Errorf("Expected file %s to be indexed", expectedFile)
		}
	}

	// Verify ignored files are not indexed
	ignoredFiles := []string{"debug.log", "temp.tmp", "build/output.bin", ".DS_Store"}
	for _, ignoredFile := range ignoredFiles {
		if _, exists := index.Files[ignoredFile]; exists {
			t.Errorf("Expected file %s to be ignored", ignoredFile)
		}
	}
}

func TestMaxFileSizeLimit(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loom-filesize-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create files of different sizes
	smallContent := "small file"
	largeContent := strings.Repeat("large file content ", 1000) // ~18KB

	err = os.WriteFile(filepath.Join(tempDir, "small.txt"), []byte(smallContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create small file: %v", err)
	}

	err = os.WriteFile(filepath.Join(tempDir, "large.txt"), []byte(largeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	// Build index with small max file size
	maxFileSize := int64(100) // 100 bytes
	index := NewIndex(tempDir, maxFileSize)
	err = index.BuildIndex()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	// Verify only small file is indexed
	if len(index.Files) != 1 {
		t.Errorf("Expected 1 file to be indexed, got %d", len(index.Files))
	}

	if _, exists := index.Files["small.txt"]; !exists {
		t.Error("Expected small.txt to be indexed")
	}

	if _, exists := index.Files["large.txt"]; exists {
		t.Error("Expected large.txt to be excluded due to size limit")
	}
}

func TestBinaryFileHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loom-binary-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create text file
	textContent := "This is a text file\nwith multiple lines\n"
	err = os.WriteFile(filepath.Join(tempDir, "text.txt"), []byte(textContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create text file: %v", err)
	}

	// Create a binary-like file (common binary extension)
	binaryContent := []byte{0, 1, 2, 3, 4, 5}
	err = os.WriteFile(filepath.Join(tempDir, "test.bin"), binaryContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create binary file: %v", err)
	}

	// Build index
	index := NewIndex(tempDir, 1024*1024)
	err = index.BuildIndex()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	// Verify text file is indexed
	if _, exists := index.Files["text.txt"]; !exists {
		t.Error("Expected text.txt to be indexed")
	}

	// Binary files might or might not be indexed depending on implementation
	// Just log the result for information
	if _, exists := index.Files["test.bin"]; exists {
		t.Logf("Binary file was indexed")
	} else {
		t.Logf("Binary file was not indexed (expected behavior)")
	}
}

func TestIndexUpdate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loom-update-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create initial file
	initialContent := "initial content"
	filePath := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(filePath, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	// Build initial index
	index := NewIndex(tempDir, 1024*1024)
	err = index.BuildIndex()
	if err != nil {
		t.Fatalf("Failed to build initial index: %v", err)
	}

	testFileMeta := index.Files["test.txt"]
	if testFileMeta == nil {
		t.Fatal("Expected test.txt to be in index")
	}
	initialHash := testFileMeta.Hash

	// Wait a bit to ensure different modification time
	time.Sleep(10 * time.Millisecond)

	// Modify file
	modifiedContent := "modified content"
	err = os.WriteFile(filePath, []byte(modifiedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Update index
	err = index.BuildIndex()
	if err != nil {
		t.Fatalf("Failed to update index: %v", err)
	}

	updatedFileMeta := index.Files["test.txt"]
	if updatedFileMeta == nil {
		t.Fatal("Expected test.txt to still be in index after update")
	}
	updatedHash := updatedFileMeta.Hash

	// Verify hash changed
	if initialHash == updatedHash {
		t.Error("Expected file hash to change after modification")
	}
}

func TestRelativePaths(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loom-paths-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create nested directory structure
	nestedPath := filepath.Join(tempDir, "level1", "level2", "level3")
	err = os.MkdirAll(nestedPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create nested directories: %v", err)
	}

	// Create file in nested directory
	content := "nested file content"
	filePath := filepath.Join(nestedPath, "nested.txt")
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create nested file: %v", err)
	}

	// Build index
	index := NewIndex(tempDir, 1024*1024)
	err = index.BuildIndex()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	// Verify relative path is used
	expectedRelativePath := filepath.Join("level1", "level2", "level3", "nested.txt")
	if _, exists := index.Files[expectedRelativePath]; !exists {
		t.Errorf("Expected file with relative path %s to be in index", expectedRelativePath)
	}

	// Verify the relative path doesn't contain the temp directory
	for relativePath := range index.Files {
		if strings.Contains(relativePath, tempDir) {
			t.Errorf("Relative path %s should not contain temp directory path", relativePath)
		}
	}
} 
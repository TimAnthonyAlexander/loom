package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRgPath(t *testing.T) {
	actualPath := rgPath()

	// Verify the path is not empty
	if actualPath == "" {
		t.Error("Expected rgPath() to return a non-empty path")
	}

	// Verify the binary exists at the returned path
	if _, err := os.Stat(actualPath); os.IsNotExist(err) {
		t.Errorf("Binary does not exist at returned path: %s", actualPath)
	}

	// Verify the binary is executable (check permissions)
	if info, err := os.Stat(actualPath); err == nil {
		mode := info.Mode()
		if mode&0111 == 0 { // Check if any execute bit is set
			t.Errorf("Binary at %s is not executable (mode: %s)", actualPath, mode)
		}
	}

	// Verify it's actually ripgrep by checking if we can run --version
	output, err := RunRipgrepWithArgs("--version")
	if err != nil {
		t.Errorf("Failed to run ripgrep --version: %v", err)
	} else {
		outputStr := string(output)
		if !strings.Contains(strings.ToLower(outputStr), "ripgrep") {
			t.Errorf("Binary doesn't appear to be ripgrep. Version output: %s", outputStr)
		}
	}

	t.Logf("✅ Ripgrep found and working at: %s", actualPath)
	t.Logf("   Platform: %s", runtime.GOOS)

	// Log which type of ripgrep installation we found
	if strings.Contains(actualPath, "/usr/bin") || strings.Contains(actualPath, "/opt/homebrew/bin") || strings.Contains(actualPath, "Program Files") {
		t.Logf("   Type: System-wide installation")
	} else if strings.Contains(actualPath, "tmp") || strings.Contains(actualPath, "temp") {
		t.Logf("   Type: Embedded binary (extracted to temp)")
	} else if strings.Contains(actualPath, "bin") {
		t.Logf("   Type: Bundled binary")
	} else {
		t.Logf("   Type: Unknown")
	}
}

func TestFindModuleRoot(t *testing.T) {
	// Test finding module root from current directory
	root, err := findModuleRoot()
	if err != nil {
		t.Fatalf("Failed to find module root: %v", err)
	}

	// Verify go.mod exists in the found root
	goModPath := filepath.Join(root, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		t.Errorf("go.mod not found in detected module root: %s", root)
	}

	t.Logf("Module root found: %s", root)

	// Test from a subdirectory
	tempDir, err := os.MkdirTemp("", "loom-module-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a nested directory structure
	nestedDir := filepath.Join(tempDir, "level1", "level2")
	err = os.MkdirAll(nestedDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create nested directories: %v", err)
	}

	// Create go.mod in temp root
	goModContent := "module test\n\ngo 1.21\n"
	err = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Change to nested directory and test finding module root
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(nestedDir)
	if err != nil {
		t.Fatalf("Failed to change to nested directory: %v", err)
	}

	foundRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("Failed to find module root from nested directory: %v", err)
	}

	// Resolve both paths to handle symlinks (e.g., /var -> /private/var on macOS)
	expectedRoot, err := filepath.EvalSymlinks(tempDir)
	if err != nil {
		expectedRoot = tempDir // fallback to original path if symlink resolution fails
	}

	actualRoot, err := filepath.EvalSymlinks(foundRoot)
	if err != nil {
		actualRoot = foundRoot // fallback to original path if symlink resolution fails
	}

	if actualRoot != expectedRoot {
		t.Errorf("Expected module root %s, got %s", expectedRoot, actualRoot)
	}
}

func TestFindModuleRootNotFound(t *testing.T) {
	// Test when go.mod is not found
	tempDir, err := os.MkdirTemp("", "loom-no-module-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	_, err = findModuleRoot()
	if err == nil {
		t.Error("Expected error when go.mod is not found")
	}

	if err != os.ErrNotExist {
		t.Errorf("Expected os.ErrNotExist, got %v", err)
	}
}

func TestRunRipgrep(t *testing.T) {
	// Create a temporary directory with test files
	tempDir, err := os.MkdirTemp("", "loom-ripgrep-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files with searchable content
	testFiles := map[string]string{
		"test1.txt": "Hello World\nThis is a test file\nContains some text",
		"test2.go":  "package main\n\nfunc main() {\n\tprintln(\"Hello World\")\n}",
		"test3.py":  "def hello():\n    print(\"Hello World\")\n    return True",
	}

	for fileName, content := range testFiles {
		filePath := filepath.Join(tempDir, fileName)
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", fileName, err)
		}
	}

	// Test searching for a pattern
	output, err := RunRipgrep("Hello", tempDir)
	if err != nil {
		t.Fatalf("RunRipgrep failed: %v", err)
	}

	outputStr := string(output)
	t.Logf("Ripgrep output: %s", outputStr)

	// Verify that the search found matches in multiple files
	if !strings.Contains(outputStr, "test1.txt") {
		t.Error("Expected output to contain test1.txt")
	}
	if !strings.Contains(outputStr, "test2.go") {
		t.Error("Expected output to contain test2.go")
	}
	if !strings.Contains(outputStr, "test3.py") {
		t.Error("Expected output to contain test3.py")
	}

	// Test searching for a pattern that doesn't exist
	output, err = RunRipgrep("NonExistentPattern", tempDir)
	if err != nil {
		// It's normal for ripgrep to return non-zero exit code when no matches found
		t.Logf("No matches found (expected): %v", err)
	}

	outputStr = string(output)
	if strings.Contains(outputStr, "test1.txt") || strings.Contains(outputStr, "test2.go") {
		t.Error("Expected no matches for non-existent pattern")
	}
}

func TestRunRipgrepWithArgs(t *testing.T) {
	// Create a temporary directory with test files
	tempDir, err := os.MkdirTemp("", "loom-ripgrep-args-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFiles := map[string]string{
		"file1.txt": "line1\nHELLO WORLD\nline3",
		"file2.go":  "package main\nfunc hello() {}\n// HELLO comment",
		"file3.py":  "def test():\n    # hello function\n    pass",
	}

	for fileName, content := range testFiles {
		filePath := filepath.Join(tempDir, fileName)
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", fileName, err)
		}
	}

	// Test case-insensitive search
	output, err := RunRipgrepWithArgs("-i", "hello", tempDir)
	if err != nil {
		t.Fatalf("RunRipgrepWithArgs failed for case-insensitive search: %v", err)
	}

	outputStr := string(output)
	t.Logf("Case-insensitive search output: %s", outputStr)

	// Should find matches in all files (HELLO, hello)
	if !strings.Contains(outputStr, "file1.txt") {
		t.Error("Expected case-insensitive search to find match in file1.txt")
	}
	if !strings.Contains(outputStr, "file2.go") {
		t.Error("Expected case-insensitive search to find match in file2.go")
	}
	if !strings.Contains(outputStr, "file3.py") {
		t.Error("Expected case-insensitive search to find match in file3.py")
	}

	// Test file type filtering
	output, err = RunRipgrepWithArgs("-t", "go", "hello", tempDir)
	if err != nil {
		t.Fatalf("RunRipgrepWithArgs failed for Go file type filter: %v", err)
	}

	outputStr = string(output)
	t.Logf("Go files only search output: %s", outputStr)

	// Should only find matches in .go files
	if !strings.Contains(outputStr, "file2.go") {
		t.Error("Expected Go file type search to find match in file2.go")
	}
	if strings.Contains(outputStr, "file1.txt") || strings.Contains(outputStr, "file3.py") {
		t.Error("Expected Go file type search to exclude non-Go files")
	}

	// Test line number display
	output, err = RunRipgrepWithArgs("-n", "line1", tempDir)
	if err != nil {
		t.Fatalf("RunRipgrepWithArgs failed for line number display: %v", err)
	}

	outputStr = string(output)
	t.Logf("Line number search output: %s", outputStr)

	// Should include line numbers in output
	if !strings.Contains(outputStr, ":1:") {
		t.Error("Expected line number output to include ':1:'")
	}

	// Test count only
	output, err = RunRipgrepWithArgs("-c", "hello", tempDir)
	if err != nil {
		t.Fatalf("RunRipgrepWithArgs failed for count only: %v", err)
	}

	outputStr = string(output)
	t.Logf("Count only search output: %s", outputStr)

	// Should only show counts, not the actual matches
	if strings.Contains(outputStr, "HELLO WORLD") || strings.Contains(outputStr, "hello function") {
		t.Error("Expected count only output to not include match content")
	}
}

func TestRipgrepBinaryExists(t *testing.T) {
	// Test that the ripgrep binary actually exists at the expected path
	rgBinaryPath := rgPath()

	// Check if the binary file exists
	if _, err := os.Stat(rgBinaryPath); os.IsNotExist(err) {
		t.Errorf("Ripgrep binary not found at expected path: %s", rgBinaryPath)
	}

	// Try to run ripgrep with --version to verify it's executable
	output, err := RunRipgrepWithArgs("--version")
	if err != nil {
		t.Fatalf("Failed to run ripgrep --version: %v", err)
	}

	outputStr := string(output)
	if !strings.Contains(strings.ToLower(outputStr), "ripgrep") {
		t.Errorf("Expected version output to contain 'ripgrep', got: %s", outputStr)
	}

	t.Logf("Ripgrep version: %s", strings.TrimSpace(outputStr))
}

func TestRipgrepErrorHandling(t *testing.T) {
	// Test with invalid arguments
	_, err := RunRipgrepWithArgs("--invalid-flag")
	if err == nil {
		t.Error("Expected error when using invalid flag")
	}

	// Test with non-existent directory
	_, err = RunRipgrep("pattern", "/non/existent/directory")
	if err == nil {
		t.Error("Expected error when searching in non-existent directory")
	}
}

func TestEmbeddedRipgrepFunctionality(t *testing.T) {
	// Test that the embedded ripgrep getter mechanism works
	// This tests the dependency injection pattern we use to avoid import cycles

	// Mock embedded ripgrep getter for testing
	mockEmbeddedPath := "/tmp/mock-rg"
	mockError := fmt.Errorf("mock error")

	// Test with successful embedded getter
	SetEmbeddedRipgrepGetter(func() (string, error) {
		return mockEmbeddedPath, nil
	})

	// Since we set a mock getter, rgPath should try the embedded path first
	// However, since the mock path doesn't exist, it will fall back to system ripgrep
	actualPath := rgPath()

	// The actual path should still work (either system or bundled)
	if actualPath == "" {
		t.Error("rgPath() should still return a valid path even with mock embedded getter")
	}

	// Test with failing embedded getter
	SetEmbeddedRipgrepGetter(func() (string, error) {
		return "", mockError
	})

	// Should fall back to system/bundled ripgrep
	actualPath = rgPath()
	if actualPath == "" {
		t.Error("rgPath() should fall back when embedded getter fails")
	}

	// Reset to nil for other tests
	SetEmbeddedRipgrepGetter(nil)

	t.Logf("✅ Embedded ripgrep functionality test passed")
}

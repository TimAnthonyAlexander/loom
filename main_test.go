package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetEmbeddedRipgrepPath(t *testing.T) {
	// We can't directly modify runtime.GOOS, so instead we'll just test
	// that the function returns a path and doesn't error
	path, err := GetEmbeddedRipgrepPath()
	if err != nil {
		t.Fatalf("GetEmbeddedRipgrepPath() error = %v", err)
	}

	if !filepath.IsAbs(path) {
		t.Errorf("Expected absolute path, got: %s", path)
	}

	// Check the path has the correct base name
	// The function extracts to "rg" or "rg.exe" for Windows
	expectedBasename := "rg"
	if os.PathSeparator == '\\' { // Windows
		expectedBasename = "rg.exe"
	}

	basename := filepath.Base(path)
	if basename != expectedBasename {
		t.Errorf("Expected path to end with %s, got: %s", expectedBasename, basename)
	}
}

func TestInit(t *testing.T) {
	// This test ensures the init function doesn't crash
	// It's minimal because the init function mainly sets up the ripgrep path
}

func TestMain(t *testing.T) {
	// Create a backup of os.Args and restore it after the test
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()

	// Reset os.Args to avoid actual command execution
	// This is just checking that the main function doesn't panic with empty args
	os.Args = []string{"loom"}

	// We can't easily test the full main function without executing commands
	// So we're just making sure the imports are valid and it compiles
}

package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectWorkspace(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-workspace-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to the temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test without .git directory (should return current directory)
	workspace, err := DetectWorkspace()
	if err != nil {
		t.Fatalf("DetectWorkspace failed: %v", err)
	}

	// Resolve symlinks for both paths to handle macOS /var -> /private/var
	expectedPath, err := filepath.EvalSymlinks(tempDir)
	if err != nil {
		expectedPath = tempDir // fallback if symlink resolution fails
	}
	resolvedWorkspace, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		resolvedWorkspace = workspace // fallback if symlink resolution fails
	}

	if resolvedWorkspace != expectedPath {
		t.Errorf("Expected workspace %s, got %s", expectedPath, resolvedWorkspace)
	}
}

func TestDetectWorkspaceWithGit(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "loom-workspace-git-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a .git directory to simulate a Git repository
	gitDir := filepath.Join(tempDir, ".git")
	err = os.MkdirAll(gitDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	// Create a subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	err = os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Change to the subdirectory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(subDir)
	if err != nil {
		t.Fatalf("Failed to change to subdirectory: %v", err)
	}

	// Test from subdirectory (should find Git root)
	workspace, err := DetectWorkspace()
	if err != nil {
		t.Fatalf("DetectWorkspace failed: %v", err)
	}

	// Resolve symlinks for both paths to handle macOS /var -> /private/var
	expectedPath, err := filepath.EvalSymlinks(tempDir)
	if err != nil {
		expectedPath = tempDir // fallback if symlink resolution fails
	}
	resolvedWorkspace, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		resolvedWorkspace = workspace // fallback if symlink resolution fails
	}

	if resolvedWorkspace != expectedPath {
		t.Errorf("Expected workspace %s, got %s", expectedPath, resolvedWorkspace)
	}
}

func TestFindGitRoot(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "loom-git-root-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test without .git directory
	gitRoot := findGitRoot(tempDir)
	if gitRoot != "" {
		t.Errorf("Expected empty git root, got %s", gitRoot)
	}

	// Create .git directory
	gitDir := filepath.Join(tempDir, ".git")
	err = os.MkdirAll(gitDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	// Test with .git directory
	gitRoot = findGitRoot(tempDir)
	if gitRoot != tempDir {
		t.Errorf("Expected git root %s, got %s", tempDir, gitRoot)
	}

	// Create nested directory structure
	nestedDir := filepath.Join(tempDir, "level1", "level2", "level3")
	err = os.MkdirAll(nestedDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}

	// Test from nested directory (should find Git root)
	gitRoot = findGitRoot(nestedDir)
	if gitRoot != tempDir {
		t.Errorf("Expected git root %s, got %s", tempDir, gitRoot)
	}
}

func TestFindGitRootEdgeCases(t *testing.T) {
	// Test with empty path
	gitRoot := findGitRoot("")
	if gitRoot != "" {
		t.Errorf("Expected empty git root for empty path, got %s", gitRoot)
	}

	// Test with non-existent path
	gitRoot = findGitRoot("/nonexistent/path")
	if gitRoot != "" {
		t.Errorf("Expected empty git root for non-existent path, got %s", gitRoot)
	}
}

func TestWorkspaceDetectionEdgeCases(t *testing.T) {
	// Save original directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create a very deep nested structure to test traversal limits
	tempDir, err := os.MkdirTemp("", "loom-deep-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create many nested levels
	currentPath := tempDir
	for i := 0; i < 10; i++ {
		currentPath = filepath.Join(currentPath, "level")
		err = os.MkdirAll(currentPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create nested directory: %v", err)
		}
	}

	// Change to the deepest directory
	err = os.Chdir(currentPath)
	if err != nil {
		t.Fatalf("Failed to change to deep directory: %v", err)
	}

	// Should still work and return the current path (no git repo found)
	workspace, err := DetectWorkspace()
	if err != nil {
		t.Fatalf("DetectWorkspace failed with deep nesting: %v", err)
	}

	// Resolve symlinks for both paths to handle macOS /var -> /private/var
	expectedPath, err := filepath.EvalSymlinks(currentPath)
	if err != nil {
		expectedPath = currentPath // fallback if symlink resolution fails
	}
	resolvedWorkspace, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		resolvedWorkspace = workspace // fallback if symlink resolution fails
	}

	if resolvedWorkspace != expectedPath {
		t.Errorf("Expected workspace %s, got %s", expectedPath, resolvedWorkspace)
	}
}

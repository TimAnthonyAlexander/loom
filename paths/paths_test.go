package paths

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestNewProjectPaths(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-paths-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test project paths creation
	projectPaths, err := NewProjectPaths(tempDir)
	if err != nil {
		t.Fatalf("Failed to create project paths: %v", err)
	}

	// Verify workspace path matches
	if projectPaths.WorkspacePath() != tempDir {
		t.Errorf("Expected workspace path %s, got %s", tempDir, projectPaths.WorkspacePath())
	}

	// Verify project hash is generated
	if projectPaths.ProjectHash() == "" {
		t.Error("Project hash should not be empty")
	}

	// Verify project hash is consistent
	projectPaths2, err := NewProjectPaths(tempDir)
	if err != nil {
		t.Fatalf("Failed to create second project paths: %v", err)
	}

	if projectPaths.ProjectHash() != projectPaths2.ProjectHash() {
		t.Error("Project hash should be consistent for same workspace")
	}
}

func TestProjectPathGeneration(t *testing.T) {
	// Test different workspace paths generate different hashes
	tempDir1, err := os.MkdirTemp("", "loom-test-1")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}
	defer os.RemoveAll(tempDir1)

	tempDir2, err := os.MkdirTemp("", "loom-test-2")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer os.RemoveAll(tempDir2)

	paths1, err := NewProjectPaths(tempDir1)
	if err != nil {
		t.Fatalf("Failed to create paths1: %v", err)
	}

	paths2, err := NewProjectPaths(tempDir2)
	if err != nil {
		t.Fatalf("Failed to create paths2: %v", err)
	}

	if paths1.ProjectHash() == paths2.ProjectHash() {
		t.Error("Different workspaces should generate different project hashes")
	}
}

func TestEnsureProjectDir(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-paths-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	projectPaths, err := NewProjectPaths(tempDir)
	if err != nil {
		t.Fatalf("Failed to create project paths: %v", err)
	}

	// Ensure project directories are created
	err = projectPaths.EnsureProjectDir()
	if err != nil {
		t.Fatalf("Failed to ensure project dir: %v", err)
	}

	// Check that all required directories exist
	dirs := []string{
		projectPaths.ProjectDir(),
		projectPaths.SessionsDir(),
		projectPaths.HistoryDir(),
		projectPaths.BackupsDir(),
		projectPaths.UndoDir(),
		projectPaths.StagingDir(),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory %s was not created", dir)
		}
	}
}

func TestPathMethods(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loom-paths-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	projectPaths, err := NewProjectPaths(tempDir)
	if err != nil {
		t.Fatalf("Failed to create project paths: %v", err)
	}

	// Test all path methods return non-empty strings
	paths := map[string]string{
		"ConfigPath":     projectPaths.ConfigPath(),
		"SessionsDir":    projectPaths.SessionsDir(),
		"HistoryDir":     projectPaths.HistoryDir(),
		"BackupsDir":     projectPaths.BackupsDir(),
		"UndoDir":        projectPaths.UndoDir(),
		"StagingDir":     projectPaths.StagingDir(),
		"IndexCachePath": projectPaths.IndexCachePath(),
		"MemoriesPath":   projectPaths.MemoriesPath(),
		"RulesPath":      projectPaths.RulesPath(),
	}

	for methodName, path := range paths {
		if path == "" {
			t.Errorf("%s returned empty path", methodName)
		}

		// Verify all paths contain the project hash
		if !strings.Contains(path, projectPaths.ProjectHash()) {
			t.Errorf("%s path %s does not contain project hash %s", methodName, path, projectPaths.ProjectHash())
		}
	}
}

func TestGetUserLoomDir(t *testing.T) {
	userLoomDir, err := GetUserLoomDir()
	if err != nil {
		t.Fatalf("Failed to get user loom dir: %v", err)
	}

	if userLoomDir == "" {
		t.Error("User loom dir should not be empty")
	}

	// Verify it ends with .loom
	if !strings.HasSuffix(userLoomDir, ".loom") {
		t.Errorf("User loom dir should end with .loom, got: %s", userLoomDir)
	}
}

func TestGetGlobalConfigPath(t *testing.T) {
	globalConfigPath, err := GetGlobalConfigPath()
	if err != nil {
		t.Fatalf("Failed to get global config path: %v", err)
	}

	if globalConfigPath == "" {
		t.Error("Global config path should not be empty")
	}

	// Verify it ends with config.json
	if !strings.HasSuffix(globalConfigPath, "config.json") {
		t.Errorf("Global config path should end with config.json, got: %s", globalConfigPath)
	}
}

func TestCrossPlatformPaths(t *testing.T) {
	// Test that paths work correctly on different platforms
	tempDir, err := os.MkdirTemp("", "loom-paths-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	projectPaths, err := NewProjectPaths(tempDir)
	if err != nil {
		t.Fatalf("Failed to create project paths: %v", err)
	}

	// Verify all paths use forward slashes for consistency
	paths := []string{
		projectPaths.ConfigPath(),
		projectPaths.IndexCachePath(),
		projectPaths.MemoriesPath(),
		projectPaths.RulesPath(),
	}

	for _, path := range paths {
		// On Windows, the path should still be valid
		if runtime.GOOS == "windows" {
			if !strings.Contains(path, "\\") && !strings.Contains(path, "/") {
				t.Errorf("Path should contain path separators: %s", path)
			}
		} else {
			// On Unix-like systems, should use forward slashes
			if strings.Contains(path, "\\") {
				t.Errorf("Path should not contain backslashes on Unix: %s", path)
			}
		}
	}
}

func TestProjectInfo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "loom-paths-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	projectPaths, err := NewProjectPaths(tempDir)
	if err != nil {
		t.Fatalf("Failed to create project paths: %v", err)
	}

	info := projectPaths.GetProjectInfo()

	// Verify required fields are present
	requiredFields := []string{"workspace_path", "project_hash", "project_dir"}
	for _, field := range requiredFields {
		if info[field] == nil {
			t.Errorf("Project info missing field: %s", field)
		}
	}

	// Verify workspace path matches
	if info["workspace_path"] != tempDir {
		t.Errorf("Expected workspace path %s, got %v", tempDir, info["workspace_path"])
	}
}

package workspace

import (
	"os"
	"path/filepath"
)

// DetectWorkspace detects the workspace root directory
// It tries to find the Git repository root, otherwise uses the current directory
func DetectWorkspace() (string, error) {
	// Get current working directory
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Try to find Git repository root
	gitRoot := findGitRoot(pwd)
	if gitRoot != "" {
		return gitRoot, nil
	}

	// If no Git repository found, use current directory
	return pwd, nil
}

// findGitRoot walks up the directory tree looking for a .git directory
func findGitRoot(startPath string) string {
	currentPath := startPath

	for {
		gitPath := filepath.Join(currentPath, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return currentPath
		}

		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			// Reached the root directory
			break
		}
		currentPath = parentPath
	}

	return ""
}

// EnsureLoomDir creates a .loom directory in the given workspace path
// This is kept for backward compatibility with tests

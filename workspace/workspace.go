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

// EnsureLoomDir creates the .loom directory if it doesn't exist
func EnsureLoomDir(workspacePath string) error {
	loomDir := filepath.Join(workspacePath, ".loom")

	// Check if .loom directory already exists
	if _, err := os.Stat(loomDir); os.IsNotExist(err) {
		// Create .loom directory
		err := os.MkdirAll(loomDir, 0755)
		if err != nil {
			return err
		}

		// Create empty index.cache file
		indexPath := filepath.Join(loomDir, "index.cache")
		file, err := os.Create(indexPath)
		if err != nil {
			return err
		}
		file.Close()
	}

	return nil
}

package paths

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ProjectPaths provides access to all loom directories for a specific project
type ProjectPaths struct {
	workspacePath string
	projectHash   string
	projectDir    string
}

// NewProjectPaths creates a new ProjectPaths instance for the given workspace
func NewProjectPaths(workspacePath string) (*ProjectPaths, error) {
	// Normalize workspace path to ensure consistent hashing
	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute workspace path: %w", err)
	}

	// Generate project hash from workspace path
	projectHash := generateProjectHash(absPath)

	// Get user loom directory
	userLoomDir, err := GetUserLoomDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user loom directory: %w", err)
	}

	projectDir := filepath.Join(userLoomDir, "projects", projectHash)

	return &ProjectPaths{
		workspacePath: absPath,
		projectHash:   projectHash,
		projectDir:    projectDir,
	}, nil
}

// GetUserLoomDir returns the user-level loom directory (~/.loom)
func GetUserLoomDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(homeDir, ".loom"), nil
}

// GetGlobalConfigPath returns the path to the global config file
func GetGlobalConfigPath() (string, error) {
	userLoomDir, err := GetUserLoomDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(userLoomDir, "config.json"), nil
}

// ProjectDir returns the project-specific loom directory
func (p *ProjectPaths) ProjectDir() string {
	return p.projectDir
}

// ConfigPath returns the path to the project-specific config file
func (p *ProjectPaths) ConfigPath() string {
	return filepath.Join(p.projectDir, "config.json")
}

// SessionsDir returns the sessions directory for this project
func (p *ProjectPaths) SessionsDir() string {
	return filepath.Join(p.projectDir, "sessions")
}

// HistoryDir returns the chat history directory for this project
func (p *ProjectPaths) HistoryDir() string {
	return filepath.Join(p.projectDir, "history")
}

// BackupsDir returns the backups directory for this project
func (p *ProjectPaths) BackupsDir() string {
	return filepath.Join(p.projectDir, "backups")
}

// UndoDir returns the undo directory for this project
func (p *ProjectPaths) UndoDir() string {
	return filepath.Join(p.projectDir, "undo")
}

// StagingDir returns the staging directory for this project
func (p *ProjectPaths) StagingDir() string {
	return filepath.Join(p.projectDir, "staging")
}

// IndexCachePath returns the path to the index cache file
func (p *ProjectPaths) IndexCachePath() string {
	return filepath.Join(p.projectDir, "index.cache")
}

// MemoriesPath returns the path to the memories file
func (p *ProjectPaths) MemoriesPath() string {
	return filepath.Join(p.projectDir, "memories.json")
}

// RulesPath returns the path to the project rules file
func (p *ProjectPaths) RulesPath() string {
	return filepath.Join(p.projectDir, "rules.json")
}

// EnsureProjectDir creates the project directory and all required subdirectories
func (p *ProjectPaths) EnsureProjectDir() error {
	dirs := []string{
		p.ProjectDir(),
		p.SessionsDir(),
		p.HistoryDir(),
		p.BackupsDir(),
		p.UndoDir(),
		p.StagingDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// WorkspacePath returns the original workspace path
func (p *ProjectPaths) WorkspacePath() string {
	return p.workspacePath
}

// ProjectHash returns the project hash
func (p *ProjectPaths) ProjectHash() string {
	return p.projectHash
}

// generateProjectHash creates a unique hash for a workspace path
func generateProjectHash(workspacePath string) string {
	// Normalize path separators for cross-platform consistency
	normalizedPath := filepath.ToSlash(workspacePath)

	// On Windows, convert to lowercase for case-insensitive filesystem
	if runtime.GOOS == "windows" {
		normalizedPath = strings.ToLower(normalizedPath)
	}

	// Create hash
	hasher := sha256.New()
	hasher.Write([]byte(normalizedPath))
	hash := fmt.Sprintf("%x", hasher.Sum(nil))

	// Return first 16 characters for brevity while maintaining uniqueness
	return hash[:16]
}

// GetProjectInfo returns human-readable info about the project
func (p *ProjectPaths) GetProjectInfo() map[string]interface{} {
	return map[string]interface{}{
		"workspace_path": p.workspacePath,
		"project_hash":   p.projectHash,
		"project_dir":    p.projectDir,
	}
}

// ListAllProjects returns information about all projects in the user loom directory

// Check if projects directory exists

// Note: We can't easily reverse the hash to get the original path,
// but we could store it in a metadata file if needed

// CleanupEmptyProjects removes project directories that are empty or contain only empty subdirectories

// Check if projects directory exists

// Check if directory is empty or contains only empty subdirectories

// Skip on error

// Ignore errors during cleanup

// isDirectoryEmpty checks if a directory is empty or contains only empty subdirectories

// Found a file, directory is not empty

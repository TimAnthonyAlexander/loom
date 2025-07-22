package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MigrationStatus represents the status of migration for a workspace
type MigrationStatus struct {
	WorkspacePath     string   `json:"workspace_path"`
	HasLegacyLoom     bool     `json:"has_legacy_loom"`
	MigrationNeeded   bool     `json:"migration_needed"`
	MigrationComplete bool     `json:"migration_complete"`
	BackupCreated     bool     `json:"backup_created"`
	Issues            []string `json:"issues,omitempty"`
}

// CheckMigrationStatus checks if a workspace needs migration
func CheckMigrationStatus(workspacePath string) (*MigrationStatus, error) {
	status := &MigrationStatus{
		WorkspacePath: workspacePath,
		Issues:        []string{},
	}

	// Check if legacy .loom directory exists
	legacyLoomDir := filepath.Join(workspacePath, ".loom")
	if _, err := os.Stat(legacyLoomDir); err == nil {
		status.HasLegacyLoom = true

		// Check if it has any files that need migration
		files, err := os.ReadDir(legacyLoomDir)
		if err != nil {
			status.Issues = append(status.Issues, fmt.Sprintf("Cannot read legacy .loom directory: %v", err))
			return status, nil
		}

		// Check for files that should be migrated
		migrateableFiles := []string{"config.json", "memories.json", "rules.json", "index.cache"}
		migrateableDirs := []string{"sessions", "history", "backups", "undo", "staging"}

		for _, file := range files {
			name := file.Name()

			// Check for files to migrate
			for _, migratable := range migrateableFiles {
				if name == migratable {
					status.MigrationNeeded = true
					break
				}
			}

			// Check for directories to migrate
			if file.IsDir() {
				for _, migratable := range migrateableDirs {
					if name == migratable {
						status.MigrationNeeded = true
						break
					}
				}
			}
		}
	}

	// Check if migration has already been completed
	projectPaths, err := NewProjectPaths(workspacePath)
	if err == nil {
		if _, err := os.Stat(projectPaths.ProjectDir()); err == nil {
			status.MigrationComplete = true
		}
	}

	return status, nil
}

// MigrateWorkspace migrates a workspace from legacy .loom to new user-level structure
func MigrateWorkspace(workspacePath string, createBackup bool) error {
	status, err := CheckMigrationStatus(workspacePath)
	if err != nil {
		return fmt.Errorf("failed to check migration status: %w", err)
	}

	if !status.HasLegacyLoom {
		return fmt.Errorf("no legacy .loom directory found in workspace")
	}

	if !status.MigrationNeeded {
		return fmt.Errorf("no migration needed - legacy directory is empty")
	}

	// Get project paths for destination
	projectPaths, err := NewProjectPaths(workspacePath)
	if err != nil {
		return fmt.Errorf("failed to create project paths: %w", err)
	}

	// Ensure destination directories exist
	if err := projectPaths.EnsureProjectDir(); err != nil {
		return fmt.Errorf("failed to create destination directories: %w", err)
	}

	legacyLoomDir := filepath.Join(workspacePath, ".loom")

	// Create backup if requested
	if createBackup {
		backupPath := filepath.Join(workspacePath, ".loom.backup")
		if err := copyDirectory(legacyLoomDir, backupPath); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
		fmt.Printf("Created backup: %s\n", backupPath)
	}

	// Migrate files
	migrationMap := map[string]string{
		"config.json":   projectPaths.ConfigPath(),
		"memories.json": projectPaths.MemoriesPath(),
		"rules.json":    projectPaths.RulesPath(),
		"index.cache":   projectPaths.IndexCachePath(),
	}

	for legacyName, destPath := range migrationMap {
		legacyPath := filepath.Join(legacyLoomDir, legacyName)
		if _, err := os.Stat(legacyPath); err == nil {
			if err := copyFile(legacyPath, destPath); err != nil {
				return fmt.Errorf("failed to migrate %s: %w", legacyName, err)
			}
			fmt.Printf("Migrated: %s\n", legacyName)
		}
	}

	// Migrate directories
	dirMigrationMap := map[string]string{
		"sessions": projectPaths.SessionsDir(),
		"history":  projectPaths.HistoryDir(),
		"backups":  projectPaths.BackupsDir(),
		"undo":     projectPaths.UndoDir(),
		"staging":  projectPaths.StagingDir(),
	}

	for legacyDirName, destDir := range dirMigrationMap {
		legacyDir := filepath.Join(legacyLoomDir, legacyDirName)
		if _, err := os.Stat(legacyDir); err == nil {
			if err := copyDirectory(legacyDir, destDir); err != nil {
				return fmt.Errorf("failed to migrate %s directory: %w", legacyDirName, err)
			}
			fmt.Printf("Migrated directory: %s\n", legacyDirName)
		}
	}

	fmt.Printf("Migration completed successfully!\n")
	fmt.Printf("New location: %s\n", projectPaths.ProjectDir())

	return nil
}

// RemoveLegacyLoom removes the legacy .loom directory after successful migration
func RemoveLegacyLoom(workspacePath string, force bool) error {
	legacyLoomDir := filepath.Join(workspacePath, ".loom")

	if !force {
		// Check if migration was completed
		status, err := CheckMigrationStatus(workspacePath)
		if err != nil {
			return fmt.Errorf("failed to check migration status: %w", err)
		}

		if !status.MigrationComplete {
			return fmt.Errorf("migration not completed - use force flag to override")
		}
	}

	if _, err := os.Stat(legacyLoomDir); os.IsNotExist(err) {
		return fmt.Errorf("legacy .loom directory does not exist")
	}

	if err := os.RemoveAll(legacyLoomDir); err != nil {
		return fmt.Errorf("failed to remove legacy .loom directory: %w", err)
	}

	fmt.Printf("Removed legacy .loom directory: %s\n", legacyLoomDir)
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, 0644)
}

// copyDirectory recursively copies a directory
func copyDirectory(src, dst string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(path, destPath)
	})
}

// ListLegacyWorkspaces finds all workspaces that have legacy .loom directories
func ListLegacyWorkspaces(searchPaths []string) ([]MigrationStatus, error) {
	var legacyWorkspaces []MigrationStatus

	for _, searchPath := range searchPaths {
		err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			// Look for .loom directories
			if info.IsDir() && info.Name() == ".loom" {
				workspacePath := filepath.Dir(path)

				// Skip if this is inside another .loom directory
				if strings.Contains(workspacePath, ".loom") {
					return nil
				}

				status, err := CheckMigrationStatus(workspacePath)
				if err != nil {
					return nil // Skip errors
				}

				if status.HasLegacyLoom && status.MigrationNeeded {
					legacyWorkspaces = append(legacyWorkspaces, *status)
				}

				return filepath.SkipDir // Don't recurse into .loom
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to search %s: %w", searchPath, err)
		}
	}

	return legacyWorkspaces, nil
}

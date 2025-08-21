package write

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/loom/loom/internal/profiler/shared"
)

// ProjectProfileWriter writes the main project profile JSON
type ProjectProfileWriter struct {
	root string
}

// NewProjectProfileWriter creates a new project profile writer
func NewProjectProfileWriter(root string) *ProjectProfileWriter {
	return &ProjectProfileWriter{root: root}
}

// Write writes the project profile to .loom/project_profile.json
func (w *ProjectProfileWriter) Write(profile *shared.Profile) error {
	// Ensure .loom directory exists
	loomDir := filepath.Join(w.root, ".loom")
	if err := os.MkdirAll(loomDir, 0755); err != nil {
		return err
	}

	// Set creation timestamp
	profile.CreatedAtUnix = time.Now().Unix()
	profile.Version = "1"

	// Write to temporary file first for atomicity
	tempPath := filepath.Join(loomDir, "project_profile.json.tmp")
	file, err := os.Create(tempPath)
	if err != nil {
		return err
	}

	// Write JSON with indentation for readability
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(profile); err != nil {
		_ = file.Close()
		_ = os.Remove(tempPath)
		return err
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	// Atomic rename
	finalPath := filepath.Join(loomDir, "project_profile.json")
	return os.Rename(tempPath, finalPath)
}

// Read reads an existing project profile if it exists
func (w *ProjectProfileWriter) Read() (*shared.Profile, error) {
	profilePath := filepath.Join(w.root, ".loom", "project_profile.json")

	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, err
	}

	var profile shared.Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, err
	}

	// Check version compatibility
	if !w.isVersionCompatible(profile.Version) {
		return nil, &VersionIncompatibleError{
			CurrentVersion:  profile.Version,
			ExpectedVersion: "2",
		}
	}

	return &profile, nil
}

// VersionIncompatibleError indicates the profile version is incompatible
type VersionIncompatibleError struct {
	CurrentVersion  string
	ExpectedVersion string
}

func (e *VersionIncompatibleError) Error() string {
	return fmt.Sprintf("profile version %s is incompatible with expected version %s", e.CurrentVersion, e.ExpectedVersion)
}

// isVersionCompatible checks if a profile version is compatible
func (w *ProjectProfileWriter) isVersionCompatible(version string) bool {
	// For now, only version "2" is compatible
	// Future versions should maintain backward compatibility or handle migration
	return version == "2"
}

// Exists checks if a project profile exists
func (w *ProjectProfileWriter) Exists() bool {
	profilePath := filepath.Join(w.root, ".loom", "project_profile.json")
	_, err := os.Stat(profilePath)
	return err == nil
}

// IsStale checks if the profile is older than any of the key files
func (w *ProjectProfileWriter) IsStale() bool {
	if !w.Exists() {
		return true
	}

	profilePath := filepath.Join(w.root, ".loom", "project_profile.json")
	profileInfo, err := os.Stat(profilePath)
	if err != nil {
		return true
	}

	profileTime := profileInfo.ModTime()

	// Check key files that should trigger a refresh
	keyFiles := []string{
		"package.json",
		"composer.json",
		"go.mod",
		"Cargo.toml",
		"pyproject.toml",
		"wails.json",
		"README.md",
		"Makefile",
		"Dockerfile",
		"docker-compose.yml",
		".gitignore",
	}

	for _, keyFile := range keyFiles {
		filePath := filepath.Join(w.root, keyFile)
		if info, err := os.Stat(filePath); err == nil {
			if info.ModTime().After(profileTime) {
				return true
			}
		}
	}

	return false
}

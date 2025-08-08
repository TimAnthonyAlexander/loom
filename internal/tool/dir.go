package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListDirArgs represents the arguments for the list_dir tool.
type ListDirArgs struct {
	Path string `json:"path"`
}

// ListDirResult represents the result of the list_dir tool.
type ListDirResult struct {
	Path     string     `json:"path"`
	Entries  []DirEntry `json:"entries"`
	IsDir    bool       `json:"is_dir"`
	Error    string     `json:"error,omitempty"`
	FullPath string     `json:"-"` // Full absolute path (not sent to LLM)
}

// DirEntry represents a single entry in a directory.
type DirEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size,omitempty"` // Size in bytes, only for files
	ModTime string `json:"mod_time"`       // ISO format timestamp
}

// RegisterListDir registers the list_dir tool with the registry.
func RegisterListDir(registry *Registry, workspacePath string) error {
	return registry.Register(Definition{
		Name:        "list_dir",
		Description: "List the contents of a directory in the workspace",
		Safe:        true, // Listing directories is safe
		JSONSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the directory, relative to the workspace root (default: current directory)",
				},
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args ListDirArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("failed to parse arguments: %w", err)
			}

			// Default to root directory if not specified
			if args.Path == "" {
				args.Path = "."
			}

			return listDir(ctx, workspacePath, args)
		},
	})
}

// listDir implements the directory listing logic.
func listDir(ctx context.Context, workspacePath string, args ListDirArgs) (*ListDirResult, error) {
	// Normalize and validate the path
	absPath, err := validatePath(workspacePath, args.Path)
	if err != nil {
		return &ListDirResult{
			Path:  args.Path,
			Error: err.Error(),
		}, nil
	}

	// Get file info to check if it's a directory
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return &ListDirResult{
			Path:  args.Path,
			Error: fmt.Sprintf("Failed to access path: %v", err),
		}, nil
	}

	// If it's not a directory, return info about the file
	if !fileInfo.IsDir() {
		return &ListDirResult{
			Path:  args.Path,
			IsDir: false,
			Entries: []DirEntry{
				{
					Name:    fileInfo.Name(),
					IsDir:   false,
					Size:    fileInfo.Size(),
					ModTime: fileInfo.ModTime().Format("2006-01-02T15:04:05Z07:00"),
				},
			},
			FullPath: absPath,
		}, nil
	}

	// Read directory contents
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return &ListDirResult{
			Path:  args.Path,
			Error: fmt.Sprintf("Failed to read directory: %v", err),
		}, nil
	}

	// Convert directory entries to our format
	dirEntries := make([]DirEntry, 0, len(entries))
	for _, entry := range entries {
		// Skip .git directory and other hidden files by default
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		// Create our entry
		dirEntry := DirEntry{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
		}

		// If it's a file, get its size
		if info, err := entry.Info(); err == nil && !entry.IsDir() {
			dirEntry.Size = info.Size()
			dirEntry.ModTime = info.ModTime().Format("2006-01-02T15:04:05Z07:00")
		}

		dirEntries = append(dirEntries, dirEntry)
	}

	// Sort entries: directories first, then files, both alphabetically
	sort.Slice(dirEntries, func(i, j int) bool {
		if dirEntries[i].IsDir != dirEntries[j].IsDir {
			return dirEntries[i].IsDir // Directories come first
		}
		return dirEntries[i].Name < dirEntries[j].Name // Alpha sort within each group
	})

	return &ListDirResult{
		Path:     args.Path,
		IsDir:    true,
		Entries:  dirEntries,
		FullPath: absPath,
	}, nil
}

// validatePath ensures the path is valid and within the workspace.
func validatePath(workspacePath string, dirPath string) (string, error) {
	// Convert to absolute path if needed
	var absPath string
	if filepath.IsAbs(dirPath) {
		absPath = dirPath
	} else {
		absPath = filepath.Join(workspacePath, dirPath)
	}

	// Clean the path to remove ../ and ./ segments
	absPath = filepath.Clean(absPath)

	// Ensure the path is within the workspace
	workspacePath = filepath.Clean(workspacePath)
	if !strings.HasPrefix(absPath, workspacePath) {
		return "", fmt.Errorf("path must be within the workspace")
	}

	return absPath, nil
}

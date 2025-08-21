package write

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/loom/loom/internal/profiler/shared"
)

// HotlistWriter writes the hotlist.txt file
type HotlistWriter struct {
	root string
}

// NewHotlistWriter creates a new hotlist writer
func NewHotlistWriter(root string) *HotlistWriter {
	return &HotlistWriter{root: root}
}

// Write writes the top files to .loom/hotlist.txt
func (w *HotlistWriter) Write(importantFiles []shared.ImportantFile) error {
	// Ensure .loom directory exists
	loomDir := filepath.Join(w.root, ".loom")
	if err := os.MkdirAll(loomDir, 0755); err != nil {
		return err
	}

	// Take top 50 files for hotlist
	maxFiles := 50
	hotlistFiles := importantFiles
	if len(hotlistFiles) > maxFiles {
		hotlistFiles = hotlistFiles[:maxFiles]
	}

	// Build content
	var content strings.Builder
	content.WriteString("# Loom Hotlist - Top Important Files\n")
	content.WriteString("# Generated automatically by Loom Project Profiler\n")
	content.WriteString("# Files listed in order of importance\n\n")

	for i, file := range hotlistFiles {
		content.WriteString(file.Path)
		if i < len(hotlistFiles)-1 {
			content.WriteString("\n")
		}
	}

	// Write to temporary file first for atomicity
	tempPath := filepath.Join(loomDir, "hotlist.txt.tmp")
	if err := os.WriteFile(tempPath, []byte(content.String()), 0644); err != nil {
		return err
	}

	// Atomic rename
	finalPath := filepath.Join(loomDir, "hotlist.txt")
	return os.Rename(tempPath, finalPath)
}

// Read reads the hotlist if it exists
func (w *HotlistWriter) Read() ([]string, error) {
	hotlistPath := filepath.Join(w.root, ".loom", "hotlist.txt")

	data, err := os.ReadFile(hotlistPath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var files []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip comments and empty lines
		if line != "" && !strings.HasPrefix(line, "#") {
			files = append(files, line)
		}
	}

	return files, nil
}

// Exists checks if a hotlist exists
func (w *HotlistWriter) Exists() bool {
	hotlistPath := filepath.Join(w.root, ".loom", "hotlist.txt")
	_, err := os.Stat(hotlistPath)
	return err == nil
}

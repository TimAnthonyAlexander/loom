package editor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// LineRange represents a range of lines in a file.
type LineRange struct {
	StartLine int
	EndLine   int
}

// EditPlan represents a planned file edit.
type EditPlan struct {
	FilePath     string
	OldContent   string
	NewContent   string
	Diff         string
	IsCreation   bool
	IsDeletion   bool
	ChangedLines LineRange
}

// ValidationError represents an error during edit validation.
type ValidationError struct {
	Message string
	Code    string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s (code: %s)", e.Message, e.Code)
}

// validatePath ensures the file path is valid and within the workspace.
func validatePath(workspacePath string, filePath string) (string, error) {
	// Convert to absolute path if needed
	var absPath string
	if filepath.IsAbs(filePath) {
		absPath = filePath
	} else {
		absPath = filepath.Join(workspacePath, filePath)
	}

	// Clean the path to remove ../ and ./ segments
	absPath = filepath.Clean(absPath)

	// Ensure the path is within the workspace
	if !strings.HasPrefix(absPath, workspacePath) {
		return "", ValidationError{
			Message: "File path must be within the workspace",
			Code:    "PATH_TRAVERSAL",
		}
	}

	return absPath, nil
}

// generateDiff creates a unified diff between old and new content.
func generateDiff(oldContent, newContent, fileName string) string {
	// Handle special cases
	if oldContent == "" {
		return fmt.Sprintf("Creating new file: %s\n\n%s", fileName, newContent)
	}

	if newContent == "" {
		return fmt.Sprintf("Deleting file: %s\n\n%s", fileName, oldContent)
	}

	// Use a proper diff library
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldContent, newContent, false)

	// Convert to a user-friendly format with line numbers
	oldLines := strings.Split(oldContent, "\n")

	diffText := fmt.Sprintf("--- %s (before)\n+++ %s (after)\n\n", fileName, fileName)

	// Generate line-by-line diff
	oldLineNum := 1
	newLineNum := 1

	// Track differences by line
	changedLines := make(map[int]bool)
	for _, diff := range diffs {
		diffLines := strings.Split(diff.Text, "\n")

		for i, line := range diffLines {
			// Skip empty last element from the split
			if i == len(diffLines)-1 && line == "" && len(diffLines) > 1 {
				continue
			}

			switch diff.Type {
			case diffmatchpatch.DiffDelete:
				diffText += fmt.Sprintf("-%4d: %s\n", oldLineNum, line)
				changedLines[oldLineNum] = true
				oldLineNum++
			case diffmatchpatch.DiffInsert:
				diffText += fmt.Sprintf("+%4d: %s\n", newLineNum, line)
				changedLines[newLineNum] = true
				newLineNum++
			case diffmatchpatch.DiffEqual:
				// Show context lines (3 before and after changes)
				shouldShowContext := false
				for i := -3; i <= 3; i++ {
					if changedLines[oldLineNum+i] {
						shouldShowContext = true
						break
					}
				}

				if shouldShowContext || oldLineNum <= 3 || oldLineNum > len(oldLines)-3 {
					diffText += fmt.Sprintf(" %4d: %s\n", oldLineNum, line)
				} else if oldLineNum == 4 || oldLineNum == len(oldLines)-3 {
					diffText += "  ...\n"
				}

				oldLineNum++
				newLineNum++
			}
		}
	}

	// Add a summary of changes
	totalChanges := len(changedLines)
	diffText += fmt.Sprintf("\n%d line(s) changed\n", totalChanges)

	return diffText
}

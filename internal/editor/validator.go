package editor

import (
	"fmt"
	"os"
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

// ProposeEdit validates and creates an edit plan for file modifications.
func ProposeEdit(
	workspacePath string,
	filePath string,
	oldString string,
	newString string,
) (*EditPlan, error) {
	// Normalize and validate file path
	absPath, err := validatePath(workspacePath, filePath)
	if err != nil {
		return nil, err
	}

	// Check if file exists
	fileExists := true
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			fileExists = false
		} else {
			return nil, ValidationError{
				Message: fmt.Sprintf("Failed to access file: %v", err),
				Code:    "FILE_ACCESS_ERROR",
			}
		}
	}

	// Check if it's a directory
	if fileExists && fileInfo.IsDir() {
		return nil, ValidationError{
			Message: "Cannot edit a directory",
			Code:    "IS_DIRECTORY",
		}
	}

	// Handle file creation
	if !fileExists {
		// For file creation, oldString should be empty
		if oldString != "" {
			return nil, ValidationError{
				Message: "Cannot replace text in a file that doesn't exist",
				Code:    "FILE_NOT_EXIST",
			}
		}

		// Create a new file edit plan
		lineCount := 1 + strings.Count(newString, "\n")
		return &EditPlan{
			FilePath:   absPath,
			OldContent: "",
			NewContent: newString,
			Diff:       generateDiff("", newString, filepath.Base(absPath)),
			IsCreation: true,
			ChangedLines: LineRange{
				StartLine: 1,
				EndLine:   lineCount,
			},
		}, nil
	}

	// Read existing file content
	oldContent, err := os.ReadFile(absPath)
	if err != nil {
		return nil, ValidationError{
			Message: fmt.Sprintf("Failed to read file: %v", err),
			Code:    "FILE_READ_ERROR",
		}
	}

	oldContentStr := string(oldContent)

	// Handle file deletion
	if newString == "" && oldString == "" {
		lineCount := 1 + strings.Count(oldContentStr, "\n")
		return &EditPlan{
			FilePath:   absPath,
			OldContent: oldContentStr,
			NewContent: "",
			Diff:       generateDiff(oldContentStr, "", filepath.Base(absPath)),
			IsDeletion: true,
			ChangedLines: LineRange{
				StartLine: 1,
				EndLine:   lineCount,
			},
		}, nil
	}

	// Handle normal edits
	if oldString == "" {
		return nil, ValidationError{
			Message: "Old string cannot be empty for existing files",
			Code:    "EMPTY_OLD_STRING",
		}
	}

	// Check if old string exists in file
	if !strings.Contains(oldContentStr, oldString) {
		return nil, ValidationError{
			Message: "Old string not found in file",
			Code:    "STRING_NOT_FOUND",
		}
	}

	// Check if the replacement is ambiguous (multiple occurrences)
	count := strings.Count(oldContentStr, oldString)
	if count > 1 {
		return nil, ValidationError{
			Message: fmt.Sprintf("Old string occurs %d times, replacement is ambiguous", count),
			Code:    "AMBIGUOUS_REPLACEMENT",
		}
	}

	// Create new content by replacing the string
	newContent := strings.Replace(oldContentStr, oldString, newString, 1)

	// Calculate line range affected by the change
	lineRange, err := calculateLineRange(oldContentStr, oldString, newString)
	if err != nil {
		return nil, ValidationError{
			Message: fmt.Sprintf("Failed to calculate line range: %v", err),
			Code:    "LINE_RANGE_ERROR",
		}
	}

	return &EditPlan{
		FilePath:     absPath,
		OldContent:   oldContentStr,
		NewContent:   newContent,
		Diff:         generateDiff(oldContentStr, newContent, filepath.Base(absPath)),
		ChangedLines: lineRange,
	}, nil
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

// calculateLineRange determines which lines are affected by an edit.
func calculateLineRange(content, oldString, newString string) (LineRange, error) {
	// Find the offset of the old string in the content
	offset := strings.Index(content, oldString)
	if offset == -1 {
		return LineRange{}, fmt.Errorf("could not locate the string to replace")
	}

	// Calculate line numbers
	beforeOffset := content[:offset]
	startLine := 1 + strings.Count(beforeOffset, "\n")

	// Find end line of the old string
	endLine := startLine + strings.Count(oldString, "\n")

	// Note: We've calculated the affected lines based on the string positions

	// Create a buffer zone of context
	const contextLines = 3
	startLineWithContext := startLine - contextLines
	if startLineWithContext < 1 {
		startLineWithContext = 1
	}

	endLineWithContext := endLine + contextLines
	totalLines := strings.Count(content, "\n") + 1
	if endLineWithContext > totalLines {
		endLineWithContext = totalLines
	}

	return LineRange{
		StartLine: startLineWithContext,
		EndLine:   endLineWithContext,
	}, nil
}

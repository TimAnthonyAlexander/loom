package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EditPlan represents a planned file edit.
type EditPlan struct {
	FilePath   string
	OldContent string
	NewContent string
	Diff       string
	IsCreation bool
	IsDeletion bool
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
		return &EditPlan{
			FilePath:   absPath,
			OldContent: "",
			NewContent: newString,
			Diff:       generateDiff("", newString, filepath.Base(absPath)),
			IsCreation: true,
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
		return &EditPlan{
			FilePath:   absPath,
			OldContent: oldContentStr,
			NewContent: "",
			Diff:       generateDiff(oldContentStr, "", filepath.Base(absPath)),
			IsDeletion: true,
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

	return &EditPlan{
		FilePath:   absPath,
		OldContent: oldContentStr,
		NewContent: newContent,
		Diff:       generateDiff(oldContentStr, newContent, filepath.Base(absPath)),
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
	// Simple diff implementation for now
	// In a real implementation, this would use a proper diff library
	if oldContent == "" {
		return fmt.Sprintf("Creating new file: %s\n\n%s", fileName, newContent)
	}

	if newContent == "" {
		return fmt.Sprintf("Deleting file: %s\n\n%s", fileName, oldContent)
	}

	return fmt.Sprintf("--- %s (before)\n+++ %s (after)\n\nOld content:\n%s\n\nNew content:\n%s",
		fileName, fileName, oldContent, newContent)
}

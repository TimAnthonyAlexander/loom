package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ActionType defines the type of advanced edit to perform.
type ActionType string

const (
	ActionCreate        ActionType = "CREATE"
	ActionReplaceLines  ActionType = "REPLACE"
	ActionInsertAfter   ActionType = "INSERT_AFTER"
	ActionInsertBefore  ActionType = "INSERT_BEFORE"
	ActionDeleteLines   ActionType = "DELETE"
	ActionSearchReplace ActionType = "SEARCH_REPLACE"
)

// AdvancedEditRequest captures parameters for advanced edits.
type AdvancedEditRequest struct {
	FilePath string
	Action   ActionType
	// Common content payload for actions that add/replace text
	Content string
	// Line-based addressing (1-indexed, inclusive)
	StartLine int
	EndLine   int
	Line      int // Used for insert before/after
	// Search/replace payload
	OldString string
	NewString string
}

// ProposeAdvancedEdit validates and constructs an EditPlan based on an AdvancedEditRequest.
func ProposeAdvancedEdit(workspacePath string, req AdvancedEditRequest) (*EditPlan, error) {
	// Normalize and validate file path
	absPath, err := validatePath(workspacePath, req.FilePath)
	if err != nil {
		return nil, err
	}

	// Determine file existence
	fileInfo, statErr := os.Stat(absPath)
	fileExists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, ValidationError{
			Message: fmt.Sprintf("Failed to access file: %v", statErr),
			Code:    "FILE_ACCESS_ERROR",
		}
	}

	// Disallow directory edits
	if fileExists && fileInfo.IsDir() {
		return nil, ValidationError{
			Message: "Cannot edit a directory",
			Code:    "IS_DIRECTORY",
		}
	}

	switch req.Action {
	case ActionCreate:
		// CREATE new file
		if fileExists {
			return nil, ValidationError{
				Message: "File already exists",
				Code:    "FILE_EXISTS",
			}
		}
		lineCount := 1
		if req.Content != "" {
			lineCount = 1 + strings.Count(req.Content, "\n")
		}
		return &EditPlan{
			FilePath:   absPath,
			OldContent: "",
			NewContent: req.Content,
			Diff:       generateDiff("", req.Content, filepath.Base(absPath)),
			IsCreation: true,
			ChangedLines: LineRange{
				StartLine: 1,
				EndLine:   lineCount,
			},
		}, nil

	case ActionReplaceLines, ActionInsertAfter, ActionInsertBefore, ActionDeleteLines, ActionSearchReplace:
		if !fileExists {
			return nil, ValidationError{
				Message: "File does not exist",
				Code:    "FILE_NOT_EXIST",
			}
		}

		// Load current content
		bytes, err := os.ReadFile(absPath)
		if err != nil {
			return nil, ValidationError{
				Message: fmt.Sprintf("Failed to read file: %v", err),
				Code:    "FILE_READ_ERROR",
			}
		}

		oldContent := string(bytes)
		lines := splitToLinesPreserveEOF(oldContent)

		var newContent string
		var changed LineRange

		switch req.Action {
		case ActionReplaceLines:
			if req.StartLine <= 0 || req.EndLine <= 0 || req.StartLine > req.EndLine {
				return nil, ValidationError{
					Message: "Invalid line range for REPLACE",
					Code:    "INVALID_RANGE",
				}
			}
			startIdx := req.StartLine - 1
			endIdx := req.EndLine - 1
			if startIdx >= len(lines) || endIdx >= len(lines) {
				return nil, ValidationError{
					Message: "Line range out of bounds",
					Code:    "RANGE_OOB",
				}
			}
			replacement := strings.Split(req.Content, "\n")
			// Perform replacement
			var merged []string
			merged = append(merged, lines[:startIdx]...)
			merged = append(merged, replacement...)
			merged = append(merged, lines[endIdx+1:]...)
			newContent = strings.Join(merged, "\n")
			changed = LineRange{StartLine: req.StartLine, EndLine: req.StartLine + len(replacement) - 1}

		case ActionInsertBefore:
			if req.Line <= 0 {
				return nil, ValidationError{
					Message: "Line must be >= 1 for INSERT_BEFORE",
					Code:    "INVALID_LINE",
				}
			}
			insertIdx := req.Line - 1
			if insertIdx > len(lines) { // allow at most len(lines) for inserting at EOF
				return nil, ValidationError{
					Message: "Insert position out of bounds",
					Code:    "LINE_OOB",
				}
			}
			insertion := strings.Split(req.Content, "\n")
			var merged []string
			merged = append(merged, lines[:insertIdx]...)
			merged = append(merged, insertion...)
			merged = append(merged, lines[insertIdx:]...)
			newContent = strings.Join(merged, "\n")
			changed = LineRange{StartLine: req.Line, EndLine: req.Line + len(insertion) - 1}

		case ActionInsertAfter:
			if req.Line < 0 {
				return nil, ValidationError{
					Message: "Line must be >= 0 for INSERT_AFTER",
					Code:    "INVALID_LINE",
				}
			}
			// After last line means append at end
			if req.Line > len(lines) {
				return nil, ValidationError{
					Message: "Insert position out of bounds",
					Code:    "LINE_OOB",
				}
			}
			insertion := strings.Split(req.Content, "\n")
			insertIdx := req.Line // because it's after the given line (1-indexed)
			if insertIdx < 0 {
				insertIdx = 0
			}
			if insertIdx > len(lines) {
				insertIdx = len(lines)
			}
			var merged []string
			merged = append(merged, lines[:insertIdx]...)
			merged = append(merged, insertion...)
			merged = append(merged, lines[insertIdx:]...)
			newContent = strings.Join(merged, "\n")
			changed = LineRange{StartLine: req.Line + 1, EndLine: req.Line + len(insertion)}

		case ActionDeleteLines:
			if req.StartLine <= 0 || req.EndLine <= 0 || req.StartLine > req.EndLine {
				return nil, ValidationError{
					Message: "Invalid line range for DELETE",
					Code:    "INVALID_RANGE",
				}
			}
			startIdx := req.StartLine - 1
			endIdx := req.EndLine - 1
			if startIdx >= len(lines) || endIdx >= len(lines) {
				return nil, ValidationError{
					Message: "Line range out of bounds",
					Code:    "RANGE_OOB",
				}
			}
			var merged []string
			merged = append(merged, lines[:startIdx]...)
			if endIdx+1 < len(lines) {
				merged = append(merged, lines[endIdx+1:]...)
			}
			newContent = strings.Join(merged, "\n")
			changed = LineRange{StartLine: req.StartLine, EndLine: req.EndLine}

		case ActionSearchReplace:
			if req.OldString == "" {
				return nil, ValidationError{
					Message: "old_string cannot be empty for SEARCH_REPLACE",
					Code:    "EMPTY_OLD_STRING",
				}
			}
			occurrences := strings.Count(oldContent, req.OldString)
			if occurrences == 0 {
				return nil, ValidationError{
					Message: "Old string not found in file",
					Code:    "STRING_NOT_FOUND",
				}
			}
			newContent = strings.ReplaceAll(oldContent, req.OldString, req.NewString)

			// Determine affected line range (min..max lines that contained the old string)
			minLine := 0
			maxLine := 0
			for i, ln := range strings.Split(oldContent, "\n") {
				if strings.Contains(ln, req.OldString) {
					lineNum := i + 1
					if minLine == 0 || lineNum < minLine {
						minLine = lineNum
					}
					if lineNum > maxLine {
						maxLine = lineNum
					}
				}
			}
			if minLine == 0 {
				minLine = 1
			}
			if maxLine == 0 {
				maxLine = strings.Count(oldContent, "\n") + 1
			}
			changed = LineRange{StartLine: minLine, EndLine: maxLine}
		}

		return &EditPlan{
			FilePath:     absPath,
			OldContent:   oldContent,
			NewContent:   newContent,
			Diff:         generateDiff(oldContent, newContent, filepath.Base(absPath)),
			ChangedLines: changed,
		}, nil

	default:
		return nil, ValidationError{
			Message: fmt.Sprintf("Unsupported action: %s", req.Action),
			Code:    "UNSUPPORTED_ACTION",
		}
	}
}

// splitToLinesPreserveEOF splits into lines without dropping the last empty line when the file ends with a newline.
func splitToLinesPreserveEOF(content string) []string {
	// Using strings.Split preserves trailing empty segment when content ends with a newline
	return strings.Split(content, "\n")
}

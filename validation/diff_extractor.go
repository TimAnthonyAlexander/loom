package validation

import (
	"fmt"
	"os"
	"strings"

	"loom/loom_edit"
)

// ExtractEditContext extracts the context around an edit operation for verification
func ExtractEditContext(filePath string, editCmd *loom_edit.EditCommand, originalContent string) (*EditContext, error) {
	// Read the current (post-edit) file content
	currentContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read current file content: %w", err)
	}

	// Normalize line endings for consistent processing
	normalizedCurrent := strings.ReplaceAll(string(currentContent), "\r\n", "\n")
	normalizedCurrent = strings.ReplaceAll(normalizedCurrent, "\r", "\n")

	normalizedOriginal := strings.ReplaceAll(originalContent, "\r\n", "\n")
	normalizedOriginal = strings.ReplaceAll(normalizedOriginal, "\r", "\n")

	// Split into lines
	currentLines := strings.Split(normalizedCurrent, "\n")
	originalLines := strings.Split(normalizedOriginal, "\n")

	// Remove trailing empty line if it exists (artifact of splitting)
	if len(currentLines) > 0 && currentLines[len(currentLines)-1] == "" {
		currentLines = currentLines[:len(currentLines)-1]
	}
	if len(originalLines) > 0 && originalLines[len(originalLines)-1] == "" {
		originalLines = originalLines[:len(originalLines)-1]
	}

	// Detect language
	languageID := DetectLanguage(filePath)

	// Create the context based on the edit action
	context := &EditContext{
		FilePath:   filePath,
		StartLine:  editCmd.Start,
		EndLine:    editCmd.End,
		LanguageID: languageID,
		EditAction: editCmd.Action,
	}

	// Default context size - could be made configurable
	contextSize := 8

	// Extract context for different edit actions
	switch editCmd.Action {
	case "REPLACE":
		return extractReplaceContext(context, currentLines, originalLines, editCmd, contextSize)
	case "INSERT_AFTER", "INSERT_BEFORE":
		return extractInsertContext(context, currentLines, originalLines, editCmd, contextSize)
	case "DELETE":
		return extractDeleteContext(context, currentLines, originalLines, editCmd, contextSize)
	case "CREATE":
		return extractCreateContext(context, currentLines, editCmd, contextSize)
	case "SEARCH_REPLACE":
		return extractSearchReplaceContext(context, currentLines, originalLines, editCmd, contextSize)
	default:
		return nil, fmt.Errorf("unsupported edit action: %s", editCmd.Action)
	}
}

// extractReplaceContext handles REPLACE operations
func extractReplaceContext(context *EditContext, currentLines, originalLines []string, editCmd *loom_edit.EditCommand, contextSize int) (*EditContext, error) {
	// For REPLACE, the line numbers in editCmd refer to the original file
	originalStart := editCmd.Start - 1 // Convert to 0-based
	originalEnd := editCmd.End         // End is inclusive in LOOM_EDIT, exclusive for slicing

	// Extract original lines that were replaced
	if originalStart >= 0 && originalEnd <= len(originalLines) {
		context.OriginalLines = make([]string, originalEnd-originalStart)
		copy(context.OriginalLines, originalLines[originalStart:originalEnd])
	}

	// Calculate new content range - this is trickier since the file may have changed size
	newContent := strings.Split(editCmd.NewText, "\n")
	newStartLine := editCmd.Start
	newEndLine := editCmd.Start + len(newContent) - 1

	// Update context to reflect actual current state
	context.EndLine = newEndLine

	// Extract current lines in the modified range
	currentStart := newStartLine - 1 // Convert to 0-based
	currentEnd := newEndLine         // End is inclusive

	if currentStart >= 0 && currentEnd <= len(currentLines) {
		context.ModifiedLines = make([]string, currentEnd-currentStart)
		copy(context.ModifiedLines, currentLines[currentStart:currentEnd])
	}

	// Extract context before and after
	contextStart := max(0, newStartLine-contextSize-1)
	contextEnd := min(len(currentLines), newEndLine+contextSize)

	if contextStart < newStartLine-1 {
		context.ContextBefore = make([]string, newStartLine-1-contextStart)
		copy(context.ContextBefore, currentLines[contextStart:newStartLine-1])
	}

	if newEndLine < contextEnd {
		context.ContextAfter = make([]string, contextEnd-newEndLine)
		copy(context.ContextAfter, currentLines[newEndLine:contextEnd])
	}

	return context, nil
}

// extractInsertContext handles INSERT_AFTER and INSERT_BEFORE operations
func extractInsertContext(context *EditContext, currentLines, originalLines []string, editCmd *loom_edit.EditCommand, contextSize int) (*EditContext, error) {
	// For inserts, the line number refers to where the insertion happened
	insertPoint := editCmd.Start
	newContent := strings.Split(editCmd.NewText, "\n")

	// Calculate the range of newly inserted lines
	var newStartLine, newEndLine int
	if editCmd.Action == "INSERT_AFTER" {
		newStartLine = insertPoint + 1
		newEndLine = insertPoint + len(newContent)
	} else { // INSERT_BEFORE
		newStartLine = insertPoint
		newEndLine = insertPoint + len(newContent) - 1
	}

	context.StartLine = newStartLine
	context.EndLine = newEndLine

	// Extract the inserted lines
	currentStart := newStartLine - 1 // Convert to 0-based
	currentEnd := newEndLine         // End is inclusive

	if currentStart >= 0 && currentEnd <= len(currentLines) {
		context.ModifiedLines = make([]string, currentEnd-currentStart)
		copy(context.ModifiedLines, currentLines[currentStart:currentEnd])
	}

	// No original lines for insert operations
	context.OriginalLines = []string{}

	// Extract context before and after
	contextStart := max(0, newStartLine-contextSize-1)
	contextEnd := min(len(currentLines), newEndLine+contextSize)

	if contextStart < newStartLine-1 {
		context.ContextBefore = make([]string, newStartLine-1-contextStart)
		copy(context.ContextBefore, currentLines[contextStart:newStartLine-1])
	}

	if newEndLine < contextEnd {
		context.ContextAfter = make([]string, contextEnd-newEndLine)
		copy(context.ContextAfter, currentLines[newEndLine:contextEnd])
	}

	return context, nil
}

// extractDeleteContext handles DELETE operations
func extractDeleteContext(context *EditContext, currentLines, originalLines []string, editCmd *loom_edit.EditCommand, contextSize int) (*EditContext, error) {
	// For DELETE, we show the context around where the deletion happened
	deleteStart := editCmd.Start - 1 // Convert to 0-based
	deleteEnd := editCmd.End         // End is inclusive

	// Extract original lines that were deleted
	if deleteStart >= 0 && deleteEnd <= len(originalLines) {
		context.OriginalLines = make([]string, deleteEnd-deleteStart)
		copy(context.OriginalLines, originalLines[deleteStart:deleteEnd])
	}

	// No modified lines for delete operations
	context.ModifiedLines = []string{}

	// For context, use the point where deletion occurred
	contextPoint := editCmd.Start - 1 // Point in current file where deletion happened
	contextStart := max(0, contextPoint-contextSize)
	contextEnd := min(len(currentLines), contextPoint+contextSize)

	if contextStart < contextPoint {
		context.ContextBefore = make([]string, contextPoint-contextStart)
		copy(context.ContextBefore, currentLines[contextStart:contextPoint])
	}

	if contextPoint < contextEnd {
		context.ContextAfter = make([]string, contextEnd-contextPoint)
		copy(context.ContextAfter, currentLines[contextPoint:contextEnd])
	}

	return context, nil
}

// extractCreateContext handles CREATE operations (new files)
func extractCreateContext(context *EditContext, currentLines []string, editCmd *loom_edit.EditCommand, contextSize int) (*EditContext, error) {
	// For CREATE, show the entire file content (or first part if it's very large)
	context.StartLine = 1
	context.EndLine = len(currentLines)
	context.OriginalLines = []string{} // No original content for new files

	// Limit the content shown to avoid overwhelming the LLM
	maxLines := contextSize * 4 // Show more for new files, but still limit
	if len(currentLines) <= maxLines {
		context.ModifiedLines = make([]string, len(currentLines))
		copy(context.ModifiedLines, currentLines)
	} else {
		// Show first part and indicate truncation
		context.ModifiedLines = make([]string, maxLines+1)
		copy(context.ModifiedLines[:maxLines], currentLines[:maxLines])
		context.ModifiedLines[maxLines] = fmt.Sprintf("... (file continues for %d more lines)", len(currentLines)-maxLines)
		context.EndLine = maxLines
	}

	// No before/after context for new files
	context.ContextBefore = []string{}
	context.ContextAfter = []string{}

	return context, nil
}

// extractSearchReplaceContext handles SEARCH_REPLACE operations
func extractSearchReplaceContext(context *EditContext, currentLines, originalLines []string, editCmd *loom_edit.EditCommand, contextSize int) (*EditContext, error) {
	// For SEARCH_REPLACE, we need to find where the changes actually occurred
	// This is more complex since the search string could span multiple lines or occur multiple times

	// For now, provide a general context around the file
	// TODO: Implement more sophisticated detection of actual change locations

	context.StartLine = 1
	context.EndLine = len(currentLines)
	context.OriginalLines = []string{fmt.Sprintf("Search pattern: %s", editCmd.OldString)}
	context.ModifiedLines = []string{fmt.Sprintf("Replacement: %s", editCmd.NewString)}

	// Show beginning and end of file as context
	showLines := min(contextSize, len(currentLines))

	if showLines > 0 {
		context.ContextBefore = make([]string, showLines)
		copy(context.ContextBefore, currentLines[:showLines])

		if len(currentLines) > showLines {
			startIdx := max(0, len(currentLines)-showLines)
			context.ContextAfter = make([]string, len(currentLines)-startIdx)
			copy(context.ContextAfter, currentLines[startIdx:])
		}
	}

	return context, nil
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

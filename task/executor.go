package task

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"loom/indexer"
	"loom/memory"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// Constants for directory listing limits
const (
	MaxDirectoryListingFiles = 1000   // Maximum number of files to list
	MaxDirectoryListingDepth = 10     // Maximum depth for recursive listing
	MaxListingOutputSize     = 100000 // Maximum characters in listing output (~25k tokens)
)

// Executor handles task execution with security constraints
type Executor struct {
	workspacePath       string
	enableShell         bool
	maxFileSize         int64
	gitIgnore           *indexer.GitIgnore
	interactiveExecutor *InteractiveExecutor
	memoryStore         *memory.MemoryStore
	loomEditProcessor   *LoomEditProcessor
}

// NewExecutor creates a new task executor
func NewExecutor(workspacePath string, enableShell bool, maxFileSize int64) *Executor {
	// Load gitignore patterns
	gitIgnore, err := indexer.LoadGitIgnore(workspacePath)
	if err != nil {
		// Continue without .gitignore if it fails to load
		gitIgnore = &indexer.GitIgnore{}
	}

	// Create interactive executor
	interactiveExecutor := NewInteractiveExecutor(workspacePath, enableShell)

	// Create memory store
	memoryStore := memory.NewMemoryStore(workspacePath)

	// Create LOOM_EDIT processor
	loomEditProcessor := NewLoomEditProcessor(workspacePath)

	return &Executor{
		workspacePath:       workspacePath,
		enableShell:         enableShell,
		maxFileSize:         maxFileSize,
		gitIgnore:           gitIgnore,
		interactiveExecutor: interactiveExecutor,
		memoryStore:         memoryStore,
		loomEditProcessor:   loomEditProcessor,
	}
}

// Execute runs a single task and returns the response
func (e *Executor) Execute(task *Task) *TaskResponse {
	response := &TaskResponse{
		Task:    *task,
		Success: false,
	}

	switch task.Type {
	case TaskTypeReadFile:
		return e.executeReadFile(task)
	case TaskTypeEditFile:
		return e.executeEditFile(task)
	case TaskTypeListDir:
		return e.executeListDir(task)
	case TaskTypeRunShell:
		return e.executeRunShell(task)
	case TaskTypeSearch:
		return e.executeSearch(task)
	case TaskTypeMemory:
		return e.executeMemory(task)
	default:
		response.Error = fmt.Sprintf("unknown task type: %s", task.Type)
		return response
	}
}

// executeReadFile reads a file with optional line/size limits
func (e *Executor) executeReadFile(task *Task) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// Security: ensure path is within workspace
	fullPath, err := e.securePath(task.Path)
	if err != nil {
		response.Error = err.Error()
		return response
	}

	// Check if file exists
	info, err := os.Stat(fullPath)
	if err != nil {
		response.Error = fmt.Sprintf("file not found: %s", task.Path)
		return response
	}

	// Check if it's a directory
	if info.IsDir() {
		response.Error = fmt.Sprintf("path is a directory: %s", task.Path)
		return response
	}

	// Check file size
	if info.Size() > e.maxFileSize {
		response.Error = fmt.Sprintf("file too large: %s (%.2f MB > %.2f MB)",
			task.Path, float64(info.Size())/1024/1024, float64(e.maxFileSize)/1024/1024)
		return response
	}

	// Check if file is binary
	if e.isBinaryFile(fullPath) {
		response.Error = fmt.Sprintf("cannot read binary file: %s", task.Path)
		return response
	}

	// Read file to get total line count first
	totalLines, err := e.countFileLines(fullPath)
	if err != nil {
		response.Error = fmt.Sprintf("failed to count file lines: %v", err)
		return response
	}

	// Read file
	file, err := os.Open(fullPath)
	if err != nil {
		response.Error = fmt.Sprintf("failed to open file: %v", err)
		return response
	}
	defer file.Close()

	var content strings.Builder
	scanner := bufio.NewScanner(file)
	lineNum := 0
	linesRead := 0
	skippedLines := 0

	// Determine reading strategy
	startLine := task.StartLine
	endLine := task.EndLine
	maxLines := task.MaxLines

	// Set defaults and validate ranges
	if startLine <= 0 {
		startLine = 1
	}
	if endLine > 0 && endLine < startLine {
		response.Error = fmt.Sprintf("invalid line range: start_line (%d) > end_line (%d)", startLine, endLine)
		return response
	}
	if maxLines <= 0 {
		maxLines = DefaultMaxLines // Default limit
	}

	// Calculate effective reading window
	effectiveEndLine := endLine
	if endLine <= 0 || endLine > totalLines {
		effectiveEndLine = totalLines
	}

	// If we have a specific range, but it would exceed maxLines, adjust
	if endLine > 0 {
		rangeSize := endLine - startLine + 1
		if rangeSize > maxLines {
			effectiveEndLine = startLine + maxLines - 1
		}
	}

	// Read the file
	for scanner.Scan() {
		lineNum++

		// Skip lines before start
		if lineNum < startLine {
			skippedLines++
			continue
		}

		// Stop if we've reached the end of our range
		if effectiveEndLine > 0 && lineNum > effectiveEndLine {
			break
		}

		// Stop if we've read enough lines
		if linesRead >= maxLines {
			break
		}

		if linesRead > 0 {
			content.WriteString("\n")
		}

		// Add line numbers if requested
		if task.ShowLineNumbers {
			content.WriteString(fmt.Sprintf("%4d: %s", lineNum, scanner.Text()))
		} else {
			content.WriteString(scanner.Text())
		}
		linesRead++
	}

	if err := scanner.Err(); err != nil {
		response.Error = fmt.Sprintf("error reading file: %v", err)
		return response
	}

	// Build the result content with context information
	var result strings.Builder

	// Add file header with context
	if skippedLines > 0 {
		result.WriteString(fmt.Sprintf("... (skipped first %d lines)\n", skippedLines))
	}

	result.WriteString(content.String())

	// Add truncation info and continuation hint
	lastLineRead := startLine + linesRead - 1
	remainingLines := totalLines - lastLineRead

	if remainingLines > 0 {
		result.WriteString(fmt.Sprintf("\n... (truncated after %d lines)", linesRead))

		// Smart continuation suggestion
		nextStart := lastLineRead + 1
		suggestedEnd := nextStart + maxLines - 1
		if suggestedEnd > totalLines {
			suggestedEnd = totalLines
		}

		result.WriteString(fmt.Sprintf("\n\n[FILE CONTINUES: %d more lines remaining (lines %d-%d)",
			remainingLines, nextStart, totalLines))
		result.WriteString(fmt.Sprintf("\nTo continue reading, use: {\"type\": \"ReadFile\", \"path\": \"%s\", \"start_line\": %d, \"end_line\": %d}]",
			task.Path, nextStart, suggestedEnd))
	}

	// Redact secrets from the actual content for LLM
	actualContent := e.redactSecrets(result.String())

	// Store actual content for LLM (will be used internally)
	response.ActualContent = actualContent

	response.Success = true

	// Enhanced status message for user
	var statusMsg string
	if task.StartLine > 0 || task.EndLine > 0 {
		statusMsg = fmt.Sprintf("Reading file: %s (lines %d-%d, %d lines read, %d total lines)",
			task.Path, startLine, lastLineRead, linesRead, totalLines)
	} else {
		statusMsg = fmt.Sprintf("Reading file: %s (%d lines read, %d total lines)",
			task.Path, linesRead, totalLines)
	}

	if remainingLines > 0 {
		statusMsg += fmt.Sprintf(", %d more lines available", remainingLines)
	}

	response.Output = statusMsg
	return response
}

// executeEditFile applies a diff or replaces content
func (e *Executor) executeEditFile(task *Task) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// Security: ensure path is within workspace
	fullPath, err := e.securePath(task.Path)
	if err != nil {
		response.Error = err.Error()
		return response
	}

	// Check if path exists and is not a directory
	if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
		response.Error = fmt.Sprintf("path is a directory: %s", task.Path)
		return response
	}

	// ULTRA-SAFE: Check for SafeEdit format first (highest priority - mandatory context validation)
	if task.SafeEditMode {
		return e.applySafeEdit(task, fullPath)
	}

	// NEW: Check for line-based editing (most precise method)
	if task.TargetLine > 0 || (task.TargetStartLine > 0 && task.TargetEndLine > 0) {
		return e.applyLineBasedEdit(task, fullPath)
	}

	if task.Diff != "" {
		return e.applyDiff(task, fullPath)
	} else if task.Content != "" {
		// CRITICAL FIX: Check if content looks like diff format instead of final content
		if e.isDiffFormattedContent(task.Content) {
			// Convert diff-formatted content to proper diff and apply it
			return e.applyDiffFormattedContent(task, fullPath)
		}

		// Check if this is a targeted edit with context
		if task.StartContext != "" || task.InsertMode != "" {
			return e.applyTargetedEdit(task, fullPath)
		}
		// Otherwise use full content replacement
		return e.replaceContent(task, fullPath)
	} else if task.Intent != "" {
		// Check if this is a replace_all operation that doesn't need additional content
		if task.InsertMode == "replace_all" && task.StartContext != "" && task.EndContext != "" {
			return e.applyTargetedEdit(task, fullPath)
		}

		// Handle natural language edit with description but no content
		response.Error = fmt.Sprintf("Edit task has intent '%s' but no actual content provided. Please provide the file content in a code block or specify the exact changes.", task.Intent)
		return response
	}

	response.Error = "EditFile requires either diff or content"
	return response
}

// applyDiff applies a unified diff to a file
func (e *Executor) applyDiff(task *Task, fullPath string) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// Read existing content if file exists
	var originalContent string
	if _, err := os.Stat(fullPath); err == nil {
		data, err := os.ReadFile(fullPath)
		if err != nil {
			response.Error = fmt.Sprintf("failed to read existing file: %v", err)
			return response
		}
		originalContent = string(data)
	}

	// Apply diff using diffmatchpatch
	dmp := diffmatchpatch.New()
	patches, err := dmp.PatchFromText(task.Diff)
	if err != nil {
		response.Error = fmt.Sprintf("invalid diff format: %v", err)
		return response
	}

	newContent, results := dmp.PatchApply(patches, originalContent)

	// Check if all patches applied successfully
	for i, result := range results {
		if !result {
			response.Error = fmt.Sprintf("failed to apply patch %d", i)
			return response
		}
	}

	// EARLY DETECTION: Check if diff resulted in identical content
	if originalContent == newContent {
		// Generate edit summary for identical content
		editSummary := e.analyzeContentChanges(originalContent, newContent, task.Path, task)
		response.EditSummary = editSummary

		// Store clear message for LLM
		llmSummary := response.GetLLMSummary()
		response.ActualContent = fmt.Sprintf("Diff analysis for %s:\n\n%s\n\nNo file changes needed - diff results in identical content.",
			task.Path, llmSummary)

		response.Success = true
		// Provide clear status message
		response.Output = fmt.Sprintf("File unchanged: %s - %s",
			task.Path, editSummary.GetCompactSummary())

		// Store the content for reference (no changes made)
		response.Task.Content = newContent
		return response
	}

	// Create a preview of the changes
	diff := dmp.DiffMain(originalContent, newContent, false)
	preview := dmp.DiffPrettyText(diff)

	// Generate edit summary
	editSummary := e.analyzeContentChanges(originalContent, newContent, task.Path, task)
	response.EditSummary = editSummary

	// Store actual diff preview for LLM with edit summary
	llmSummary := response.GetLLMSummary()
	response.ActualContent = fmt.Sprintf("Diff preview for %s:\n\n%s\n\n%s\nReady to apply changes.",
		task.Path, preview, llmSummary)

	// CRITICAL FIX: Actually write the file immediately
	task.Content = newContent
	err = e.applyEditInternal(task)
	if err != nil {
		response.Error = fmt.Sprintf("failed to write file: %v", err)
		return response
	}

	response.Success = true
	// Show status message to user with edit summary
	response.Output = fmt.Sprintf("File edited: %s (diff applied) - %s",
		task.Path, editSummary.GetCompactSummary())

	// Store the new content for reference
	response.Task.Content = newContent
	return response
}

// replaceContent replaces entire file content
func (e *Executor) replaceContent(task *Task, fullPath string) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// Read existing content for preview if file exists
	var originalContent string
	var err error
	var data []byte
	fileExists := false
	if _, err = os.Stat(fullPath); err == nil {
		data, err = os.ReadFile(fullPath)
		if err != nil {
			response.Error = fmt.Sprintf("failed to read existing file: %v", err)
			return response
		}
		originalContent = string(data)
		fileExists = true
	}

	// EARLY DETECTION: Check if content is identical to avoid unnecessary processing
	if fileExists && originalContent == task.Content {
		// Generate edit summary for identical content
		editSummary := e.analyzeContentChanges(originalContent, task.Content, task.Path, task)
		response.EditSummary = editSummary

		// Store clear message for LLM
		llmSummary := response.GetLLMSummary()
		response.ActualContent = fmt.Sprintf("Content analysis for %s:\n\n%s\n\nNo file changes needed - content already matches.",
			task.Path, llmSummary)

		response.Success = true
		// Provide clear status message
		response.Output = fmt.Sprintf("File unchanged: %s - %s",
			task.Path, editSummary.GetCompactSummary())

		// Store the content for reference (no changes made)
		response.Task.Content = task.Content
		return response
	}

	// SAFETY CHECK: Prevent accidental file truncation (only for non-identical content)
	if fileExists && len(originalContent) > 0 {
		originalLines := strings.Split(originalContent, "\n")
		newLines := strings.Split(task.Content, "\n")

		// Check for significant content reduction (more than 50% reduction in lines)
		if len(newLines) < len(originalLines)/2 && len(originalLines) > 10 {
			response.Error = fmt.Sprintf("SAFETY CHECK FAILED: Provided content (%d lines) is significantly shorter than original file (%d lines). This might truncate the file. Please read the ENTIRE file first with 🔧 READ %s (with sufficient line limits), then provide the COMPLETE updated content.",
				len(newLines), len(originalLines), task.Path)
			return response
		}

		// Check if new content appears to be incomplete for certain file types
		if e.looksIncomplete(task.Path, task.Content, originalContent) {
			response.Error = fmt.Sprintf("SAFETY CHECK FAILED: The provided content appears incomplete for %s. Please read the ENTIRE file first to understand its complete structure, then provide the COMPLETE updated content.", task.Path)
			return response
		}

		// Check if the new content is suspiciously small compared to original
		if len(task.Content) < len(originalContent)/3 && len(originalContent) > 500 {
			response.Error = fmt.Sprintf("SAFETY CHECK FAILED: New content (%d chars) is much smaller than original (%d chars). This suggests partial content that would truncate the file. Read the full file first, then provide complete updated content.",
				len(task.Content), len(originalContent))
			return response
		}
	}

	// Create diff preview
	dmp := diffmatchpatch.New()
	diff := dmp.DiffMain(originalContent, task.Content, false)
	preview := dmp.DiffPrettyText(diff)

	// Generate edit summary
	editSummary := e.analyzeContentChanges(originalContent, task.Content, task.Path, task)
	response.EditSummary = editSummary

	// Store actual preview for LLM with edit summary
	llmSummary := response.GetLLMSummary()
	response.ActualContent = fmt.Sprintf("Content replacement preview for %s:\n\n%s\n\n%s\nReady to apply changes.",
		task.Path, preview, llmSummary)

	// CRITICAL FIX: Actually write the file immediately instead of just preparing
	err = e.applyEditInternal(task)
	if err != nil {
		response.Error = fmt.Sprintf("failed to write file: %v", err)
		return response
	}

	response.Success = true
	// Show status message to user with edit summary
	response.Output = fmt.Sprintf("File edited: %s - %s",
		task.Path, editSummary.GetCompactSummary())

	// Store the new content for reference
	response.Task.Content = task.Content
	return response
}

// applySafeEdit applies the ultra-safe SafeEdit format with mandatory context validation
func (e *Executor) applySafeEdit(task *Task, fullPath string) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// Read existing content if file exists
	var originalContent string
	fileExists := false
	if _, err := os.Stat(fullPath); err == nil {
		data, err := os.ReadFile(fullPath)
		if err != nil {
			response.Error = fmt.Sprintf("failed to read existing file: %v", err)
			return response
		}
		originalContent = string(data)
		fileExists = true
	}

	if !fileExists {
		response.Error = fmt.Sprintf("SafeEdit requires existing file for context validation: %s", task.Path)
		return response
	}

	lines := strings.Split(originalContent, "\n")
	totalLines := len(lines)

	// Determine target line range
	var targetStart, targetEnd int
	if task.TargetLine > 0 {
		// Single line edit
		targetStart = task.TargetLine
		targetEnd = task.TargetLine
	} else if task.TargetStartLine > 0 && task.TargetEndLine > 0 {
		// Range edit
		targetStart = task.TargetStartLine
		targetEnd = task.TargetEndLine
	} else {
		response.Error = "SafeEdit requires exact line numbers (TargetLine or TargetStartLine/TargetEndLine)"
		return response
	}

	// Validate line numbers
	if targetStart < 1 || targetStart > totalLines {
		response.Error = fmt.Sprintf("target start line %d is out of range (file has %d lines)", targetStart, totalLines)
		return response
	}

	if targetEnd < targetStart || targetEnd > totalLines {
		response.Error = fmt.Sprintf("target end line %d is invalid (start: %d, file has %d lines)", targetEnd, targetStart, totalLines)
		return response
	}

	// MANDATORY CONTEXT VALIDATION - this is what makes SafeEdit ultra-safe
	if err := e.validateSafeEditContext(lines, task, targetStart, targetEnd); err != nil {
		response.Error = fmt.Sprintf("CONTEXT VALIDATION FAILED: %v", err)
		return response
	}

	// Apply the edit
	newContent, err := e.performSafeEdit(originalContent, task, targetStart, targetEnd)
	if err != nil {
		response.Error = err.Error()
		return response
	}

	// Create diff preview
	dmp := diffmatchpatch.New()
	diff := dmp.DiffMain(originalContent, newContent, false)
	preview := dmp.DiffPrettyText(diff)

	// Generate edit summary
	editSummary := e.analyzeContentChanges(originalContent, newContent, task.Path, task)
	response.EditSummary = editSummary

	// Store actual preview for LLM with edit summary
	llmSummary := response.GetLLMSummary()
	response.ActualContent = fmt.Sprintf("SafeEdit preview for %s (lines %d-%d):\n\n%s\n\nContext validation: PASSED ✓\n%s\nReady to apply changes.",
		task.Path, targetStart, targetEnd, preview, llmSummary)

	// CRITICAL FIX: Actually write the file immediately
	task.Content = newContent
	err = e.applyEditInternal(task)
	if err != nil {
		response.Error = fmt.Sprintf("failed to write file: %v", err)
		return response
	}

	response.Success = true
	// Show status message to user with edit summary
	if targetStart == targetEnd {
		response.Output = fmt.Sprintf("File edited: %s (SafeEdit line %d) - %s",
			task.Path, targetStart, editSummary.GetCompactSummary())
	} else {
		response.Output = fmt.Sprintf("File edited: %s (SafeEdit lines %d-%d) - %s",
			task.Path, targetStart, targetEnd, editSummary.GetCompactSummary())
	}

	// Store the new content for reference
	response.Task.Content = newContent
	return response
}

// validateSafeEditContext performs mandatory context validation for SafeEdit
func (e *Executor) validateSafeEditContext(lines []string, task *Task, targetStart, targetEnd int) error {
	totalLines := len(lines)

	// Validate BEFORE_CONTEXT
	if task.BeforeContext == "" {
		return fmt.Errorf("BeforeContext is required for SafeEdit")
	}

	beforeLines := strings.Split(task.BeforeContext, "\n")

	// Try multiple interpretations of BeforeContext to be more flexible
	var beforeStartIdx int
	var validationPassed bool

	// Declare variables here to avoid goto issues
	searchRange := 5 // Search within 5 lines of expected position
	bestMatchScore := 0
	bestMatchIdx := -1

	// Try interpretation 1: BeforeContext ends right before target (traditional)
	beforeStartIdx = targetStart - len(beforeLines) - 1 // Convert to 0-indexed
	if beforeStartIdx >= 0 {
		validationPassed = e.tryContextValidation(lines, beforeLines, beforeStartIdx)
		if validationPassed {
			goto validateAfter // Success with interpretation 1
		}
	}

	// Try interpretation 2: BeforeContext includes target line(s) - common with AI output
	beforeStartIdx = targetStart - len(beforeLines) // Convert to 0-indexed
	if beforeStartIdx >= 0 {
		validationPassed = e.tryContextValidation(lines, beforeLines, beforeStartIdx)
		if validationPassed {
			goto validateAfter // Success with interpretation 2
		}
	}

	// Try interpretation 3: BeforeContext is exactly the lines leading up to and including target
	// This handles cases where AI provides "line 35, line 36, line 37" for target line 36
	if len(beforeLines) >= 2 {
		// Try matching the context ending exactly at the target line
		beforeStartIdx = targetStart - len(beforeLines) + 1 // Convert to 0-indexed, ending at target
		if beforeStartIdx >= 0 {
			validationPassed = e.tryContextValidation(lines, beforeLines, beforeStartIdx)
			if validationPassed {
				goto validateAfter // Success with interpretation 3
			}
		}
	}

	// Try interpretation 4: Smart context matching - find the best match within a reasonable range
	// This is for cases where line numbers don't align perfectly but content matches
	for offset := -searchRange; offset <= searchRange; offset++ {
		testIdx := (targetStart - len(beforeLines)) + offset
		if testIdx >= 0 && testIdx+len(beforeLines) <= totalLines {
			score := e.calculateContextMatchScore(lines, beforeLines, testIdx)
			minRequiredScore := int(float64(len(beforeLines)) * 0.8) // At least 80% match
			if score > bestMatchScore && score >= minRequiredScore {
				bestMatchScore = score
				bestMatchIdx = testIdx
			}
		}
	}

	if bestMatchIdx >= 0 {
		validationPassed = true
		goto validateAfter // Success with smart matching
	}

	return fmt.Errorf("before context validation failed: context does not match file content around target lines")

validateAfter:
	// Validate AFTER_CONTEXT
	if task.AfterContext == "" {
		return fmt.Errorf("AfterContext is required for SafeEdit")
	}

	afterLines := strings.Split(task.AfterContext, "\n")
	afterStartIdx := targetEnd // Start right after target range (0-indexed)

	if afterStartIdx >= totalLines {
		return fmt.Errorf("not enough lines after target for context validation (need %d lines after line %d)", len(afterLines), targetEnd)
	}

	// Try flexible after context validation as well
	for offset := 0; offset <= 2; offset++ { // Try exact position and a couple of lines after
		testIdx := afterStartIdx + offset
		if testIdx < totalLines && e.tryContextValidation(lines, afterLines, testIdx) {
			return nil // Success!
		}
	}

	return fmt.Errorf("after context validation failed: context does not match file content after target lines")
}

// calculateContextMatchScore calculates how well the expected context matches the actual lines
func (e *Executor) calculateContextMatchScore(lines []string, expectedLines []string, startIdx int) int {
	if startIdx < 0 || startIdx >= len(lines) {
		return 0
	}

	score := 0
	for i, expectedLine := range expectedLines {
		actualIdx := startIdx + i
		if actualIdx >= len(lines) {
			break
		}

		actualLine := lines[actualIdx]
		cleanActual := e.cleanLineForComparison(actualLine)
		cleanExpected := e.cleanLineForComparison(expectedLine)

		if cleanActual == cleanExpected {
			score++
		}
	}

	return score
}

// tryContextValidation attempts to validate context at a specific starting position
func (e *Executor) tryContextValidation(lines []string, expectedLines []string, startIdx int) bool {
	if startIdx < 0 || startIdx >= len(lines) {
		return false
	}

	// Check each line of context
	for i, expectedLine := range expectedLines {
		actualIdx := startIdx + i
		if actualIdx >= len(lines) {
			return false
		}

		actualLine := lines[actualIdx]

		// Clean both lines for comparison
		cleanActual := e.cleanLineForComparison(actualLine)
		cleanExpected := e.cleanLineForComparison(expectedLine)

		if cleanActual != cleanExpected {
			return false
		}
	}

	return true
}

// cleanLineForComparison cleans a line for context validation comparison
func (e *Executor) cleanLineForComparison(line string) string {
	// Remove line number prefixes (e.g., "35      " or "  15: ")
	cleaned := e.stripLineNumberPrefix(line)

	// Simple approach: just trim whitespace and compare the core content
	// This is more reliable than trying to normalize indentation perfectly
	return strings.TrimSpace(cleaned)
}

// normalizeWhitespace normalizes whitespace in a line for comparison while preserving structure
// NOTE: This function is now simplified - we use simple TrimSpace instead of complex normalization
func (e *Executor) normalizeWhitespace(line string) string {
	return strings.TrimSpace(line)
}

// stripLineNumberPrefix removes line number prefixes from context lines
func (e *Executor) stripLineNumberPrefix(line string) string {
	// Pattern 1: "35      content" (number followed by spaces)
	pattern1 := regexp.MustCompile(`^\s*\d+\s+(.*)$`)
	if matches := pattern1.FindStringSubmatch(line); len(matches) > 1 {
		return matches[1]
	}

	// Pattern 2: "  15: content" (spaces, number, colon, space)
	pattern2 := regexp.MustCompile(`^\s*\d+:\s+(.*)$`)
	if matches := pattern2.FindStringSubmatch(line); len(matches) > 1 {
		return matches[1]
	}

	// Pattern 3: "15|content" (number, pipe, content)
	pattern3 := regexp.MustCompile(`^\s*\d+\|\s*(.*)$`)
	if matches := pattern3.FindStringSubmatch(line); len(matches) > 1 {
		return matches[1]
	}

	// If no line number prefix found, return original line
	return line
}

// performSafeEdit performs the actual SafeEdit with validated context
func (e *Executor) performSafeEdit(originalContent string, task *Task, targetStart, targetEnd int) (string, error) {
	lines := strings.Split(originalContent, "\n")

	// Replace the target lines with new content
	newLines := make([]string, 0, len(lines))

	// Add lines before the target range (convert 1-indexed to 0-indexed)
	beforeLines := lines[:targetStart-1]
	newLines = append(newLines, beforeLines...)

	// Add new content (split by newlines if multi-line)
	if task.Content != "" {
		contentLines := strings.Split(task.Content, "\n")
		newLines = append(newLines, contentLines...)
	}

	// Add lines after target range
	// Since targetEnd is 1-indexed and inclusive, we need lines starting from targetEnd (0-indexed)
	afterIndex := targetEnd // This gives us the lines after the target range
	if afterIndex < len(lines) {
		afterLines := lines[afterIndex:]
		newLines = append(newLines, afterLines...)
	}

	result := strings.Join(newLines, "\n")

	return result, nil
}

// ApplyEditWithConfirmation applies file changes after user confirmation
// This method should be used by Manager.ConfirmTask() to ensure proper edit summary
// feedback is sent to the LLM. For testing purposes, use ApplyEditForTesting().
func (e *Executor) ApplyEditWithConfirmation(task *Task) error {
	return e.applyEditInternal(task)
}

// ApplyEditForTesting applies file changes for testing purposes only
// This bypasses the confirmation flow and should not be used in production code.
func (e *Executor) ApplyEditForTesting(task *Task) error {
	return e.applyEditInternal(task)
}

// Deprecated: Use ApplyEditWithConfirmation() instead
// This method is kept for backward compatibility but will be removed.
func (e *Executor) ApplyEdit(task *Task) error {
	return e.applyEditInternal(task)
}

// applyEditInternal contains the actual implementation
func (e *Executor) applyEditInternal(task *Task) error {
	fullPath, err := e.securePath(task.Path)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Write the content
	if err := os.WriteFile(fullPath, []byte(task.Content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}

// applyLineBasedEdit applies precise line-based edits to a file
func (e *Executor) applyLineBasedEdit(task *Task, fullPath string) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// Read existing content if file exists
	var originalContent string
	fileExists := false
	if _, err := os.Stat(fullPath); err == nil {
		data, err := os.ReadFile(fullPath)
		if err != nil {
			response.Error = fmt.Sprintf("failed to read existing file: %v", err)
			return response
		}
		originalContent = string(data)
		fileExists = true
	}

	lines := strings.Split(originalContent, "\n")
	totalLines := len(lines)

	// Determine target line range
	var targetStart, targetEnd int
	if task.TargetLine > 0 {
		// Single line edit
		targetStart = task.TargetLine
		targetEnd = task.TargetLine
	} else {
		// Range edit
		targetStart = task.TargetStartLine
		targetEnd = task.TargetEndLine
	}

	// Validate line numbers
	if !fileExists && targetStart > 1 {
		response.Error = fmt.Sprintf("cannot edit line %d in non-existent file %s", targetStart, task.Path)
		return response
	}

	if fileExists && (targetStart < 1 || targetStart > totalLines) {
		response.Error = fmt.Sprintf("target line %d is out of range (file has %d lines)", targetStart, totalLines)
		return response
	}

	if targetEnd > 0 && fileExists && targetEnd > totalLines {
		response.Error = fmt.Sprintf("target end line %d is out of range (file has %d lines)", targetEnd, totalLines)
		return response
	}

	if targetEnd > 0 && targetEnd < targetStart {
		response.Error = fmt.Sprintf("invalid range: end line %d is before start line %d", targetEnd, targetStart)
		return response
	}

	// Optional context validation for safety
	if task.ContextValidation != "" && fileExists {
		targetLineContent := ""
		if targetStart <= len(lines) {
			targetLineContent = strings.TrimSpace(lines[targetStart-1]) // Convert to 0-indexed
		}

		if !strings.Contains(strings.ToLower(targetLineContent), strings.ToLower(task.ContextValidation)) {
			response.Error = fmt.Sprintf("context validation failed: expected line %d to contain '%s', but found: '%s'",
				targetStart, task.ContextValidation, targetLineContent)
			return response
		}
	}

	// Apply the edit based on Intent or Content
	newContent, err := e.performLineBasedEdit(originalContent, task, targetStart, targetEnd)
	if err != nil {
		response.Error = err.Error()
		return response
	}

	// Create diff preview
	dmp := diffmatchpatch.New()
	diff := dmp.DiffMain(originalContent, newContent, false)
	preview := dmp.DiffPrettyText(diff)

	// Generate edit summary
	editSummary := e.analyzeContentChanges(originalContent, newContent, task.Path, task)
	response.EditSummary = editSummary

	// Store actual preview for LLM with edit summary
	llmSummary := response.GetLLMSummary()
	response.ActualContent = fmt.Sprintf("Line-based edit preview for %s (lines %d-%d):\n\n%s\n\n%s\nReady to apply changes.",
		task.Path, targetStart, targetEnd, preview, llmSummary)

	// CRITICAL FIX: Actually write the file immediately
	task.Content = newContent
	err = e.applyEditInternal(task)
	if err != nil {
		response.Error = fmt.Sprintf("failed to write file: %v", err)
		return response
	}

	response.Success = true
	// Show status message to user with edit summary
	if targetStart == targetEnd {
		response.Output = fmt.Sprintf("File edited: %s (line %d) - %s",
			task.Path, targetStart, editSummary.GetCompactSummary())
	} else {
		response.Output = fmt.Sprintf("File edited: %s (lines %d-%d) - %s",
			task.Path, targetStart, targetEnd, editSummary.GetCompactSummary())
	}

	// Store the new content for reference
	response.Task.Content = newContent
	return response
}

// performLineBasedEdit performs the actual line-based edit logic
func (e *Executor) performLineBasedEdit(originalContent string, task *Task, targetStart, targetEnd int) (string, error) {
	lines := strings.Split(originalContent, "\n")

	// Handle new file creation
	if originalContent == "" && targetStart == 1 {
		return task.Content, nil
	}

	// Determine edit operation based on Intent
	intent := strings.ToLower(task.Intent)

	if strings.Contains(intent, "replace") {
		// Replace the target line(s) with new content
		newLines := make([]string, 0, len(lines))
		newLines = append(newLines, lines[:targetStart-1]...) // Lines before target (0-indexed)

		// Add new content (split by newlines if multi-line)
		if task.Content != "" {
			contentLines := strings.Split(task.Content, "\n")
			newLines = append(newLines, contentLines...)
		}

		// Add lines after target range
		if targetEnd < len(lines) {
			newLines = append(newLines, lines[targetEnd:]...)
		}

		return strings.Join(newLines, "\n"), nil

	} else if strings.Contains(intent, "insert") && strings.Contains(intent, "before") {
		// Insert content before the target line
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:targetStart-1]...) // Lines before target

		if task.Content != "" {
			contentLines := strings.Split(task.Content, "\n")
			newLines = append(newLines, contentLines...)
		}

		newLines = append(newLines, lines[targetStart-1:]...) // Original target line and after
		return strings.Join(newLines, "\n"), nil

	} else if strings.Contains(intent, "insert") && strings.Contains(intent, "after") {
		// Insert content after the target line
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:targetStart]...) // Lines up to and including target

		if task.Content != "" {
			contentLines := strings.Split(task.Content, "\n")
			newLines = append(newLines, contentLines...)
		}

		if targetStart < len(lines) {
			newLines = append(newLines, lines[targetStart:]...) // Lines after target
		}

		return strings.Join(newLines, "\n"), nil

	} else {
		// Default: replace the target line(s)
		newLines := make([]string, 0, len(lines))
		newLines = append(newLines, lines[:targetStart-1]...) // Lines before target

		if task.Content != "" {
			contentLines := strings.Split(task.Content, "\n")
			newLines = append(newLines, contentLines...)
		}

		// Add lines after target range
		if targetEnd < len(lines) {
			newLines = append(newLines, lines[targetEnd:]...)
		}

		return strings.Join(newLines, "\n"), nil
	}
}

// executeListDir lists files in a directory with limits and gitignore support
func (e *Executor) executeListDir(task *Task) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// Security: ensure path is within workspace
	fullPath, err := e.securePath(task.Path)
	if err != nil {
		response.Error = err.Error()
		return response
	}

	// Check if directory exists
	info, err := os.Stat(fullPath)
	if err != nil {
		response.Error = fmt.Sprintf("directory not found: %s", task.Path)
		return response
	}

	if !info.IsDir() {
		response.Error = fmt.Sprintf("path is not a directory: %s", task.Path)
		return response
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Directory listing for %s:\n\n", task.Path))

	fileCount := 0
	truncated := false

	if task.Recursive {
		err = filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			// Calculate depth relative to starting directory
			relPath, _ := filepath.Rel(fullPath, path)
			depth := strings.Count(relPath, string(filepath.Separator))
			if relPath == "." {
				depth = 0
			}

			// Check depth limit
			if depth > MaxDirectoryListingDepth {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Get relative path from workspace root for gitignore checking
			workspaceRelPath, _ := filepath.Rel(e.workspacePath, path)

			// Skip if matches gitignore patterns
			if e.shouldSkipPath(workspaceRelPath, info.IsDir()) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Check file count limit
			if fileCount >= MaxDirectoryListingFiles {
				truncated = true
				return fmt.Errorf("limit reached") // Stop walking
			}

			// Check output size limit
			if output.Len() >= MaxListingOutputSize {
				truncated = true
				return fmt.Errorf("output size limit reached")
			}

			if info.IsDir() {
				output.WriteString(fmt.Sprintf("📁 %s/\n", workspaceRelPath))
			} else {
				size := e.formatFileSize(info.Size())
				output.WriteString(fmt.Sprintf("📄 %s (%s)\n", workspaceRelPath, size))
			}
			fileCount++
			return nil
		})

		// If error is our limit check, clear it
		if err != nil && (strings.Contains(err.Error(), "limit reached") || strings.Contains(err.Error(), "output size limit")) {
			err = nil
		}
	} else {
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			response.Error = fmt.Sprintf("failed to read directory: %v", err)
			return response
		}

		for _, entry := range entries {
			// Check file count limit
			if fileCount >= MaxDirectoryListingFiles {
				truncated = true
				break
			}

			// Check output size limit
			if output.Len() >= MaxListingOutputSize {
				truncated = true
				break
			}

			// Get relative path for gitignore checking
			entryPath := filepath.Join(task.Path, entry.Name())
			if task.Path == "." {
				entryPath = entry.Name()
			}

			// Skip if matches gitignore patterns
			if e.shouldSkipPath(entryPath, entry.IsDir()) {
				continue
			}

			if entry.IsDir() {
				output.WriteString(fmt.Sprintf("📁 %s/\n", entry.Name()))
			} else {
				info, _ := entry.Info()
				size := e.formatFileSize(info.Size())
				output.WriteString(fmt.Sprintf("📄 %s (%s)\n", entry.Name(), size))
			}
			fileCount++
		}
	}

	// Add truncation notice if needed
	if truncated {
		output.WriteString(fmt.Sprintf("\n⚠️  Listing truncated at %d items (limits: %d files, %d chars, %d depth)\n",
			fileCount, MaxDirectoryListingFiles, MaxListingOutputSize, MaxDirectoryListingDepth))
	}

	// Store actual directory listing for LLM
	response.ActualContent = output.String()

	response.Success = true
	// Show status message to user
	statusMsg := ""
	if task.Recursive {
		statusMsg = fmt.Sprintf("Reading folder structure: %s (recursive, %d items", task.Path, fileCount)
	} else {
		statusMsg = fmt.Sprintf("Reading folder structure: %s (%d items", task.Path, fileCount)
	}

	if truncated {
		statusMsg += ", truncated"
	}
	statusMsg += ")"

	response.Output = statusMsg
	return response
}

// shouldSkipPath checks if a path should be skipped based on gitignore patterns and common ignore rules
func (e *Executor) shouldSkipPath(relPath string, isDir bool) bool {
	// Skip common directories that should never be listed
	skipDirs := []string{".git", "node_modules", "vendor", ".vscode", ".idea", "target", "dist", "__pycache__", ".next", ".nuxt", "build", "out"}

	if isDir {
		dirName := filepath.Base(relPath)
		for _, skip := range skipDirs {
			if dirName == skip {
				return true
			}
		}

		// Check if directory path matches gitignore
		if e.gitIgnore != nil && e.gitIgnore.MatchesPath(relPath) {
			return true
		}

		// Check directory path with trailing slash for gitignore
		if e.gitIgnore != nil && e.gitIgnore.MatchesPath(relPath+"/") {
			return true
		}
	} else {
		// For files, check gitignore patterns
		if e.gitIgnore != nil && e.gitIgnore.MatchesPath(relPath) {
			return true
		}

		// Skip common temporary and build files
		fileName := filepath.Base(relPath)
		skipFiles := []string{".DS_Store", "Thumbs.db", "*.tmp", "*.log", "*.swp", "*.swo"}
		for _, pattern := range skipFiles {
			if pattern == fileName || strings.HasSuffix(fileName, strings.TrimPrefix(pattern, "*")) {
				return true
			}
		}
	}

	return false
}

// executeRunShell runs a shell command (if enabled)
func (e *Executor) executeRunShell(task *Task) *TaskResponse {
	response := &TaskResponse{Task: *task}

	if !e.enableShell {
		response.Error = "shell execution is disabled (set enable_shell: true in config)"
		return response
	}

	// Check if this is an interactive command or has interactive flags set
	if task.Interactive || task.InputMode != "" || len(task.ExpectedPrompts) > 0 || len(task.PredefinedInput) > 0 {
		// Delegate to interactive executor
		return e.interactiveExecutor.ExecuteInteractiveCommand(task)
	}

	// For non-interactive commands, use the existing implementation
	return e.executeRegularShellCommand(task)
}

// executeRegularShellCommand handles traditional non-interactive shell commands
func (e *Executor) executeRegularShellCommand(task *Task) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(task.Timeout)*time.Second)
	defer cancel()

	// Run command in workspace directory
	cmd := exec.CommandContext(ctx, "sh", "-c", task.Command)
	cmd.Dir = e.workspacePath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Prepare output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Command: %s\n", task.Command))
	output.WriteString(fmt.Sprintf("Exit code: %d\n\n", cmd.ProcessState.ExitCode()))

	if stdout.Len() > 0 {
		output.WriteString("STDOUT:\n")
		output.WriteString(stdout.String())
		output.WriteString("\n")
	}

	if stderr.Len() > 0 {
		output.WriteString("STDERR:\n")
		output.WriteString(stderr.String())
		output.WriteString("\n")
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			response.Error = fmt.Sprintf("command timed out after %d seconds", task.Timeout)
		} else {
			response.Error = fmt.Sprintf("command failed: %v", err)
		}
	} else {
		response.Success = true
	}

	response.Output = output.String()
	return response
}

// executeSearch runs a ripgrep search with specified parameters
func (e *Executor) executeSearch(task *Task) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// Security: ensure search path is within workspace
	searchPath, err := e.securePath(task.Path)
	if err != nil {
		response.Error = err.Error()
		return response
	}

	// Build ripgrep command arguments
	args := []string{}

	// Add pattern (query)
	args = append(args, task.Query)

	// Add search path
	args = append(args, searchPath)

	// Add flags based on task options
	if task.IgnoreCase {
		args = append([]string{"-i"}, args...)
	}

	if task.WholeWord {
		args = append([]string{"-w"}, args...)
	}

	if task.FixedString {
		args = append([]string{"-F"}, args...)
	}

	if task.FilenamesOnly {
		args = append([]string{"-l"}, args...)
	}

	if task.CountMatches {
		args = append([]string{"-c"}, args...)
	}

	if task.UsePCRE2 {
		args = append([]string{"-P"}, args...)
	}

	// Add context options
	if task.ContextBefore > 0 {
		args = append([]string{fmt.Sprintf("-B%d", task.ContextBefore)}, args...)
	}

	if task.ContextAfter > 0 {
		args = append([]string{fmt.Sprintf("-A%d", task.ContextAfter)}, args...)
	}

	// Add file type filters
	for _, fileType := range task.FileTypes {
		args = append([]string{"-t", fileType}, args...)
	}

	for _, excludeType := range task.ExcludeTypes {
		args = append([]string{"-T", excludeType}, args...)
	}

	// Add glob patterns
	for _, glob := range task.GlobPatterns {
		args = append([]string{"-g", glob}, args...)
	}

	for _, excludeGlob := range task.ExcludeGlobs {
		args = append([]string{"-g", "!" + excludeGlob}, args...)
	}

	// Add search hidden files option
	if task.SearchHidden {
		args = append([]string{"--hidden"}, args...)
	}

	// Limit output to prevent overwhelming results
	if task.MaxResults > 0 {
		args = append([]string{"-m", fmt.Sprintf("%d", task.MaxResults)}, args...)
	}

	// Always add line numbers for better context
	if !task.FilenamesOnly && !task.CountMatches {
		args = append([]string{"-n"}, args...)
	}

	// Add color output for better readability
	args = append([]string{"--color=always"}, args...)

	// Execute ripgrep
	output, err := indexer.RunRipgrepWithArgs(args...)
	if err != nil {
		// Check if it's just "no matches found" (exit code 1)
		if strings.Contains(err.Error(), "exit status 1") {
			// No matches found is not an error, just empty results
			response.Success = true
			response.Output = fmt.Sprintf("Search completed: '%s' - No matches found", task.Query)
			response.ActualContent = fmt.Sprintf("No matches found for search query: '%s'\n\nSearch parameters:\n- Path: %s\n- Options: %s",
				task.Query, task.Path, e.formatSearchOptions(task))
			return response
		}

		response.Error = fmt.Sprintf("ripgrep search failed: %v", err)
		return response
	}

	// Process and format output
	outputStr := string(output)

	// Count matches and files for summary
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	if len(lines) == 1 && lines[0] == "" {
		// Empty output
		response.Success = true
		response.Output = fmt.Sprintf("Search completed: '%s' - No matches found", task.Query)
		response.ActualContent = fmt.Sprintf("No matches found for search query: '%s'\n\nSearch parameters:\n- Path: %s\n- Options: %s",
			task.Query, task.Path, e.formatSearchOptions(task))
		return response
	}

	// Count matches and files
	matchCount := 0
	fileSet := make(map[string]bool)

	for _, line := range lines {
		if line != "" {
			matchCount++
			// Extract filename (everything before first colon)
			if colonIdx := strings.Index(line, ":"); colonIdx > 0 {
				filename := line[:colonIdx]
				fileSet[filename] = true
			}
		}
	}

	fileCount := len(fileSet)

	// Format search results with intelligent truncation
	formattedOutput := e.formatSearchResults(outputStr, task, matchCount, fileCount)

	// Store actual search results for LLM
	response.ActualContent = formattedOutput

	response.Success = true

	// Create concise user status message
	if task.FilenamesOnly {
		response.Output = fmt.Sprintf("Search completed: '%s' - Found %d files with matches", task.Query, fileCount)
	} else if task.CountMatches {
		response.Output = fmt.Sprintf("Search completed: '%s' - Found %d total matches", task.Query, matchCount)
	} else {
		response.Output = fmt.Sprintf("Search completed: '%s' - Found %d matches in %d files", task.Query, matchCount, fileCount)
	}

	return response
}

// formatSearchOptions formats search options for display
func (e *Executor) formatSearchOptions(task *Task) string {
	var options []string

	if task.IgnoreCase {
		options = append(options, "case-insensitive")
	}
	if task.WholeWord {
		options = append(options, "whole words")
	}
	if task.FixedString {
		options = append(options, "literal string")
	}
	if len(task.FileTypes) > 0 {
		options = append(options, fmt.Sprintf("file types: %s", strings.Join(task.FileTypes, ",")))
	}
	if len(task.ExcludeTypes) > 0 {
		options = append(options, fmt.Sprintf("exclude types: %s", strings.Join(task.ExcludeTypes, ",")))
	}
	if len(task.GlobPatterns) > 0 {
		options = append(options, fmt.Sprintf("include: %s", strings.Join(task.GlobPatterns, ",")))
	}
	if task.ContextBefore > 0 || task.ContextAfter > 0 {
		options = append(options, fmt.Sprintf("context: %d/%d", task.ContextBefore, task.ContextAfter))
	}

	if len(options) == 0 {
		return "default"
	}

	return strings.Join(options, ", ")
}

// formatSearchResults formats ripgrep output for better readability
func (e *Executor) formatSearchResults(output string, task *Task, matchCount, fileCount int) string {
	var result strings.Builder

	// Add search summary header
	result.WriteString(fmt.Sprintf("🔍 Search Results for: '%s'\n", task.Query))
	result.WriteString(fmt.Sprintf("📁 Path: %s\n", task.Path))
	result.WriteString(fmt.Sprintf("📊 Summary: %d matches in %d files\n", matchCount, fileCount))

	if len(task.FileTypes) > 0 {
		result.WriteString(fmt.Sprintf("📋 File types: %s\n", strings.Join(task.FileTypes, ", ")))
	}

	if task.IgnoreCase || task.WholeWord || task.FixedString {
		options := []string{}
		if task.IgnoreCase {
			options = append(options, "case-insensitive")
		}
		if task.WholeWord {
			options = append(options, "whole words")
		}
		if task.FixedString {
			options = append(options, "literal")
		}
		result.WriteString(fmt.Sprintf("⚙️  Options: %s\n", strings.Join(options, ", ")))
	}

	result.WriteString("\n" + strings.Repeat("─", 50) + "\n\n")

	// Add the actual ripgrep output
	result.WriteString(output)

	// Add usage hints for large result sets
	if matchCount > 50 {
		result.WriteString("\n\n💡 Tip: Use more specific search terms or file type filters to narrow results:")
		result.WriteString("\n   🔧 SEARCH \"specific phrase\" type:go")
		result.WriteString("\n   🔧 SEARCH pattern glob:*.ts -glob:*.test.ts")
	}

	return result.String()
}

// securePath ensures the path is within the workspace and returns the full path
func (e *Executor) securePath(relPath string) (string, error) {
	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(relPath)

	// Convert to absolute path
	fullPath := filepath.Join(e.workspacePath, cleanPath)

	// Ensure the path is still within the workspace
	absWorkspace, err := filepath.Abs(e.workspacePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute workspace path: %v", err)
	}

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %v", err)
	}

	if !strings.HasPrefix(absPath, absWorkspace) {
		return "", fmt.Errorf("path outside workspace: %s", relPath)
	}

	return fullPath, nil
}

// isBinaryFile checks if a file is binary
func (e *Executor) isBinaryFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return true // Assume binary if can't read
	}
	defer file.Close()

	// Read first 512 bytes
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return true
	}

	// Check for null bytes (common in binary files)
	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			return true
		}
	}

	return false
}

// redactSecrets removes potential secrets from file content
func (e *Executor) redactSecrets(content string) string {
	// Simple regex patterns for common secrets
	patterns := []string{
		`(?i)api[_-]?key[_-]?=[\s]*["']?([a-zA-Z0-9]{20,})["']?`,
		`(?i)secret[_-]?key[_-]?=[\s]*["']?([a-zA-Z0-9]{20,})["']?`,
		`(?i)password[_-]?=[\s]*["']?([a-zA-Z0-9]{8,})["']?`,
		`(?i)token[_-]?=[\s]*["']?([a-zA-Z0-9]{20,})["']?`,
		`Bearer\s+[a-zA-Z0-9\-_\.]{20,}`,
	}

	result := content
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllStringFunc(result, func(match string) string {
			// Replace the secret part with [REDACTED]
			parts := re.FindStringSubmatch(match)
			if len(parts) > 1 {
				return strings.Replace(match, parts[1], "[REDACTED]", 1)
			}
			return "[REDACTED]"
		})
	}

	return result
}

// formatFileSize formats file size in human-readable format
func (e *Executor) formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	} else {
		return fmt.Sprintf("%.1fMB", float64(size)/1024/1024)
	}
}

// countFileLines counts the total number of lines in a file
func (e *Executor) countFileLines(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return lineCount, nil
}

// looksIncomplete checks if content appears to be incomplete for certain file types
func (e *Executor) looksIncomplete(filePath, newContent, originalContent string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".go":
		// Go files should have package declaration and proper structure
		if !strings.Contains(newContent, "package ") {
			return true
		}
		// Check for incomplete function/struct definitions
		openBraces := strings.Count(newContent, "{")
		closeBraces := strings.Count(newContent, "}")
		if openBraces != closeBraces && openBraces > 0 {
			return true
		}

	case ".json":
		// JSON should be properly closed
		newContent = strings.TrimSpace(newContent)
		if strings.HasPrefix(newContent, "{") && !strings.HasSuffix(newContent, "}") {
			return true
		}
		if strings.HasPrefix(newContent, "[") && !strings.HasSuffix(newContent, "]") {
			return true
		}

	case ".md", ".markdown":
		// Markdown files with headers should maintain structure
		originalHeaders := strings.Count(originalContent, "#")
		newHeaders := strings.Count(newContent, "#")
		// If original had many headers but new content has very few, likely incomplete
		if originalHeaders > 5 && newHeaders < originalHeaders/3 {
			return true
		}

	case ".yaml", ".yml":
		// YAML should maintain proper indentation and structure
		if strings.Contains(originalContent, ":\n") && !strings.Contains(newContent, ":\n") {
			return true
		}
	}

	// General checks
	// If original content ends with specific patterns, new content should too
	originalTrimmed := strings.TrimSpace(originalContent)
	newTrimmed := strings.TrimSpace(newContent)

	// Check for truncated content (ends abruptly without proper closing)
	if len(originalTrimmed) > 100 && len(newTrimmed) > 50 {
		// If original ends with proper structure but new doesn't
		if (strings.HasSuffix(originalTrimmed, "}") || strings.HasSuffix(originalTrimmed, ">")) &&
			!strings.HasSuffix(newTrimmed, "}") && !strings.HasSuffix(newTrimmed, ">") {
			// But allow if it's clearly intentional (new content ends with a complete line)
			if !strings.HasSuffix(newTrimmed, ".") && !strings.HasSuffix(newTrimmed, "\n") &&
				!strings.HasSuffix(newTrimmed, ";") {
				return true
			}
		}
	}

	return false
}

// applyTargetedEdit applies an edit to a specific section of a file based on context
func (e *Executor) applyTargetedEdit(task *Task, fullPath string) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// Read existing content
	var originalContent string
	if _, err := os.Stat(fullPath); err == nil {
		data, err := os.ReadFile(fullPath)
		if err != nil {
			response.Error = fmt.Sprintf("failed to read existing file: %v", err)
			return response
		}
		originalContent = string(data)
	}

	// Apply targeted edit
	newContent, err := e.performTargetedEdit(originalContent, task)
	if err != nil {
		response.Error = err.Error()
		return response
	}

	// Create diff preview
	dmp := diffmatchpatch.New()
	diff := dmp.DiffMain(originalContent, newContent, false)
	preview := dmp.DiffPrettyText(diff)

	// Generate edit summary
	editSummary := e.analyzeContentChanges(originalContent, newContent, task.Path, task)
	response.EditSummary = editSummary

	// Store actual preview for LLM with edit summary
	llmSummary := response.GetLLMSummary()
	response.ActualContent = fmt.Sprintf("Targeted edit preview for %s:\n\n%s\n\n%s\nReady to apply changes.",
		task.Path, preview, llmSummary)

	// CRITICAL FIX: Actually write the file immediately
	task.Content = newContent
	err = e.applyEditInternal(task)
	if err != nil {
		response.Error = fmt.Sprintf("failed to write file: %v", err)
		return response
	}

	response.Success = true
	// Show status message to user with edit summary
	response.Output = fmt.Sprintf("File edited: %s (%s) - %s",
		task.Path, task.InsertMode, editSummary.GetCompactSummary())

	// Store the new content for reference
	response.Task.Content = newContent
	return response
}

// performTargetedEdit performs the actual targeted editing logic
func (e *Executor) performTargetedEdit(originalContent string, task *Task) (string, error) {
	lines := strings.Split(originalContent, "\n")

	switch task.InsertMode {
	case "append":
		// Add content at the end of the file
		if originalContent == "" {
			return task.Content, nil
		}
		return originalContent + "\n" + task.Content, nil

	case "insert_before":
		if task.StartContext == "BEGINNING_OF_FILE" {
			return task.Content + "\n" + originalContent, nil
		}

		// Find the line matching StartContext
		for i, line := range lines {
			if e.matchesContext(line, task.StartContext) {
				// Insert before this line
				newLines := make([]string, 0, len(lines)+strings.Count(task.Content, "\n")+1)
				newLines = append(newLines, lines[:i]...)
				newLines = append(newLines, strings.Split(task.Content, "\n")...)
				newLines = append(newLines, lines[i:]...)
				return strings.Join(newLines, "\n"), nil
			}
		}
		return "", fmt.Errorf("could not find start context: %s", task.StartContext)

	case "insert_after":
		// Find the line matching StartContext
		for i, line := range lines {
			if e.matchesContext(line, task.StartContext) {
				// Insert after this line
				newLines := make([]string, 0, len(lines)+strings.Count(task.Content, "\n")+1)
				newLines = append(newLines, lines[:i+1]...)
				newLines = append(newLines, strings.Split(task.Content, "\n")...)
				newLines = append(newLines, lines[i+1:]...)
				return strings.Join(newLines, "\n"), nil
			}
		}
		return "", fmt.Errorf("could not find start context: %s", task.StartContext)

	case "replace":
		// Find and replace the section matching StartContext
		startIdx := -1
		endIdx := -1

		// Find start context
		for i, line := range lines {
			if e.matchesContext(line, task.StartContext) {
				startIdx = i
				break
			}
		}

		if startIdx == -1 {
			return "", fmt.Errorf("could not find start context: %s", task.StartContext)
		}

		// If EndContext is specified, find it; otherwise replace just the start line
		if task.EndContext != "" {
			for i := startIdx + 1; i < len(lines); i++ {
				if e.matchesContext(lines[i], task.EndContext) {
					endIdx = i
					break
				}
			}
			if endIdx == -1 {
				return "", fmt.Errorf("could not find end context: %s", task.EndContext)
			}
		} else {
			endIdx = startIdx
		}

		// Replace the section
		newLines := make([]string, 0, len(lines))
		newLines = append(newLines, lines[:startIdx]...)
		newLines = append(newLines, strings.Split(task.Content, "\n")...)
		newLines = append(newLines, lines[endIdx+1:]...)
		return strings.Join(newLines, "\n"), nil

	case "replace_all":
		// Global find and replace operation
		if task.StartContext == "" || task.EndContext == "" {
			return "", fmt.Errorf("replace_all requires both StartContext (find) and EndContext (replace with)")
		}

		findText := task.StartContext
		replaceText := task.EndContext

		// Perform global replacement in the entire content
		result := strings.ReplaceAll(originalContent, findText, replaceText)

		// Enhanced feedback: Detect if no replacements were made
		if result == originalContent {
			// Check if the text exists in the file at all (for better error messages)
			if strings.Contains(strings.ToLower(originalContent), strings.ToLower(findText)) {
				return "", fmt.Errorf("no exact matches for '%s' found in file - found similar text with different case. Replace operations are case-sensitive. Try using the exact case or a case-insensitive search", findText)
			} else {
				return "", fmt.Errorf("no occurrences of '%s' found in file - please verify the exact text exists and check spelling", findText)
			}
		}

		return result, nil

	case "insert_between":
		if task.StartContext == "" || task.EndContext == "" {
			return "", fmt.Errorf("insert_between requires both start and end context")
		}

		startIdx := -1
		endIdx := -1

		// Find start and end contexts
		for i, line := range lines {
			if startIdx == -1 && e.matchesContext(line, task.StartContext) {
				startIdx = i
			} else if startIdx != -1 && e.matchesContext(line, task.EndContext) {
				endIdx = i
				break
			}
		}

		if startIdx == -1 {
			return "", fmt.Errorf("could not find start context: %s", task.StartContext)
		}
		if endIdx == -1 {
			return "", fmt.Errorf("could not find end context: %s", task.EndContext)
		}

		// Insert between the contexts
		newLines := make([]string, 0, len(lines)+strings.Count(task.Content, "\n")+1)
		newLines = append(newLines, lines[:startIdx+1]...)
		newLines = append(newLines, strings.Split(task.Content, "\n")...)
		newLines = append(newLines, lines[endIdx:]...)
		return strings.Join(newLines, "\n"), nil

	default:
		return "", fmt.Errorf("unknown insert mode: %s", task.InsertMode)
	}
}

// matchesContext checks if a line matches the given context string or pattern
func (e *Executor) matchesContext(line, context string) bool {
	line = strings.TrimSpace(line)
	context = strings.TrimSpace(context)

	// Exact match
	if line == context {
		return true
	}

	// Contains match (case-insensitive)
	if strings.Contains(strings.ToLower(line), strings.ToLower(context)) {
		return true
	}

	// Try regex match if context looks like a pattern
	if strings.Contains(context, "*") || strings.Contains(context, "^") || strings.Contains(context, "$") {
		if matched, _ := regexp.MatchString(context, line); matched {
			return true
		}
	}

	return false
}

// isDiffFormattedContent detects if content looks like diff format instead of final content
func (e *Executor) isDiffFormattedContent(content string) bool {
	lines := strings.Split(content, "\n")
	diffLineCount := 0
	totalLines := len(lines)

	// Count lines that start with - or +
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "+") {
			diffLineCount++
		}
	}

	// If more than 20% of lines look like diff format, treat it as diff
	return totalLines > 0 && float64(diffLineCount)/float64(totalLines) > 0.2
}

// applyDiffFormattedContent processes content that looks like diff format
func (e *Executor) applyDiffFormattedContent(task *Task, fullPath string) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// Read existing content if file exists
	var originalContent string
	if _, err := os.Stat(fullPath); err == nil {
		data, err := os.ReadFile(fullPath)
		if err != nil {
			response.Error = fmt.Sprintf("failed to read existing file: %v", err)
			return response
		}
		originalContent = string(data)
	}

	// Parse the diff-formatted content to extract the final result
	newContent, err := e.parseDiffFormattedContent(task.Content, originalContent)
	if err != nil {
		response.Error = fmt.Sprintf("failed to parse diff-formatted content: %v", err)
		return response
	}

	// Create diff preview for display
	dmp := diffmatchpatch.New()
	diff := dmp.DiffMain(originalContent, newContent, false)
	preview := dmp.DiffPrettyText(diff)

	// Generate edit summary
	editSummary := e.analyzeContentChanges(originalContent, newContent, task.Path, task)
	response.EditSummary = editSummary

	// Store actual preview for LLM with edit summary
	llmSummary := response.GetLLMSummary()
	response.ActualContent = fmt.Sprintf("Processed diff-formatted content for %s:\n\n%s\n\n%s\nReady to apply changes.",
		task.Path, preview, llmSummary)

	// CRITICAL FIX: Actually write the file immediately
	task.Content = newContent
	err = e.applyEditInternal(task)
	if err != nil {
		response.Error = fmt.Sprintf("failed to write file: %v", err)
		return response
	}

	response.Success = true
	// Show status message to user with edit summary
	response.Output = fmt.Sprintf("File edited: %s (diff processed) - %s",
		task.Path, editSummary.GetCompactSummary())

	// Store the new content for reference
	response.Task.Content = newContent
	return response
}

// parseDiffFormattedContent converts diff-formatted content to final content
func (e *Executor) parseDiffFormattedContent(diffContent string, originalContent string) (string, error) {
	lines := strings.Split(diffContent, "\n")

	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "+") {
			// This is a line to add - remove the + prefix and preserve spacing
			newLine := strings.TrimPrefix(trimmed, "+")
			if len(newLine) > 0 && newLine[0] == ' ' {
				newLine = newLine[1:] // Remove the single space after +
			}
			result = append(result, newLine)
		} else if strings.HasPrefix(trimmed, "-") {
			// This is a line to remove - skip it
			continue
		} else {
			// This is an unchanged line (context) or blank line - preserve as-is
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n"), nil
}

// analyzeContentChanges generates an EditSummary by analyzing differences between old and new content
func (e *Executor) analyzeContentChanges(originalContent, newContent, filePath string, task *Task) *EditSummary {
	summary := &EditSummary{
		FilePath:      filePath,
		WasSuccessful: true,
	}

	// Check if content is identical (this is the key fix for the feedback loop issue)
	if originalContent == newContent {
		summary.IsIdenticalContent = true
		summary.EditType = "modify" // Even though it's identical, we treat it as a modify for consistency
		summary.TotalLines = len(strings.Split(newContent, "\n"))
		if newContent == "" {
			summary.TotalLines = 0
		}

		// Generate a clear summary for identical content
		if task.Intent != "" {
			summary.Summary = fmt.Sprintf("%s - File already contains the desired content", task.Intent)
		} else {
			summary.Summary = "File already contains the desired content - no changes needed"
		}

		return summary
	}

	// Determine edit type
	originalExists := originalContent != ""
	newExists := newContent != ""

	if !originalExists && newExists {
		summary.EditType = "create"
	} else if originalExists && !newExists {
		summary.EditType = "delete"
	} else {
		summary.EditType = "modify"
	}

	// Calculate line-based changes
	originalLines := strings.Split(originalContent, "\n")
	newLines := strings.Split(newContent, "\n")

	if !originalExists {
		originalLines = []string{}
	}
	if !newExists {
		newLines = []string{}
	}

	// Calculate total lines after edit
	summary.TotalLines = len(newLines)

	// Calculate character changes
	summary.CharactersAdded = len(newContent) - len(originalContent)
	if summary.CharactersAdded < 0 {
		summary.CharactersRemoved = -summary.CharactersAdded
		summary.CharactersAdded = 0
	}

	// For detailed line analysis, use diffmatchpatch to get precise changes
	if originalExists && newExists {
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(originalContent, newContent, false)

		// Analyze diffs to count line changes
		linesAdded, linesRemoved, linesModified := e.analyzeDiffs(diffs)
		summary.LinesAdded = linesAdded
		summary.LinesRemoved = linesRemoved
		summary.LinesModified = linesModified
	} else if summary.EditType == "create" {
		summary.LinesAdded = len(newLines)
		summary.CharactersAdded = len(newContent)
	} else if summary.EditType == "delete" {
		summary.LinesRemoved = len(originalLines)
		summary.CharactersRemoved = len(originalContent)
	}

	// Generate a descriptive summary based on the task and changes
	summary.Summary = e.generateChangeSummary(task, summary)

	return summary
}

// analyzeDiffs analyzes diffmatchpatch diffs to count line-level changes using
// sophisticated pattern matching to distinguish modifications from additions/deletions
func (e *Executor) analyzeDiffs(diffs []diffmatchpatch.Diff) (linesAdded, linesRemoved, linesModified int) {
	// Convert character-level diffs to line-level analysis
	return e.analyzeContentChangesLineBased(diffs)
}

// analyzeContentChangesLineBased performs line-based diff analysis
func (e *Executor) analyzeContentChangesLineBased(diffs []diffmatchpatch.Diff) (linesAdded, linesRemoved, linesModified int) {
	// Reconstruct original and new content from diffs
	var originalContent, newContent string

	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffEqual:
			originalContent += diff.Text
			newContent += diff.Text
		case diffmatchpatch.DiffDelete:
			originalContent += diff.Text
		case diffmatchpatch.DiffInsert:
			newContent += diff.Text
		}
	}

	// Split into lines for line-based analysis
	originalLines := strings.Split(originalContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Remove empty trailing lines
	originalLines = e.removeTrailingEmpty(originalLines)
	newLines = e.removeTrailingEmpty(newLines)

	// Use line-based diff matching
	return e.performLineDiffAnalysis(originalLines, newLines)
}

// removeTrailingEmpty removes trailing empty strings from a slice
func (e *Executor) removeTrailingEmpty(lines []string) []string {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// performLineDiffAnalysis performs sophisticated line-by-line diff analysis
func (e *Executor) performLineDiffAnalysis(originalLines, newLines []string) (linesAdded, linesRemoved, linesModified int) {
	// Create line-level diffs using a custom LCS approach
	lineDiffs := e.createLineLevelDiffs(originalLines, newLines)

	// Analyze the line diffs to detect modifications
	i := 0
	for i < len(lineDiffs) {
		diff := lineDiffs[i]

		switch diff.Type {
		case diffmatchpatch.DiffEqual:
			// Skip equal lines
			i++

		case diffmatchpatch.DiffDelete:
			// Look ahead for potential modification (delete followed by insert)
			if i+1 < len(lineDiffs) && lineDiffs[i+1].Type == diffmatchpatch.DiffInsert {
				// This is a modification pattern: delete + insert
				deletedLines := diff.Lines
				insertedLines := lineDiffs[i+1].Lines

				// Calculate modifications using content similarity analysis
				modifications, pureDeletes, pureInserts := e.analyzeModificationPattern(
					deletedLines, insertedLines)

				linesModified += modifications
				linesRemoved += pureDeletes
				linesAdded += pureInserts

				// Skip the insert operation since we processed it
				i += 2
			} else {
				// Standalone deletion
				linesRemoved += len(diff.Lines)
				i++
			}

		case diffmatchpatch.DiffInsert:
			// Standalone insertion (not part of a delete+insert modification)
			linesAdded += len(diff.Lines)
			i++
		}
	}

	return linesAdded, linesRemoved, linesModified
}

// LineDiff represents a line-level diff operation
type LineDiff struct {
	Type  diffmatchpatch.Operation
	Lines []string
}

// createLineLevelDiffs creates line-level diffs using LCS algorithm
func (e *Executor) createLineLevelDiffs(originalLines, newLines []string) []LineDiff {
	// Use Myers algorithm / LCS to find line-level differences
	lcs := e.longestCommonSubsequence(originalLines, newLines)

	var diffs []LineDiff

	i, j := 0, 0
	for k := 0; k < len(lcs); k++ {
		// Handle deletions before this LCS point
		var deletedLines []string
		for i < len(originalLines) && (k >= len(lcs) || originalLines[i] != lcs[k]) {
			deletedLines = append(deletedLines, originalLines[i])
			i++
		}
		if len(deletedLines) > 0 {
			diffs = append(diffs, LineDiff{Type: diffmatchpatch.DiffDelete, Lines: deletedLines})
		}

		// Handle insertions before this LCS point
		var insertedLines []string
		for j < len(newLines) && (k >= len(lcs) || newLines[j] != lcs[k]) {
			insertedLines = append(insertedLines, newLines[j])
			j++
		}
		if len(insertedLines) > 0 {
			diffs = append(diffs, LineDiff{Type: diffmatchpatch.DiffInsert, Lines: insertedLines})
		}

		// Handle equal line
		if k < len(lcs) {
			diffs = append(diffs, LineDiff{Type: diffmatchpatch.DiffEqual, Lines: []string{lcs[k]}})
			i++
			j++
		}
	}

	// Handle remaining deletions
	var remainingDeleted []string
	for i < len(originalLines) {
		remainingDeleted = append(remainingDeleted, originalLines[i])
		i++
	}
	if len(remainingDeleted) > 0 {
		diffs = append(diffs, LineDiff{Type: diffmatchpatch.DiffDelete, Lines: remainingDeleted})
	}

	// Handle remaining insertions
	var remainingInserted []string
	for j < len(newLines) {
		remainingInserted = append(remainingInserted, newLines[j])
		j++
	}
	if len(remainingInserted) > 0 {
		diffs = append(diffs, LineDiff{Type: diffmatchpatch.DiffInsert, Lines: remainingInserted})
	}

	return diffs
}

// longestCommonSubsequence finds the LCS of two string slices
func (e *Executor) longestCommonSubsequence(a, b []string) []string {
	m, n := len(a), len(b)

	// Create LCS table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	// Fill LCS table
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = maxInt(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	// Reconstruct LCS
	var lcs []string
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			lcs = append([]string{a[i-1]}, lcs...)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	return lcs
}

// maxInt returns the maximum of two integers
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// DiffOperation represents a preprocessed diff operation with line information
type DiffOperation struct {
	Type      diffmatchpatch.Operation
	Lines     []string
	LineCount int
}

// preprocessDiffOperations converts raw diffs into line-aware operations
func (e *Executor) preprocessDiffOperations(diffs []diffmatchpatch.Diff) []DiffOperation {
	var operations []DiffOperation

	for _, diff := range diffs {
		lines := strings.Split(diff.Text, "\n")

		// Handle trailing newlines consistently
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		// Skip empty operations
		if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
			continue
		}

		operations = append(operations, DiffOperation{
			Type:      diff.Type,
			Lines:     lines,
			LineCount: len(lines),
		})
	}

	return operations
}

// analyzeModificationPattern analyzes a delete+insert pair to distinguish
// between true modifications and independent deletions/insertions
func (e *Executor) analyzeModificationPattern(deletedLines, insertedLines []string) (modifications, pureDeletes, pureInserts int) {
	deletedCount := len(deletedLines)
	insertedCount := len(insertedLines)

	// Use content similarity to determine actual modifications
	modifications = e.countSimilarLines(deletedLines, insertedLines)

	// Calculate remaining pure operations
	pureDeletes = deletedCount - modifications
	pureInserts = insertedCount - modifications

	// Ensure non-negative values
	if pureDeletes < 0 {
		pureDeletes = 0
	}
	if pureInserts < 0 {
		pureInserts = 0
	}

	return modifications, pureDeletes, pureInserts
}

// countSimilarLines counts how many deleted lines have similar counterparts
// in the inserted lines, indicating modifications rather than complete rewrites
func (e *Executor) countSimilarLines(deletedLines, insertedLines []string) int {
	modifications := 0
	usedInserted := make(map[int]bool)

	// For each deleted line, find the most similar inserted line
	for _, deletedLine := range deletedLines {
		bestMatch := -1
		bestSimilarity := 0.0

		for i, insertedLine := range insertedLines {
			if usedInserted[i] {
				continue // Already matched
			}

			similarity := e.calculateLineSimilarity(deletedLine, insertedLine)

			// Consider it a modification if similarity is above threshold
			// This threshold can be tuned based on requirements
			if similarity > 0.3 && similarity > bestSimilarity {
				bestSimilarity = similarity
				bestMatch = i
			}
		}

		if bestMatch != -1 {
			modifications++
			usedInserted[bestMatch] = true
		}
	}

	return modifications
}

// calculateLineSimilarity calculates similarity between two lines using
// a combination of character overlap and structural similarity
func (e *Executor) calculateLineSimilarity(line1, line2 string) float64 {
	// Normalize lines for comparison
	norm1 := strings.TrimSpace(line1)
	norm2 := strings.TrimSpace(line2)

	if norm1 == norm2 {
		return 1.0 // Identical
	}

	if norm1 == "" || norm2 == "" {
		return 0.0 // One is empty
	}

	// Use Levenshtein distance to calculate similarity
	distance := e.levenshteinDistance(norm1, norm2)
	maxLen := len(norm1)
	if len(norm2) > maxLen {
		maxLen = len(norm2)
	}

	if maxLen == 0 {
		return 1.0
	}

	similarity := 1.0 - float64(distance)/float64(maxLen)

	// Boost similarity for lines that share common structure (indentation, brackets, etc.)
	structuralBonus := e.calculateStructuralSimilarity(line1, line2)
	similarity += structuralBonus * 0.2 // 20% weight for structural similarity

	if similarity > 1.0 {
		similarity = 1.0
	}

	return similarity
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func (e *Executor) levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}

	// Initialize first row and column
	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}

			matrix[i][j] = minOfThree(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

// calculateStructuralSimilarity calculates similarity based on code structure
func (e *Executor) calculateStructuralSimilarity(line1, line2 string) float64 {
	score := 0.0

	// Check indentation similarity
	indent1 := len(line1) - len(strings.TrimLeft(line1, " \t"))
	indent2 := len(line2) - len(strings.TrimLeft(line2, " \t"))

	if indent1 == indent2 {
		score += 0.3 // Same indentation level
	} else if abs(indent1-indent2) <= 2 {
		score += 0.1 // Similar indentation level
	}

	// Check for common patterns
	patterns := []string{"{", "}", "(", ")", "[", "]", "=", ";", ":", ","}
	for _, pattern := range patterns {
		if strings.Contains(line1, pattern) && strings.Contains(line2, pattern) {
			score += 0.1
		}
	}

	// Limit total structural bonus
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// Helper function for minimum of three integers
func minOfThree(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// Helper function for absolute value
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// generateChangeSummary creates a human-readable summary of what changed
func (e *Executor) generateChangeSummary(task *Task, summary *EditSummary) string {
	switch summary.EditType {
	case "create":
		if task.Intent != "" {
			return fmt.Sprintf("Created new file: %s", task.Intent)
		}
		return "Created new file"

	case "delete":
		return "Deleted file"

	case "modify":
		if task.Intent != "" {
			return task.Intent
		}

		// Generate summary based on change patterns
		totalChanges := summary.LinesAdded + summary.LinesRemoved + summary.LinesModified
		if totalChanges == 0 {
			return "No significant changes"
		}

		var changes []string
		if summary.LinesAdded > 0 {
			changes = append(changes, fmt.Sprintf("added %d lines", summary.LinesAdded))
		}
		if summary.LinesRemoved > 0 {
			changes = append(changes, fmt.Sprintf("removed %d lines", summary.LinesRemoved))
		}
		if summary.LinesModified > 0 {
			changes = append(changes, fmt.Sprintf("modified %d lines", summary.LinesModified))
		}

		if len(changes) > 0 {
			return "Code changes: " + strings.Join(changes, ", ")
		}

		return "Content modified"

	default:
		return "File edited"
	}
}

// executeMemory handles memory operations
func (e *Executor) executeMemory(task *Task) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// Create memory request from task
	req := &memory.MemoryRequest{
		Operation:   memory.MemoryOperation(task.MemoryOperation),
		ID:          task.MemoryID,
		Content:     task.MemoryContent,
		Tags:        task.MemoryTags,
		Active:      task.MemoryActive,
		Description: task.MemoryDescription,
	}

	// Process the memory request
	memResponse := e.memoryStore.ProcessRequest(req)

	// Convert memory response to task response
	response.Success = memResponse.Success
	if !memResponse.Success {
		response.Error = memResponse.Error
		return response
	}

	// Format output based on operation
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Memory Operation: %s\n", task.MemoryOperation))

	if memResponse.Message != "" {
		output.WriteString(fmt.Sprintf("Result: %s\n", memResponse.Message))
	}

	switch strings.ToLower(task.MemoryOperation) {
	case "create", "update", "get":
		if memResponse.Memory != nil {
			output.WriteString(fmt.Sprintf("\nMemory Details:\n"))
			output.WriteString(fmt.Sprintf("  ID: %s\n", memResponse.Memory.ID))
			output.WriteString(fmt.Sprintf("  Content: %s\n", memResponse.Memory.Content))
			if memResponse.Memory.Description != "" {
				output.WriteString(fmt.Sprintf("  Description: %s\n", memResponse.Memory.Description))
			}
			if len(memResponse.Memory.Tags) > 0 {
				output.WriteString(fmt.Sprintf("  Tags: %s\n", strings.Join(memResponse.Memory.Tags, ", ")))
			}
			output.WriteString(fmt.Sprintf("  Active: %t\n", memResponse.Memory.Active))
			output.WriteString(fmt.Sprintf("  Created: %s\n", memResponse.Memory.CreatedAt.Format("2006-01-02 15:04:05")))
			output.WriteString(fmt.Sprintf("  Updated: %s\n", memResponse.Memory.UpdatedAt.Format("2006-01-02 15:04:05")))
		}

	case "list":
		if len(memResponse.Memories) > 0 {
			output.WriteString(fmt.Sprintf("\nFound %d memories:\n", len(memResponse.Memories)))
			for i, mem := range memResponse.Memories {
				output.WriteString(fmt.Sprintf("\n%d. %s\n", i+1, mem.ID))
				output.WriteString(fmt.Sprintf("   Content: %s\n", mem.Content))
				if mem.Description != "" {
					output.WriteString(fmt.Sprintf("   Description: %s\n", mem.Description))
				}
				if len(mem.Tags) > 0 {
					output.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(mem.Tags, ", ")))
				}
				output.WriteString(fmt.Sprintf("   Active: %t\n", mem.Active))
				output.WriteString(fmt.Sprintf("   Created: %s\n", mem.CreatedAt.Format("2006-01-02 15:04:05")))
			}
		} else {
			output.WriteString("\nNo memories found.\n")
		}

	case "delete":
		if memResponse.Memory != nil {
			output.WriteString(fmt.Sprintf("\nDeleted memory: %s\n", memResponse.Memory.ID))
		}
	}

	// Add memory count summary
	total, active := e.memoryStore.GetMemoryCount()
	output.WriteString(fmt.Sprintf("\nMemory Store Summary: %d total memories, %d active\n", total, active))

	response.Output = output.String()
	return response
}

// GetMemoryStore returns the memory store for external access
func (e *Executor) GetMemoryStore() *memory.MemoryStore {
	return e.memoryStore
}

// ProcessLoomEditMessage processes a message that may contain LOOM_EDIT blocks
func (e *Executor) ProcessLoomEditMessage(message string) (*LoomEditResponse, error) {
	result, err := e.loomEditProcessor.ProcessMessage(message)
	if err != nil {
		return nil, err
	}

	response := &LoomEditResponse{
		LoomEditResult: *result,
		Success:        true,
	}

	if result.BlocksFound > 0 {
		response.Message = fmt.Sprintf("Applied %d LOOM_EDIT blocks, edited %d files: %s", 
			result.BlocksFound, len(result.FilesEdited), strings.Join(result.FilesEdited, ", "))
	} else {
		response.Message = "No LOOM_EDIT blocks found in message"
	}

	return response, nil
}

// LoomEditResponse contains the result of processing LOOM_EDIT blocks
type LoomEditResponse struct {
	LoomEditResult
	Success bool
	Message string
	Error   string
}

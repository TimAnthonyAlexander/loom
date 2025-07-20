package task

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"loom/indexer"
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
	workspacePath string
	enableShell   bool
	maxFileSize   int64
	gitIgnore     *indexer.GitIgnore
}

// NewExecutor creates a new task executor
func NewExecutor(workspacePath string, enableShell bool, maxFileSize int64) *Executor {
	// Load gitignore patterns
	gitIgnore, err := indexer.LoadGitIgnore(workspacePath)
	if err != nil {
		// Continue without .gitignore if it fails to load
		gitIgnore = &indexer.GitIgnore{}
	}

	return &Executor{
		workspacePath: workspacePath,
		enableShell:   enableShell,
		maxFileSize:   maxFileSize,
		gitIgnore:     gitIgnore,
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
		content.WriteString(scanner.Text())
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

	if task.Diff != "" {
		return e.applyDiff(task, fullPath)
	} else if task.Content != "" {
		return e.replaceContent(task, fullPath)
	} else if task.Intent != "" {
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

	// Create a preview of the changes
	diff := dmp.DiffMain(originalContent, newContent, false)
	preview := dmp.DiffPrettyText(diff)

	// Store actual diff preview for LLM
	response.ActualContent = fmt.Sprintf("Diff preview for %s:\n\n%s\n\nReady to apply changes.", task.Path, preview)

	response.Success = true
	// Show only status message to user, not the actual diff
	response.Output = fmt.Sprintf("Editing file: %s (diff preview prepared)", task.Path)

	// Store the new content for later application
	response.Task.Content = newContent
	return response
}

// replaceContent replaces entire file content
func (e *Executor) replaceContent(task *Task, fullPath string) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// Read existing content for preview if file exists
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

	// SAFETY CHECK: Prevent accidental file truncation
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

	// Store actual preview for LLM
	response.ActualContent = fmt.Sprintf("Content replacement preview for %s:\n\n%s\n\nReady to apply changes.", task.Path, preview)

	response.Success = true
	// Show only status message to user, not the actual diff
	response.Output = fmt.Sprintf("Editing file: %s (content replacement prepared)", task.Path)

	return response
}

// ApplyEdit actually writes the file changes (called after user confirmation)
func (e *Executor) ApplyEdit(task *Task) error {
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
	skipDirs := []string{".git", "node_modules", "vendor", ".loom", ".vscode", ".idea", "target", "dist", "__pycache__", ".next", ".nuxt", "build", "out"}

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

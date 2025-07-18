package task

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// Executor handles task execution with security constraints
type Executor struct {
	workspacePath string
	enableShell   bool
	maxFileSize   int64
}

// NewExecutor creates a new task executor
func NewExecutor(workspacePath string, enableShell bool, maxFileSize int64) *Executor {
	return &Executor{
		workspacePath: workspacePath,
		enableShell:   enableShell,
		maxFileSize:   maxFileSize,
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

	// Set default max lines if not specified
	maxLines := task.MaxLines
	if maxLines <= 0 {
		maxLines = 200 // Default limit
	}

	for scanner.Scan() {
		lineNum++

		// Check line range if specified
		if task.StartLine > 0 && lineNum < task.StartLine {
			continue
		}
		if task.EndLine > 0 && lineNum > task.EndLine {
			break
		}

		// Check max lines limit
		if linesRead >= maxLines {
			content.WriteString(fmt.Sprintf("\n... (truncated after %d lines)\n", maxLines))
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

	// Redact secrets
	output := e.redactSecrets(content.String())

	response.Success = true
	response.Output = fmt.Sprintf("File: %s (%d lines read)\n\n%s", task.Path, linesRead, output)
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

	response.Success = true
	response.Output = fmt.Sprintf("Diff preview for %s:\n\n%s\n\nReady to apply changes.",
		task.Path, preview)

	// Store the new content for later application
	response.Task.Content = newContent
	return response
}

// replaceContent replaces entire file content
func (e *Executor) replaceContent(task *Task, fullPath string) *TaskResponse {
	response := &TaskResponse{Task: *task}

	// Read existing content for preview if file exists
	var originalContent string
	if _, err := os.Stat(fullPath); err == nil {
		data, err := os.ReadFile(fullPath)
		if err != nil {
			response.Error = fmt.Sprintf("failed to read existing file: %v", err)
			return response
		}
		originalContent = string(data)
	}

	// Create diff preview
	dmp := diffmatchpatch.New()
	diff := dmp.DiffMain(originalContent, task.Content, false)
	preview := dmp.DiffPrettyText(diff)

	response.Success = true
	response.Output = fmt.Sprintf("Content replacement preview for %s:\n\n%s\n\nReady to apply changes.",
		task.Path, preview)

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

// executeListDir lists files in a directory
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

	if task.Recursive {
		err = filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			relPath, _ := filepath.Rel(e.workspacePath, path)
			if info.IsDir() {
				output.WriteString(fmt.Sprintf("📁 %s/\n", relPath))
			} else {
				size := e.formatFileSize(info.Size())
				output.WriteString(fmt.Sprintf("📄 %s (%s)\n", relPath, size))
			}
			return nil
		})
	} else {
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			response.Error = fmt.Sprintf("failed to read directory: %v", err)
			return response
		}

		for _, entry := range entries {
			if entry.IsDir() {
				output.WriteString(fmt.Sprintf("📁 %s/\n", entry.Name()))
			} else {
				info, _ := entry.Info()
				size := e.formatFileSize(info.Size())
				output.WriteString(fmt.Sprintf("📄 %s (%s)\n", entry.Name(), size))
			}
		}
	}

	response.Success = true
	response.Output = output.String()
	return response
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

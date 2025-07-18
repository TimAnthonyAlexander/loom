package task

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Global debug flag for task parsing - can be enabled with environment variable
var debugTaskParsing = os.Getenv("LOOM_DEBUG_TASKS") == "1"

// TaskType represents the type of task to execute
type TaskType string

const (
	TaskTypeReadFile TaskType = "ReadFile"
	TaskTypeEditFile TaskType = "EditFile"
	TaskTypeListDir  TaskType = "ListDir"
	TaskTypeRunShell TaskType = "RunShell"
)

// Task represents a single task to be executed
type Task struct {
	Type TaskType `json:"type"`

	// Common fields
	Path string `json:"path,omitempty"`

	// ReadFile specific
	MaxLines  int `json:"max_lines,omitempty"`
	StartLine int `json:"start_line,omitempty"`
	EndLine   int `json:"end_line,omitempty"`

	// EditFile specific
	Diff    string `json:"diff,omitempty"`
	Content string `json:"content,omitempty"`

	// RunShell specific
	Command string `json:"command,omitempty"`
	Timeout int    `json:"timeout,omitempty"` // seconds

	// ListDir specific
	Recursive bool `json:"recursive,omitempty"`
}

// TaskList represents a list of tasks from the LLM
type TaskList struct {
	Tasks []Task `json:"tasks"`
}

// TaskResponse represents the result of executing a task
type TaskResponse struct {
	Task          Task   `json:"task"`
	Success       bool   `json:"success"`
	Output        string `json:"output,omitempty"`         // Display message for user
	ActualContent string `json:"actual_content,omitempty"` // Actual content for LLM (hidden from user)
	Error         string `json:"error,omitempty"`
	Approved      bool   `json:"approved,omitempty"` // For tasks requiring confirmation
}

// ParseTasks extracts and parses task JSON blocks from LLM response
func ParseTasks(llmResponse string) (*TaskList, error) {
	// Look for JSON code blocks
	re := regexp.MustCompile("(?s)```(?:json)?\n?(.*?)\n?```")
	matches := re.FindAllStringSubmatch(llmResponse, -1)

	if len(matches) == 0 {
		// Debug: Check if the response mentions tasks or actions that should trigger task execution
		if debugTaskParsing {
			lowerResponse := strings.ToLower(llmResponse)
			if strings.Contains(lowerResponse, "create") || strings.Contains(lowerResponse, "edit") || 
			   strings.Contains(lowerResponse, "file") || strings.Contains(lowerResponse, "license") ||
			   strings.Contains(lowerResponse, "i'll") || strings.Contains(lowerResponse, "i will") {
				// This looks like it should have had tasks, but no JSON blocks found
				fmt.Printf("DEBUG: LLM response suggests action but no JSON tasks found. Response contains action words but no code blocks.\n")
			}
		}
		// No JSON blocks found - this is normal for regular chat responses
		return nil, nil
	}

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		jsonStr := strings.TrimSpace(match[1])
		if jsonStr == "" {
			continue
		}

		// Debug: Print what we're trying to parse
		if debugTaskParsing {
			fmt.Printf("DEBUG: Attempting to parse JSON task block: %s\n", jsonStr[:min(len(jsonStr), 100)])
		}

		// Try to parse as TaskList
		var taskList TaskList
		if err := json.Unmarshal([]byte(jsonStr), &taskList); err != nil {
			if debugTaskParsing {
				fmt.Printf("DEBUG: Failed to parse JSON task block: %v\n", err)
			}
			continue // Skip invalid JSON blocks
		}

		// Validate tasks
		if err := validateTasks(&taskList); err != nil {
			return nil, fmt.Errorf("invalid tasks: %w", err)
		}

		if debugTaskParsing {
			fmt.Printf("DEBUG: Successfully parsed %d tasks\n", len(taskList.Tasks))
		}
		return &taskList, nil
	}

	return nil, nil
}

// Helper function for debug output
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// validateTasks performs basic validation on tasks
func validateTasks(taskList *TaskList) error {
	if len(taskList.Tasks) == 0 {
		return fmt.Errorf("no tasks found")
	}

	for i, task := range taskList.Tasks {
		if err := validateTask(&task); err != nil {
			return fmt.Errorf("task %d: %w", i, err)
		}
	}

	return nil
}

// validateTask validates a single task
func validateTask(task *Task) error {
	switch task.Type {
	case TaskTypeReadFile:
		if task.Path == "" {
			return fmt.Errorf("ReadFile requires path")
		}
		if task.MaxLines < 0 {
			return fmt.Errorf("MaxLines must be non-negative")
		}
		if task.StartLine < 0 || task.EndLine < 0 {
			return fmt.Errorf("StartLine and EndLine must be non-negative")
		}
		if task.StartLine > 0 && task.EndLine > 0 && task.StartLine > task.EndLine {
			return fmt.Errorf("StartLine must be <= EndLine")
		}

	case TaskTypeEditFile:
		if task.Path == "" {
			return fmt.Errorf("EditFile requires path")
		}
		if task.Diff == "" && task.Content == "" {
			return fmt.Errorf("EditFile requires either diff or content")
		}

	case TaskTypeListDir:
		if task.Path == "" {
			task.Path = "." // Default to current directory
		}

	case TaskTypeRunShell:
		if task.Command == "" {
			return fmt.Errorf("RunShell requires command")
		}
		if task.Timeout <= 0 {
			task.Timeout = 3 // Default 3 second timeout
		}

	default:
		return fmt.Errorf("unknown task type: %s", task.Type)
	}

	return nil
}

// IsDestructive returns true if the task modifies files or executes shell commands
func (t *Task) IsDestructive() bool {
	return t.Type == TaskTypeEditFile || t.Type == TaskTypeRunShell
}

// RequiresConfirmation returns true if the task requires user confirmation
func (t *Task) RequiresConfirmation() bool {
	return t.IsDestructive()
}

// Description returns a human-readable description of the task
func (t *Task) Description() string {
	switch t.Type {
	case TaskTypeReadFile:
		if t.StartLine > 0 && t.EndLine > 0 {
			return fmt.Sprintf("Read %s (lines %d-%d)", t.Path, t.StartLine, t.EndLine)
		} else if t.MaxLines > 0 {
			return fmt.Sprintf("Read %s (max %d lines)", t.Path, t.MaxLines)
		}
		return fmt.Sprintf("Read %s", t.Path)

	case TaskTypeEditFile:
		if t.Diff != "" {
			return fmt.Sprintf("Edit %s (apply diff)", t.Path)
		}
		return fmt.Sprintf("Edit %s (replace content)", t.Path)

	case TaskTypeListDir:
		if t.Recursive {
			return fmt.Sprintf("List directory %s (recursive)", t.Path)
		}
		return fmt.Sprintf("List directory %s", t.Path)

	case TaskTypeRunShell:
		return fmt.Sprintf("Run command: %s", t.Command)

	default:
		return fmt.Sprintf("Unknown task: %s", t.Type)
	}
}

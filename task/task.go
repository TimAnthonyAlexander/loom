package task

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv" // Used for parsing integers in natural language task commands
	"strings"
)

// Default values for task parameters
const (
	DefaultMaxLines = 200 // Default maximum lines to read from a file
	DefaultTimeout  = 30  // Default timeout in seconds for shell commands
)

// Global debug flag for task parsing - can be enabled with environment variable
var debugTaskParsing = os.Getenv("LOOM_DEBUG_TASKS") == "1"

// EnableTaskDebug enables debug output for task parsing (for troubleshooting)
func EnableTaskDebug() {
	debugTaskParsing = true
}

// DisableTaskDebug disables debug output for task parsing
func DisableTaskDebug() {
	debugTaskParsing = false
}

// IsTaskDebugEnabled returns whether task debug mode is enabled
func IsTaskDebugEnabled() bool {
	return debugTaskParsing
}

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
	Intent  string `json:"intent,omitempty"` // Natural language description of what to do

	// Targeted editing fields
	StartContext string `json:"start_context,omitempty"` // Line or pattern marking start of edit section
	EndContext   string `json:"end_context,omitempty"`   // Line or pattern marking end of edit section
	InsertMode   string `json:"insert_mode,omitempty"`   // "replace", "insert_before", "insert_after", "append"

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

// tryNaturalLanguageParsing attempts to parse natural language task commands
func tryNaturalLanguageParsing(llmResponse string) *TaskList {
	if debugTaskParsing {
		fmt.Printf("DEBUG: Attempting natural language task parsing...\n")
	}

	lines := strings.Split(llmResponse, "\n")
	var tasks []Task
	// Use map for O(1) duplicate detection instead of O(n) linear search
	seenTasks := make(map[string]bool)

	// Look for task indicators with emoji prefixes
	taskPattern := regexp.MustCompile(`^🔧\s+(READ|EDIT|LIST|RUN)\s+(.+)`)

	for i, line := range lines {
		line = strings.TrimSpace(line)
		matches := taskPattern.FindStringSubmatch(line)

		if len(matches) == 3 {
			taskType := strings.ToUpper(matches[1])
			taskArgs := strings.TrimSpace(matches[2])

			task := parseNaturalLanguageTask(taskType, taskArgs)
			if task != nil {
				// For EDIT tasks, look for content in subsequent code blocks
				if task.Type == TaskTypeEditFile && task.Content == "" {
					if content := extractContentFromCodeBlock(lines, i+1); content != "" {
						task.Content = content
						if debugTaskParsing {
							fmt.Printf("DEBUG: Found content in code block for edit task\n")
						}
					}
				}

				// Create unique key for duplicate detection
				taskKey := fmt.Sprintf("%s:%s", task.Type, task.Path)
				if !seenTasks[taskKey] {
					seenTasks[taskKey] = true
					tasks = append(tasks, *task)
					if debugTaskParsing {
						fmt.Printf("DEBUG: Parsed natural language task - Type: %s, Path: %s\n", task.Type, task.Path)
					}
				}
			}
		}
	}

	// Also look for simpler patterns without emoji
	simplePattern := regexp.MustCompile(`(?i)^(read|edit|list|run)\s+(.+)`)

	for i, line := range lines {
		line = strings.TrimSpace(line)
		matches := simplePattern.FindStringSubmatch(line)

		if len(matches) == 3 {
			taskType := strings.ToUpper(matches[1])
			taskArgs := strings.TrimSpace(matches[2])

			task := parseNaturalLanguageTask(taskType, taskArgs)
			if task != nil {
				// Create unique key for duplicate detection
				taskKey := fmt.Sprintf("%s:%s", task.Type, task.Path)
				if !seenTasks[taskKey] {
					// For EDIT tasks, look for content in subsequent code blocks
					if task.Type == TaskTypeEditFile && task.Content == "" {
						if content := extractContentFromCodeBlock(lines, i+1); content != "" {
							task.Content = content
							if debugTaskParsing {
								fmt.Printf("DEBUG: Found content in code block for simple edit task\n")
							}
						}
					}

					seenTasks[taskKey] = true
					tasks = append(tasks, *task)
					if debugTaskParsing {
						fmt.Printf("DEBUG: Parsed simple natural language task - Type: %s, Path: %s\n", task.Type, task.Path)
					}
				}
			}
		}
	}

	if len(tasks) == 0 {
		if debugTaskParsing {
			fmt.Printf("DEBUG: No natural language tasks found\n")
		}
		return nil
	}

	if debugTaskParsing {
		fmt.Printf("DEBUG: Successfully parsed %d natural language tasks\n", len(tasks))
	}

	return &TaskList{Tasks: tasks}
}

// parseNaturalLanguageTask parses a single natural language task
func parseNaturalLanguageTask(taskType, args string) *Task {
	switch taskType {
	case "READ":
		return parseReadTask(args)
	case "EDIT":
		return parseEditTask(args)
	case "LIST":
		return parseListTask(args)
	case "RUN":
		return parseRunTask(args)
	default:
		return nil
	}
}

// parseReadTask parses natural language READ commands
func parseReadTask(args string) *Task {
	task := &Task{Type: TaskTypeReadFile}

	// Extract file path and options
	// Examples:
	// - "main.go"
	// - "main.go (max: 100 lines)"
	// - "main.go (lines 50-100)"
	// - "config.go (first 200 lines)"

	// Look for parenthetical options
	optionsPattern := regexp.MustCompile(`^(.+?)\s*\((.+)\)$`)
	matches := optionsPattern.FindStringSubmatch(args)

	if len(matches) == 3 {
		task.Path = strings.TrimSpace(matches[1])
		options := strings.TrimSpace(matches[2])

		// Parse options
		if strings.Contains(options, "max:") {
			maxPattern := regexp.MustCompile(`max:\s*(\d+)`)
			maxMatches := maxPattern.FindStringSubmatch(options)
			if len(maxMatches) == 2 {
				if maxLines, err := strconv.Atoi(maxMatches[1]); err == nil {
					task.MaxLines = maxLines
				}
			}
		}

		if strings.Contains(options, "first") && strings.Contains(options, "lines") {
			firstPattern := regexp.MustCompile(`first\s+(\d+)\s+lines`)
			firstMatches := firstPattern.FindStringSubmatch(options)
			if len(firstMatches) == 2 {
				if maxLines, err := strconv.Atoi(firstMatches[1]); err == nil {
					task.MaxLines = maxLines
				}
			}
		}

		// Parse line ranges like "lines 50-100"
		rangePattern := regexp.MustCompile(`lines?\s+(\d+)-(\d+)`)
		rangeMatches := rangePattern.FindStringSubmatch(options)
		if len(rangeMatches) == 3 {
			if startLine, err := strconv.Atoi(rangeMatches[1]); err == nil {
				if endLine, err := strconv.Atoi(rangeMatches[2]); err == nil {
					task.StartLine = startLine
					task.EndLine = endLine
				}
			}
		}
	} else {
		// Simple path without options
		task.Path = strings.TrimSpace(args)
	}

	// Set default max lines if not specified
	if task.MaxLines == 0 && task.StartLine == 0 && task.EndLine == 0 {
		task.MaxLines = DefaultMaxLines
	}

	return task
}

// parseEditTask parses natural language EDIT commands
func parseEditTask(args string) *Task {
	task := &Task{Type: TaskTypeEditFile}

	// Look for arrow notation: "file.go → description"
	arrowPattern := regexp.MustCompile(`^(.+?)\s*→\s*(.+)$`)
	matches := arrowPattern.FindStringSubmatch(args)

	if len(matches) == 3 {
		task.Path = strings.TrimSpace(matches[1])
		description := strings.TrimSpace(matches[2])

		// Store the description of what to do, not the actual content
		// The actual content should come from a code block or be generated by the LLM
		task.Intent = description

		// Extract context information from description for targeted editing
		parseEditContext(task, description)
	} else {
		// Simple path, content will be provided separately
		task.Path = strings.TrimSpace(args)
	}

	return task
}

// parseEditContext extracts context information from edit descriptions
func parseEditContext(task *Task, description string) {
	desc := strings.ToLower(description)

	// Pattern: "add X after Y" or "insert X after Y"
	afterPattern := regexp.MustCompile(`(?i)(?:add|insert)\s+.+?\s+after\s+["']?([^"']+)["']?`)
	if matches := afterPattern.FindStringSubmatch(description); len(matches) > 1 {
		task.StartContext = strings.TrimSpace(matches[1])
		task.InsertMode = "insert_after"
		return
	}

	// Pattern: "add X before Y" or "insert X before Y"
	beforePattern := regexp.MustCompile(`(?i)(?:add|insert)\s+.+?\s+before\s+["']?([^"']+)["']?`)
	if matches := beforePattern.FindStringSubmatch(description); len(matches) > 1 {
		task.StartContext = strings.TrimSpace(matches[1])
		task.InsertMode = "insert_before"
		return
	}

	// Pattern: "replace X with Y" or "replace X"
	replacePattern := regexp.MustCompile(`(?i)replace\s+["']?([^"']+)["']?`)
	if matches := replacePattern.FindStringSubmatch(description); len(matches) > 1 {
		task.StartContext = strings.TrimSpace(matches[1])
		task.InsertMode = "replace"
		return
	}

	// Pattern: "add X at the end" or "append X"
	if strings.Contains(desc, "at the end") || strings.Contains(desc, "append") {
		task.InsertMode = "append"
		return
	}

	// Pattern: "add X at the beginning" or "prepend X"
	if strings.Contains(desc, "at the beginning") || strings.Contains(desc, "prepend") || strings.Contains(desc, "at the top") {
		task.InsertMode = "insert_before"
		task.StartContext = "BEGINNING_OF_FILE"
		return
	}

	// Pattern: "add X between Y and Z"
	betweenPattern := regexp.MustCompile(`(?i)(?:add|insert)\s+.+?\s+between\s+["']?([^"']+)["']?\s+and\s+["']?([^"']+)["']?`)
	if matches := betweenPattern.FindStringSubmatch(description); len(matches) > 2 {
		task.StartContext = strings.TrimSpace(matches[1])
		task.EndContext = strings.TrimSpace(matches[2])
		task.InsertMode = "insert_between"
		return
	}
}

// parseListTask parses natural language LIST commands
func parseListTask(args string) *Task {
	task := &Task{Type: TaskTypeListDir}

	// Check for recursive indication
	if strings.Contains(strings.ToLower(args), "recursive") {
		task.Recursive = true
		// Remove "recursively" or "recursive" from path
		args = regexp.MustCompile(`\s+recursively?\s*$`).ReplaceAllString(args, "")
		args = regexp.MustCompile(`\s+recursive\s*$`).ReplaceAllString(args, "")
	}

	task.Path = strings.TrimSpace(args)
	if task.Path == "" {
		task.Path = "."
	}

	return task
}

// parseRunTask parses natural language RUN commands
func parseRunTask(args string) *Task {
	task := &Task{Type: TaskTypeRunShell}

	// Look for timeout specification
	timeoutPattern := regexp.MustCompile(`^(.+?)\s*\(timeout:\s*(\d+)s?\)$`)
	matches := timeoutPattern.FindStringSubmatch(args)

	if len(matches) == 3 {
		task.Command = strings.TrimSpace(matches[1])
		if timeout, err := strconv.Atoi(matches[2]); err == nil {
			task.Timeout = timeout
		}
	} else {
		task.Command = strings.TrimSpace(args)
	}

	// Set default timeout
	if task.Timeout == 0 {
		task.Timeout = DefaultTimeout
	}

	return task
}

// extractContentFromCodeBlock looks for content in code blocks following a task command
func extractContentFromCodeBlock(lines []string, startIdx int) string {
	if startIdx >= len(lines) {
		return ""
	}

	// Look for a code block starting within the next few lines
	for i := startIdx; i < len(lines) && i < startIdx+10; i++ {
		line := strings.TrimSpace(lines[i])

		// Found start of code block
		if strings.HasPrefix(line, "```") {
			var content []string

			// Collect lines until end of code block
			for j := i + 1; j < len(lines); j++ {
				blockLine := lines[j]
				if strings.TrimSpace(blockLine) == "```" {
					// End of code block found
					if len(content) > 0 {
						return strings.Join(content, "\n")
					}
					return ""
				}
				content = append(content, blockLine)
			}

			// Code block wasn't closed, but return what we have
			if len(content) > 0 {
				return strings.Join(content, "\n")
			}
		}
	}

	return ""
}

// tryFallbackJSONParsing attempts to parse raw JSON tasks when no backtick-wrapped JSON is found
func tryFallbackJSONParsing(llmResponse string) *TaskList {
	// Look for JSON-like patterns that might be tasks
	// This regex looks for JSON objects that contain "type" field with known task types
	taskTypePattern := `\{"type":\s*"(?:ReadFile|EditFile|ListDir|RunShell)"`
	re := regexp.MustCompile(taskTypePattern)

	if debugTaskParsing {
		fmt.Printf("DEBUG: Searching for raw JSON patterns in response...\n")
	}

	// Find potential JSON task objects
	lines := strings.Split(llmResponse, "\n")
	var jsonCandidates []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and obvious non-JSON lines
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}

		// Check if this line matches a task pattern
		if re.MatchString(line) {
			if debugTaskParsing {
				fmt.Printf("DEBUG: Found potential raw JSON task: %s\n", line[:min(len(line), 100)])
			}
			jsonCandidates = append(jsonCandidates, line)
		}
	}

	// Try to parse each candidate
	for _, jsonStr := range jsonCandidates {
		if debugTaskParsing {
			fmt.Printf("DEBUG: Attempting to parse raw JSON: %s\n", jsonStr[:min(len(jsonStr), 100)])
		}

		// Try to parse as single Task object
		var singleTask Task
		if err := json.Unmarshal([]byte(jsonStr), &singleTask); err == nil {
			// Validate the task
			if err := validateTask(&singleTask); err == nil {
				if debugTaskParsing {
					fmt.Printf("DEBUG: Successfully parsed raw JSON task - Type: %s, Path: %s\n", singleTask.Type, singleTask.Path)
				}

				// Create TaskList with single task
				taskList := TaskList{Tasks: []Task{singleTask}}
				return &taskList
			} else {
				if debugTaskParsing {
					fmt.Printf("DEBUG: Raw JSON task validation failed: %v\n", err)
				}
			}
		} else {
			if debugTaskParsing {
				fmt.Printf("DEBUG: Failed to parse raw JSON: %v\n", err)
			}
		}

		// Try to parse as TaskList
		var taskList TaskList
		if err := json.Unmarshal([]byte(jsonStr), &taskList); err == nil {
			if len(taskList.Tasks) > 0 {
				if err := validateTasks(&taskList); err == nil {
					if debugTaskParsing {
						fmt.Printf("DEBUG: Successfully parsed raw JSON TaskList with %d tasks\n", len(taskList.Tasks))
					}
					return &taskList
				} else {
					if debugTaskParsing {
						fmt.Printf("DEBUG: Raw JSON TaskList validation failed: %v\n", err)
					}
				}
			}
		}
	}

	if debugTaskParsing {
		fmt.Printf("DEBUG: No valid raw JSON tasks found\n")
	}
	return nil
}

// ParseTasks extracts and parses tasks from LLM response (tries natural language first, then JSON)
func ParseTasks(llmResponse string) (*TaskList, error) {
	// First, try natural language parsing
	if result := tryNaturalLanguageParsing(llmResponse); result != nil {
		if debugTaskParsing {
			fmt.Printf("DEBUG: Successfully parsed %d tasks using natural language parsing\n", len(result.Tasks))
		}
		return result, nil
	}

	// Fall back to JSON parsing
	if debugTaskParsing {
		fmt.Printf("DEBUG: Natural language parsing found no tasks, trying JSON parsing...\n")
	}

	// Look for JSON code blocks
	re := regexp.MustCompile("(?s)```(?:json)?\n?(.*?)\n?```")
	matches := re.FindAllStringSubmatch(llmResponse, -1)

	if len(matches) == 0 {
		// FALLBACK: Try to parse raw JSON if no backtick-wrapped JSON found
		if debugTaskParsing {
			fmt.Printf("DEBUG: No backtick-wrapped JSON found, trying fallback raw JSON parsing...\n")
		}

		// Try fallback parsing for raw JSON
		if result := tryFallbackJSONParsing(llmResponse); result != nil {
			if debugTaskParsing {
				fmt.Printf("DEBUG: Successfully parsed %d tasks using fallback raw JSON parsing\n", len(result.Tasks))
			}
			return result, nil
		}

		// Enhanced debug: Check if the response mentions tasks or actions that should trigger task execution
		if debugTaskParsing {
			lowerResponse := strings.ToLower(llmResponse)
			actionWords := []string{
				"reading file", "📖", "🔧", "creating", "editing", "modifying",
				"create", "edit", "file", "license", "i'll", "i will", "let me",
				"executing", "running", "applying", "writing to", "updating",
			}

			foundActions := []string{}
			for _, word := range actionWords {
				if strings.Contains(lowerResponse, word) {
					foundActions = append(foundActions, word)
				}
			}

			if len(foundActions) > 0 {
				fmt.Printf("🚨 DEBUG: LLM response suggests action but no JSON tasks found!\n")
				fmt.Printf("   Found action indicators: %v\n", foundActions)
				fmt.Printf("   Response (first 200 chars): %s...\n", llmResponse[:min(len(llmResponse), 200)])
				fmt.Printf("   ✅ Expected: JSON code block like:\n")
				fmt.Printf("   " + "```" + "json\n")
				fmt.Printf("   {\"type\": \"ReadFile\", \"path\": \"README.md\", \"max_lines\": 100}\n")
				fmt.Printf("   " + "```" + "\n")
				fmt.Printf("   📝 OR for multiple tasks:\n")
				fmt.Printf("   " + "```" + "json\n")
				fmt.Printf("   {\n")
				fmt.Printf("     \"tasks\": [\n")
				fmt.Printf("       {\"type\": \"ReadFile\", \"path\": \"README.md\", \"max_lines\": 100}\n")
				fmt.Printf("     ]\n")
				fmt.Printf("   }\n")
				fmt.Printf("   " + "```" + "\n")
				fmt.Printf("   💡 The LLM may need better prompting to output actual task JSON.\n\n")
			} else {
				fmt.Printf("✅ DEBUG: No JSON tasks found - this appears to be a regular Q&A response.\n")
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

		// Try to parse as TaskList first
		var taskList TaskList
		if err := json.Unmarshal([]byte(jsonStr), &taskList); err == nil {
			if debugTaskParsing {
				fmt.Printf("DEBUG: Parsed as TaskList with %d tasks\n", len(taskList.Tasks))
			}

			// Only proceed with TaskList if it actually has tasks
			if len(taskList.Tasks) > 0 {
				// Successfully parsed as TaskList
				if err := validateTasks(&taskList); err != nil {
					return nil, fmt.Errorf("invalid tasks: %w", err)
				}

				if debugTaskParsing {
					fmt.Printf("DEBUG: Successfully parsed %d tasks (as TaskList)\n", len(taskList.Tasks))
				}
				return &taskList, nil
			}

			if debugTaskParsing {
				fmt.Printf("DEBUG: TaskList was empty, trying single task parsing\n")
			}
		}

		// Try to parse as single Task object
		var singleTask Task
		if err := json.Unmarshal([]byte(jsonStr), &singleTask); err != nil {
			if debugTaskParsing {
				fmt.Printf("DEBUG: Failed to parse JSON as either TaskList or single Task: %v\n", err)
			}
			continue // Skip invalid JSON blocks
		}

		if debugTaskParsing {
			fmt.Printf("DEBUG: Parsed single task - Type: %s, Path: %s\n", singleTask.Type, singleTask.Path)
		}

		// Create TaskList with single task
		taskList = TaskList{Tasks: []Task{singleTask}}

		if debugTaskParsing {
			fmt.Printf("DEBUG: Created TaskList with %d tasks\n", len(taskList.Tasks))
		}

		// Validate tasks
		if err := validateTasks(&taskList); err != nil {
			if debugTaskParsing {
				fmt.Printf("DEBUG: Task validation failed: %v\n", err)
			}
			return nil, fmt.Errorf("invalid tasks: %w", err)
		}

		if debugTaskParsing {
			fmt.Printf("DEBUG: Successfully parsed 1 task (as single Task object)\n")
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
		} else if t.StartLine > 0 {
			if t.MaxLines > 0 {
				return fmt.Sprintf("Read %s (from line %d, max %d lines)", t.Path, t.StartLine, t.MaxLines)
			}
			return fmt.Sprintf("Read %s (from line %d)", t.Path, t.StartLine)
		} else if t.MaxLines > 0 {
			return fmt.Sprintf("Read %s (max %d lines)", t.Path, t.MaxLines)
		}
		return fmt.Sprintf("Read %s", t.Path)

	case TaskTypeEditFile:
		if t.Diff != "" {
			return fmt.Sprintf("Edit %s (apply diff)", t.Path)
		}
		// Check if this is a targeted edit
		if t.InsertMode != "" {
			switch t.InsertMode {
			case "append":
				return fmt.Sprintf("Edit %s (append content)", t.Path)
			case "insert_after":
				return fmt.Sprintf("Edit %s (insert after)", t.Path)
			case "insert_before":
				return fmt.Sprintf("Edit %s (insert before)", t.Path)
			case "replace":
				return fmt.Sprintf("Edit %s (replace section)", t.Path)
			case "insert_between":
				return fmt.Sprintf("Edit %s (insert between)", t.Path)
			default:
				return fmt.Sprintf("Edit %s (targeted edit)", t.Path)
			}
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

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
	TaskTypeSearch   TaskType = "Search" // NEW: Search files using ripgrep
	TaskTypeMemory   TaskType = "Memory" // NEW: Memory management operations
)

// Task represents a single task to be executed
type Task struct {
	Type TaskType `json:"type"`

	// Common fields
	Path string `json:"path,omitempty"`

	// ReadFile specific
	MaxLines        int  `json:"max_lines,omitempty"`
	StartLine       int  `json:"start_line,omitempty"`
	EndLine         int  `json:"end_line,omitempty"`
	ShowLineNumbers bool `json:"show_line_numbers,omitempty"` // Show line numbers for precise editing

	// EditFile specific
	Diff    string `json:"diff,omitempty"`
	Content string `json:"content,omitempty"`
	Intent  string `json:"intent,omitempty"` // Natural language description of what to do

	// Line-based editing (NEW - more precise than context-based)
	TargetLine        int    `json:"target_line,omitempty"`        // Single line to edit (1-indexed)
	TargetStartLine   int    `json:"target_start_line,omitempty"`  // Start of line range to edit (1-indexed)
	TargetEndLine     int    `json:"target_end_line,omitempty"`    // End of line range to edit (1-indexed)
	ContextValidation string `json:"context_validation,omitempty"` // Optional: expected content for safety validation

	// SafeEdit format (ULTRA-SAFE - mandatory context validation)
	SafeEditMode  bool   `json:"safe_edit_mode,omitempty"` // Flag indicating this uses SafeEdit format
	BeforeContext string `json:"before_context,omitempty"` // Lines immediately before edit range (mandatory for validation)
	AfterContext  string `json:"after_context,omitempty"`  // Lines immediately after edit range (mandatory for validation)
	EditReason    string `json:"edit_reason,omitempty"`    // Clear explanation of what's being changed and why

	// Targeted editing fields (LEGACY - kept for backward compatibility)
	StartContext string `json:"start_context,omitempty"` // Line or pattern marking start of edit section
	EndContext   string `json:"end_context,omitempty"`   // Line or pattern marking end of edit section
	InsertMode   string `json:"insert_mode,omitempty"`   // "replace", "insert_before", "insert_after", "append"

	// RunShell specific
	Command string `json:"command,omitempty"`
	Timeout int    `json:"timeout,omitempty"` // Timeout in seconds

	// NEW: Interactive shell support
	Interactive     bool                `json:"interactive,omitempty"`      // Flag indicating this command needs interaction
	InputMode       string              `json:"input_mode,omitempty"`       // "auto", "prompt", "predefined"
	PredefinedInput []string            `json:"predefined_input,omitempty"` // Pre-defined responses to prompts
	ExpectedPrompts []InteractivePrompt `json:"expected_prompts,omitempty"` // Expected prompts and their responses
	AllowUserInput  bool                `json:"allow_user_input,omitempty"` // Whether to allow real-time user input during execution

	// ListDir specific
	Recursive bool `json:"recursive,omitempty"`

	// Search specific (NEW)
	Query         string   `json:"query,omitempty"`          // The search pattern/regex
	FileTypes     []string `json:"file_types,omitempty"`     // File type filters (e.g., ["go", "js"])
	ExcludeTypes  []string `json:"exclude_types,omitempty"`  // Exclude file types
	GlobPatterns  []string `json:"glob_patterns,omitempty"`  // Include glob patterns (e.g., ["*.tf"])
	ExcludeGlobs  []string `json:"exclude_globs,omitempty"`  // Exclude glob patterns
	IgnoreCase    bool     `json:"ignore_case,omitempty"`    // Case-insensitive search
	WholeWord     bool     `json:"whole_word,omitempty"`     // Match whole words only
	FixedString   bool     `json:"fixed_string,omitempty"`   // Treat query as literal string, not regex
	ContextBefore int      `json:"context_before,omitempty"` // Lines of context before matches
	ContextAfter  int      `json:"context_after,omitempty"`  // Lines of context after matches
	MaxResults    int      `json:"max_results,omitempty"`    // Limit number of results
	FilenamesOnly bool     `json:"filenames_only,omitempty"` // Show only filenames with matches
	CountMatches  bool     `json:"count_matches,omitempty"`  // Count matches per file
	SearchHidden  bool     `json:"search_hidden,omitempty"`  // Search hidden files and directories
	UsePCRE2      bool     `json:"use_pcre2,omitempty"`      // Use PCRE2 regex engine for advanced features

	// Memory specific (NEW)
	MemoryOperation   string   `json:"memory_operation,omitempty"`   // "create", "update", "delete", "get", "list"
	MemoryID          string   `json:"memory_id,omitempty"`          // Memory identifier
	MemoryContent     string   `json:"memory_content,omitempty"`     // Memory content
	MemoryTags        []string `json:"memory_tags,omitempty"`        // Memory tags
	MemoryActive      *bool    `json:"memory_active,omitempty"`      // Whether memory is active (nil means no change for update)
	MemoryDescription string   `json:"memory_description,omitempty"` // Memory description
}

// InteractivePrompt represents an expected prompt and its response in interactive commands
type InteractivePrompt struct {
	Prompt      string `json:"prompt"`      // Expected prompt text (can be regex)
	Response    string `json:"response"`    // Response to send
	IsRegex     bool   `json:"is_regex"`    // Whether prompt is a regex pattern
	Optional    bool   `json:"optional"`    // Whether this prompt might not appear
	Description string `json:"description"` // Human-readable description of what this prompt is for
}

// TaskList represents a list of tasks from the LLM
type TaskList struct {
	Tasks []Task `json:"tasks"`
}

// EditSummary represents detailed information about a file edit operation
type EditSummary struct {
	FilePath          string `json:"file_path"`
	EditType          string `json:"edit_type"` // "create", "modify", "delete"
	LinesAdded        int    `json:"lines_added"`
	LinesRemoved      int    `json:"lines_removed"`
	LinesModified     int    `json:"lines_modified"`
	TotalLines        int    `json:"total_lines"` // Total lines after edit
	CharactersAdded   int    `json:"characters_added"`
	CharactersRemoved int    `json:"characters_removed"`
	Summary           string `json:"summary"` // Brief description of changes
	WasSuccessful     bool   `json:"was_successful"`
}

// TaskResponse represents the result of executing a task
type TaskResponse struct {
	Task          Task         `json:"task"`
	Success       bool         `json:"success"`
	Output        string       `json:"output,omitempty"`         // Display message for user
	ActualContent string       `json:"actual_content,omitempty"` // Actual content for LLM (hidden from user)
	EditSummary   *EditSummary `json:"edit_summary,omitempty"`   // Detailed edit information (for EditFile tasks)
	Error         string       `json:"error,omitempty"`
	Approved      bool         `json:"approved,omitempty"` // For tasks requiring confirmation
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
	taskPattern := regexp.MustCompile(`^🔧\s+(READ|EDIT|LIST|RUN|SEARCH|MEMORY)\s+(.+)`)

	for i, line := range lines {
		line = strings.TrimSpace(line)
		matches := taskPattern.FindStringSubmatch(line)

		if len(matches) == 3 {
			taskType := strings.ToUpper(matches[1])
			taskArgs := strings.TrimSpace(matches[2])

			task := parseNaturalLanguageTask(taskType, taskArgs)
			if task != nil {
				// For EDIT tasks, look for SafeEdit format or content in subsequent code blocks
				if task.Type == TaskTypeEditFile && task.Content == "" {
					// First try to parse SafeEdit format
					if parseSafeEditFormat(task, lines, i+1) {
						if debugTaskParsing {
							fmt.Printf("DEBUG: Parsed SafeEdit format for edit task\n")
						}
					} else if content := extractContentFromCodeBlock(lines, i+1); content != "" {
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
	simplePattern := regexp.MustCompile(`(?i)^(read|edit|list|run|search|memory)\s+(.+)`)

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
	case "SEARCH":
		return parseSearchTask(args)
	case "MEMORY":
		return parseMemoryTask(args)
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

		// Check for line numbers request
		if strings.Contains(strings.ToLower(options), "line numbers") ||
			strings.Contains(strings.ToLower(options), "with numbers") ||
			strings.Contains(strings.ToLower(options), "numbered") {
			task.ShowLineNumbers = true
		}
	} else {
		// Check for line numbers request in simple format like "file.go with line numbers"
		if strings.Contains(strings.ToLower(args), " with line numbers") ||
			strings.Contains(strings.ToLower(args), " with numbers") ||
			strings.Contains(strings.ToLower(args), " numbered") {
			task.ShowLineNumbers = true
			// Remove the line numbers request from the path
			args = regexp.MustCompile(`\s+with\s+(?:line\s+)?numbers?`).ReplaceAllString(args, "")
			args = regexp.MustCompile(`\s+numbered`).ReplaceAllString(args, "")
		}

		// Simple path without options
		task.Path = strings.TrimSpace(args)
	}

	// Set default max lines if not specified
	if task.MaxLines == 0 && task.StartLine == 0 && task.EndLine == 0 {
		task.MaxLines = DefaultMaxLines
	}

	return task
}

// parseEditTask parses natural language EDIT commands with optional line number support
func parseEditTask(args string) *Task {
	task := &Task{Type: TaskTypeEditFile}

	// Look for arrow notation: "file.go:15 → description" or "file.go:15 -> description"
	arrowPattern := regexp.MustCompile(`^(.+?)\s*(?:→|->)\s*(.+)$`)
	matches := arrowPattern.FindStringSubmatch(args)

	if len(matches) == 3 {
		pathWithLines := strings.TrimSpace(matches[1])
		description := strings.TrimSpace(matches[2])

		// Parse line numbers from path (e.g., "file.go:15" or "file.go:10-20")
		parsePathWithLines(task, pathWithLines)

		// Store the description of what to do, not the actual content
		// The actual content should come from a code block or be generated by the LLM
		task.Intent = description

		// Extract context information from description for targeted editing (legacy support)
		parseEditContext(task, description)
	} else {
		// Simple path, content will be provided separately
		// Also check for line numbers in simple format
		parsePathWithLines(task, strings.TrimSpace(args))
	}

	return task
}

// parsePathWithLines extracts file path and line numbers from various formats
func parsePathWithLines(task *Task, pathWithLines string) {
	// Pattern 1: "file.go:15" (single line)
	singleLinePattern := regexp.MustCompile(`^(.+):(\d+)$`)
	if matches := singleLinePattern.FindStringSubmatch(pathWithLines); len(matches) == 3 {
		task.Path = strings.TrimSpace(matches[1])
		if lineNum, err := strconv.Atoi(matches[2]); err == nil {
			task.TargetLine = lineNum
		}
		return
	}

	// Pattern 2: "file.go:10-20" (line range)
	rangePattern := regexp.MustCompile(`^(.+):(\d+)-(\d+)$`)
	if matches := rangePattern.FindStringSubmatch(pathWithLines); len(matches) == 4 {
		task.Path = strings.TrimSpace(matches[1])
		if startLine, err := strconv.Atoi(matches[2]); err == nil {
			if endLine, err := strconv.Atoi(matches[3]); err == nil {
				task.TargetStartLine = startLine
				task.TargetEndLine = endLine
			}
		}
		return
	}

	// Pattern 3: Plain file path (no line numbers)
	task.Path = strings.TrimSpace(pathWithLines)
}

// parseEditContext extracts context information from edit descriptions
func parseEditContext(task *Task, description string) {
	desc := strings.ToLower(description)

	// Pattern: "replace all occurrences of X with Y" or "replace all X with Y"
	replaceAllPattern := regexp.MustCompile(`(?i)replace\s+all\s+(?:occurrences\s+of\s+)?["']?([^"']+?)["']?\s+with\s+["']?([^"']+?)["']?$`)
	if matches := replaceAllPattern.FindStringSubmatch(description); len(matches) > 2 {
		task.StartContext = strings.TrimSpace(matches[1]) // What to find
		task.EndContext = strings.TrimSpace(matches[2])   // What to replace with
		task.InsertMode = "replace_all"
		return
	}

	// Pattern: "find and replace X with Y" or "find X and replace with Y"
	findReplacePattern := regexp.MustCompile(`(?i)(?:find\s+(?:and\s+)?replace|find)\s+["']?([^"']+?)["']?\s+(?:and\s+replace\s+)?with\s+["']?([^"']+?)["']?$`)
	if matches := findReplacePattern.FindStringSubmatch(description); len(matches) > 2 {
		task.StartContext = strings.TrimSpace(matches[1]) // What to find
		task.EndContext = strings.TrimSpace(matches[2])   // What to replace with
		task.InsertMode = "replace_all"
		return
	}

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

	// Look for interactive flags
	interactivePattern := regexp.MustCompile(`--interactive(?:\s+(\w+))?`)
	if matches := interactivePattern.FindStringSubmatch(args); len(matches) > 0 {
		task.Interactive = true
		if len(matches) > 1 && matches[1] != "" {
			task.InputMode = matches[1] // auto, prompt, predefined
		} else {
			task.InputMode = "prompt" // default
		}
		// Remove interactive flag from command
		args = interactivePattern.ReplaceAllString(args, "")
	}

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

	// Auto-detect interactive commands if not explicitly specified
	if !task.Interactive && isLikelyInteractiveCommand(task.Command) {
		task.Interactive = true
		task.InputMode = "auto" // Use automatic response handling
	}

	return task
}

// parseSearchTask parses natural language SEARCH commands
func parseSearchTask(args string) *Task {
	task := &Task{Type: TaskTypeSearch}

	// Parse search query and options
	parts := strings.Fields(args)
	if len(parts) == 0 {
		return nil
	}

	// First part is always the search query
	task.Query = parts[0]

	// Remove quotes if present
	if len(task.Query) >= 2 {
		if (strings.HasPrefix(task.Query, "\"") && strings.HasSuffix(task.Query, "\"")) ||
			(strings.HasPrefix(task.Query, "'") && strings.HasSuffix(task.Query, "'")) {
			task.Query = task.Query[1 : len(task.Query)-1]
		}
	}

	// Parse additional options
	for i := 1; i < len(parts); i++ {
		part := strings.ToLower(parts[i])

		switch {
		case strings.HasPrefix(part, "type:"):
			// File type filter: type:go,js
			types := strings.TrimPrefix(part, "type:")
			task.FileTypes = strings.Split(types, ",")

		case strings.HasPrefix(part, "-type:"):
			// Exclude file type: -type:md
			types := strings.TrimPrefix(part, "-type:")
			task.ExcludeTypes = strings.Split(types, ",")

		case strings.HasPrefix(part, "glob:"):
			// Glob pattern: glob:*.tf
			glob := strings.TrimPrefix(part, "glob:")
			task.GlobPatterns = append(task.GlobPatterns, glob)

		case strings.HasPrefix(part, "-glob:"):
			// Exclude glob: -glob:*.md
			glob := strings.TrimPrefix(part, "-glob:")
			task.ExcludeGlobs = append(task.ExcludeGlobs, glob)

		case strings.HasPrefix(part, "context:"):
			// Context lines: context:3
			if contextStr := strings.TrimPrefix(part, "context:"); contextStr != "" {
				if context, err := strconv.Atoi(contextStr); err == nil {
					task.ContextBefore = context
					task.ContextAfter = context
				}
			}

		case strings.HasPrefix(part, "max:"):
			// Max results: max:50
			if maxStr := strings.TrimPrefix(part, "max:"); maxStr != "" {
				if max, err := strconv.Atoi(maxStr); err == nil {
					task.MaxResults = max
				}
			}

		case part == "case-insensitive" || part == "ignore-case" || part == "-i":
			task.IgnoreCase = true

		case part == "whole-word" || part == "-w":
			task.WholeWord = true

		case part == "fixed-string" || part == "-f":
			task.FixedString = true

		case part == "filenames-only" || part == "-l":
			task.FilenamesOnly = true

		case part == "count" || part == "-c":
			task.CountMatches = true

		case part == "hidden" || part == "all":
			task.SearchHidden = true

		case part == "pcre2" || part == "-p":
			task.UsePCRE2 = true

		case strings.HasPrefix(part, "in:"):
			// Search in specific directory: in:src/
			if path := strings.TrimPrefix(part, "in:"); path != "" {
				task.Path = path
			}
		}
	}

	// Set defaults
	if task.MaxResults == 0 {
		task.MaxResults = 100 // Default limit to prevent overwhelming output
	}

	if task.Path == "" {
		task.Path = "." // Default to current directory
	}

	return task
}

// parseMemoryTask parses natural language MEMORY commands
func parseMemoryTask(args string) *Task {
	task := &Task{Type: TaskTypeMemory}

	// Parse memory operation and options with quote-aware parsing
	// Examples:
	// - "create user-preferences content:\"Remember user prefers dark mode\""
	// - "update user-preferences content:\"User prefers dark mode and large fonts\""
	// - "delete user-preferences"
	// - "get user-preferences"
	// - "list"
	// - "list active:true"

	parts := parseQuotedArgs(args)
	if len(parts) == 0 {
		return nil
	}

	// First part is the operation
	operation := strings.ToLower(parts[0])
	task.MemoryOperation = operation

	// For operations other than list, second part is the memory ID
	if operation != "list" && len(parts) > 1 {
		task.MemoryID = parts[1]
	}

	// Parse additional options
	for i := 2; i < len(parts); i++ {
		part := parts[i]

		switch {
		case strings.HasPrefix(part, "content:"):
			// Memory content: content:"Remember user preferences"
			content := strings.TrimPrefix(part, "content:")
			// Remove quotes if present
			if len(content) >= 2 {
				if (strings.HasPrefix(content, "\"") && strings.HasSuffix(content, "\"")) ||
					(strings.HasPrefix(content, "'") && strings.HasSuffix(content, "'")) {
					content = content[1 : len(content)-1]
				}
			}
			task.MemoryContent = content

		case strings.HasPrefix(part, "description:"):
			// Memory description: description:"User interface preferences"
			desc := strings.TrimPrefix(part, "description:")
			// Remove quotes if present
			if len(desc) >= 2 {
				if (strings.HasPrefix(desc, "\"") && strings.HasSuffix(desc, "\"")) ||
					(strings.HasPrefix(desc, "'") && strings.HasSuffix(desc, "'")) {
					desc = desc[1 : len(desc)-1]
				}
			}
			task.MemoryDescription = desc

		case strings.HasPrefix(part, "tags:"):
			// Memory tags: tags:ui,preferences,user
			tags := strings.TrimPrefix(part, "tags:")
			task.MemoryTags = strings.Split(tags, ",")

		case strings.HasPrefix(part, "active:"):
			// Memory active status: active:true or active:false
			activeStr := strings.ToLower(strings.TrimPrefix(part, "active:"))
			if activeStr == "true" {
				active := true
				task.MemoryActive = &active
			} else if activeStr == "false" {
				active := false
				task.MemoryActive = &active
			}
		}
	}

	// For operations that require content, try to extract it from the remaining arguments
	// if not already specified with content: prefix
	if task.MemoryContent == "" && (operation == "create" || operation == "update") {
		// Look for content in the remaining arguments
		if len(parts) > 2 {
			contentParts := parts[2:]
			// Skip parts that are options (contain colons)
			var contentWords []string
			for _, part := range contentParts {
				if !strings.Contains(part, ":") {
					contentWords = append(contentWords, part)
				}
			}
			if len(contentWords) > 0 {
				task.MemoryContent = strings.Join(contentWords, " ")
			}
		}
	}

	return task
}

// parseQuotedArgs parses a string into arguments while respecting quoted strings
func parseQuotedArgs(input string) []string {
	var args []string
	var current strings.Builder
	var inQuotes bool
	var quoteChar rune

	input = strings.TrimSpace(input)

	for _, char := range input {
		switch {
		case !inQuotes && (char == '"' || char == '\''):
			// Start of quoted string
			inQuotes = true
			quoteChar = char
			current.WriteRune(char)

		case inQuotes && char == quoteChar:
			// End of quoted string
			current.WriteRune(char)
			inQuotes = false

		case !inQuotes && char == ' ':
			// Space outside quotes - end current argument
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			// Note: consecutive spaces will just result in empty current.Len()
			// and will be skipped naturally

		default:
			// Regular character or space inside quotes
			current.WriteRune(char)
		}
	}

	// Add the last argument if there's one
	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// isLikelyInteractiveCommand checks if a command typically requires user interaction
func isLikelyInteractiveCommand(command string) bool {
	interactivePatterns := []string{
		"npm init",
		"yarn init",
		"git config --global",
		"ssh-keygen",
		"openssl",
		"gpg",
		"sudo",
		"apt install",
		"yum install",
		"brew install",
		"pip install",
		"docker run.*-it",
		"mysql.*-p",
		"psql.*-W",
	}

	cmdLower := strings.ToLower(command)
	for _, pattern := range interactivePatterns {
		if matched, _ := regexp.MatchString(pattern, cmdLower); matched {
			return true
		}
	}

	return false
}

// parseSafeEditFormat attempts to parse SafeEdit format (both old and new fenced formats)
func parseSafeEditFormat(task *Task, lines []string, startIdx int) bool {
	if startIdx >= len(lines) {
		return false
	}

	var beforeContext []string
	var editContent []string
	var afterContext []string
	var currentSection string

	// Look for SafeEdit format markers in the following lines
	for i := startIdx; i < len(lines) && i < startIdx+50; i++ { // Look within reasonable distance
		line := strings.TrimSpace(lines[i])

		// Detect section markers - support both old and new formats
		if line == "BEFORE_CONTEXT:" || line == "--- BEFORE ---" {
			currentSection = "before"
			continue
		} else if strings.HasPrefix(line, "EDIT_LINES:") {
			currentSection = "edit"

			// Extract line range from EDIT_LINES marker
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				lineRange := strings.TrimSpace(parts[1])
				parseEditLineRange(task, lineRange)
			}
			continue
		} else if line == "--- CHANGE ---" {
			currentSection = "change"
			continue
		} else if line == "AFTER_CONTEXT:" || line == "--- AFTER ---" {
			currentSection = "after"
			continue
		}

		// Handle EDIT_LINES within CHANGE section (new fenced format)
		if currentSection == "change" && strings.HasPrefix(line, "EDIT_LINES:") {
			// Extract line range from EDIT_LINES marker
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				lineRange := strings.TrimSpace(parts[1])
				parseEditLineRange(task, lineRange)
			}
			continue
		}

		// Skip empty lines only at section boundaries (before any section is set)
		if line == "" && currentSection == "" {
			continue
		}

		// If we hit another task or end of meaningful content, stop
		if strings.HasPrefix(line, "🔧") || strings.HasPrefix(line, "```") {
			break
		}

		// Collect content based on current section (preserve all lines including empty ones)
		switch currentSection {
		case "before":
			beforeContext = append(beforeContext, lines[i]) // Preserve original formatting including empty lines
		case "edit":
			editContent = append(editContent, lines[i]) // Preserve original formatting including empty lines
		case "change":
			// In the new fenced format, content comes after EDIT_LINES
			if !strings.HasPrefix(line, "EDIT_LINES:") {
				editContent = append(editContent, lines[i]) // Preserve original formatting including empty lines
			}
		case "after":
			afterContext = append(afterContext, lines[i]) // Preserve original formatting including empty lines
		}
	}

	// Validate we found all required sections
	if len(beforeContext) == 0 || len(editContent) == 0 || len(afterContext) == 0 {
		return false // Not a valid SafeEdit format
	}

	// Populate task fields
	task.SafeEditMode = true
	task.BeforeContext = strings.Join(beforeContext, "\n")
	task.Content = strings.Join(editContent, "\n")
	task.AfterContext = strings.Join(afterContext, "\n")

	if debugTaskParsing {
		fmt.Printf("DEBUG: SafeEdit parsed - BeforeContext: %d lines, EditContent: %d lines, AfterContext: %d lines\n",
			len(beforeContext), len(editContent), len(afterContext))
	}

	return true
}

// parseEditLineRange extracts line range information from EDIT_LINES marker
func parseEditLineRange(task *Task, lineRange string) {
	// Handle single line: "15"
	if !strings.Contains(lineRange, "-") {
		if lineNum, err := strconv.Atoi(strings.TrimSpace(lineRange)); err == nil {
			task.TargetLine = lineNum
		}
		return
	}

	// Handle range: "15-17"
	parts := strings.Split(lineRange, "-")
	if len(parts) == 2 {
		if startLine, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
			if endLine, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
				task.TargetStartLine = startLine
				task.TargetEndLine = endLine
			}
		}
	}
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

	case TaskTypeSearch:
		if task.Query == "" {
			return fmt.Errorf("Search requires query")
		}
		if task.Path == "" {
			task.Path = "." // Default to current directory
		}
		if task.MaxResults <= 0 {
			task.MaxResults = 100 // Default limit
		}

	case TaskTypeMemory:
		if task.MemoryOperation == "" {
			return fmt.Errorf("Memory requires operation (create, update, delete, get, list)")
		}

		operation := strings.ToLower(task.MemoryOperation)
		switch operation {
		case "create":
			if task.MemoryID == "" {
				return fmt.Errorf("Memory create requires ID")
			}
			if task.MemoryContent == "" {
				return fmt.Errorf("Memory create requires content")
			}
		case "update":
			if task.MemoryID == "" {
				return fmt.Errorf("Memory update requires ID")
			}
			// For updates, at least one field must be provided
			if task.MemoryContent == "" && len(task.MemoryTags) == 0 &&
				task.MemoryActive == nil && task.MemoryDescription == "" {
				return fmt.Errorf("Memory update requires at least one field to update (content, tags, active, description)")
			}
		case "delete", "get":
			if task.MemoryID == "" {
				return fmt.Errorf("Memory %s requires ID", operation)
			}
		case "list":
			// List doesn't require any additional validation
		default:
			return fmt.Errorf("unknown memory operation: %s (supported: create, update, delete, get, list)", operation)
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
			case "replace_all":
				return fmt.Sprintf("Edit %s (replace all occurrences)", t.Path)
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

	case TaskTypeSearch:
		description := fmt.Sprintf("Search for '%s'", t.Query)
		if t.Path != "." {
			description += fmt.Sprintf(" in %s", t.Path)
		}
		if len(t.FileTypes) > 0 {
			description += fmt.Sprintf(" (types: %s)", strings.Join(t.FileTypes, ","))
		}
		if t.IgnoreCase {
			description += " (case-insensitive)"
		}
		if t.WholeWord {
			description += " (whole words)"
		}
		return description

	case TaskTypeMemory:
		switch strings.ToLower(t.MemoryOperation) {
		case "create":
			return fmt.Sprintf("Create memory '%s'", t.MemoryID)
		case "update":
			return fmt.Sprintf("Update memory '%s'", t.MemoryID)
		case "delete":
			return fmt.Sprintf("Delete memory '%s'", t.MemoryID)
		case "get":
			return fmt.Sprintf("Get memory '%s'", t.MemoryID)
		case "list":
			if t.MemoryActive != nil {
				if *t.MemoryActive {
					return "List active memories"
				} else {
					return "List inactive memories"
				}
			}
			return "List all memories"
		default:
			return fmt.Sprintf("Memory operation '%s' on '%s'", t.MemoryOperation, t.MemoryID)
		}

	default:
		return fmt.Sprintf("Unknown task: %s", t.Type)
	}
}

// GetEditSummaryText returns a human-readable summary of the edit changes
func (es *EditSummary) GetEditSummaryText() string {
	if es == nil {
		return ""
	}

	var parts []string

	// Add edit type and file
	parts = append(parts, fmt.Sprintf("%s %s", es.EditType, es.FilePath))

	// Add change statistics
	if es.EditType == "create" {
		parts = append(parts, fmt.Sprintf("(%d lines, %d characters)",
			es.TotalLines, es.CharactersAdded))
	} else if es.EditType == "modify" {
		var changes []string
		if es.LinesAdded > 0 {
			changes = append(changes, fmt.Sprintf("+%d lines", es.LinesAdded))
		}
		if es.LinesRemoved > 0 {
			changes = append(changes, fmt.Sprintf("-%d lines", es.LinesRemoved))
		}
		if es.LinesModified > 0 {
			changes = append(changes, fmt.Sprintf("~%d lines", es.LinesModified))
		}
		if len(changes) > 0 {
			parts = append(parts, fmt.Sprintf("(%s)", strings.Join(changes, ", ")))
		}
	}

	// Add summary if available
	if es.Summary != "" {
		parts = append(parts, "- "+es.Summary)
	}

	// Add success status
	if es.WasSuccessful {
		parts = append(parts, "✓")
	} else {
		parts = append(parts, "✗")
	}

	return strings.Join(parts, " ")
}

// GetCompactSummary returns a brief one-line summary of the edit
func (es *EditSummary) GetCompactSummary() string {
	if es == nil {
		return ""
	}

	switch es.EditType {
	case "create":
		return fmt.Sprintf("Created %s (%d lines)", es.FilePath, es.TotalLines)
	case "modify":
		totalChanges := es.LinesAdded + es.LinesRemoved + es.LinesModified
		return fmt.Sprintf("Modified %s (%d changes)", es.FilePath, totalChanges)
	case "delete":
		return fmt.Sprintf("Deleted %s", es.FilePath)
	default:
		return fmt.Sprintf("Edited %s", es.FilePath)
	}
}

// GetLLMSummary returns a detailed summary formatted for LLM consumption
func (tr *TaskResponse) GetLLMSummary() string {
	if tr.EditSummary == nil {
		return ""
	}

	es := tr.EditSummary
	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("Edit completed: %s\n", es.GetCompactSummary()))
	summary.WriteString(fmt.Sprintf("Success: %t\n", es.WasSuccessful))

	if es.EditType == "create" {
		summary.WriteString(fmt.Sprintf("- Created new file with %d lines (%d characters)\n",
			es.TotalLines, es.CharactersAdded))
	} else if es.EditType == "modify" {
		summary.WriteString("Changes made:\n")
		if es.LinesAdded > 0 {
			summary.WriteString(fmt.Sprintf("- Added %d lines\n", es.LinesAdded))
		}
		if es.LinesRemoved > 0 {
			summary.WriteString(fmt.Sprintf("- Removed %d lines\n", es.LinesRemoved))
		}
		if es.LinesModified > 0 {
			summary.WriteString(fmt.Sprintf("- Modified %d lines\n", es.LinesModified))
		}

		netLineChange := es.LinesAdded - es.LinesRemoved
		if netLineChange > 0 {
			summary.WriteString(fmt.Sprintf("- Net increase: +%d lines\n", netLineChange))
		} else if netLineChange < 0 {
			summary.WriteString(fmt.Sprintf("- Net decrease: %d lines\n", netLineChange))
		}

		summary.WriteString(fmt.Sprintf("- File now has %d total lines\n", es.TotalLines))
	}

	if es.Summary != "" {
		summary.WriteString(fmt.Sprintf("Description: %s\n", es.Summary))
	}

	return summary.String()
}

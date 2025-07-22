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

// DebugHandler is a function type for handling debug messages
type DebugHandler func(message string)

// Global debug handler - if set, debug messages go here instead of fmt.Printf
var debugHandler DebugHandler

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

// SetDebugHandler sets the global debug message handler
func SetDebugHandler(handler DebugHandler) {
	debugHandler = handler
}

// debugLog sends a debug message either to the handler or fmt.Printf as fallback
func debugLog(message string) {
	if !debugTaskParsing {
		return
	}

	if debugHandler != nil {
		debugHandler(message)
	} else {
		fmt.Printf("DEBUG: %s\n", message)
	}
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
	Diff            string `json:"diff,omitempty"`
	Content         string `json:"content,omitempty"`
	Intent          string `json:"intent,omitempty"`            // Natural language description of what to do
	LoomEditCommand bool   `json:"loom_edit_command,omitempty"` // Flag indicating this contains a LOOM_EDIT command

	// Line-based editing (NEW - more precise than context-based)
	TargetLine        int    `json:"target_line,omitempty"`        // Single line to edit (1-indexed)
	TargetStartLine   int    `json:"target_start_line,omitempty"`  // Start of line range to edit (1-indexed)
	TargetEndLine     int    `json:"target_end_line,omitempty"`    // End of line range to edit (1-indexed)
	ContextValidation string `json:"context_validation,omitempty"` // Optional: expected content for safety validation

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

// EditSummary provides detailed information about file edit operations
type EditSummary struct {
	FilePath           string `json:"file_path"`
	EditType           string `json:"edit_type"` // "create", "modify", "delete"
	LinesAdded         int    `json:"lines_added"`
	LinesRemoved       int    `json:"lines_removed"`
	LinesModified      int    `json:"lines_modified"`
	TotalLines         int    `json:"total_lines"` // Total lines after edit
	CharactersAdded    int    `json:"characters_added"`
	CharactersRemoved  int    `json:"characters_removed"`
	Summary            string `json:"summary"` // Brief description of changes
	WasSuccessful      bool   `json:"was_successful"`
	IsIdenticalContent bool   `json:"is_identical_content"` // True when new content is identical to existing content
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

// tryLoomEditParsing attempts to parse LOOM_EDIT commands from LLM response
func tryLoomEditParsing(llmResponse string) *TaskList {
	debugLog("DEBUG: Attempting LOOM_EDIT parsing...")

	// Look for LOOM_EDIT command blocks - support both >>LOOM_EDIT and 🔧 LOOM_EDIT formats
	re := regexp.MustCompile(`(?s)(?:>>|🔧 )LOOM_EDIT.*?<<LOOM_EDIT`)
	matches := re.FindAllString(llmResponse, -1)

	if len(matches) == 0 {
		return nil
	}

	var tasks []Task

	for _, match := range matches {
		// Parse the LOOM_EDIT command - we'll need to create a basic task
		// that contains the raw LOOM_EDIT command for the executor to handle

		// Extract the file path from the command for the task - handle both formats
		filePathRe := regexp.MustCompile(`file=([^\s]+)`)
		fileMatches := filePathRe.FindStringSubmatch(match)

		if len(fileMatches) < 2 {
			debugLog("DEBUG: Could not extract file path from LOOM_EDIT command")
			continue
		}

		filePath := fileMatches[1]

		// Create a task with the LOOM_EDIT command in the content
		task := &Task{
			Type:            TaskTypeEditFile,
			Path:            filePath,
			Content:         match, // Store the entire LOOM_EDIT command
			LoomEditCommand: true,  // Flag to indicate this is a LOOM_EDIT command
		}

		tasks = append(tasks, *task)
		debugLog(fmt.Sprintf("DEBUG: Parsed LOOM_EDIT task for file: %s\n", filePath))
	}

	if len(tasks) == 0 {
		return nil
	}

	return &TaskList{
		Tasks: tasks,
	}
}

// NOTE: Natural language parsing removed - only LOOM_EDIT and JSON formats are supported

// NOTE: Natural language task parsing functions removed - only LOOM_EDIT and JSON formats are supported

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

// NOTE: Natural language parsing functions removed - only LOOM_EDIT and JSON formats are supported

// NOTE: Natural language parsing functions removed - only LOOM_EDIT and JSON formats are supported

// NOTE: Natural language parsing functions removed - only LOOM_EDIT and JSON formats are supported

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
	// - "\"memory-id\": content" (defaults to create)
	// - "\"memory-id\" content:\"some content\"" (defaults to create)

	// Handle literal \n sequences in the input
	args = strings.ReplaceAll(args, "\\n", "\n")

	// Check for colon-separated format: "memory-id": content
	// This should only match when the format is like '"id": content' or 'id: content'
	// and NOT match 'content:"value"' patterns
	colonIndex := strings.Index(args, ":")
	if colonIndex > 0 {
		// Extract the memory ID part (before colon)
		idPart := strings.TrimSpace(args[:colonIndex])

		// Only use colon format if:
		// 1. The part before colon doesn't contain spaces (except within quotes), OR
		// 2. The part before colon is fully quoted
		isQuotedID := len(idPart) >= 2 && ((idPart[0] == '"' && idPart[len(idPart)-1] == '"') ||
			(idPart[0] == '\'' && idPart[len(idPart)-1] == '\''))

		// Check if this looks like a simple ID followed by colon (not a content: prefix)
		containsSpaceOutsideQuotes := false
		if !isQuotedID {
			// For unquoted strings, check if there are spaces (which would indicate multiple args)
			containsSpaceOutsideQuotes = strings.Contains(idPart, " ")
		}

		// Only treat as colon format if it's a simple quoted ID or unquoted ID without spaces
		if isQuotedID || !containsSpaceOutsideQuotes {
			contentPart := strings.TrimSpace(args[colonIndex+1:])

			// Remove quotes from ID if present
			if isQuotedID {
				idPart = idPart[1 : len(idPart)-1]
			}

			// Clean up content - remove leading/trailing whitespace and normalize newlines
			contentPart = strings.TrimSpace(contentPart)
			// Handle cases where content starts with a newline or dash
			contentPart = strings.TrimLeft(contentPart, "\n\r")
			contentPart = strings.TrimSpace(contentPart)

			// Default to create operation for colon format
			task.MemoryOperation = "create"
			task.MemoryID = idPart
			task.MemoryContent = contentPart
			return task
		}
	}

	parts := parseQuotedArgs(args)
	if len(parts) == 0 {
		return nil
	}

	// Check if first part is a valid operation
	firstPart := strings.ToLower(parts[0])
	validOperations := []string{"create", "update", "delete", "get", "list"}
	isValidOperation := false
	for _, op := range validOperations {
		if firstPart == op {
			isValidOperation = true
			break
		}
	}

	var operation string
	var memoryIDIndex int

	if isValidOperation {
		// First part is the operation
		operation = firstPart
		memoryIDIndex = 1
	} else {
		// First part is the memory ID, default to create operation
		operation = "create"
		memoryIDIndex = 0
	}

	task.MemoryOperation = operation

	// For operations other than list, extract the memory ID
	if operation != "list" && len(parts) > memoryIDIndex {
		memoryID := parts[memoryIDIndex]
		// Remove quotes if present
		if len(memoryID) >= 2 {
			if (strings.HasPrefix(memoryID, "\"") && strings.HasSuffix(memoryID, "\"")) ||
				(strings.HasPrefix(memoryID, "'") && strings.HasSuffix(memoryID, "'")) {
				memoryID = memoryID[1 : len(memoryID)-1]
			}
		}
		task.MemoryID = memoryID
	}

	// Parse additional options
	optionsStartIndex := memoryIDIndex + 1
	if operation == "list" {
		optionsStartIndex = memoryIDIndex
	}
	for i := optionsStartIndex; i < len(parts); i++ {
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
		if len(parts) > optionsStartIndex {
			contentParts := parts[optionsStartIndex:]
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

// isConversationalText checks if a line that matches a command pattern is actually conversational text
func isConversationalText(line, taskType, taskArgs string) bool {
	// Convert to lowercase for pattern matching
	lowerLine := strings.ToLower(line)
	lowerArgs := strings.ToLower(taskArgs)

	// Exclude lines that end with exclamation marks (conversational tone)
	if strings.HasSuffix(strings.TrimSpace(line), "!") {
		return true
	}

	// Exclude lines containing typical completion/status words
	conversationalWords := []string{
		"saved!", "created!", "completed!", "successfully!", "finished!",
		"saved", "created successfully", "completed successfully", "finished successfully",
		"usage of", "performance of", "behavior of", "implementation of",
		"i'll remember", "has been", "will be", "let me", "i can",
	}

	for _, word := range conversationalWords {
		if strings.Contains(lowerLine, word) {
			return true
		}
	}

	// For MEMORY specifically, exclude typical conversational patterns
	if taskType == "MEMORY" {
		memoryConversationalPatterns := []string{
			"saved!", "created", "stored", "remembered", "updated successfully",
			"usage", "allocation", "leak", "consumption", "performance",
		}

		for _, pattern := range memoryConversationalPatterns {
			if strings.Contains(lowerArgs, pattern) {
				return true
			}
		}

		// If the args start with typical conversational words, it's probably not a command
		if strings.HasPrefix(lowerArgs, "saved") ||
			strings.HasPrefix(lowerArgs, "created") ||
			strings.HasPrefix(lowerArgs, "updated") ||
			strings.HasPrefix(lowerArgs, "stored") {
			return true
		}
	}

	// For EDIT specifically, exclude completion messages
	if taskType == "EDIT" {
		editConversationalPatterns := []string{
			"completed", "finished", "successful", "done", "applied",
			"has been updated", "has been modified", "has been changed",
		}

		for _, pattern := range editConversationalPatterns {
			if strings.Contains(lowerArgs, pattern) {
				return true
			}
		}
	}

	// Additional heuristic: if the args contain common sentence patterns, it's conversational
	sentencePatterns := []string{
		"i'll", "i will", "this is", "this has", "the file", "the memory",
		"has been", "will be", "it is", "it has", "we should", "you can",
	}

	for _, pattern := range sentencePatterns {
		if strings.Contains(lowerArgs, pattern) {
			return true
		}
	}

	return false
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

		// Found start of backtick code block
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

		// Found start of triple-quote code block (common LLM format)
		if strings.HasPrefix(line, `"""`) {
			var content []string

			// Collect lines until end of triple-quote block
			for j := i + 1; j < len(lines); j++ {
				blockLine := lines[j]
				if strings.TrimSpace(blockLine) == `"""` {
					// End of triple-quote block found
					if len(content) > 0 {
						return strings.Join(content, "\n")
					}
					return ""
				}
				content = append(content, blockLine)
			}

			// Triple-quote block wasn't closed, but return what we have
			if len(content) > 0 {
				return strings.Join(content, "\n")
			}
		}
	}

	return ""
}

// extractMemoryContent looks for memory content on subsequent lines after a MEMORY command
func extractMemoryContent(lines []string, startIdx int) string {
	if startIdx >= len(lines) {
		return ""
	}

	var content []string

	// Look for content on the next few lines
	for i := startIdx; i < len(lines) && i < startIdx+5; i++ {
		line := lines[i]
		trimmedLine := strings.TrimSpace(line)

		// Stop if we encounter another task directive
		taskPattern := regexp.MustCompile(`^🔧\s+(READ|EDIT|LIST|RUN|SEARCH|MEMORY)\s+`)
		if taskPattern.MatchString(trimmedLine) {
			break
		}

		// Stop if we encounter what looks like another major section
		if strings.HasPrefix(trimmedLine, "#") || strings.HasPrefix(trimmedLine, "---") {
			break
		}

		// Stop if we encounter explanatory text (sentences that explain what will happen)
		if strings.Contains(strings.ToLower(trimmedLine), "this will") ||
			strings.Contains(strings.ToLower(trimmedLine), "this helps") ||
			strings.Contains(strings.ToLower(trimmedLine), "let me") ||
			strings.Contains(strings.ToLower(trimmedLine), "i'll") ||
			strings.Contains(strings.ToLower(trimmedLine), "remember key details") {
			break
		}

		// Skip empty lines at the beginning
		if len(content) == 0 && trimmedLine == "" {
			continue
		}

		// Stop after we encounter an empty line (end of memory content block)
		if len(content) > 0 && trimmedLine == "" {
			break
		}

		// Include this line as memory content
		content = append(content, line)

		// For memory content, we typically want just one meaningful line
		// Stop after we get the first substantive content line
		if len(content) >= 1 && trimmedLine != "" {
			break
		}
	}

	if len(content) > 0 {
		// Join the lines and clean up
		fullContent := strings.Join(content, "\n")
		fullContent = strings.TrimSpace(fullContent)
		return fullContent
	}

	return ""
}

// extractDirectContent looks for content that follows directly after a task command line
// This handles cases where content isn't wrapped in code blocks
func extractDirectContent(lines []string, startIdx int) string {
	if startIdx >= len(lines) {
		return ""
	}

	var content []string
	foundStructuredContent := false

	// Start looking from the line after the task directive
	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines at the beginning
		if len(content) == 0 && trimmedLine == "" {
			continue
		}

		// Stop if we encounter another task directive
		taskPattern := regexp.MustCompile(`^🔧\s+(READ|EDIT|LIST|RUN|SEARCH|MEMORY)\s+`)
		if taskPattern.MatchString(trimmedLine) {
			break
		}

		// Stop if we encounter a code block (this should be handled by extractContentFromCodeBlock)
		if strings.HasPrefix(trimmedLine, "```") || strings.HasPrefix(trimmedLine, `"""`) {
			break
		}

		// Stop if we encounter what looks like another major section or command
		if strings.HasPrefix(trimmedLine, "#") || strings.HasPrefix(trimmedLine, "---") {
			break
		}

		// Check if this line looks like structured content rather than descriptive text
		isStructuredContent := isStructuredContentLine(trimmedLine)

		// If we haven't found structured content yet, and this line looks like descriptive text, skip it
		if !foundStructuredContent && !isStructuredContent {
			// Check if it's a sentence-like description (starts with capital, ends with period, etc.)
			if isDescriptiveText(trimmedLine) {
				continue
			}
		}

		// If this looks like structured content, include it
		if isStructuredContent || foundStructuredContent {
			foundStructuredContent = true
			content = append(content, line)
		}
	}

	if len(content) > 0 && foundStructuredContent {
		// Trim trailing empty lines
		for len(content) > 0 && strings.TrimSpace(content[len(content)-1]) == "" {
			content = content[:len(content)-1]
		}

		if len(content) > 0 {
			fullContent := strings.Join(content, "\n")

			// CRITICAL FIX: If content looks like JSON with a "content" field, extract it
			if extractedFromJSON := extractContentFromJSON(fullContent); extractedFromJSON != "" {
				return extractedFromJSON
			}

			// ADDITIONAL FIX: Handle simple JSON case where content might be just {"content":"..."}
			if strings.HasPrefix(strings.TrimSpace(fullContent), `{"content":"`) && strings.HasSuffix(strings.TrimSpace(fullContent), `"}`) {
				// Try to extract content using a simple approach for this common pattern
				trimmed := strings.TrimSpace(fullContent)
				if contentStart := strings.Index(trimmed, `"content":"`); contentStart != -1 {
					contentStart += len(`"content":"`)
					if contentEnd := strings.LastIndex(trimmed, `"}`); contentEnd != -1 && contentEnd > contentStart {
						extracted := trimmed[contentStart:contentEnd]
						// Unescape common JSON escapes
						extracted = strings.ReplaceAll(extracted, `\"`, `"`)
						extracted = strings.ReplaceAll(extracted, `\\`, `\`)
						return extracted
					}
				}
			}

			return fullContent
		}
	}

	return ""
}

// extractContentFromJSON attempts to parse JSON and extract the "content" field
func extractContentFromJSON(jsonStr string) string {
	// Clean and prepare the JSON string for parsing
	trimmed := strings.TrimSpace(jsonStr)

	// Only try to parse if it looks like it might be JSON
	if !strings.HasPrefix(trimmed, "{") {
		return ""
	}

	// For multiline JSON, we need to be more careful about finding the end
	// Try to find the matching closing brace
	braceCount := 0
	endPos := -1
	inString := false
	escaped := false

	for i, char := range trimmed {
		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if !inString {
			if char == '{' {
				braceCount++
			} else if char == '}' {
				braceCount--
				if braceCount == 0 {
					endPos = i + 1
					break
				}
			}
		}
	}

	// If we found a complete JSON object, extract it
	var jsonToParse string
	if endPos > 0 {
		jsonToParse = trimmed[:endPos]
	} else if strings.HasSuffix(trimmed, "}") {
		// Simple case - use the whole string
		jsonToParse = trimmed
	} else {
		// Not a complete JSON object
		return ""
	}

	// Try to parse as JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonToParse), &data); err != nil {
		// If direct parsing fails, try the whole trimmed string
		if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
			return ""
		}
	}

	// Extract the "content" field if it exists
	if contentValue, ok := data["content"]; ok {
		if contentStr, ok := contentValue.(string); ok {
			return contentStr
		}
	}

	return ""
}

// isStructuredContentLine checks if a line looks like structured content (JSON, code, etc.)
func isStructuredContentLine(line string) bool {
	if line == "" {
		return false
	}

	// JSON/Object indicators
	if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "}") ||
		strings.HasPrefix(line, "[") || strings.HasPrefix(line, "]") ||
		strings.Contains(line, `"name":`) || strings.Contains(line, `"version":`) ||
		strings.Contains(line, `"dependencies":`) {
		return true
	}

	// YAML indicators
	if strings.Contains(line, ": ") && !strings.Contains(line, ". ") {
		// Simple heuristic: if it has a colon but not a period, might be YAML
		return true
	}

	// Code indicators (function definitions, variable assignments, etc.)
	if strings.Contains(line, "function") || strings.Contains(line, "const ") ||
		strings.Contains(line, "let ") || strings.Contains(line, "var ") ||
		strings.Contains(line, "def ") || strings.Contains(line, "class ") ||
		strings.Contains(line, "import ") || strings.Contains(line, "export ") {
		return true
	}

	// HTML/XML indicators
	if strings.HasPrefix(line, "<") && strings.HasSuffix(line, ">") {
		return true
	}

	// Configuration file patterns
	if strings.Contains(line, "=") && !strings.Contains(line, " ") {
		return true
	}

	return false
}

// isDescriptiveText checks if a line looks like descriptive/explanatory text
func isDescriptiveText(line string) bool {
	if line == "" {
		return false
	}

	// Sentence-like patterns
	if strings.HasSuffix(line, ".") || strings.HasSuffix(line, "!") || strings.HasSuffix(line, "?") {
		// Check if it starts with a capital letter or common sentence starters
		if len(line) > 0 && (line[0] >= 'A' && line[0] <= 'Z') {
			return true
		}
		if strings.HasPrefix(line, "This will") || strings.HasPrefix(line, "This is") ||
			strings.HasPrefix(line, "The ") || strings.HasPrefix(line, "It will") {
			return true
		}
	}

	// Common descriptive phrases
	if strings.Contains(line, "will improve") || strings.Contains(line, "will add") ||
		strings.Contains(line, "necessary") || strings.Contains(line, "configuration") {
		return true
	}

	return false
}

// tryFallbackJSONParsing attempts to parse raw JSON tasks when no backtick-wrapped JSON is found
func tryFallbackJSONParsing(llmResponse string) *TaskList {
	// Look for JSON-like patterns that might be tasks
	// This regex looks for JSON objects that contain "type" field with known task types
	taskTypePattern := `\{"type":\s*"(?:ReadFile|EditFile|ListDir|RunShell)"`
	re := regexp.MustCompile(taskTypePattern)

	debugLog("DEBUG: Searching for raw JSON patterns in response...")

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
			debugLog(fmt.Sprintf("DEBUG: Found potential raw JSON task: %s\n", line[:min(len(line), 100)]))
			jsonCandidates = append(jsonCandidates, line)
		}
	}

	// Try to parse each candidate
	for _, jsonStr := range jsonCandidates {
		debugLog(fmt.Sprintf("DEBUG: Attempting to parse raw JSON: %s\n", jsonStr[:min(len(jsonStr), 100)]))

		// Try to parse as single Task object
		var singleTask Task
		if err := json.Unmarshal([]byte(jsonStr), &singleTask); err == nil {
			// Validate the task
			if err := validateTask(&singleTask); err == nil {
				debugLog(fmt.Sprintf("DEBUG: Successfully parsed raw JSON task - Type: %s, Path: %s\n", singleTask.Type, singleTask.Path))

				// Create TaskList with single task
				taskList := TaskList{Tasks: []Task{singleTask}}
				return &taskList
			} else {
				debugLog(fmt.Sprintf("DEBUG: Raw JSON task validation failed: %v\n", err))
			}
		} else {
			debugLog(fmt.Sprintf("DEBUG: Failed to parse raw JSON: %v\n", err))
		}

		// Try to parse as TaskList
		var taskList TaskList
		if err := json.Unmarshal([]byte(jsonStr), &taskList); err == nil {
			if len(taskList.Tasks) > 0 {
				if err := validateTasks(&taskList); err == nil {
					debugLog(fmt.Sprintf("DEBUG: Successfully parsed raw JSON TaskList with %d tasks\n", len(taskList.Tasks)))
					return &taskList
				} else {
					debugLog(fmt.Sprintf("DEBUG: Raw JSON TaskList validation failed: %v\n", err))
				}
			}
		}
	}

	debugLog("DEBUG: No valid raw JSON tasks found")
	return nil
}

// ParseTasks extracts and parses tasks from LLM response (tries LOOM_EDIT first, then natural language, then JSON)
func ParseTasks(llmResponse string) (*TaskList, error) {
	// First, try LOOM_EDIT parsing
	if result := tryLoomEditParsing(llmResponse); result != nil {
		debugLog(fmt.Sprintf("DEBUG: Successfully parsed %d LOOM_EDIT tasks\n", len(result.Tasks)))
		return result, nil
	}

	// Skip natural language parsing - only use LOOM_EDIT format for edits
	// Fall back to JSON parsing only
	if debugTaskParsing {
		debugLog("DEBUG: LOOM_EDIT parsing found no tasks, trying JSON parsing...")
	}

	// Look for JSON code blocks
	re := regexp.MustCompile("(?s)```(?:json)?\n?(.*?)\n?```")
	matches := re.FindAllStringSubmatch(llmResponse, -1)

	if len(matches) > 0 {
		debugLog("DEBUG: Found JSON code blocks, attempting to parse...")
		for _, match := range matches {
			if len(match) > 1 {
				jsonContent := strings.TrimSpace(match[1])
				if result := tryFallbackJSONParsing(jsonContent); result != nil {
					debugLog(fmt.Sprintf("DEBUG: Successfully parsed %d tasks from JSON\n", len(result.Tasks)))
					return result, nil
				}
			}
		}
	}

	// Try parsing the entire response as JSON
	if result := tryFallbackJSONParsing(llmResponse); result != nil {
		debugLog(fmt.Sprintf("DEBUG: Successfully parsed %d tasks from direct JSON\n", len(result.Tasks)))
		return result, nil
	}

	debugLog("DEBUG: No valid tasks found in LLM response")
	return &TaskList{Tasks: []Task{}}, nil
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
	// MAJOR CHANGE: No more confirmations - execute file operations immediately
	return false
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

	// Handle identical content case with clear messaging
	if es.IsIdenticalContent {
		return fmt.Sprintf("File %s already contains the desired content - no changes needed", es.FilePath)
	}

	switch es.EditType {
	case "create":
		return fmt.Sprintf("Created %s (%d lines)", es.FilePath, es.TotalLines)
	case "modify":
		totalChanges := es.LinesAdded + es.LinesRemoved + es.LinesModified
		if totalChanges == 0 {
			return fmt.Sprintf("File %s unchanged - content already matches", es.FilePath)
		}
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

	// Handle identical content case with very clear messaging for LLM
	if es.IsIdenticalContent {
		summary.WriteString(fmt.Sprintf("Edit completed: %s\n", es.GetCompactSummary()))
		summary.WriteString("Success: true\n")
		summary.WriteString("🔍 ANALYSIS: The file already contained the exact content you wanted to write.\n")
		summary.WriteString("📝 RESULT: No changes were made because the content is already correct.\n")
		summary.WriteString("✅ CONCLUSION: Your intended edit is already in effect - the task is complete.\n")
		summary.WriteString(fmt.Sprintf("Description: %s", es.Summary))
		return summary.String()
	}

	summary.WriteString(fmt.Sprintf("Edit completed: %s\n", es.GetCompactSummary()))
	summary.WriteString(fmt.Sprintf("Success: %t\n", es.WasSuccessful))

	if es.EditType == "create" {
		summary.WriteString(fmt.Sprintf("- Created new file with %d lines (%d characters)\n",
			es.TotalLines, es.CharactersAdded))
	} else if es.EditType == "modify" {
		totalChanges := es.LinesAdded + es.LinesRemoved + es.LinesModified
		if totalChanges == 0 {
			summary.WriteString("🔍 ANALYSIS: No changes were needed - file content already matches your intent.\n")
			summary.WriteString("✅ RESULT: The file already contains the desired content.\n")
		} else {
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
		}
	} else if es.EditType == "delete" {
		summary.WriteString(fmt.Sprintf("- Deleted file (%d lines removed)\n", es.LinesRemoved))
	}

	if es.Summary != "" {
		summary.WriteString(fmt.Sprintf("Description: %s", es.Summary))
	}

	return summary.String()
}

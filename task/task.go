package task

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv" // Used for parsing integers in natural language task commands
	"strings"
	"time"
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
	TaskTypeTodo     TaskType = "Todo"   // NEW: Todo list management operations
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
	Diff            string `json:"diff,omitempty"` // DEPRECATED: Only LOOM_EDIT is supported now
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
	Query          string   `json:"query,omitempty"`            // The search pattern/regex
	FileTypes      []string `json:"file_types,omitempty"`       // File type filters (e.g., ["go", "js"])
	ExcludeTypes   []string `json:"exclude_types,omitempty"`    // Exclude file types
	GlobPatterns   []string `json:"glob_patterns,omitempty"`    // Include glob patterns (e.g., ["*.tf"])
	ExcludeGlobs   []string `json:"exclude_globs,omitempty"`    // Exclude glob patterns
	IgnoreCase     bool     `json:"ignore_case,omitempty"`      // Case-insensitive search
	WholeWord      bool     `json:"whole_word,omitempty"`       // Match whole words only
	FixedString    bool     `json:"fixed_string,omitempty"`     // Treat query as literal string, not regex
	ContextBefore  int      `json:"context_before,omitempty"`   // Lines of context before matches
	ContextAfter   int      `json:"context_after,omitempty"`    // Lines of context after matches
	MaxResults     int      `json:"max_results,omitempty"`      // Limit number of results
	FilenamesOnly  bool     `json:"filenames_only,omitempty"`   // Show only filenames with matches
	CountMatches   bool     `json:"count_matches,omitempty"`    // Count matches per file
	SearchHidden   bool     `json:"search_hidden,omitempty"`    // Search hidden files and directories
	UsePCRE2       bool     `json:"use_pcre2,omitempty"`        // Use PCRE2 regex engine for advanced features
	SearchNames    bool     `json:"search_names,omitempty"`     // Also search in filenames (not just content)
	FuzzyMatch     bool     `json:"fuzzy_match,omitempty"`      // Use fuzzy matching for filenames
	CombineResults bool     `json:"combine_results,omitempty"`  // Combine results from content and filename searches
	MaxNameResults int      `json:"max_name_results,omitempty"` // Limit number of filename match results

	// Memory specific (NEW)
	MemoryOperation   string   `json:"memory_operation,omitempty"`   // "create", "update", "delete", "get", "list"
	MemoryID          string   `json:"memory_id,omitempty"`          // Memory identifier
	MemoryContent     string   `json:"memory_content,omitempty"`     // Memory content
	MemoryTags        []string `json:"memory_tags,omitempty"`        // Memory tags
	MemoryActive      *bool    `json:"memory_active,omitempty"`      // Whether memory is active (nil means no change for update)
	MemoryDescription string   `json:"memory_description,omitempty"` // Memory description

	// Todo specific (NEW)
	TodoOperation string   `json:"todo_operation,omitempty"`  // "create", "check", "uncheck", "show", "clear"
	TodoTitles    []string `json:"todo_titles,omitempty"`     // Titles for create operation
	TodoItemOrder int      `json:"todo_item_order,omitempty"` // Item order for check/uncheck operations (1-based)

	// Progressive validation and dry-run support (NEW)
	DryRun                bool `json:"dry_run,omitempty"`                 // If true, validate and preview but don't apply changes
	ProgressiveValidation bool `json:"progressive_validation,omitempty"`  // If true, provide detailed validation feedback
	ValidationStages      bool `json:"validation_stages,omitempty"`       // If true, show each validation stage
	SkipFinalConfirmation bool `json:"skip_final_confirmation,omitempty"` // If true, skip final confirmation for dry-runs
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

	// Enhanced feedback fields
	LinesBefore       int                `json:"lines_before"`       // Lines before edit
	LinesAfter        int                `json:"lines_after"`        // Lines after edit
	FileSizeBefore    int64              `json:"file_size_before"`   // File size before edit in bytes
	FileSizeAfter     int64              `json:"file_size_after"`    // File size after edit in bytes
	DetailedDiff      []LineDiffEntry    `json:"detailed_diff"`      // Line-by-line diff details
	ValidationSummary *ValidationSummary `json:"validation_summary"` // Validation results summary
}

// LineDiffEntry represents a single line change in a diff
type LineDiffEntry struct {
	LineNumber int    `json:"line_number"` // Line number (1-based)
	ChangeType string `json:"change_type"` // "added", "removed", "modified", "unchanged"
	OldContent string `json:"old_content"` // Original line content (empty for additions)
	NewContent string `json:"new_content"` // New line content (empty for deletions)
	Context    string `json:"context"`     // Additional context about the change
}

// ContextualError provides rich context for edit operation errors
type ContextualError struct {
	Type            string        `json:"type"`             // "line_range", "file_not_found", "syntax", "validation", etc.
	Message         string        `json:"message"`          // Human-readable error message
	FilePath        string        `json:"file_path"`        // File being operated on
	FileExists      bool          `json:"file_exists"`      // Whether file exists
	CurrentLines    int           `json:"current_lines"`    // Current number of lines in file
	CurrentSize     int64         `json:"current_size"`     // Current file size in bytes
	RequestedAction string        `json:"requested_action"` // REPLACE, INSERT_AFTER, etc.
	RequestedStart  int           `json:"requested_start"`  // Requested start line
	RequestedEnd    int           `json:"requested_end"`    // Requested end line
	ActualContent   []string      `json:"actual_content"`   // Actual line content around error
	ContextLines    []ContextLine `json:"context_lines"`    // Context around error with line numbers
	Suggestions     []string      `json:"suggestions"`      // Suggested corrections
	RequiredActions []string      `json:"required_actions"` // What the LLM should do next
	PreventionTips  []string      `json:"prevention_tips"`  // How to prevent this error
}

// ContextLine represents a line with its number and content
type ContextLine struct {
	LineNumber int    `json:"line_number"` // 1-based line number
	Content    string `json:"content"`     // Line content
	IsTarget   bool   `json:"is_target"`   // Whether this is the target line for the operation
	IsError    bool   `json:"is_error"`    // Whether this line has an error
}

// ValidationSummary provides a summary of validation results for LLM feedback
type ValidationSummary struct {
	IsValid           bool     `json:"is_valid"`           // Overall validation status
	ErrorCount        int      `json:"error_count"`        // Number of errors
	WarningCount      int      `json:"warning_count"`      // Number of warnings
	HintCount         int      `json:"hint_count"`         // Number of hints
	CriticalErrors    []string `json:"critical_errors"`    // Critical error messages
	ValidatorUsed     string   `json:"validator_used"`     // Which validator was used
	ProcessTimeMs     int64    `json:"process_time_ms"`    // Validation processing time in milliseconds
	RollbackTriggered bool     `json:"rollback_triggered"` // Whether rollback was triggered
}

// ValidationStage represents a single stage in progressive validation
type ValidationStage struct {
	Name        string   `json:"name"`        // Stage name (syntax, file_state, line_range, content, preview)
	Status      string   `json:"status"`      // "pending", "passed", "failed", "warning"
	Message     string   `json:"message"`     // Stage result message
	Details     string   `json:"details"`     // Additional details
	Suggestions []string `json:"suggestions"` // Stage-specific suggestions
	Duration    int64    `json:"duration_ms"` // Time taken for this stage in milliseconds
}

// ProgressiveValidationResult contains the results of all validation stages
type ProgressiveValidationResult struct {
	OverallStatus   string            `json:"overall_status"`    // "passed", "failed", "warnings"
	CurrentStage    string            `json:"current_stage"`     // Current or failed stage
	Stages          []ValidationStage `json:"stages"`            // All validation stages
	DryRunPreview   *DryRunPreview    `json:"dry_run_preview"`   // Preview of changes (if requested)
	ActionAnalysis  *ActionAnalysis   `json:"action_analysis"`   // Smart action selection analysis
	CanProceed      bool              `json:"can_proceed"`       // Whether edit can proceed
	TotalDurationMs int64             `json:"total_duration_ms"` // Total validation time
	FailureStage    string            `json:"failure_stage"`     // Stage where validation failed
	ValidationCount int               `json:"validation_count"`  // Number of stages completed
}

// DryRunPreview shows what changes would be made without applying them
type DryRunPreview struct {
	FilePath           string        `json:"file_path"`           // Target file path
	FileExists         bool          `json:"file_exists"`         // Whether file currently exists
	CurrentLines       int           `json:"current_lines"`       // Current line count
	CurrentSize        int64         `json:"current_size"`        // Current file size
	ExpectedLines      int           `json:"expected_lines"`      // Expected line count after edit
	ExpectedSize       int64         `json:"expected_size"`       // Expected file size after edit
	LineDelta          int           `json:"line_delta"`          // Change in line count (+/-)
	SizeDelta          int64         `json:"size_delta"`          // Change in size (+/-)
	PreviewLines       []PreviewLine `json:"preview_lines"`       // Lines that will be affected
	ChangesSummary     string        `json:"changes_summary"`     // Summary of changes
	SafetyWarnings     []string      `json:"safety_warnings"`     // Any safety concerns
	RecommendedActions []string      `json:"recommended_actions"` // Recommended next steps
}

// EditIntent represents what the LLM is trying to accomplish with an edit
type EditIntent struct {
	IntentType    string  `json:"intent_type"`    // "text_substitution", "line_modification", "content_insertion", "content_deletion", "file_creation"
	TargetScope   string  `json:"target_scope"`   // "specific_text", "line_range", "end_of_file", "beginning_of_file", "whole_file"
	ContentType   string  `json:"content_type"`   // "code", "text", "configuration", "documentation"
	ChangeNature  string  `json:"change_nature"`  // "simple_replace", "complex_modification", "addition", "removal"
	SearchPattern string  `json:"search_pattern"` // Extracted search pattern (if applicable)
	Confidence    float64 `json:"confidence"`     // Confidence in intent analysis (0.0-1.0)
}

// ActionSuggestion represents an alternative action recommendation
type ActionSuggestion struct {
	SuggestedAction string   `json:"suggested_action"` // Recommended LOOM_EDIT action
	Reasoning       string   `json:"reasoning"`        // Why this action is better
	Benefits        []string `json:"benefits"`         // Benefits of using this action
	ExampleUsage    string   `json:"example_usage"`    // Example of how to use this action
	ConfidenceScore float64  `json:"confidence_score"` // Confidence in this suggestion (0.0-1.0)
	EfficiencyGain  string   `json:"efficiency_gain"`  // How much more efficient this would be
}

// ActionAnalysis contains analysis of action choice and recommendations
type ActionAnalysis struct {
	CurrentAction    string             `json:"current_action"`    // The action the LLM chose
	IsOptimal        bool               `json:"is_optimal"`        // Whether the current action is optimal
	AnalysisType     string             `json:"analysis_type"`     // "optimal", "suboptimal", "inefficient", "problematic"
	EditIntent       EditIntent         `json:"edit_intent"`       // Analyzed intent
	Suggestions      []ActionSuggestion `json:"suggestions"`       // Alternative action suggestions
	OptimizationTips []string           `json:"optimization_tips"` // General tips for better action selection
	PatternMatches   []string           `json:"pattern_matches"`   // Recognized patterns in the edit
	ContextWarnings  []string           `json:"context_warnings"`  // Context-specific warnings
}

// PreviewLine represents a line in the dry-run preview
type PreviewLine struct {
	LineNumber int    `json:"line_number"` // 1-based line number (0 for new lines)
	ChangeType string `json:"change_type"` // "unchanged", "modified", "added", "deleted"
	OldContent string `json:"old_content"` // Original content (empty for additions)
	NewContent string `json:"new_content"` // New content (empty for deletions)
	IsTarget   bool   `json:"is_target"`   // Whether this line is the target of the operation
}

// TaskResponse represents the result of executing a task
type TaskResponse struct {
	Task                  Task                         `json:"task"`
	Success               bool                         `json:"success"`
	Output                string                       `json:"output,omitempty"`         // Display message for user
	ActualContent         string                       `json:"actual_content,omitempty"` // Actual content for LLM (hidden from user)
	EditSummary           *EditSummary                 `json:"edit_summary,omitempty"`   // Detailed edit information (for EditFile tasks)
	Error                 string                       `json:"error,omitempty"`
	ContextualError       *ContextualError             `json:"contextual_error,omitempty"`       // Rich error context for LLM
	ProgressiveValidation *ProgressiveValidationResult `json:"progressive_validation,omitempty"` // Progressive validation results
	Approved              bool                         `json:"approved,omitempty"`               // For tasks requiring confirmation
	VerificationText      string                       `json:"verification_text,omitempty"`      // Enhanced verification text for LLM
}

// UserTaskEvent represents simplified task status for user interface
// This event type contains minimal information for user display, hiding
// technical details like outputs, errors, and validation results
type UserTaskEvent struct {
	TaskID      string    `json:"taskId"`
	Type        string    `json:"type"`               // "started", "completed", "failed", "progress"
	Message     string    `json:"message"`            // Simple user-friendly message
	TaskType    string    `json:"taskType"`           // "read_file", "edit_file", "run_shell", etc.
	Description string    `json:"description"`        // Brief task description
	Progress    float64   `json:"progress,omitempty"` // 0.0 to 1.0
	Timestamp   time.Time `json:"timestamp"`
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

// tryNaturalLanguageParsing attempts to parse natural language task commands
func tryNaturalLanguageParsing(llmResponse string) *TaskList {
	debugLog("DEBUG: Attempting natural language task parsing...")

	// Preprocess escaped content - handle cases where LLM sends escaped newlines and quotes
	processedResponse := strings.ReplaceAll(llmResponse, "\\n", "\n")
	processedResponse = strings.ReplaceAll(processedResponse, "\\\"", "\"")

	lines := strings.Split(processedResponse, "\n")
	var tasks []Task
	// Use map for O(1) duplicate detection instead of O(n) linear search
	seenTasks := make(map[string]bool)

	// Look for task indicators with emoji prefixes
	// Updated to match both "🔧 READ" and "📖 READ" patterns
	taskPattern := regexp.MustCompile(`^(?:🔧|📖|📂|✏️|🔍|💾|📝)\s+(READ|LIST|RUN|SEARCH|MEMORY|TODO)\s+(.+)`)

	for i, line := range lines {
		line = strings.TrimSpace(line)
		matches := taskPattern.FindStringSubmatch(line)

		if len(matches) == 3 {
			taskType := strings.ToUpper(matches[1])
			taskArgs := strings.TrimSpace(matches[2])

			task := parseNaturalLanguageTask(taskType, taskArgs)
			if task != nil {
				// For EDIT tasks, look for content in subsequent code blocks or direct content
				if task.Type == TaskTypeEditFile && task.Content == "" {
					if content := extractContentFromCodeBlock(lines, i+1); content != "" {
						task.Content = content
						debugLog("DEBUG: Found content in code block for edit task")
					} else if content := extractDirectContent(lines, i+1); content != "" {
						task.Content = content
						debugLog("DEBUG: Found direct content for edit task")
					}
				}

				// For MEMORY tasks, look for content on subsequent lines if not already set
				if task.Type == TaskTypeMemory && task.MemoryContent == "" {
					if content := extractMemoryContent(lines, i+1); content != "" {
						task.MemoryContent = content
						debugLog("DEBUG: Found memory content on subsequent lines")
					}
				}

				// Create unique key for duplicate detection
				taskKey := fmt.Sprintf("%s:%s", task.Type, task.Path)
				if !seenTasks[taskKey] {
					seenTasks[taskKey] = true
					tasks = append(tasks, *task)
					debugLog(fmt.Sprintf("DEBUG: Parsed natural language task - Type: %s, Path: %s\n", task.Type, task.Path))
				}
			}
		}
	}

	// Also look for simpler patterns without emoji, but be more restrictive to avoid conversational text
	simplePattern := regexp.MustCompile(`(?i)^(read|edit|list|run|search|memory)\s+(.+)`)

	for i, line := range lines {
		line = strings.TrimSpace(line)
		matches := simplePattern.FindStringSubmatch(line)

		if len(matches) == 3 {
			taskType := strings.ToUpper(matches[1])
			taskArgs := strings.TrimSpace(matches[2])

			// Skip lines that look like conversational text rather than commands
			if isConversationalText(line, taskType, taskArgs) {
				continue
			}

			task := parseNaturalLanguageTask(taskType, taskArgs)
			if task != nil {
				// Create unique key for duplicate detection
				taskKey := fmt.Sprintf("%s:%s", task.Type, task.Path)
				if !seenTasks[taskKey] {
					// For EDIT tasks, look for content in subsequent code blocks or direct content
					if task.Type == TaskTypeEditFile && task.Content == "" {
						if content := extractContentFromCodeBlock(lines, i+1); content != "" {
							task.Content = content
							debugLog("DEBUG: Found content in code block for simple edit task")
						} else if content := extractDirectContent(lines, i+1); content != "" {
							task.Content = content
							debugLog("DEBUG: Found direct content for simple edit task")
						}
					}

					// For MEMORY tasks, look for content on subsequent lines if not already set
					if task.Type == TaskTypeMemory && task.MemoryContent == "" {
						if content := extractMemoryContent(lines, i+1); content != "" {
							task.MemoryContent = content
							debugLog("DEBUG: Found memory content on subsequent lines (simple pattern)")
						}
					}

					seenTasks[taskKey] = true
					tasks = append(tasks, *task)
					debugLog(fmt.Sprintf("DEBUG: Parsed simple natural language task - Type: %s, Path: %s\n", task.Type, task.Path))
				}
			}
		}
	}

	if len(tasks) == 0 {
		debugLog("DEBUG: No natural language tasks found")
		return nil
	}

	debugLog(fmt.Sprintf("DEBUG: Successfully parsed %d natural language tasks\n", len(tasks)))

	return &TaskList{Tasks: tasks}
}

// parseNaturalLanguageTask parses a single natural language task
func parseNaturalLanguageTask(taskType, args string) *Task {
	switch taskType {
	case "READ":
		return parseReadTask(args)
	case "EDIT":
		// EDIT commands are still supported for backward compatibility with tests
		// but we'll output a warning that LOOM_EDIT should be used instead
		debugLog("DEBUG: Natural language EDIT command is deprecated - use LOOM_EDIT format instead")
		// For backward compatibility, we'll still parse EDIT commands for tests
		return parseEditTask(args)
	case "LIST":
		return parseListTask(args)
	case "RUN":
		return parseRunTask(args)
	case "SEARCH":
		return parseSearchTask(args)
	case "MEMORY":
		return parseMemoryTask(args)
	case "TODO":
		return parseTodoTask(args)
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
	// - "main.go (lines: 1-200)"

	// Look for parenthetical options
	optionsPattern := regexp.MustCompile(`^(.+?)\s*\((.+)\)$`)
	matches := optionsPattern.FindStringSubmatch(args)

	if len(matches) == 3 {
		task.Path = strings.TrimSpace(matches[1])
		options := strings.TrimSpace(matches[2])

		// Parse options
		if strings.Contains(options, "max:") || strings.Contains(options, "max ") {
			maxPattern := regexp.MustCompile(`max[:]*\s*(\d+)`)
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

		// Parse line ranges like "lines 50-100" or "lines: 50-100"
		rangePattern := regexp.MustCompile(`lines[:\s]*\s*(\d+)-(\d+)`)
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

	// Enhanced pattern: "search and replace X with Y" or "search_replace X with Y"
	searchReplacePattern := regexp.MustCompile(`(?i)(?:search(?:[\s_-]*)replace|search\s+and\s+replace)\s+["']?([^"']+?)["']?\s+with\s+["']?([^"']+?)["']?$`)
	if matches := searchReplacePattern.FindStringSubmatch(description); len(matches) > 2 {
		task.StartContext = strings.TrimSpace(matches[1]) // What to find
		task.EndContext = strings.TrimSpace(matches[2])   // What to replace with
		task.InsertMode = "search_replace"
		return
	}

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

		// New options
		case part == "names" || part == "filenames" || part == "-n":
			task.SearchNames = true

		case part == "fuzzy" || part == "fuzzy-match":
			task.FuzzyMatch = true
			task.SearchNames = true // Fuzzy matching implies searching filenames

		case part == "combine" || part == "combined":
			task.CombineResults = true
			task.SearchNames = true // Combining results implies searching both

		}
	}

	// Set defaults
	if task.MaxResults == 0 {
		task.MaxResults = 100 // Default limit to prevent overwhelming output
	}

	// Set default max name results if searching filenames
	if task.SearchNames && task.MaxNameResults == 0 {
		task.MaxNameResults = 50 // Default limit for filename results
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

// parseTodoTask parses natural language TODO commands
func parseTodoTask(args string) *Task {
	task := &Task{Type: TaskTypeTodo}

	// Parse todo operation and options
	// Examples:
	// - "create \"Read main.go file\" \"Identify the bug\" \"Fix the bug\""
	// - "check 1"
	// - "uncheck 2"
	// - "show"
	// - "clear"

	parts := parseQuotedArgs(args)
	if len(parts) == 0 {
		return nil
	}

	// First part is the operation
	operation := strings.ToLower(parts[0])
	task.TodoOperation = operation

	switch operation {
	case "create":
		// Extract todo titles (minimum 2, maximum 10)
		if len(parts) < 3 { // operation + at least 2 titles
			debugLog("DEBUG: todo create requires at least 2 titles")
			return nil
		}
		if len(parts) > 11 { // operation + at most 10 titles
			debugLog("DEBUG: todo create supports at most 10 titles")
			return nil
		}

		// Extract titles (remove quotes if present)
		var titles []string
		for i := 1; i < len(parts); i++ {
			title := parts[i]
			// Remove quotes if present
			if len(title) >= 2 {
				if (strings.HasPrefix(title, "\"") && strings.HasSuffix(title, "\"")) ||
					(strings.HasPrefix(title, "'") && strings.HasSuffix(title, "'")) {
					title = title[1 : len(title)-1]
				}
			}
			if strings.TrimSpace(title) != "" {
				titles = append(titles, strings.TrimSpace(title))
			}
		}

		if len(titles) < 2 {
			debugLog("DEBUG: todo create requires at least 2 non-empty titles")
			return nil
		}

		task.TodoTitles = titles

	case "check", "uncheck":
		// Extract item order
		if len(parts) < 2 {
			debugLog(fmt.Sprintf("DEBUG: todo %s requires item number", operation))
			return nil
		}

		itemOrder, err := strconv.Atoi(parts[1])
		if err != nil {
			debugLog(fmt.Sprintf("DEBUG: invalid item number for todo %s: %s", operation, parts[1]))
			return nil
		}

		if itemOrder < 1 {
			debugLog(fmt.Sprintf("DEBUG: item number must be >= 1, got: %d", itemOrder))
			return nil
		}

		task.TodoItemOrder = itemOrder

	case "show", "clear":
		// No additional parameters needed

	default:
		debugLog(fmt.Sprintf("DEBUG: unknown todo operation: %s", operation))
		return nil
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

// Handle single line: "15"

// Handle range: "15-17"

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
		taskPattern := regexp.MustCompile(`^🔧\s+(READ|LIST|RUN|SEARCH|MEMORY)\s+`)
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
		taskPattern := regexp.MustCompile(`^🔧\s+(READ|LIST|RUN|SEARCH|MEMORY)\s+`)
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

	// Second, try natural language parsing
	if result := tryNaturalLanguageParsing(llmResponse); result != nil {
		debugLog(fmt.Sprintf("DEBUG: Successfully parsed %d tasks using natural language parsing\n", len(result.Tasks)))
		return result, nil
	}

	// Fall back to JSON parsing
	if debugTaskParsing {
		debugLog("DEBUG: Natural language parsing found no tasks, trying JSON parsing...")
	}

	// Look for JSON code blocks
	re := regexp.MustCompile("(?s)```(?:json)?\n?(.*?)\n?```")
	matches := re.FindAllStringSubmatch(llmResponse, -1)

	if len(matches) == 0 {
		// FALLBACK: Try to parse raw JSON if no backtick-wrapped JSON found
		if debugTaskParsing {
			debugLog("DEBUG: No backtick-wrapped JSON found, trying fallback raw JSON parsing...")
		}

		// Try fallback parsing for raw JSON
		if result := tryFallbackJSONParsing(llmResponse); result != nil {
			debugLog(fmt.Sprintf("DEBUG: Successfully parsed %d tasks using fallback raw JSON parsing\n", len(result.Tasks)))
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
				debugLog("🚨 LLM response suggests action but no JSON tasks found!")
				debugLog(fmt.Sprintf("Found action indicators: %v", foundActions))
				debugLog(fmt.Sprintf("Response (first 200 chars): %s...", llmResponse[:min(len(llmResponse), 200)]))
				debugLog("✅ Expected: JSON code block like:")
				debugLog("```json")
				debugLog(`{"type": "ReadFile", "path": "README.md", "max_lines": 100}`)
				debugLog("```")
				debugLog("📝 OR for multiple tasks:")
				debugLog("```json")
				debugLog("{")
				debugLog(`  "tasks": [`)
				debugLog(`    {"type": "ReadFile", "path": "README.md", "max_lines": 100}`)
				debugLog("  ]")
				debugLog("}")
				debugLog("```")
				debugLog("💡 The LLM may need better prompting to output actual task JSON.")
			} else {
				debugLog("✅ No JSON tasks found - this appears to be a regular Q&A response.")
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
			debugLog(fmt.Sprintf("DEBUG: Attempting to parse JSON task block: %s\n", jsonStr[:min(len(jsonStr), 100)]))
		}

		// Try to parse as TaskList first
		var taskList TaskList
		if err := json.Unmarshal([]byte(jsonStr), &taskList); err == nil {
			if debugTaskParsing {
				debugLog(fmt.Sprintf("DEBUG: Parsed as TaskList with %d tasks\n", len(taskList.Tasks)))
			}

			// Only proceed with TaskList if it actually has tasks
			if len(taskList.Tasks) > 0 {
				// Successfully parsed as TaskList
				if err := validateTasks(&taskList); err != nil {
					return nil, fmt.Errorf("invalid tasks: %w", err)
				}

				if debugTaskParsing {
					debugLog(fmt.Sprintf("DEBUG: Successfully parsed %d tasks (as TaskList)\n", len(taskList.Tasks)))
				}
				return &taskList, nil
			}

			if debugTaskParsing {
				debugLog("DEBUG: TaskList was empty, trying single task parsing\n")
			}
		}

		// Try to parse as single Task object
		var singleTask Task
		if err := json.Unmarshal([]byte(jsonStr), &singleTask); err != nil {
			if debugTaskParsing {
				debugLog(fmt.Sprintf("DEBUG: Failed to parse JSON as either TaskList or single Task: %v\n", err))
			}
			continue // Skip invalid JSON blocks
		}

		if debugTaskParsing {
			debugLog(fmt.Sprintf("DEBUG: Parsed single task - Type: %s, Path: %s\n", singleTask.Type, singleTask.Path))
		}

		// Create TaskList with single task
		taskList = TaskList{Tasks: []Task{singleTask}}

		if debugTaskParsing {
			debugLog(fmt.Sprintf("DEBUG: Created TaskList with %d tasks\n", len(taskList.Tasks)))
		}

		// Validate tasks
		if err := validateTasks(&taskList); err != nil {
			if debugTaskParsing {
				debugLog(fmt.Sprintf("DEBUG: Task validation failed: %v\n", err))
			}
			return nil, fmt.Errorf("invalid tasks: %w", err)
		}

		if debugTaskParsing {
			debugLog("DEBUG: Successfully parsed 1 task (as single Task object)\n")
		}
		return &taskList, nil
	}

	return nil, nil
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
			return fmt.Errorf("editFile requires path")
		}
		if task.Content == "" {
			return fmt.Errorf("editFile requires content")
		}

		// For existing files, enforce LOOM_EDIT format
		// Only allow non-LOOM_EDIT format for new file creation
		if !task.LoomEditCommand {
			// Check if this is a new file (if the path doesn't exist)
			fullPath := filepath.Join(os.Getenv("LOOM_WORKSPACE"), task.Path)
			if _, err := os.Stat(fullPath); err == nil {
				// File exists, require LOOM_EDIT
				return fmt.Errorf("EditFile tasks for existing files must use the LOOM_EDIT format. Natural language edit commands are no longer supported")
			}
		}

	case TaskTypeListDir:
		if task.Path == "" {
			task.Path = "." // Default to current directory
		}

	case TaskTypeRunShell:
		if task.Command == "" {
			return fmt.Errorf("runShell requires command")
		}
		if task.Timeout <= 0 {
			task.Timeout = 3 // Default 3 second timeout
		}

	case TaskTypeSearch:
		if task.Query == "" {
			return fmt.Errorf("search requires query")
		}
		if task.Path == "" {
			task.Path = "." // Default to current directory
		}
		if task.MaxResults <= 0 {
			task.MaxResults = 100 // Default limit
		}
		// Validate new options
		if task.FuzzyMatch && !task.SearchNames {
			task.SearchNames = true // Fuzzy matching implies filename search
		}
		if task.MaxNameResults <= 0 && task.SearchNames {
			task.MaxNameResults = 50 // Default limit for filename results
		}

	case TaskTypeMemory:
		if task.MemoryOperation == "" {
			return fmt.Errorf("memory requires operation (create, update, delete, get, list)")
		}

		operation := strings.ToLower(task.MemoryOperation)
		switch operation {
		case "create":
			if task.MemoryID == "" {
				return fmt.Errorf("memory create requires ID")
			}
			if task.MemoryContent == "" {
				return fmt.Errorf("memory create requires content")
			}
		case "update":
			if task.MemoryID == "" {
				return fmt.Errorf("memory update requires ID")
			}
			// For updates, at least one field must be provided
			if task.MemoryContent == "" && len(task.MemoryTags) == 0 &&
				task.MemoryActive == nil && task.MemoryDescription == "" {
				return fmt.Errorf("memory update requires at least one field to update (content, tags, active, description)")
			}
		case "delete", "get":
			if task.MemoryID == "" {
				return fmt.Errorf("memory %s requires ID", operation)
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
			return fmt.Sprintf("📖 Read %s (lines %d-%d)", t.Path, t.StartLine, t.EndLine)
		} else if t.StartLine > 0 {
			if t.MaxLines > 0 {
				return fmt.Sprintf("📖 Read %s (from line %d, max %d lines)", t.Path, t.StartLine, t.MaxLines)
			}
			return fmt.Sprintf("📖 Read %s (from line %d)", t.Path, t.StartLine)
		} else if t.MaxLines > 0 {
			return fmt.Sprintf("📖 Read %s (max %d lines)", t.Path, t.MaxLines)
		}
		return fmt.Sprintf("📖 Read %s", t.Path)

	case TaskTypeEditFile:
		if t.LoomEditCommand {
			return fmt.Sprintf("✏️ Edit %s (LOOM_EDIT format)", t.Path)
		}
		// For backward compatibility with existing file creation
		if t.Content != "" && !t.LoomEditCommand {
			return fmt.Sprintf("✏️ Edit %s (create/replace content)", t.Path)
		}
		return fmt.Sprintf("✏️ Edit %s", t.Path)

	case TaskTypeListDir:
		if t.Recursive {
			return fmt.Sprintf("📂 List directory %s (recursive)", t.Path)
		}
		return fmt.Sprintf("📂 List directory %s", t.Path)

	case TaskTypeRunShell:
		return fmt.Sprintf("🔧 Run command: %s", t.Command)

	case TaskTypeSearch:
		description := fmt.Sprintf("🔍 Search for '%s'", t.Query)
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
		if t.SearchNames {
			if t.FuzzyMatch {
				description += " (including fuzzy filename matches)"
			} else {
				description += " (including filename matches)"
			}
		}
		return description

	case TaskTypeMemory:
		switch strings.ToLower(t.MemoryOperation) {
		case "create":
			return fmt.Sprintf("💾 Create memory '%s'", t.MemoryID)
		case "update":
			return fmt.Sprintf("💾 Update memory '%s'", t.MemoryID)
		case "delete":
			return fmt.Sprintf("💾 Delete memory '%s'", t.MemoryID)
		case "get":
			return fmt.Sprintf("💾 Get memory '%s'", t.MemoryID)
		case "list":
			if t.MemoryActive != nil {
				if *t.MemoryActive {
					return "💾 List active memories"
				} else {
					return "💾 List inactive memories"
				}
			}
			return "💾 List all memories"
		default:
			return fmt.Sprintf("💾 Memory operation '%s' on '%s'", t.MemoryOperation, t.MemoryID)
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

// GetLLMSummary returns a detailed summary formatted for LLM consumption with enhanced feedback
func (tr *TaskResponse) GetLLMSummary() string {
	// Priority 1: Progressive validation results (includes dry-run previews)
	if tr.ProgressiveValidation != nil {
		return tr.ProgressiveValidation.FormatForLLM()
	}

	// Priority 2: Contextual errors
	if tr.ContextualError != nil {
		return tr.ContextualError.FormatForLLM()
	}

	// Priority 3: Standard edit summary
	if tr.EditSummary == nil {
		return ""
	}

	es := tr.EditSummary
	var summary strings.Builder

	// Header with edit status
	summary.WriteString("🔧 EDIT OPERATION COMPLETE\n")
	summary.WriteString("=" + strings.Repeat("=", 40) + "\n\n")

	// Handle identical content case with very clear messaging for LLM
	if es.IsIdenticalContent {
		summary.WriteString(fmt.Sprintf("📁 File: %s\n", es.FilePath))
		summary.WriteString("✅ Status: SUCCESS (No changes needed)\n")
		summary.WriteString("🔍 ANALYSIS: The file already contained the exact content you wanted to write.\n")
		summary.WriteString("📝 RESULT: No changes were made because the content is already correct.\n")
		summary.WriteString("✅ CONCLUSION: Your intended edit is already in effect - the task is complete.\n\n")

		// File state information
		summary.WriteString("📊 FILE STATE:\n")
		summary.WriteString(fmt.Sprintf("- Lines: %d (unchanged)\n", es.TotalLines))
		summary.WriteString(fmt.Sprintf("- Size: %d bytes (unchanged)\n", es.FileSizeAfter))

		if es.ValidationSummary != nil {
			summary.WriteString(tr.formatValidationSummary(es.ValidationSummary))
		}

		summary.WriteString(fmt.Sprintf("\n📋 Description: %s\n", es.Summary))
		return summary.String()
	}

	// Main edit result summary
	summary.WriteString(fmt.Sprintf("📁 File: %s\n", es.FilePath))
	summary.WriteString(fmt.Sprintf("✅ Status: %s\n", map[bool]string{true: "SUCCESS", false: "FAILED"}[es.WasSuccessful]))
	summary.WriteString(fmt.Sprintf("🔄 Operation: %s\n\n", strings.ToUpper(es.EditType)))

	// Before/After comparison
	summary.WriteString("📊 BEFORE/AFTER COMPARISON:\n")
	summary.WriteString(fmt.Sprintf("- Lines: %d → %d (%+d)\n", es.LinesBefore, es.LinesAfter, es.LinesAfter-es.LinesBefore))
	summary.WriteString(fmt.Sprintf("- Size: %d → %d bytes (%+d)\n", es.FileSizeBefore, es.FileSizeAfter, es.FileSizeAfter-es.FileSizeBefore))

	// Detailed change breakdown
	summary.WriteString("\n📝 CHANGE BREAKDOWN:\n")
	if es.EditType == "create" {
		summary.WriteString(fmt.Sprintf("- Created new file with %d lines (%d characters)\n", es.TotalLines, es.CharactersAdded))
	} else if es.EditType == "modify" {
		totalChanges := es.LinesAdded + es.LinesRemoved + es.LinesModified
		if totalChanges == 0 {
			summary.WriteString("- No line changes detected (content identical)\n")
		} else {
			if es.LinesAdded > 0 {
				summary.WriteString(fmt.Sprintf("- Lines added: %d\n", es.LinesAdded))
			}
			if es.LinesRemoved > 0 {
				summary.WriteString(fmt.Sprintf("- Lines removed: %d\n", es.LinesRemoved))
			}
			if es.LinesModified > 0 {
				summary.WriteString(fmt.Sprintf("- Lines modified: %d\n", es.LinesModified))
			}
		}
	} else if es.EditType == "delete" {
		summary.WriteString(fmt.Sprintf("- Deleted entire file (%d lines removed)\n", es.LinesRemoved))
	}

	// Line-by-line diff details
	if len(es.DetailedDiff) > 0 {
		summary.WriteString("\n📋 LINE-BY-LINE CHANGES:\n")
		changeCount := 0
		for _, diff := range es.DetailedDiff {
			if diff.ChangeType != "unchanged" && changeCount < 10 { // Limit to first 10 changes for readability
				switch diff.ChangeType {
				case "added":
					summary.WriteString(fmt.Sprintf("+ Line %d: %s\n", diff.LineNumber, diff.NewContent))
				case "removed":
					summary.WriteString(fmt.Sprintf("- Line %d: %s\n", diff.LineNumber, diff.OldContent))
				case "modified":
					summary.WriteString(fmt.Sprintf("~ Line %d: %s → %s\n", diff.LineNumber, diff.OldContent, diff.NewContent))
				case "summary":
					summary.WriteString(fmt.Sprintf("  %s\n", diff.Context))
				}
				changeCount++
			}
		}
		if len(es.DetailedDiff) > 10 {
			summary.WriteString(fmt.Sprintf("  ... and %d more changes\n", len(es.DetailedDiff)-10))
		}
	}

	// Validation results
	if es.ValidationSummary != nil {
		summary.WriteString(tr.formatValidationSummary(es.ValidationSummary))
	}

	// Summary description
	if es.Summary != "" {
		summary.WriteString(fmt.Sprintf("\n📋 Summary: %s\n", es.Summary))
	}

	return summary.String()
}

// formatValidationSummary formats validation results for LLM feedback
func (tr *TaskResponse) formatValidationSummary(vs *ValidationSummary) string {
	var summary strings.Builder

	summary.WriteString("\n🔍 VALIDATION RESULTS:\n")

	if vs.IsValid {
		summary.WriteString("✅ Status: VALID - No syntax errors detected\n")
	} else {
		summary.WriteString("⚠️  Status: ISSUES DETECTED\n")
	}

	summary.WriteString(fmt.Sprintf("- Validator: %s (took %dms)\n", vs.ValidatorUsed, vs.ProcessTimeMs))

	if vs.ErrorCount > 0 {
		summary.WriteString(fmt.Sprintf("- ❌ Errors: %d\n", vs.ErrorCount))
		for i, err := range vs.CriticalErrors {
			if i < 3 { // Show first 3 errors
				summary.WriteString(fmt.Sprintf("  • %s\n", err))
			}
		}
		if len(vs.CriticalErrors) > 3 {
			summary.WriteString(fmt.Sprintf("  • ... and %d more errors\n", len(vs.CriticalErrors)-3))
		}
	}

	if vs.WarningCount > 0 {
		summary.WriteString(fmt.Sprintf("- ⚠️  Warnings: %d\n", vs.WarningCount))
	}

	if vs.HintCount > 0 {
		summary.WriteString(fmt.Sprintf("- 💡 Hints: %d\n", vs.HintCount))
	}

	if vs.RollbackTriggered {
		summary.WriteString("🔄 Action: Edit was rolled back due to critical errors\n")
	}

	return summary.String()
}

// NewContextualError creates a contextual error with file state information
func NewContextualError(errorType, message, filePath string) *ContextualError {
	return &ContextualError{
		Type:            errorType,
		Message:         message,
		FilePath:        filePath,
		Suggestions:     []string{},
		RequiredActions: []string{},
		PreventionTips:  []string{},
		ContextLines:    []ContextLine{},
		ActualContent:   []string{},
	}
}

// SetFileContext adds file state context to the error
func (ce *ContextualError) SetFileContext(exists bool, lines int, size int64) *ContextualError {
	ce.FileExists = exists
	ce.CurrentLines = lines
	ce.CurrentSize = size
	return ce
}

// SetRequestContext adds request context to the error
func (ce *ContextualError) SetRequestContext(action string, start, end int) *ContextualError {
	ce.RequestedAction = action
	ce.RequestedStart = start
	ce.RequestedEnd = end
	return ce
}

// AddContextLines adds line content around the error location
func (ce *ContextualError) AddContextLines(fileContent []string, targetLine int, contextRange int) *ContextualError {
	if len(fileContent) == 0 {
		return ce
	}

	// Inline helper functions
	max := func(a, b int) int {
		if a > b {
			return a
		}
		return b
	}
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}

	startLine := max(1, targetLine-contextRange)
	endLine := min(len(fileContent), targetLine+contextRange)

	for i := startLine; i <= endLine; i++ {
		if i <= len(fileContent) {
			content := ""
			if i-1 < len(fileContent) {
				content = fileContent[i-1] // Convert to 0-based index
			}

			ce.ContextLines = append(ce.ContextLines, ContextLine{
				LineNumber: i,
				Content:    content,
				IsTarget:   i == targetLine,
				IsError:    i == targetLine,
			})
		}
	}

	return ce
}

// AddSuggestion adds a suggested correction
func (ce *ContextualError) AddSuggestion(suggestion string) *ContextualError {
	ce.Suggestions = append(ce.Suggestions, suggestion)
	return ce
}

// AddRequiredAction adds a required action for the LLM
func (ce *ContextualError) AddRequiredAction(action string) *ContextualError {
	ce.RequiredActions = append(ce.RequiredActions, action)
	return ce
}

// AddPreventionTip adds a tip for preventing this error
func (ce *ContextualError) AddPreventionTip(tip string) *ContextualError {
	ce.PreventionTips = append(ce.PreventionTips, tip)
	return ce
}

// FormatForLLM formats the contextual error for LLM consumption
func (ce *ContextualError) FormatForLLM() string {
	var formatted strings.Builder

	// Header
	formatted.WriteString("🚫 EDIT OPERATION FAILED\n")
	formatted.WriteString("=" + strings.Repeat("=", 40) + "\n\n")

	// Error type and message
	formatted.WriteString(fmt.Sprintf("❌ Error Type: %s\n", strings.ToUpper(ce.Type)))
	formatted.WriteString(fmt.Sprintf("📁 File: %s\n", ce.FilePath))
	formatted.WriteString(fmt.Sprintf("💬 Message: %s\n\n", ce.Message))

	// File state information
	formatted.WriteString("📊 CURRENT FILE STATE:\n")
	if ce.FileExists {
		formatted.WriteString(fmt.Sprintf("✅ File exists: %d lines, %d bytes\n", ce.CurrentLines, ce.CurrentSize))
	} else {
		formatted.WriteString("❌ File does not exist\n")
	}

	// Request context
	if ce.RequestedAction != "" {
		formatted.WriteString(fmt.Sprintf("🔧 Requested: %s lines %d-%d\n", ce.RequestedAction, ce.RequestedStart, ce.RequestedEnd))
	}

	// Line context
	if len(ce.ContextLines) > 0 {
		formatted.WriteString("\n📝 FILE CONTENT AROUND ERROR:\n")
		for _, line := range ce.ContextLines {
			prefix := "  "
			if line.IsTarget {
				prefix = "►"
			}
			if line.IsError {
				prefix = "❌"
			}
			formatted.WriteString(fmt.Sprintf("%s Line %d: %s\n", prefix, line.LineNumber, line.Content))
		}
	}

	// Suggestions
	if len(ce.Suggestions) > 0 {
		formatted.WriteString("\n💡 SUGGESTED CORRECTIONS:\n")
		for i, suggestion := range ce.Suggestions {
			formatted.WriteString(fmt.Sprintf("%d. %s\n", i+1, suggestion))
		}
	}

	// Required actions
	if len(ce.RequiredActions) > 0 {
		formatted.WriteString("\n🎯 REQUIRED ACTIONS:\n")
		for i, action := range ce.RequiredActions {
			formatted.WriteString(fmt.Sprintf("%d. %s\n", i+1, action))
		}
	}

	// Prevention tips
	if len(ce.PreventionTips) > 0 {
		formatted.WriteString("\n🛡️ PREVENTION TIPS:\n")
		for i, tip := range ce.PreventionTips {
			formatted.WriteString(fmt.Sprintf("%d. %s\n", i+1, tip))
		}
	}

	return formatted.String()
}

// NewProgressiveValidationResult creates a new progressive validation result
func NewProgressiveValidationResult() *ProgressiveValidationResult {
	return &ProgressiveValidationResult{
		OverallStatus:   "pending",
		CurrentStage:    "syntax",
		Stages:          []ValidationStage{},
		CanProceed:      false,
		ValidationCount: 0,
	}
}

// AddStage adds a validation stage to the result
func (pvr *ProgressiveValidationResult) AddStage(name, status, message string) *ValidationStage {
	stage := ValidationStage{
		Name:        name,
		Status:      status,
		Message:     message,
		Suggestions: []string{},
		Duration:    0,
	}

	pvr.Stages = append(pvr.Stages, stage)
	pvr.CurrentStage = name
	pvr.ValidationCount++

	return &pvr.Stages[len(pvr.Stages)-1]
}

// SetStageDetails updates the details and suggestions for the last added stage
func (pvr *ProgressiveValidationResult) SetStageDetails(details string, suggestions []string, duration int64) {
	if len(pvr.Stages) > 0 {
		lastStage := &pvr.Stages[len(pvr.Stages)-1]
		lastStage.Details = details
		lastStage.Suggestions = suggestions
		lastStage.Duration = duration
	}
}

// MarkStageComplete marks the current stage as completed
func (pvr *ProgressiveValidationResult) MarkStageComplete(status, message string) {
	if len(pvr.Stages) > 0 {
		lastStage := &pvr.Stages[len(pvr.Stages)-1]
		lastStage.Status = status
		lastStage.Message = message

		// Update overall status based on stage completion
		if status == "failed" {
			pvr.OverallStatus = "failed"
			pvr.FailureStage = lastStage.Name
			pvr.CanProceed = false
		} else if status == "warning" && pvr.OverallStatus != "failed" {
			pvr.OverallStatus = "warnings"
		} else if status == "passed" && pvr.OverallStatus == "pending" {
			pvr.OverallStatus = "passed"
		}
	}
}

// SetCanProceed sets whether the edit can proceed after validation
func (pvr *ProgressiveValidationResult) SetCanProceed(canProceed bool) {
	pvr.CanProceed = canProceed
	if canProceed && pvr.OverallStatus != "failed" {
		if pvr.OverallStatus == "pending" {
			pvr.OverallStatus = "passed"
		}
	}
}

// CalculateTotalDuration calculates the total duration from all stages
func (pvr *ProgressiveValidationResult) CalculateTotalDuration() {
	total := int64(0)
	for _, stage := range pvr.Stages {
		total += stage.Duration
	}
	pvr.TotalDurationMs = total
}

// FormatForLLM formats the progressive validation result for LLM consumption
func (pvr *ProgressiveValidationResult) FormatForLLM() string {
	var formatted strings.Builder

	// Header
	formatted.WriteString("🔍 PROGRESSIVE VALIDATION REPORT\n")
	formatted.WriteString("=" + strings.Repeat("=", 45) + "\n\n")

	// Overall status
	statusIcon := "✅"
	if pvr.OverallStatus == "failed" {
		statusIcon = "❌"
	} else if pvr.OverallStatus == "warnings" {
		statusIcon = "⚠️"
	} else if pvr.OverallStatus == "pending" {
		statusIcon = "⏳"
	}

	formatted.WriteString(fmt.Sprintf("%s Overall Status: %s\n", statusIcon, strings.ToUpper(pvr.OverallStatus)))
	formatted.WriteString(fmt.Sprintf("📊 Validation Stages: %d completed\n", pvr.ValidationCount))
	formatted.WriteString(fmt.Sprintf("⏱️  Total Time: %dms\n", pvr.TotalDurationMs))

	if pvr.CanProceed {
		formatted.WriteString("✅ Can Proceed: YES\n")
	} else {
		formatted.WriteString("❌ Can Proceed: NO\n")
	}

	if pvr.FailureStage != "" {
		formatted.WriteString(fmt.Sprintf("🚫 Failed at: %s stage\n", pvr.FailureStage))
	}

	// Individual stages
	formatted.WriteString("\n📋 VALIDATION STAGES:\n")
	for i, stage := range pvr.Stages {
		stageIcon := "⏳"
		switch stage.Status {
		case "passed":
			stageIcon = "✅"
		case "failed":
			stageIcon = "❌"
		case "warning":
			stageIcon = "⚠️"
		}

		formatted.WriteString(fmt.Sprintf("%d. %s %s (%dms)\n", i+1, stageIcon, strings.ToTitle(stage.Name), stage.Duration))
		formatted.WriteString(fmt.Sprintf("   %s\n", stage.Message))

		if stage.Details != "" {
			formatted.WriteString(fmt.Sprintf("   Details: %s\n", stage.Details))
		}

		if len(stage.Suggestions) > 0 {
			formatted.WriteString("   Suggestions:\n")
			for j, suggestion := range stage.Suggestions {
				formatted.WriteString(fmt.Sprintf("   %d) %s\n", j+1, suggestion))
			}
		}
		formatted.WriteString("\n")
	}

	// Action analysis if available
	if pvr.ActionAnalysis != nil {
		formatted.WriteString(pvr.ActionAnalysis.FormatForLLM())
	}

	// Dry run preview if available
	if pvr.DryRunPreview != nil {
		formatted.WriteString(pvr.DryRunPreview.FormatForLLM())
	}

	return formatted.String()
}

// FormatForLLM formats the dry-run preview for LLM consumption
func (drp *DryRunPreview) FormatForLLM() string {
	var formatted strings.Builder

	formatted.WriteString("👁️  DRY RUN PREVIEW\n")
	formatted.WriteString("-" + strings.Repeat("-", 30) + "\n")

	formatted.WriteString(fmt.Sprintf("📁 File: %s\n", drp.FilePath))

	if drp.FileExists {
		formatted.WriteString(fmt.Sprintf("📊 Current: %d lines, %d bytes\n", drp.CurrentLines, drp.CurrentSize))
		formatted.WriteString(fmt.Sprintf("📈 After Edit: %d lines, %d bytes\n", drp.ExpectedLines, drp.ExpectedSize))

		deltaIcon := "📉"
		if drp.LineDelta > 0 {
			deltaIcon = "📈"
		} else if drp.LineDelta == 0 {
			deltaIcon = "📊"
		}
		formatted.WriteString(fmt.Sprintf("%s Line Change: %+d\n", deltaIcon, drp.LineDelta))

		sizeIcon := "📉"
		if drp.SizeDelta > 0 {
			sizeIcon = "📈"
		} else if drp.SizeDelta == 0 {
			sizeIcon = "📊"
		}
		formatted.WriteString(fmt.Sprintf("%s Size Change: %+d bytes\n", sizeIcon, drp.SizeDelta))
	} else {
		formatted.WriteString("📝 New file will be created\n")
		formatted.WriteString(fmt.Sprintf("📊 Expected: %d lines, %d bytes\n", drp.ExpectedLines, drp.ExpectedSize))
	}

	formatted.WriteString(fmt.Sprintf("📝 Changes: %s\n", drp.ChangesSummary))

	// Show preview lines
	if len(drp.PreviewLines) > 0 {
		formatted.WriteString("\n📄 PREVIEW OF CHANGES:\n")
		for _, line := range drp.PreviewLines {
			prefix := "  "
			if line.IsTarget {
				prefix = "►"
			}

			switch line.ChangeType {
			case "added":
				formatted.WriteString(fmt.Sprintf("%s + %s\n", prefix, line.NewContent))
			case "deleted":
				formatted.WriteString(fmt.Sprintf("%s - %s\n", prefix, line.OldContent))
			case "modified":
				formatted.WriteString(fmt.Sprintf("%s ~ %s\n", prefix, line.NewContent))
			case "unchanged":
				formatted.WriteString(fmt.Sprintf("%s   %s\n", prefix, line.OldContent))
			}
		}
	}

	// Safety warnings
	if len(drp.SafetyWarnings) > 0 {
		formatted.WriteString("\n⚠️  SAFETY WARNINGS:\n")
		for i, warning := range drp.SafetyWarnings {
			formatted.WriteString(fmt.Sprintf("%d. %s\n", i+1, warning))
		}
	}

	// Recommended actions
	if len(drp.RecommendedActions) > 0 {
		formatted.WriteString("\n💡 RECOMMENDED ACTIONS:\n")
		for i, action := range drp.RecommendedActions {
			formatted.WriteString(fmt.Sprintf("%d. %s\n", i+1, action))
		}
	}

	return formatted.String()
}

// FormatForLLM formats the action analysis for LLM consumption
func (aa *ActionAnalysis) FormatForLLM() string {
	var formatted strings.Builder

	formatted.WriteString("🎯 SMART ACTION SELECTION ANALYSIS\n")
	formatted.WriteString("-" + strings.Repeat("-", 35) + "\n")

	// Current action assessment
	statusIcon := "✅"
	if aa.AnalysisType == "suboptimal" {
		statusIcon = "⚠️"
	} else if aa.AnalysisType == "inefficient" {
		statusIcon = "🔄"
	} else if aa.AnalysisType == "problematic" {
		statusIcon = "❌"
	}

	formatted.WriteString(fmt.Sprintf("%s Current Action: %s (%s)\n", statusIcon, aa.CurrentAction, strings.ToUpper(aa.AnalysisType)))

	// Edit intent analysis
	formatted.WriteString(fmt.Sprintf("🎯 Detected Intent: %s\n", aa.EditIntent.IntentType))
	formatted.WriteString(fmt.Sprintf("📍 Target Scope: %s\n", aa.EditIntent.TargetScope))
	formatted.WriteString(fmt.Sprintf("📄 Content Type: %s\n", aa.EditIntent.ContentType))

	if aa.EditIntent.SearchPattern != "" {
		formatted.WriteString(fmt.Sprintf("🔍 Search Pattern: %s\n", aa.EditIntent.SearchPattern))
	}

	formatted.WriteString(fmt.Sprintf("🎲 Confidence: %.1f%%\n", aa.EditIntent.Confidence*100))

	// Pattern matches
	if len(aa.PatternMatches) > 0 {
		formatted.WriteString("\n🧩 RECOGNIZED PATTERNS:\n")
		for i, pattern := range aa.PatternMatches {
			formatted.WriteString(fmt.Sprintf("%d. %s\n", i+1, pattern))
		}
	}

	// Action suggestions (if not optimal)
	if len(aa.Suggestions) > 0 {
		formatted.WriteString("\n💡 SUGGESTED ACTIONS:\n")
		for i, suggestion := range aa.Suggestions {
			formatted.WriteString(fmt.Sprintf("%d. **%s** (%.1f%% confidence)\n", i+1, suggestion.SuggestedAction, suggestion.ConfidenceScore*100))
			formatted.WriteString(fmt.Sprintf("   📝 Reasoning: %s\n", suggestion.Reasoning))

			if len(suggestion.Benefits) > 0 {
				formatted.WriteString("   ✅ Benefits:\n")
				for _, benefit := range suggestion.Benefits {
					formatted.WriteString(fmt.Sprintf("     • %s\n", benefit))
				}
			}

			if suggestion.ExampleUsage != "" {
				formatted.WriteString(fmt.Sprintf("   💻 Example: %s\n", suggestion.ExampleUsage))
			}

			if suggestion.EfficiencyGain != "" {
				formatted.WriteString(fmt.Sprintf("   ⚡ Efficiency: %s\n", suggestion.EfficiencyGain))
			}
			formatted.WriteString("\n")
		}
	}

	// Optimization tips
	if len(aa.OptimizationTips) > 0 {
		formatted.WriteString("🔧 OPTIMIZATION TIPS:\n")
		for i, tip := range aa.OptimizationTips {
			formatted.WriteString(fmt.Sprintf("%d. %s\n", i+1, tip))
		}
	}

	// Context warnings
	if len(aa.ContextWarnings) > 0 {
		formatted.WriteString("\n⚠️  CONTEXT WARNINGS:\n")
		for i, warning := range aa.ContextWarnings {
			formatted.WriteString(fmt.Sprintf("%d. %s\n", i+1, warning))
		}
	}

	return formatted.String()
}

// GetLLMSummaryWithProgressive returns enhanced summary with progressive validation
func (tr *TaskResponse) GetLLMSummaryWithProgressive() string {
	// If there's progressive validation, return that
	if tr.ProgressiveValidation != nil {
		return tr.ProgressiveValidation.FormatForLLM()
	}

	// Fall back to existing summary logic
	return tr.GetLLMSummary()
}

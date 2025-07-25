package task

import (
	"encoding/json"
	"fmt"
	contextMgr "loom/context"
	"loom/debug"
	"loom/indexer"
	"loom/llm"
	"regexp"
	"strings"
	"time"
)

// ExplorationPhase represents the current phase of objective exploration
type ExplorationPhase string

const (
	PhaseObjectiveSetting      ExplorationPhase = "objective_setting"
	PhaseSuppressedExploration ExplorationPhase = "suppressed_exploration"
	PhaseSynthesis             ExplorationPhase = "synthesis"
)

// ObjectiveExploration tracks an ongoing objective-driven exploration
type ObjectiveExploration struct {
	Objective       string
	Phase           ExplorationPhase
	TasksExecuted   int
	AccumulatedData []TaskResponse
	StartTime       time.Time
}

// SequentialTaskManager processes tasks one at a time with hidden exploration context
// This prevents fragmented output and enables comprehensive synthesis like Cursor
type SequentialTaskManager struct {
	executor           *Executor
	llmAdapter         llm.LLMAdapter
	chatSession        ChatSession
	explorationContext []llm.Message // Hidden context for iterative exploration
	maxIterations      int           // Safety limit to prevent infinite loops
	currentIteration   int
	isExploring        bool
	initialUserQuery   string
	completionSignals  []string

	// Objective-driven exploration
	currentObjective *ObjectiveExploration
	contextManager   *contextMgr.ContextManager // Added for context optimization
}

// ExplorationResult represents the final result of a sequential exploration
type ExplorationResult struct {
	Success          bool
	FinalSynthesis   string
	TasksExecuted    int
	Duration         time.Duration
	CompletionReason string
}

// NewSequentialTaskManager creates a new sequential task manager
func NewSequentialTaskManager(executor *Executor, llmAdapter llm.LLMAdapter, chatSession ChatSession) *SequentialTaskManager {
	return &SequentialTaskManager{
		executor:           executor,
		llmAdapter:         llmAdapter,
		chatSession:        chatSession,
		explorationContext: make([]llm.Message, 0),
		maxIterations:      15, // Reasonable limit for exploration
		completionSignals: []string{
			"EXPLORATION_COMPLETE:",
			"ANALYSIS_COMPLETE:",
			"TASK_COMPLETE:",
			"OBJECTIVE_COMPLETE:",
			"exploration complete",
			"analysis complete",
		},
		// We'll set the contextManager later with SetContextManager
	}
}

// SetContextManager sets the context manager for optimized context management
func (stm *SequentialTaskManager) SetContextManager(index *indexer.Index, maxContextTokens int) {
	stm.contextManager = contextMgr.NewContextManager(index, maxContextTokens)
}

// CreateSequentialSystemMessage creates a system message optimized for sequential exploration
func (stm *SequentialTaskManager) CreateSequentialSystemMessage() llm.Message {
	return llm.Message{
		Role: "system",
		Content: `You are an AI coding assistant that executes tasks sequentially.

EXECUTION MODEL:
- Execute commands (tasks, edits) ONE at a time
- Wait for each command to complete before proceeding
- After ALL commands are complete, provide a text-only final response
- Never mix commands with explanatory text in the same response

Each response should contain EITHER:
1. A SINGLE command (READ, SEARCH, LIST, etc.)
2. A final text-only explanation with no commands

DO NOT respond with multiple commands at once.
DO NOT include explanations with your commands.
After all necessary commands are executed, end with a text-only summary.`,
		Timestamp: time.Now(),
	}
}

// createSynthesisPrompt creates the final synthesis prompt
// ParseSingleTask extracts a single task from the LLM response
func (stm *SequentialTaskManager) ParseSingleTask(llmResponse string) (*Task, string, error) {
	debug.LogToFile(llmResponse)

	// Print part of the response for debugging
	responsePreview := llmResponse
	if len(responsePreview) > 100 {
		responsePreview = responsePreview[:100] + "..."
	}

	emojiSearchPattern := regexp.MustCompile(`(?i)(?:Task:)?\s*🔍\s*Search\s+(?:for)?\s*['"]?(.+?)['"]?(?:\s+\(([^)]+)\))?$`)
	if matches := emojiSearchPattern.FindStringSubmatch(llmResponse); len(matches) > 0 {
		// Extract search query (first capture group)
		searchQuery := matches[1]
		var options string
		if len(matches) > 2 {
			options = matches[2]
		}

		// Create a search task
		task := &Task{
			Type:  TaskTypeSearch,
			Path:  ".",
			Query: searchQuery,
		}

		// Parse options, including the "including filename matches" phrase
		if strings.Contains(strings.ToLower(options), "filename") ||
			strings.Contains(strings.ToLower(options), "name") {
			task.SearchNames = true
		}

		if strings.Contains(strings.ToLower(options), "content") {
			task.CombineResults = true
		}

		// Default to searching both if no specific option is provided
		if !task.SearchNames && !task.CombineResults {
			task.SearchNames = true
			task.CombineResults = true
		}

		return task, "", nil
	}

	// Look for single SEARCH pattern (standard format)
	searchPattern := regexp.MustCompile(`(?i)^SEARCH\s+(.+?)(?:\s+(.+))?$`)
	if matches := searchPattern.FindStringSubmatch(llmResponse); len(matches) > 0 {
		// Extract search query and options
		searchQuery := matches[1]
		var options string
		if len(matches) > 2 {
			options = matches[2]
		}

		// Create a basic search task
		task := &Task{
			Type:  TaskTypeSearch,
			Path:  ".",
			Query: searchQuery,
		}

		// Parse search flags
		if strings.Contains(strings.ToLower(options), "name") ||
			strings.Contains(strings.ToLower(options), "file") {
			task.SearchNames = true
		}

		if strings.Contains(strings.ToLower(options), "content") {
			task.CombineResults = true
		}

		// Default to both if neither is specified
		if !task.SearchNames && !task.CombineResults {
			task.SearchNames = true
			task.CombineResults = true
		}

		return task, "", nil
	}

	// Try parsing as JSON task
	var taskList TaskList
	if err := json.Unmarshal([]byte(llmResponse), &taskList); err == nil && len(taskList.Tasks) > 0 {
		return &taskList.Tasks[0], stm.extractNonTaskContent(llmResponse), nil
	}

	// Try parsing as single task
	var task Task
	if err := json.Unmarshal([]byte(llmResponse), &task); err == nil {
		if task.Type != "" {
			return &task, stm.extractNonTaskContent(llmResponse), nil
		}
	}

	// Try parsing as raw task JSON
	if task, content, err := stm.parseRawTaskJSON(llmResponse); err == nil && task != nil {
		return task, content, nil
	}

	// No task found
	return nil, llmResponse, nil
}

// parseRawTaskJSON attempts to parse a raw JSON task object from the response
func (stm *SequentialTaskManager) parseRawTaskJSON(response string) (*Task, string, error) {
	// Use the enhanced fallback parsing from ParseTasks which properly validates task types
	if result := tryFallbackJSONParsing(response); result != nil && len(result.Tasks) > 0 {
		// Extract non-task content
		content := stm.extractNonTaskContent(response)
		return &result.Tasks[0], content, nil
	}

	// Return nil to fall back to the standard ParseTasks function
	return nil, "", nil
}

// extractNonTaskContent extracts text content outside of JSON task blocks
func (stm *SequentialTaskManager) extractNonTaskContent(response string) string {
	// Remove JSON code blocks to get exploration commentary
	lines := strings.Split(response, "\n")
	var content []string
	inJsonBlock := false

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inJsonBlock = !inJsonBlock
			continue
		}

		if !inJsonBlock && strings.TrimSpace(line) != "" {
			content = append(content, line)
		}
	}

	return strings.Join(content, "\n")
}

// formatTaskResultForExploration formats task results for the hidden exploration context
func (stm *SequentialTaskManager) formatTaskResultForExploration(task *Task, response *TaskResponse) llm.Message {
	var content strings.Builder

	// Build the task result content
	content.WriteString(fmt.Sprintf("TASK_RESULT: %s\n", task.Description()))

	if response.Success {
		content.WriteString("STATUS: Success\n")
		// Use ActualContent for LLM context (includes full file content, etc.)
		if response.ActualContent != "" {
			content.WriteString(fmt.Sprintf("CONTENT:\n%s\n", response.ActualContent))
		} else if response.Output != "" {
			content.WriteString(fmt.Sprintf("CONTENT:\n%s\n", response.Output))
		}
	} else {
		content.WriteString("STATUS: Failed\n")
		if response.Error != "" {
			content.WriteString(fmt.Sprintf("ERROR: %s\n", response.Error))
		}
	}

	// CRITICAL: Use "system" role for task results instead of "assistant"
	// This ensures the LLM treats this as factual information from the environment
	// rather than as the AI's own thoughts or actions
	resultMsg := llm.Message{
		Role:      "system",
		Content:   content.String(),
		Timestamp: time.Now(),
	}

	return resultMsg
}

// FormatTaskResultForTest exposes formatTaskResultForExploration for testing
func (stm *SequentialTaskManager) FormatTaskResultForTest(task *Task, response *TaskResponse) string {
	msg := stm.formatTaskResultForExploration(task, response)
	return msg.Content
}

// GetExplorationContext returns the current exploration context (for debugging)
func (stm *SequentialTaskManager) GetExplorationContext() []llm.Message {
	return stm.explorationContext
}

// IsExploring returns whether the manager is currently in exploration mode
func (stm *SequentialTaskManager) IsExploring() bool {
	return stm.isExploring
}

// GetCurrentIteration returns the current exploration iteration
func (stm *SequentialTaskManager) GetCurrentIteration() int {
	return stm.currentIteration
}

// StartObjectiveExploration initiates objective-driven exploration
func (stm *SequentialTaskManager) StartObjectiveExploration(userQuery string) {
	stm.currentObjective = &ObjectiveExploration{
		Phase:           PhaseObjectiveSetting,
		TasksExecuted:   0,
		AccumulatedData: make([]TaskResponse, 0),
		StartTime:       time.Now(),
	}
	stm.isExploring = true
	stm.initialUserQuery = userQuery
}

// ExtractObjective extracts objective from LLM response
func (stm *SequentialTaskManager) ExtractObjective(response string) string {
	// For backwards compatibility, still extract objectives from responses
	// But we won't be relying on them for the new execution model
	pattern := regexp.MustCompile(`(?i)OBJECTIVE:\s*(.*?)(?:\n|$)`)
	matches := pattern.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// SetObjective sets the current exploration objective and moves to suppressed phase
func (stm *SequentialTaskManager) SetObjective(objective string) {
	if stm.currentObjective != nil {
		stm.currentObjective.Objective = objective
		stm.currentObjective.Phase = PhaseSuppressedExploration
	}
}

// IsObjectiveComplete checks if the response indicates objective completion
func (stm *SequentialTaskManager) IsObjectiveComplete(response string) bool {
	// In our new model, text-only responses with no commands signal completion
	taskPatterns := []string{
		"🔧 READ", "📖 READ",
		"🔧 LIST", "📂 LIST",
		"🔧 SEARCH", "🔍 SEARCH",
		"🔧 RUN",
		"🔧 MEMORY", "💾 MEMORY",
		">>LOOM_EDIT", "✏️ Edit",
	}

	for _, pattern := range taskPatterns {
		if strings.Contains(response, pattern) {
			return false
		}
	}

	// Look for LOOM_EDIT blocks
	if regexp.MustCompile(`(?s)>>LOOM_EDIT.*?<<LOOM_EDIT`).MatchString(response) {
		return false
	}

	// Check for natural language task patterns at the beginning of lines
	naturalLangPatterns := []string{
		"READ ",
		"LIST ",
		"SEARCH ",
		"RUN ",
		"MEMORY ",
	}

	for _, pattern := range naturalLangPatterns {
		if regexp.MustCompile(`(?m)^` + pattern).MatchString(response) {
			return false
		}
	}

	// If no commands are found and there's substantial content, consider it complete
	if len(response) > 80 || strings.Contains(response, "\n") {
		return true
	}

	return false
}

// AddTaskResult adds a task result to the accumulated data
func (stm *SequentialTaskManager) AddTaskResult(taskResponse TaskResponse) {
	if stm.currentObjective != nil {
		stm.currentObjective.AccumulatedData = append(stm.currentObjective.AccumulatedData, taskResponse)
		stm.currentObjective.TasksExecuted++
	}
}

// GetCurrentPhase returns the current exploration phase
func (stm *SequentialTaskManager) GetCurrentPhase() ExplorationPhase {
	if stm.currentObjective != nil {
		return stm.currentObjective.Phase
	}
	return PhaseObjectiveSetting
}

// CompleteObjective completes the current objective and resets state
func (stm *SequentialTaskManager) CompleteObjective() {
	stm.currentObjective = nil
	stm.isExploring = false
	stm.currentIteration = 0
}

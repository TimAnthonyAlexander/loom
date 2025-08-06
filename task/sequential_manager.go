package task

import (
	"context"
	"fmt"
	contextMgr "loom/context"
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
	cancelFunc         context.CancelFunc // Added to track and cancel ongoing exploration
	cancelCtx          context.Context    // Added to track cancellation context

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

// StopExploration cancels any ongoing exploration
func (stm *SequentialTaskManager) StopExploration() {
	if stm.cancelFunc != nil {
		stm.cancelFunc()
		stm.cancelFunc = nil
	}
	stm.isExploring = false
	stm.currentObjective = nil
}

// HandleExplorationRequest starts a sequential exploration based on user query
func (stm *SequentialTaskManager) HandleExplorationRequest(userQuery string) (*ExplorationResult, error) {
	startTime := time.Now()

	// Cancel any existing exploration
	if stm.cancelFunc != nil {
		stm.cancelFunc()
	}

	// Create a new cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	stm.cancelCtx = ctx
	stm.cancelFunc = cancel

	// Reset state for new exploration
	stm.explorationContext = make([]llm.Message, 0)
	stm.currentIteration = 0
	stm.isExploring = true
	stm.initialUserQuery = userQuery

	// Add initial user query to exploration context
	stm.addToExplorationContext(llm.Message{
		Role:      "user",
		Content:   userQuery,
		Timestamp: time.Now(),
	})

	// Execute exploration loop with cancellation context
	result, err := stm.executeExplorationLoop(ctx)
	if err != nil {
		// Check if it was cancelled
		if ctx.Err() == context.Canceled {
			return &ExplorationResult{
				Success:          false,
				TasksExecuted:    stm.currentIteration,
				Duration:         time.Since(startTime),
				CompletionReason: "Exploration cancelled by user",
			}, nil
		}

		return &ExplorationResult{
			Success:          false,
			TasksExecuted:    stm.currentIteration,
			Duration:         time.Since(startTime),
			CompletionReason: fmt.Sprintf("Error: %v", err),
		}, err
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// executeExplorationLoop runs the iterative exploration process
func (stm *SequentialTaskManager) executeExplorationLoop(ctx context.Context) (*ExplorationResult, error) {
	for stm.currentIteration < stm.maxIterations {
		// Check for cancellation first
		select {
		case <-ctx.Done():
			return &ExplorationResult{
				Success:          false,
				TasksExecuted:    stm.currentIteration,
				CompletionReason: "Exploration cancelled by user",
			}, nil
		default:
			// Continue with exploration
		}

		// Get current context for LLM (system + exploration context)
		messages := stm.buildLLMContext()

		// Send to LLM with parent cancellation context
		llmCtx, llmCancel := context.WithTimeout(ctx, 60*time.Second)
		response, err := stm.llmAdapter.Send(llmCtx, messages)
		llmCancel()

		if err != nil {
			// Check if it was cancelled
			if ctx.Err() == context.Canceled {
				return &ExplorationResult{
					Success:          false,
					TasksExecuted:    stm.currentIteration,
					CompletionReason: "Exploration cancelled by user",
				}, nil
			}
			return nil, fmt.Errorf("LLM request failed at iteration %d: %w", stm.currentIteration, err)
		}

		// Check for completion signal
		if isComplete, synthesis := stm.checkCompletionSignal(response.Content); isComplete {
			// Add final synthesis to chat session for user
			return stm.finalizeSynthesis(synthesis)
		}

		// Parse single task from response
		task, explorationContent, err := stm.ParseSingleTask(response.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to parse task at iteration %d: %w", stm.currentIteration, err)
		}

		if task == nil {
			// No task found - LLM might be providing analysis without action
			// Add the response to exploration context and continue
			stm.addToExplorationContext(*response)
			stm.currentIteration++
			continue
		}

		// Check for cancellation before executing task
		select {
		case <-ctx.Done():
			return &ExplorationResult{
				Success:          false,
				TasksExecuted:    stm.currentIteration,
				CompletionReason: "Exploration cancelled by user",
			}, nil
		default:
			// Continue with task execution
		}

		// Execute the task
		taskResponse := stm.executor.Execute(task)

		// Check if task requires confirmation (critical safety check missing!)
		if taskResponse.Success && task.RequiresConfirmation() {
			return nil, fmt.Errorf("sequential manager does not support destructive tasks that require confirmation. Task: %s requires user approval but sequential exploration cannot handle confirmations", task.Description())
		}

		// Add task result to exploration context (hidden from user)
		taskResultMsg := stm.formatTaskResultForExploration(task, taskResponse)
		stm.addToExplorationContext(taskResultMsg)

		// If there was additional exploration content, add it too
		if strings.TrimSpace(explorationContent) != "" {
			stm.addToExplorationContext(llm.Message{
				Role:      "user",
				Content:   explorationContent,
				Timestamp: time.Now(),
			})
		}

		stm.currentIteration++
	}

	// Max iterations reached - force completion
	return &ExplorationResult{
		Success:          false,
		TasksExecuted:    stm.currentIteration,
		CompletionReason: "Maximum exploration iterations reached",
	}, fmt.Errorf("exploration exceeded maximum iterations (%d)", stm.maxIterations)
}

// buildLLMContext creates the context for LLM with system message and exploration history
func (stm *SequentialTaskManager) buildLLMContext() []llm.Message {
	// Get system message with sequential exploration instructions
	systemMsg := stm.CreateSequentialSystemMessage()

	// Try to use a context manager for optimized context if available
	if stm.contextManager != nil {
		// Start with system message
		messages := []llm.Message{systemMsg}

		// Add exploration context
		messages = append(messages, stm.explorationContext...)

		// Optimize the messages
		optimized, err := stm.contextManager.OptimizeMessages(messages)
		if err == nil {
			return optimized
		}
		// Fall back to full context if optimization fails
	}

	// Combine system message with exploration context without optimization
	messages := []llm.Message{systemMsg}
	messages = append(messages, stm.explorationContext...)
	return messages
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

// createObjectiveSettingPrompt creates the initial objective-setting prompt

// createSuppressedExplorationPrompt creates the suppressed exploration prompt

// createSynthesisPrompt creates the final synthesis prompt

// ParseSingleTask extracts the first task from LLM response and any additional content
func (stm *SequentialTaskManager) ParseSingleTask(llmResponse string) (*Task, string, error) {
	// First try to find a single task JSON object (new sequential format)
	task, content, err := stm.parseRawTaskJSON(llmResponse)
	if task != nil {
		return task, content, err
	}

	// Fall back to existing parser for code-block wrapped tasks
	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		return nil, "", err
	}

	// If no tasks found, return the response content for analysis
	if taskList == nil || len(taskList.Tasks) == 0 {
		return nil, llmResponse, nil
	}

	// Take only the first task
	firstTask := &taskList.Tasks[0]

	// Extract non-task content (everything outside JSON blocks)
	content = stm.extractNonTaskContent(llmResponse)

	return firstTask, content, nil
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

// checkCompletionSignal checks if the response indicates objective completion
func (stm *SequentialTaskManager) checkCompletionSignal(response string) (bool, string) {
	// Check if the response is a text-only message with no commands
	// This would signal completion in our new model
	taskPatterns := []string{
		"🔧 READ", "📖 READ",
		"🔧 LIST", "📂 LIST",
		"🔧 SEARCH", "🔍 SEARCH",
		"🔧 RUN",
		"🔧 MEMORY", "💾 MEMORY",
		"🔧 TODO", "📝 TODO",
		">>LOOM_EDIT", "✏️ Edit",
		"\nREAD ",
		"\nLIST ",
		"\nSEARCH ",
		"\nRUN ",
		"\nMEMORY ",
		"\nTODO ",
	}

	for _, pattern := range taskPatterns {
		if strings.Contains(response, pattern) {
			return false, ""
		}
	}

	// Look for LOOM_EDIT blocks
	if regexp.MustCompile(`(?s)>>LOOM_EDIT.*?<<LOOM_EDIT`).MatchString(response) {
		return false, ""
	}

	// If no commands are found and there's substantial content,
	// consider it a completion signal
	if len(response) > 80 || strings.Contains(response, "\n") {
		return true, response
	}

	return false, ""
}

// finalizeSynthesis processes the final synthesis and adds it to chat
func (stm *SequentialTaskManager) finalizeSynthesis(synthesis string) (*ExplorationResult, error) {
	// Clean up the synthesis content
	cleanSynthesis := stm.cleanSynthesisContent(synthesis)

	// Add final synthesis to chat session for user display
	finalMessage := llm.Message{
		Role:      "user",
		Content:   cleanSynthesis,
		Timestamp: time.Now(),
	}

	if err := stm.chatSession.AddMessage(finalMessage); err != nil {
		return nil, fmt.Errorf("failed to add synthesis to chat: %w", err)
	}

	return &ExplorationResult{
		Success:          true,
		FinalSynthesis:   cleanSynthesis,
		TasksExecuted:    stm.currentIteration,
		CompletionReason: "Exploration completed successfully",
	}, nil
}

// cleanSynthesisContent removes completion signals and formats the synthesis
func (stm *SequentialTaskManager) cleanSynthesisContent(synthesis string) string {
	content := synthesis

	// Remove completion signal prefixes
	for _, signal := range stm.completionSignals {
		if strings.HasPrefix(strings.ToUpper(content), strings.ToUpper(signal)) {
			content = strings.TrimPrefix(content, signal)
			content = strings.TrimPrefix(content, ":")
			content = strings.TrimSpace(content)
			break
		}
	}

	return content
}

// formatTaskResultForExploration formats task results for the hidden exploration context
func (stm *SequentialTaskManager) formatTaskResultForExploration(task *Task, response *TaskResponse) llm.Message {
	var content strings.Builder

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
		// Even for failed commands, include any available output so the LLM
		// can inspect logs, exit codes, or diagnostic messages.
		if response.ActualContent != "" {
			content.WriteString(fmt.Sprintf("CONTENT:\n%s\n", response.ActualContent))
		} else if response.Output != "" {
			content.WriteString(fmt.Sprintf("CONTENT:\n%s\n", response.Output))
		}
	}

	return llm.Message{
		Role:      "user",
		Content:   content.String(),
		Timestamp: time.Now(),
	}
}

// addToExplorationContext adds a message to the hidden exploration context
func (stm *SequentialTaskManager) addToExplorationContext(message llm.Message) {
	stm.explorationContext = append(stm.explorationContext, message)

	// Limit context size to prevent token overflow
	maxContextMessages := 50
	if len(stm.explorationContext) > maxContextMessages {
		// Keep first message (user query) and trim from middle
		start := stm.explorationContext[:1]
		end := stm.explorationContext[len(stm.explorationContext)-maxContextMessages+1:]
		stm.explorationContext = append(start, end...)
	}
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
	// Cancel any existing exploration
	if stm.cancelFunc != nil {
		stm.cancelFunc()
	}

	// Create new cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	stm.cancelCtx = ctx
	stm.cancelFunc = cancel

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
		"🔧 TODO", "📝 TODO",
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
	// Cancel any ongoing context
	if stm.cancelFunc != nil {
		stm.cancelFunc()
		stm.cancelFunc = nil
		stm.cancelCtx = nil
	}

	stm.currentObjective = nil
	stm.isExploring = false
	stm.currentIteration = 0
}

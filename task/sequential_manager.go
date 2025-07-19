package task

import (
	"context"
	"encoding/json"
	"fmt"
	"loom/llm"
	"strings"
	"time"
)

// ExplorationPhase represents the current phase of objective exploration
type ExplorationPhase string

const (
	PhaseObjectiveSetting ExplorationPhase = "objective_setting"
	PhaseSuppressedExploration ExplorationPhase = "suppressed_exploration" 
	PhaseSynthesis ExplorationPhase = "synthesis"
)

// ObjectiveExploration tracks an ongoing objective-driven exploration
type ObjectiveExploration struct {
	Objective         string
	Phase            ExplorationPhase
	TasksExecuted    int
	AccumulatedData  []TaskResponse
	StartTime        time.Time
}

// SequentialTaskManager processes tasks one at a time with hidden exploration context
// This prevents fragmented output and enables comprehensive synthesis like Cursor
type SequentialTaskManager struct {
	executor            *Executor
	llmAdapter          llm.LLMAdapter
	chatSession         ChatSession
	explorationContext  []llm.Message // Hidden context for iterative exploration
	maxIterations       int           // Safety limit to prevent infinite loops
	currentIteration    int
	isExploring         bool
	initialUserQuery    string
	completionSignals   []string
	
	// Objective-driven exploration
	currentObjective    *ObjectiveExploration
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
		executor:          executor,
		llmAdapter:        llmAdapter,
		chatSession:       chatSession,
		explorationContext: make([]llm.Message, 0),
		maxIterations:     15, // Reasonable limit for exploration
		completionSignals: []string{
			"EXPLORATION_COMPLETE:",
			"ANALYSIS_COMPLETE:",
			"TASK_COMPLETE:",
			"OBJECTIVE_COMPLETE:",
			"exploration complete",
			"analysis complete",
		},
	}
}

// HandleExplorationRequest starts a sequential exploration based on user query
func (stm *SequentialTaskManager) HandleExplorationRequest(userQuery string) (*ExplorationResult, error) {
	startTime := time.Now()
	
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
	
	// Execute exploration loop
	result, err := stm.executeExplorationLoop()
	if err != nil {
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
func (stm *SequentialTaskManager) executeExplorationLoop() (*ExplorationResult, error) {
	for stm.currentIteration < stm.maxIterations {
		// Get current context for LLM (system + exploration context)
		messages := stm.buildLLMContext()
		
		// Send to LLM
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		response, err := stm.llmAdapter.Send(ctx, messages)
		cancel()
		
		if err != nil {
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
		
		// Execute the task
		taskResponse := stm.executor.Execute(task)
		
		// Add task result to exploration context (hidden from user)
		taskResultMsg := stm.formatTaskResultForExploration(task, taskResponse)
		stm.addToExplorationContext(taskResultMsg)
		
		// If there was additional exploration content, add it too
		if strings.TrimSpace(explorationContent) != "" {
			stm.addToExplorationContext(llm.Message{
				Role:      "assistant",
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
	
	// Combine system message with exploration context
	messages := []llm.Message{systemMsg}
	messages = append(messages, stm.explorationContext...)
	
	return messages
}

// CreateSequentialSystemMessage creates a system message optimized for sequential exploration
func (stm *SequentialTaskManager) CreateSequentialSystemMessage() llm.Message {
	var content string
	
	switch stm.GetCurrentPhase() {
	case PhaseObjectiveSetting:
		content = stm.createObjectiveSettingPrompt()
	case PhaseSuppressedExploration:
		content = stm.createSuppressedExplorationPrompt()
	case PhaseSynthesis:
		content = stm.createSynthesisPrompt()
	default:
		content = stm.createObjectiveSettingPrompt()
	}

	return llm.Message{
		Role:      "system",
		Content:   content,
		Timestamp: time.Now(),
	}
}

// createObjectiveSettingPrompt creates the initial objective-setting prompt
func (stm *SequentialTaskManager) createObjectiveSettingPrompt() string {
	return `You are Loom starting an OBJECTIVE-DRIVEN EXPLORATION.

## PHASE 1: OBJECTIVE SETTING

Analyze the user's request and establish a clear exploration objective:

1. **Set Clear Objective**: Start with "OBJECTIVE: [specific exploration goal]"
2. **Begin First Task**: Immediately provide the first logical task
3. **Stay Focused**: Keep the objective specific and achievable

### Task Format (JSON only):
{"type": "ReadFile", "path": "README.md", "max_lines": 300}

### Available Task Types:
- **ReadFile**: Read file contents (prefer max_lines: 200-300 for key files)
- **ListDir**: List directory contents (recursive: true for deep analysis)
- **EditFile**: Create/modify files (requires user confirmation)
- **RunShell**: Execute commands (requires user confirmation)

### Example Response:
OBJECTIVE: Understand this Go project's architecture and key components

{"type": "ReadFile", "path": "README.md", "max_lines": 300}

Set your objective and begin exploration immediately.`
}

// createSuppressedExplorationPrompt creates the suppressed exploration prompt
func (stm *SequentialTaskManager) createSuppressedExplorationPrompt() string {
	return `You are Loom in SUPPRESSED EXPLORATION MODE.

## PHASE 2: SUPPRESSED EXPLORATION

Continue pursuing your objective with minimal output:

**SUPPRESSED OUTPUT**: Provide ONLY the next logical task
- NO explanations or analysis
- NO verbose responses  
- Think internally about what you learned
- Focus on systematic exploration
- Continue until objective is complete

### When Complete:
Signal completion with: **OBJECTIVE_COMPLETE:** followed by comprehensive analysis

### Next Task Only:
{"type": "ReadFile", "path": "main.go", "max_lines": 200}

Provide only the next task needed to complete your objective.`
}

// createSynthesisPrompt creates the final synthesis prompt
func (stm *SequentialTaskManager) createSynthesisPrompt() string {
	objectiveText := "the exploration objective"
	if stm.currentObjective != nil && stm.currentObjective.Objective != "" {
		objectiveText = stm.currentObjective.Objective
	}
	
	return fmt.Sprintf(`You are Loom completing an OBJECTIVE-DRIVEN EXPLORATION.

## PHASE 3: SYNTHESIS

You have completed your objective: %s

Provide a comprehensive analysis using ALL the information you've gathered during exploration.

**NO MORE TASKS** - Focus entirely on synthesis and analysis.

Provide a complete, detailed response that fully addresses the original user question.`, objectiveText)
}

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
	// Look for JSON-like patterns in the response
	lines := strings.Split(response, "\n")
	var taskJSON string
	var nonTaskLines []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Check if this line looks like a JSON task
		if strings.HasPrefix(line, `{"type":`) && strings.HasSuffix(line, `}`) {
			taskJSON = line
		} else if line != "" {
			nonTaskLines = append(nonTaskLines, line)
		}
	}
	
	if taskJSON == "" {
		return nil, "", nil
	}
	
	// Try to parse the task JSON
	var task Task
	if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
		return nil, "", fmt.Errorf("failed to parse task JSON: %w", err)
	}
	
	// Validate the task
	if err := validateTask(&task); err != nil {
		return nil, "", fmt.Errorf("invalid task: %w", err)
	}
	
	// Return non-task content
	nonTaskContent := strings.Join(nonTaskLines, "\n")
	
	return &task, nonTaskContent, nil
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

// checkCompletionSignal checks if the LLM has signaled exploration completion
func (stm *SequentialTaskManager) checkCompletionSignal(response string) (bool, string) {
	lowerResponse := strings.ToLower(response)
	
	for _, signal := range stm.completionSignals {
		if strings.Contains(lowerResponse, strings.ToLower(signal)) {
			// Found completion signal - extract the synthesis
			return true, response
		}
	}
	
	return false, ""
}

// finalizeSynthesis processes the final synthesis and adds it to chat
func (stm *SequentialTaskManager) finalizeSynthesis(synthesis string) (*ExplorationResult, error) {
	// Clean up the synthesis content
	cleanSynthesis := stm.cleanSynthesisContent(synthesis)
	
	// Add final synthesis to chat session for user display
	finalMessage := llm.Message{
		Role:      "assistant",
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
	}
	
	return llm.Message{
		Role:      "assistant",
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
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "OBJECTIVE:") {
			objective := strings.TrimSpace(line[10:]) // Remove "OBJECTIVE:"
			return objective
		}
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

// IsObjectiveComplete checks if the exploration objective is complete
func (stm *SequentialTaskManager) IsObjectiveComplete(response string) bool {
	upperResponse := strings.ToUpper(response)
	return strings.Contains(upperResponse, "OBJECTIVE_COMPLETE:") ||
		   strings.Contains(upperResponse, "EXPLORATION_COMPLETE:")
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
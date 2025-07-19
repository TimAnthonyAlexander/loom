package task

import (
	"context"
	"encoding/json"
	"fmt"
	"loom/llm"
	"strings"
	"time"
)

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
	content := `You are Loom operating in SEQUENTIAL EXPLORATION MODE for comprehensive codebase analysis.

## CRITICAL: Sequential Task Rules

**ONE TASK AT A TIME**: You must provide exactly ONE task, analyze its results, then decide the next step.

### Task Format (JSON only):
{"type": "ReadFile", "path": "README.md", "max_lines": 200}

### Available Task Types:
- **ReadFile**: Read file contents (prefer max_lines: 200-300 for key files)
- **ListDir**: List directory contents (recursive: true for deep analysis)
- **EditFile**: Create/modify files (requires user confirmation)
- **RunShell**: Execute commands (requires user confirmation)

### Exploration Strategy:
1. **Start with README.md** to understand project purpose
2. **Check main entry points** (main.go, package.json, etc.)
3. **Explore key directories** based on what you discover
4. **Read core implementation files** to understand architecture
5. **Continue until complete understanding** is achieved

### Completion Signal:
When you have sufficient information for comprehensive analysis, start your response with:
**EXPLORATION_COMPLETE:** followed by your detailed architectural analysis.

### Example Flow:
1. Read README.md → Learn project purpose and structure
2. Read main.go → Understand entry point and CLI structure  
3. List key directories → Map out code organization
4. Read core package files → Understand implementation patterns
5. EXPLORATION_COMPLETE: [comprehensive analysis]

**IMPORTANT**: 
- Provide ONE task per response
- Analyze each result before deciding next step
- Build understanding progressively
- Signal completion when ready for synthesis
- Be thorough but efficient

Continue exploring until you can provide a complete architectural understanding.`

	return llm.Message{
		Role:      "system",
		Content:   content,
		Timestamp: time.Now(),
	}
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
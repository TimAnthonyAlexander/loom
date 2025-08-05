package services

import (
	"context"
	"fmt"
	"loom/chat"
	"loom/config"
	contextMgr "loom/context"
	"loom/indexer"
	"loom/llm"
	"loom/memory"
	"loom/shared/events"
	"loom/shared/models"
	taskPkg "loom/task"
	"loom/todo"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ChatService handles chat operations and LLM interactions with full Loom functionality
type ChatService struct {
	session       *chat.Session
	llmAdapter    llm.LLMAdapter
	eventBus      *events.EventBus
	streamChan    chan llm.StreamChunk
	streamCancel  context.CancelFunc
	isStreaming   bool
	workspacePath string
	mutex         sync.RWMutex

	// Core Loom components
	config            *config.Config
	index             *indexer.Index
	taskManager       *taskPkg.Manager
	enhancedManager   *taskPkg.EnhancedManager
	sequentialManager *taskPkg.SequentialTaskManager
	taskExecutor      *taskPkg.Executor
	memoryStore       *memory.MemoryStore
	todoManager       *todo.TodoManager
	promptEnhancer    *llm.PromptEnhancer

	// Context management
	contextManager   *contextMgr.ContextManager
	maxContextTokens int

	// Task execution tracking
	currentExecution  *taskPkg.TaskExecution
	taskEventChan     chan taskPkg.TaskExecutionEvent
	userTaskEventChan chan taskPkg.UserTaskEvent
}

// NewChatService creates a new chat service with full Loom integration
func NewChatService(workspacePath string, llmAdapter llm.LLMAdapter, eventBus *events.EventBus, cfg *config.Config, idx *indexer.Index) *ChatService {
	session := chat.NewSession(workspacePath, 50) // Max 50 messages

	// Initialize core Loom components following TUI pattern
	memoryStore := memory.NewMemoryStore(workspacePath)
	todoManager := todo.NewTodoManager(workspacePath)
	taskExecutor := taskPkg.NewExecutor(workspacePath, cfg.EnableShell, 10*1024*1024) // 10MB max file size

	// Memory and todo managers are already set in the task executor

	// Initialize task managers (following TUI pattern)
	var taskManager *taskPkg.Manager
	var enhancedManager *taskPkg.EnhancedManager
	var sequentialManager *taskPkg.SequentialTaskManager
	var taskEventChan chan taskPkg.TaskExecutionEvent
	var userTaskEventChan chan taskPkg.UserTaskEvent

	if llmAdapter != nil {
		enhancedManager = taskPkg.NewEnhancedManager(taskExecutor, llmAdapter, session, idx)
		taskManager = enhancedManager.Manager // For compatibility
		sequentialManager = taskPkg.NewSequentialTaskManager(taskExecutor, llmAdapter, session)
		taskEventChan = make(chan taskPkg.TaskExecutionEvent, 10)
		userTaskEventChan = make(chan taskPkg.UserTaskEvent, 10)
	}

	// Get max context tokens from config or use default
	maxContextTokens := 6000 // Default
	if tokenValue, err := cfg.Get("max_context_tokens"); err == nil {
		if tokenInt, ok := tokenValue.(int); ok && tokenInt > 0 {
			maxContextTokens = tokenInt
		}
	}

	// Set context manager for optimized context
	if sequentialManager != nil {
		sequentialManager.SetContextManager(idx, maxContextTokens)
	}

	// Create context manager
	contextManager := contextMgr.NewContextManager(idx, maxContextTokens)

	// Initialize prompt enhancer
	promptEnhancer := llm.NewPromptEnhancer(workspacePath, idx)
	promptEnhancer.SetMemoryStore(memoryStore)
	promptEnhancer.SetTodoManager(todoManager)

	cs := &ChatService{
		session:           session,
		llmAdapter:        llmAdapter,
		eventBus:          eventBus,
		workspacePath:     workspacePath,
		config:            cfg,
		index:             idx,
		taskManager:       taskManager,
		enhancedManager:   enhancedManager,
		sequentialManager: sequentialManager,
		taskExecutor:      taskExecutor,
		memoryStore:       memoryStore,
		todoManager:       todoManager,
		promptEnhancer:    promptEnhancer,
		contextManager:    contextManager,
		maxContextTokens:  maxContextTokens,
		taskEventChan:     taskEventChan,
		userTaskEventChan: userTaskEventChan,
	}

	// Add enhanced system prompt if this is a new session (no previous messages)
	if len(session.GetMessages()) == 0 {
		systemPrompt := promptEnhancer.CreateEnhancedSystemPrompt(cfg.EnableShell)
		if err := session.AddMessage(systemPrompt); err != nil {
			fmt.Printf("Warning: failed to add system prompt: %v\n", err)
		}
	}

	// Start task event handlers
	if taskEventChan != nil {
		go cs.handleTaskEvents()
	}
	if userTaskEventChan != nil {
		go cs.handleUserTaskEvents()
	}

	return cs
}

// GetChatState returns the current chat state
func (cs *ChatService) GetChatState() models.ChatState {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	// Convert chat session messages to models.Message (excluding system messages)
	var messages []models.Message
	for _, msg := range cs.session.GetMessages() {
		// Skip system messages - they're for LLM context, not user display
		if msg.Role == "system" {
			continue
		}

		// Apply task result filtering for assistant messages (same as TUI)
		content := msg.Content
		if msg.Role == "assistant" {
			content = cs.session.FilterTaskResultForDisplay(content)
		}

		messages = append(messages, models.Message{
			ID:        uuid.New().String(),
			Content:   content,
			IsUser:    msg.Role == "user",
			Timestamp: time.Now(), // Note: chat.Message doesn't have timestamp, using current time
			Type:      msg.Role,
		})
	}

	return models.ChatState{
		Messages:      messages,
		IsStreaming:   cs.isStreaming,
		SessionID:     "current-session", // Note: Session struct doesn't expose ID
		WorkspacePath: cs.getWorkspacePath(),
	}
}

// SendMessage sends a user message with full Loom processing including task execution
func (cs *ChatService) SendMessage(content string) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// Don't start new streaming if already streaming
	if cs.isStreaming {
		return fmt.Errorf("already streaming a response")
	}

	// Process @file mentions to include file content (like TUI)
	processedInput := cs.processFileMentions(content)

	// Add user message to session with processed content for LLM
	userMessage := llm.Message{
		Role:      "user",
		Content:   processedInput,
		Timestamp: time.Now(),
	}

	if err := cs.session.AddMessage(userMessage); err != nil {
		return fmt.Errorf("failed to save user message: %w", err)
	}

	// Reset completion detection state for new user queries (like TUI)
	// Reset any previous exploration state

	// Emit user message event (show original content to user, not processed)
	cs.eventBus.EmitChatMessage(models.Message{
		ID:        uuid.New().String(),
		Content:   content,
		IsUser:    true,
		Timestamp: time.Now(),
		Type:      "user",
	})

	// Check for exploration queries and start objective-driven exploration (like TUI)
	if cs.sequentialManager != nil && cs.isExplorationQuery(content) {
		// Start objective exploration
		cs.sequentialManager.StartObjectiveExploration(content)
		// Note: The sequential manager will use its own system message during exploration
	}

	// Start LLM streaming with task execution
	go cs.streamLLMResponseWithTasks()

	return nil
}

// streamLLMResponseWithTasks handles streaming LLM responses with task execution (following TUI pattern)
func (cs *ChatService) streamLLMResponseWithTasks() {
	cs.mutex.Lock()
	cs.isStreaming = true
	cs.streamChan = make(chan llm.StreamChunk, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	cs.streamCancel = cancel
	cs.mutex.Unlock()

	// Emit stream started event
	cs.eventBus.Emit(events.ChatStreamStarted, nil)

	// Get messages for LLM with optimized context (like TUI)
	var messages []llm.Message
	var err error

	// Use optimized context instead of full history
	messages, err = cs.session.GetOptimizedContextMessages(cs.contextManager, cs.maxContextTokens)
	if err != nil {
		cs.handleStreamError(fmt.Errorf("context optimization error: %w", err))
		return
	}

	// Channel to signal when streaming goroutine is done
	streamDone := make(chan bool, 1)

	// Start streaming
	go func() {
		defer func() {
			streamDone <- true
		}()

		// Use the Stream method from LLMAdapter
		err := cs.llmAdapter.Stream(ctx, messages, cs.streamChan)
		if err != nil {
			cs.eventBus.Emit(events.ChatError, map[string]string{
				"error": err.Error(),
			})
		}
	}()

	// Process streaming chunks
	var fullResponse string
	streamFinished := false

	// Use select to handle both channel reads and cancellation
	for !streamFinished {
		select {
		case chunk, ok := <-cs.streamChan:
			if !ok {
				streamFinished = true
				continue
			}

			if chunk.Error != nil {
				cs.eventBus.Emit(events.ChatError, map[string]string{
					"error": chunk.Error.Error(),
				})
				streamFinished = true
				continue
			}

			fullResponse += chunk.Content

			// Emit stream chunk event
			cs.eventBus.Emit(events.ChatStreamChunk, map[string]string{
				"content": chunk.Content,
				"full":    fullResponse,
			})

			if chunk.Done {
				streamFinished = true
				continue
			}
		case <-ctx.Done():
			streamFinished = true
		}
	}

	// Wait for the streaming goroutine to finish cleanup
	<-streamDone

	// Process the complete response
	if fullResponse != "" {
		// Filter potentially misleading status messages (like TUI)
		filteredContent := cs.filterMisleadingStatusMessages(fullResponse)

		// Add assistant response to session
		assistantMessage := llm.Message{
			Role:      "assistant",
			Content:   filteredContent,
			Timestamp: time.Now(),
		}
		if err := cs.session.AddMessage(assistantMessage); err != nil {
			fmt.Printf("Warning: failed to save assistant message: %v\n", err)
		}

		// Apply task result filtering for user display (same as TUI)
		displayContent := cs.session.FilterTaskResultForDisplay(filteredContent)

		// Emit final message event
		cs.eventBus.EmitChatMessage(models.Message{
			ID:        uuid.New().String(),
			Content:   displayContent,
			IsUser:    false,
			Timestamp: time.Now(),
			Type:      "assistant",
		})

		// Process LLM response for tasks (like TUI - this is the key integration!)
		go cs.handleLLMResponseForTasks(fullResponse)
	}

	cs.mutex.Lock()
	cs.isStreaming = false
	cs.streamCancel = nil
	cs.streamChan = nil
	cs.mutex.Unlock()

	// Emit stream ended event
	cs.eventBus.Emit(events.ChatStreamEnded, nil)
}

// StopStreaming cancels the current streaming operation
func (cs *ChatService) StopStreaming() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	if cs.streamCancel != nil {
		cs.streamCancel()
	}
	// Note: Don't set streamCancel to nil here - let streamLLMResponse clean it up
}

// AddSystemMessage adds a system message to the chat (for LLM context only, not displayed to user)
func (cs *ChatService) AddSystemMessage(content string) {
	systemMessage := llm.Message{
		Role:    "system",
		Content: content,
	}
	cs.session.AddMessage(systemMessage)

	// Note: Don't emit system messages as chat events - they're for LLM context only
	// and should not be displayed in the GUI chat interface
}

// ClearChat clears all messages from the chat session
func (cs *ChatService) ClearChat() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// Create new session to clear messages
	cs.session = chat.NewSession(cs.getWorkspacePath(), 50)
}

// UpdateLLMAdapter updates the LLM adapter and reinitializes dependent components
func (cs *ChatService) UpdateLLMAdapter(newAdapter llm.LLMAdapter) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	cs.llmAdapter = newAdapter

	// Reinitialize task managers if LLM adapter is available
	if newAdapter != nil {
		cs.enhancedManager = taskPkg.NewEnhancedManager(cs.taskExecutor, newAdapter, cs.session, cs.index)
		cs.taskManager = cs.enhancedManager.Manager
		cs.sequentialManager = taskPkg.NewSequentialTaskManager(cs.taskExecutor, newAdapter, cs.session)

		// Recreate event channels if needed
		if cs.taskEventChan == nil {
			cs.taskEventChan = make(chan taskPkg.TaskExecutionEvent, 10)
			go cs.handleTaskEvents()
		}
		if cs.userTaskEventChan == nil {
			cs.userTaskEventChan = make(chan taskPkg.UserTaskEvent, 10)
			go cs.handleUserTaskEvents()
		}

		// Set context manager for the new sequential manager
		if cs.sequentialManager != nil {
			cs.sequentialManager.SetContextManager(cs.index, cs.maxContextTokens)
		}
	} else {
		// Clear task managers if no LLM adapter
		cs.enhancedManager = nil
		cs.taskManager = nil
		cs.sequentialManager = nil
	}
}

// GetMessages returns all chat messages
func (cs *ChatService) GetMessages() []models.Message {
	state := cs.GetChatState()
	return state.Messages
}

// getWorkspacePath returns the workspace path
func (cs *ChatService) getWorkspacePath() string {
	return cs.workspacePath
}

// processFileMentions processes @file mentions to include file content (from TUI)
func (cs *ChatService) processFileMentions(input string) string {
	// Simple implementation for @file mentions
	// Find @filename patterns and replace with file content
	re := regexp.MustCompile(`@([^\s]+)`)
	matches := re.FindAllStringSubmatch(input, -1)

	result := input
	for _, match := range matches {
		filename := match[1]
		if content, err := cs.readFileSnippet(filename, 50); err == nil {
			replacement := fmt.Sprintf("File content for %s:\n```\n%s\n```", filename, content)
			result = strings.Replace(result, match[0], replacement, 1)
		}
	}

	return result
}

// readFileSnippet reads a snippet of a file (from TUI)
func (cs *ChatService) readFileSnippet(filename string, lineCount int) (string, error) {
	// Use the file service or indexer to read file content
	// This is a simplified implementation
	if cs.index != nil {
		// Try to read using indexer for safety
		for path := range cs.index.Files {
			if strings.HasSuffix(path, filename) {
				// Read file content (simplified)
				task := &taskPkg.Task{
					Type:     taskPkg.TaskTypeReadFile,
					Path:     path,
					MaxLines: lineCount,
				}
				response := cs.taskExecutor.Execute(task)
				if response.Success {
					return response.ActualContent, nil
				}
			}
		}
	}
	return "", fmt.Errorf("file not found: %s", filename)
}

// filterMisleadingStatusMessages filters potentially misleading status messages (from TUI)
func (cs *ChatService) filterMisleadingStatusMessages(content string) string {
	// Simple filtering - remove common misleading patterns
	filtered := content
	misleadingPatterns := []string{
		"I'll help you with that",
		"Let me read the file",
		"I'm reading the file",
		"Reading file...",
	}

	for _, pattern := range misleadingPatterns {
		filtered = strings.Replace(filtered, pattern, "", -1)
	}

	return strings.TrimSpace(filtered)
}

// handleStreamError handles streaming errors
func (cs *ChatService) handleStreamError(err error) {
	cs.mutex.Lock()
	cs.isStreaming = false
	cs.streamCancel = nil
	cs.streamChan = nil
	cs.mutex.Unlock()

	cs.eventBus.Emit(events.ChatError, map[string]string{
		"error": err.Error(),
	})
}

// handleLLMResponseForTasks processes the LLM response for task execution (key integration from TUI)
func (cs *ChatService) handleLLMResponseForTasks(llmResponse string) {
	// Handle objective-driven exploration (like TUI)
	if cs.sequentialManager != nil && cs.sequentialManager.IsExploring() {
		cs.handleObjectiveExploration(llmResponse)
		return
	}

	// For non-exploration requests, use standard task management
	if cs.taskManager != nil {
		execution, err := cs.taskManager.HandleLLMResponse(llmResponse, cs.userTaskEventChan, cs.taskEventChan)
		if err != nil {
			cs.eventBus.Emit(events.ChatError, map[string]string{
				"error": fmt.Sprintf("Failed to handle LLM response: %v", err),
			})
			return
		}

		if execution != nil {
			cs.mutex.Lock()
			cs.currentExecution = execution
			cs.mutex.Unlock()
			// Tasks were executed - continue LLM conversation automatically!
			cs.continueLLMAfterTasks()
			return
		}
	}

	// No tasks found - check for auto-continuation based on content
	if !cs.isInformationalResponse(llmResponse) {
		// This might be a text-only response that should trigger completion checking
		// Similar to TUI's AutoContinueMsg pattern, but we'll handle it differently in GUI
	}
}

// handleObjectiveExploration manages the objective-driven exploration flow (from TUI)
func (cs *ChatService) handleObjectiveExploration(llmResponse string) {
	// Check if this is setting a new objective
	if objective := cs.sequentialManager.ExtractObjective(llmResponse); objective != "" {
		cs.sequentialManager.SetObjective(objective)
		// Note: In GUI, we don't show the objective message directly like TUI does
		// The frontend can display exploration status differently
	}

	// Check if objective is complete
	if cs.sequentialManager.IsObjectiveComplete(llmResponse) {
		cs.sequentialManager.CompleteObjective()
		// This is the final synthesis - don't continue automatically
		return
	}

	// Parse and execute task
	task, _, err := cs.sequentialManager.ParseSingleTask(llmResponse)
	if err != nil {
		cs.eventBus.Emit(events.ChatError, map[string]string{
			"error": fmt.Sprintf("Failed to parse task: %v", err),
		})
		return
	}

	if task != nil {
		// Execute the task
		response := cs.taskExecutor.Execute(task)

		// Add task result to accumulated data
		cs.sequentialManager.AddTaskResult(*response)

		// Add task result to chat session for LLM to see (hidden from user display)
		taskResultMsg := cs.formatTaskResultForLLM(task, response)
		if err := cs.session.AddMessage(taskResultMsg); err != nil {
			fmt.Printf("Warning: failed to add task result to chat: %v\n", err)
		}

		// Continue the conversation for next step (key part!)
		cs.continueLLMAfterTasks()
		return
	}

	// No task found - regular response, exploration complete
}

// handleTaskEvents processes task execution events and forwards them to the frontend
func (cs *ChatService) handleTaskEvents() {
	for event := range cs.taskEventChan {
		// Convert task event to GUI event and emit
		cs.eventBus.Emit(events.TaskStatusChanged, map[string]interface{}{
			"type":           event.Type,
			"message":        event.Message,
			"task":           event.Task,
			"response":       event.Response,
			"requires_input": event.RequiresInput,
		})

		// Handle task completion - add task results to chat for LLM context
		if event.Task != nil && event.Response != nil && event.Response.Success {
			// Add task result to chat session for LLM to see (hidden from user display)
			taskResultMsg := cs.formatTaskResultForLLM(event.Task, event.Response)
			if err := cs.session.AddMessage(taskResultMsg); err != nil {
				fmt.Printf("Warning: failed to add task result to chat: %v\n", err)
			}
		}

		// Handle confirmation requests
		if event.RequiresInput && event.Task != nil && event.Response != nil {
			// Emit task confirmation event for frontend to handle
			cs.eventBus.Emit(events.TaskConfirmationNeeded, map[string]interface{}{
				"task":     event.Task,
				"response": event.Response,
				"preview":  event.Response.Output,
			})
		}

		// Handle auto-continuation events (like TUI)
		if event.Type == "objective_change_auto_continue" ||
			event.Type == "auto_continue_after_no" ||
			event.Type == "auto_continue_completion_check" ||
			event.Type == "auto_continue_after_mixed_message" ||
			event.Type == "auto_continue_mixed_message_warning" {
			// Automatically continue the LLM conversation
			cs.continueLLMAfterTasks()
		}

		// Automatically continue the LLM conversation once all tasks have finished
		// and there is no user confirmation required (like TUI)
		if event.Type == "execution_completed" && !event.RequiresInput {
			// All tasks in the current execution are done - continue LLM conversation
			cs.continueLLMAfterTasks()
		}
	}
}

// handleUserTaskEvents processes user task events and forwards simplified events to the frontend
func (cs *ChatService) handleUserTaskEvents() {
	for event := range cs.userTaskEventChan {
		// Emit simplified user task events to the frontend
		cs.eventBus.EmitUserTaskEvent(event)
	}
}

// formatTaskResultForLLM formats task results for LLM context (from TUI)
func (cs *ChatService) formatTaskResultForLLM(task *taskPkg.Task, response *taskPkg.TaskResponse) llm.Message {
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
		// Provide any output or diagnostic content to the LLM even when the
		// task fails so it can reason about the failure.
		if response.ActualContent != "" {
			content.WriteString(fmt.Sprintf("CONTENT:\n%s\n", response.ActualContent))
		} else if response.Output != "" {
			content.WriteString(fmt.Sprintf("CONTENT:\n%s\n", response.Output))
		}
	}

	return llm.Message{
		Role:      "system",
		Content:   content.String(),
		Timestamp: time.Now(),
	}
}

// continueLLMAfterTasks continues LLM conversation after task completion (from TUI)
func (cs *ChatService) continueLLMAfterTasks() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// Don't start new streaming if already streaming
	if cs.isStreaming {
		return
	}

	// Start a new LLM streaming session automatically
	go cs.streamLLMResponseWithTasks()
}

// isInformationalResponse checks if a response is purely informational (from TUI)
func (cs *ChatService) isInformationalResponse(response string) bool {
	// Check for common informational patterns
	informationalPatterns := []string{
		"I can help you",
		"Let me explain",
		"Here's what I found",
		"Based on my analysis",
		"To summarize",
		"In conclusion",
	}

	lowerResponse := strings.ToLower(response)
	for _, pattern := range informationalPatterns {
		if strings.Contains(lowerResponse, strings.ToLower(pattern)) {
			return true
		}
	}

	// Check if response contains any task patterns
	return cs.isTextOnlyResponse(response)
}

// isTextOnlyResponse checks if a response contains no tasks or commands (from TUI)
func (cs *ChatService) isTextOnlyResponse(response string) bool {
	// Check for common task patterns with emojis
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

	// Check for natural language task patterns at the beginning of lines
	naturalLangPatterns := []string{
		"READ ",
		"LIST ",
		"SEARCH ",
		"RUN ",
		"MEMORY ",
		"TODO ",
	}

	for _, pattern := range naturalLangPatterns {
		if regexp.MustCompile(`(?m)^` + pattern).MatchString(response) {
			return false
		}
	}

	// Look for LOOM_EDIT blocks
	if regexp.MustCompile(`(?s)>>LOOM_EDIT.*?<<LOOM_EDIT`).MatchString(response) {
		return false
	}

	return true
}

// isExplorationQuery determines if a user query should use objective-driven exploration (from TUI)
func (cs *ChatService) isExplorationQuery(userInput string) bool {
	// Patterns that suggest the user wants a comprehensive exploration
	explorationPatterns := []string{
		"explain this project",
		"what does this codebase do",
		"analyze this code",
		"understand this project",
		"explore this codebase",
		"tell me about this project",
		"how does this work",
		"what is this project",
		"analyze the codebase",
		"explain the architecture",
	}

	lowerInput := strings.ToLower(userInput)
	for _, pattern := range explorationPatterns {
		if strings.Contains(lowerInput, pattern) {
			return true
		}
	}

	return false
}

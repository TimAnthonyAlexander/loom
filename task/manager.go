package task

import (
	"context"
	"fmt"
	"loom/llm"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Manager orchestrates task execution and recursive chat loops
type Manager struct {
	executor           *Executor
	llmAdapter         llm.LLMAdapter
	chatSession        ChatSession
	completionDetector *CompletionDetector // Add completion detector for objective tracking
}

// ChatSession interface for managing chat history
type ChatSession interface {
	AddMessage(message llm.Message) error
	GetMessages() []llm.Message
}

// TaskExecution represents an execution session with multiple tasks
type TaskExecution struct {
	Tasks     []Task
	Responses []TaskResponse
	StartTime time.Time
	EndTime   time.Time
	Status    string // "running", "completed", "failed", "cancelled"
}

// TaskExecutionEvent represents events during task execution
type TaskExecutionEvent struct {
	Type          string // "task_started", "task_completed", "task_failed", "execution_completed"
	Task          *Task
	Response      *TaskResponse
	Execution     *TaskExecution
	Message       string
	RequiresInput bool // For confirmations
}

// NewManager creates a new task manager
func NewManager(executor *Executor, llmAdapter llm.LLMAdapter, chatSession ChatSession) *Manager {
	return &Manager{
		executor:           executor,
		llmAdapter:         llmAdapter,
		chatSession:        chatSession,
		completionDetector: NewCompletionDetector(), // Initialize completion detector
	}
}

// HandleLLMResponse processes an LLM response and executes any tasks found
// userEventChan receives simplified events for UI display
// detailedEventChan receives full events for internal LLM processing (can be nil)
func (m *Manager) HandleLLMResponse(llmResponse string, userEventChan chan<- UserTaskEvent, detailedEventChan chan<- TaskExecutionEvent) (*TaskExecution, error) {
	// Check if this sets a new objective
	newObjective := m.completionDetector.ExtractObjective(llmResponse)
	isNewObjectiveSetting := newObjective != "" && !m.completionDetector.objectiveSet

	// STEP 1: Validate objective consistency
	objectiveValidation := m.completionDetector.ValidateObjectiveConsistency(llmResponse)

	// If objective changed, send warning and request correction
	if !objectiveValidation.IsValid {
		warningMessage := m.completionDetector.FormatObjectiveWarning(objectiveValidation)

		// Add warning to chat to redirect LLM back to original objective
		correctionMessage := llm.Message{
			Role:      "system",
			Content:   warningMessage,
			Timestamp: time.Now(),
		}

		if err := m.chatSession.AddMessage(correctionMessage); err != nil {
			fmt.Printf("Warning: failed to add objective correction to chat: %v\n", err)
		}

		// Add follow-up instruction to continue with original objective
		continuationMessage := llm.Message{
			Role:      "user",
			Content:   fmt.Sprintf("Please continue working on your original objective: \"%s\". Focus on completing this objective rather than expanding the scope.", objectiveValidation.OriginalObjective),
			Timestamp: time.Now(),
		}

		if err := m.chatSession.AddMessage(continuationMessage); err != nil {
			fmt.Printf("Warning: failed to add continuation message to chat: %v\n", err)
		}

		// Create a special event to notify the UI about objective change and trigger auto-continuation
		userEventChan <- UserTaskEvent{
			Type:      "failed",
			Message:   "Objective changed - refocusing on original goal",
			TaskType:  "objective_management",
			Timestamp: time.Now(),
		}

		// Also send detailed event for internal processing if channel exists
		if detailedEventChan != nil {
			detailedEventChan <- TaskExecutionEvent{
				Type:    "objective_change_auto_continue",
				Message: fmt.Sprintf("Objective change detected. Redirecting LLM back to original objective: %s", objectiveValidation.OriginalObjective),
			}
		}

		// Return nil instead of error to allow conversation to continue
		return nil, nil
	}

	// Flag to track if this response is setting an objective for the first time
	if isNewObjectiveSetting {
		// Update the lastCompletionCheckSent flag to prevent immediate completion check
		m.completionDetector.lastCompletionCheckSent = false
	}

	// STEP 2: Parse tasks from LLM response (original logic)
	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tasks: %w", err)
	}

	// If no tasks found, this is a regular chat response
	if taskList == nil || len(taskList.Tasks) == 0 {
		// STEP 2A: Handle YES/NO completion check responses
		if m.completionDetector.IsYesNoCompletionResponse(llmResponse) {
			isComplete, shouldContinue := m.completionDetector.ParseYesNoResponse(llmResponse)

			if isComplete {
				// LLM said YES - objective complete, hand control to user
				if userEventChan != nil {
					userEventChan <- UserTaskEvent{
						Type:      "completed",
						Message:   "Objective completed",
						TaskType:  "completion_check",
						Timestamp: time.Now(),
					}
				}
				return nil, nil
			} else if shouldContinue {
				// LLM said NO - send continuation prompt
				continuationPrompt := m.completionDetector.GenerateContinuationPrompt()

				continuationMessage := llm.Message{
					Role:      "user",
					Content:   continuationPrompt,
					Timestamp: time.Now(),
				}

				if err := m.chatSession.AddMessage(continuationMessage); err != nil {
					return nil, fmt.Errorf("failed to add continuation message: %w", err)
				}

				if userEventChan != nil {
					userEventChan <- UserTaskEvent{
						Type:      "started",
						Message:   "Continuing with objective...",
						TaskType:  "completion_check",
						Timestamp: time.Now(),
					}
				}

				// Trigger auto-continuation by creating a special event
				if detailedEventChan != nil {
					detailedEventChan <- TaskExecutionEvent{
						Type:    "auto_continue_after_no",
						Message: "LLM indicated more work needed, continuing automatically",
					}
				}

				return nil, nil
			}
		}

		// STEP 2B: Check if we should send a YES/NO completion check
		if m.completionDetector.ShouldSendYesNoCheck() {
			yesNoPrompt := m.completionDetector.GenerateYesNoCompletionCheckPrompt()

			yesNoMessage := llm.Message{
				Role:      "user",
				Content:   yesNoPrompt,
				Timestamp: time.Now(),
			}

			if err := m.chatSession.AddMessage(yesNoMessage); err != nil {
				return nil, fmt.Errorf("failed to add YES/NO completion check: %w", err)
			}

			if userEventChan != nil {
				userEventChan <- UserTaskEvent{
					Type:      "started",
					Message:   "Checking if objective is complete...",
					TaskType:  "completion_check",
					Timestamp: time.Now(),
				}
			}

			// Trigger auto-continuation by creating a special event
			if detailedEventChan != nil {
				detailedEventChan <- TaskExecutionEvent{
					Type:    "auto_continue_completion_check",
					Message: "Sent YES/NO completion check, awaiting LLM response",
				}
			}

			return nil, nil
		}

		// No completion check needed - regular text response, hand control to user
		return nil, nil
	}

	// STEP 3: Execute tasks (original logic continues...)
	// Create execution session
	execution := &TaskExecution{
		Tasks:     taskList.Tasks,
		Responses: make([]TaskResponse, 0, len(taskList.Tasks)),
		StartTime: time.Now(),
		Status:    "running",
	}

	// Execute tasks sequentially
	for i, task := range execution.Tasks {
		// Create a copy of the task to avoid loop variable capture issues
		currentTask := task
		taskID := uuid.New().String()

		// Send simplified user event with task-specific messaging
		var userMessage string
		if currentTask.Type == TaskTypeReadFile {
			userMessage = fmt.Sprintf("📖 Reading %s...", currentTask.Path)
		} else {
			userMessage = fmt.Sprintf("Executing %s...", m.getSimpleTaskDescription(&currentTask))
		}

		userEventChan <- UserTaskEvent{
			TaskID:      taskID,
			Type:        "started",
			Message:     userMessage,
			TaskType:    string(currentTask.Type),
			Description: m.getSimpleTaskDescription(&currentTask),
			Progress:    float64(i) / float64(len(execution.Tasks)),
			Timestamp:   time.Now(),
		}

		// Send detailed event for internal processing if channel exists
		if detailedEventChan != nil {
			detailedEventChan <- TaskExecutionEvent{
				Type:      "task_started",
				Task:      &currentTask,
				Execution: execution,
				Message:   fmt.Sprintf("Executing task %d/%d: %s", i+1, len(execution.Tasks), currentTask.Description()),
			}
		}

		// Execute the task
		response := m.executor.Execute(&currentTask)
		execution.Responses = append(execution.Responses, *response)

		if !response.Success {
			// Send simplified user event for task failure
			userEventChan <- UserTaskEvent{
				TaskID:      taskID,
				Type:        "failed",
				Message:     fmt.Sprintf("Failed: %s", m.getSimpleTaskDescription(&currentTask)),
				TaskType:    string(currentTask.Type),
				Description: m.getSimpleTaskDescription(&currentTask),
				Progress:    float64(i+1) / float64(len(execution.Tasks)),
				Timestamp:   time.Now(),
			}

			// Send detailed event for internal processing if channel exists
			if detailedEventChan != nil {
				detailedEventChan <- TaskExecutionEvent{
					Type:      "task_failed",
					Task:      &currentTask,
					Response:  response,
					Execution: execution,
					Message:   fmt.Sprintf("Task failed: %s", response.Error),
				}
			}

			// CRITICAL FIX: Add failed task result to chat so LLM can see the error
			taskResultMessage := llm.Message{
				Role:      "assistant",
				Content:   m.formatTaskResultForLLM(&currentTask, response),
				Timestamp: time.Now(),
			}

			if err := m.chatSession.AddMessage(taskResultMessage); err != nil {
				fmt.Printf("Warning: failed to add failed task result to chat: %v\n", err)
			}

			// Continue with other tasks or stop based on configuration
			continue
		}

		// Send simplified user event for task completion with task-specific messaging
		var completionMessage string
		if currentTask.Type == TaskTypeReadFile && response.Success {
			// Extract line count from output for user feedback
			if strings.Contains(response.Output, " lines)") {
				completionMessage = fmt.Sprintf("✅ Read %s successfully", currentTask.Path)
			} else {
				completionMessage = response.Output // Use the formatted output message
			}
		} else {
			completionMessage = fmt.Sprintf("Completed: %s", m.getSimpleTaskDescription(&currentTask))
		}

		userEventChan <- UserTaskEvent{
			TaskID:      taskID,
			Type:        "completed",
			Message:     completionMessage,
			TaskType:    string(currentTask.Type),
			Description: m.getSimpleTaskDescription(&currentTask),
			Progress:    float64(i+1) / float64(len(execution.Tasks)),
			Timestamp:   time.Now(),
		}

		// Send detailed event for internal processing if channel exists
		if detailedEventChan != nil {
			detailedEventChan <- TaskExecutionEvent{
				Type:      "task_completed",
				Task:      &currentTask,
				Response:  response,
				Execution: execution,
				Message:   fmt.Sprintf("Task completed: %s", currentTask.Description()),
			}
		}

		// Add task result to chat context for next LLM iteration
		taskResultMessage := llm.Message{
			Role:      "assistant",
			Content:   m.formatTaskResultForLLM(&task, response),
			Timestamp: time.Now(),
		}

		if err := m.chatSession.AddMessage(taskResultMessage); err != nil {
			fmt.Printf("Warning: failed to add task result to chat: %v\n", err)
		}
	}

	// All tasks completed
	execution.EndTime = time.Now()
	execution.Status = "completed"

	// Send simplified user event for execution completion
	userEventChan <- UserTaskEvent{
		TaskID:      uuid.New().String(),
		Type:        "completed",
		Message:     fmt.Sprintf("All tasks completed (%d total)", len(execution.Tasks)),
		TaskType:    "execution_summary",
		Description: "Task execution finished",
		Progress:    1.0,
		Timestamp:   time.Now(),
	}

	// Send detailed event for internal processing if channel exists
	if detailedEventChan != nil {
		detailedEventChan <- TaskExecutionEvent{
			Type:      "execution_completed",
			Execution: execution,
			Message:   fmt.Sprintf("All tasks completed (%d total)", len(execution.Tasks)),
		}
	}

	return execution, nil
}

// ConfirmTask applies a confirmed destructive task and sends enhanced feedback to LLM
//
// BREAKING CHANGE: This method signature was changed to include taskResponse parameter
// to support enhanced edit summary feedback. The taskResponse should be the response
// from the original task execution that generated the confirmation request.
//
// Parameters:
//   - task: The task to confirm and apply
//   - taskResponse: The original response containing edit summary data (required for enhanced feedback)
//   - approve: Whether the user approved the task
func (m *Manager) ConfirmTask(task *Task, taskResponse *TaskResponse, approve bool) error {
	if !approve {
		// Send cancellation feedback to LLM
		cancelMessage := llm.Message{
			Role:      "assistant",
			Content:   m.FormatConfirmationResult(task, false, nil),
			Timestamp: time.Now(),
		}
		if err := m.chatSession.AddMessage(cancelMessage); err != nil {
			fmt.Printf("Warning: failed to add cancellation message to chat: %v\n", err)
		}
		return fmt.Errorf("task cancelled by user")
	}

	var applyError error
	if task.Type == TaskTypeEditFile {
		applyError = m.executor.ApplyEditWithConfirmation(task)
	} else if task.Type == TaskTypeRunShell {
		// Check if this is an interactive shell command that needs real execution
		if task.Interactive && task.AllowUserInput {
			// For real interactive commands, we need to execute them now with user interaction
			// This would require extending the TUI to handle real-time input
			// For now, provide feedback that interactive execution is prepared
			applyError = nil
		}
		// For non-interactive or auto-interactive commands, they were already executed in the preview
		applyError = nil
	} else {
		applyError = fmt.Errorf("task type %s does not require confirmation", task.Type)
	}

	// Send enhanced confirmation result to LLM with EditSummary data
	confirmationMessage := llm.Message{
		Role:      "assistant",
		Content:   m.FormatConfirmationResultEnhanced(task, taskResponse, approve, applyError),
		Timestamp: time.Now(),
	}

	if err := m.chatSession.AddMessage(confirmationMessage); err != nil {
		fmt.Printf("Warning: failed to add confirmation result to chat: %v\n", err)
	}

	return applyError
}

// ApplyInteractiveTask applies an interactive task with user input channel
func (m *Manager) ApplyInteractiveTask(task *Task, userInputChannel chan string) error {
	if task.Type != TaskTypeRunShell {
		return fmt.Errorf("interactive tasks are only supported for shell commands")
	}

	if !task.Interactive {
		return fmt.Errorf("task is not configured as interactive")
	}

	// Use the interactive executor directly
	return m.executor.interactiveExecutor.ApplyInteractiveTask(task, userInputChannel)
}

// ContinueRecursiveChat continues the chat loop with LLM after task completion
func (m *Manager) ContinueRecursiveChat(ctx context.Context, execution *TaskExecution) error {
	// Create a summary of completed tasks
	summary := m.createTaskSummary(execution)

	// Add summary to chat
	summaryMessage := llm.Message{
		Role:      "assistant",
		Content:   summary,
		Timestamp: time.Now(),
	}

	if err := m.chatSession.AddMessage(summaryMessage); err != nil {
		return fmt.Errorf("failed to add task summary to chat: %w", err)
	}

	// Get updated message history
	messages := m.chatSession.GetMessages()

	// Send to LLM to continue the conversation
	response, err := m.llmAdapter.Send(ctx, messages)
	if err != nil {
		return fmt.Errorf("failed to get LLM response: %w", err)
	}

	// Add LLM response to chat
	if err := m.chatSession.AddMessage(*response); err != nil {
		return fmt.Errorf("failed to add LLM response to chat: %w", err)
	}

	// Check if LLM wants to perform more tasks
	if taskList, _ := ParseTasks(response.Content); taskList != nil && len(taskList.Tasks) > 0 {
		// More tasks to execute - this would trigger another execution cycle
		return fmt.Errorf("recursive task execution detected - implement cycle detection")
	}

	return nil
}

// formatTaskResult formats a task result for the chat context (user-friendly display)
func (m *Manager) formatTaskResult(task *Task, response *TaskResponse) string {
	var result strings.Builder

	// For ReadFile tasks, show minimal user-friendly messages
	if task.Type == TaskTypeReadFile {
		if response.Success {
			result.WriteString(fmt.Sprintf("✅ %s\n", response.Output))
		} else {
			result.WriteString(fmt.Sprintf("❌ Failed to read %s\n", task.Path))
			if response.Error != "" {
				result.WriteString(fmt.Sprintf("💥 Error: %s\n", response.Error))
			}
		}
		return result.String()
	}

	// For other task types, use the existing detailed format
	result.WriteString(fmt.Sprintf("🔧 Task Result: %s\n", task.Description()))

	if response.Success {
		result.WriteString("✅ Status: Success\n")
		if response.Approved {
			result.WriteString("👍 User approved changes\n")
		}
		// Show command or operation output (trimmed to avoid flooding chat)
		if response.Output != "" {
			trimmed := response.Output
			const maxLen = 500
			if len(trimmed) > maxLen {
				trimmed = trimmed[:maxLen] + "… (truncated)"
			}
			result.WriteString("📄 Output:\n")
			result.WriteString(trimmed + "\n")
		}
		// Add verification text for edit operations
		if response.VerificationText != "" {
			result.WriteString(response.VerificationText + "\n")
		}
	} else {
		result.WriteString("❌ Status: Failed\n")
		if response.Error != "" {
			result.WriteString(fmt.Sprintf("💥 Error: %s\n", response.Error))
		}
		if response.Output != "" {
			trimmed := response.Output
			const maxLen = 500
			if len(trimmed) > maxLen {
				trimmed = trimmed[:maxLen] + "… (truncated)"
			}
			result.WriteString("📄 Output (partial):\n")
			result.WriteString(trimmed + "\n")
		}
	}

	return result.String()
}

// formatTaskResultForLLM formats task results for LLM context (includes actual content)
func (m *Manager) formatTaskResultForLLM(task *Task, response *TaskResponse) string {
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

	return content.String()
}

// getSimpleTaskDescription returns a simplified, user-friendly description of a task
func (m *Manager) getSimpleTaskDescription(task *Task) string {
	switch task.Type {
	case TaskTypeReadFile:
		if task.Path != "" {
			return fmt.Sprintf("📖 reading %s", task.Path)
		}
		return "📖 reading file"
	case TaskTypeEditFile:
		if task.Path != "" {
			return fmt.Sprintf("editing %s", task.Path)
		}
		return "editing file"
	case TaskTypeRunShell:
		if task.Command != "" {
			// Truncate long commands for user display
			cmd := task.Command
			if len(cmd) > 50 {
				cmd = cmd[:47] + "..."
			}
			return fmt.Sprintf("running: %s", cmd)
		}
		return "running command"
	case TaskTypeListDir:
		if task.Path != "" {
			return fmt.Sprintf("listing %s", task.Path)
		}
		return "listing directory"
	case TaskTypeSearch:
		if task.Query != "" {
			return fmt.Sprintf("searching for: %s", task.Query)
		}
		return "searching files"
	case TaskTypeMemory:
		switch task.MemoryOperation {
		case "create":
			return "creating memory"
		case "update":
			return "updating memory"
		case "delete":
			return "deleting memory"
		case "get":
			return "retrieving memory"
		case "list":
			return "listing memories"
		default:
			return "managing memory"
		}
	case TaskTypeTodo:
		switch task.TodoOperation {
		case "create":
			return "creating todo items"
		case "check":
			return "checking todo item"
		case "uncheck":
			return "unchecking todo item"
		case "show":
			return "showing todo list"
		case "clear":
			return "clearing todo list"
		default:
			return "managing todo list"
		}
	default:
		return string(task.Type)
	}
}

// FormatConfirmationResult formats the result of a task confirmation for LLM feedback
func (m *Manager) FormatConfirmationResult(task *Task, approved bool, err error) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("🔧 Task Confirmation: %s\n", task.Description()))

	if !approved {
		result.WriteString("❌ Status: Cancelled by user\n")
		result.WriteString("📄 Result: Task was not applied\n")
	} else if err != nil {
		result.WriteString("❌ Status: Application failed\n")
		result.WriteString(fmt.Sprintf("💥 Error: %s\n", err.Error()))
		result.WriteString("📄 Result: Changes were not applied due to error\n")
	} else {
		result.WriteString("✅ Status: Successfully applied\n")
		result.WriteString("📄 Result: File has been modified as requested\n")
	}

	return result.String()
}

// FormatConfirmationResultEnhanced formats enhanced confirmation result with EditSummary data
func (m *Manager) FormatConfirmationResultEnhanced(task *Task, taskResponse *TaskResponse, approved bool, err error) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("🔧 Task Confirmation: %s\n", task.Description()))

	if !approved {
		result.WriteString("❌ Status: Cancelled by user\n")
		result.WriteString("📄 Result: Task was not applied\n")
	} else if err != nil {
		result.WriteString("❌ Status: Application failed\n")
		result.WriteString(fmt.Sprintf("💥 Error: %s\n", err.Error()))
		result.WriteString("📄 Result: Changes were not applied due to error\n")
	} else {
		result.WriteString("✅ Status: Successfully applied\n")

		// Include rich EditSummary data if available
		if taskResponse != nil && taskResponse.EditSummary != nil {
			editSummary := taskResponse.EditSummary
			result.WriteString(fmt.Sprintf("📊 Edit Summary: %s\n", editSummary.GetCompactSummary()))

			// Include detailed change information
			llmSummary := taskResponse.GetLLMSummary()
			if llmSummary != "" {
				result.WriteString(fmt.Sprintf("📈 Change Details:\n%s\n", llmSummary))
			}

			result.WriteString("📄 Result: File has been modified with the following changes:\n")
			result.WriteString(fmt.Sprintf("   • Edit Type: %s\n", editSummary.EditType))
			result.WriteString(fmt.Sprintf("   • Total Lines: %d\n", editSummary.TotalLines))

			if editSummary.EditType == "modify" {
				if editSummary.LinesAdded > 0 {
					result.WriteString(fmt.Sprintf("   • Lines Added: %d\n", editSummary.LinesAdded))
				}
				if editSummary.LinesRemoved > 0 {
					result.WriteString(fmt.Sprintf("   • Lines Removed: %d\n", editSummary.LinesRemoved))
				}
				if editSummary.LinesModified > 0 {
					result.WriteString(fmt.Sprintf("   • Lines Modified: %d\n", editSummary.LinesModified))
				}
			}

			if editSummary.Summary != "" {
				result.WriteString(fmt.Sprintf("   • Description: %s\n", editSummary.Summary))
			}
		} else {
			result.WriteString("📄 Result: File has been modified as requested\n")
		}
	}

	return result.String()
}

// createTaskSummary creates a summary of all tasks in an execution
func (m *Manager) createTaskSummary(execution *TaskExecution) string {
	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("📋 Task Execution Summary (%d tasks)\n", len(execution.Tasks)))
	summary.WriteString(fmt.Sprintf("⏱️  Duration: %v\n", execution.EndTime.Sub(execution.StartTime)))
	summary.WriteString(fmt.Sprintf("📊 Status: %s\n\n", execution.Status))

	successCount := 0
	for i, response := range execution.Responses {
		task := execution.Tasks[i]

		if response.Success {
			summary.WriteString("✅ ")
			successCount++
		} else {
			summary.WriteString("❌ ")
		}

		summary.WriteString(fmt.Sprintf("%s", task.Description()))

		if !response.Success && response.Error != "" {
			summary.WriteString(fmt.Sprintf(" - Error: %s", response.Error))
		}

		summary.WriteString("\n")
	}

	summary.WriteString(fmt.Sprintf("\n🏁 Results: %d successful, %d failed\n",
		successCount, len(execution.Tasks)-successCount))

	return summary.String()
}

// ResetObjectiveTracking resets objective tracking for a new conversation
func (m *Manager) ResetObjectiveTracking() {
	if m.completionDetector != nil {
		m.completionDetector.ResetObjective()
	}
}

// GetObjectiveStatus returns the current objective tracking status
func (m *Manager) GetObjectiveStatus() (string, bool, int) {
	if m.completionDetector != nil {
		return m.completionDetector.GetObjectiveStatus()
	}
	return "", false, 0
}

// IsObjectiveComplete checks if the current objective is complete
func (m *Manager) IsObjectiveComplete(response string) bool {
	if m.completionDetector != nil {
		return m.completionDetector.IsComplete(response)
	}
	return false
}

// AllowNewObjective allows setting a new objective (call after current one is complete)
func (m *Manager) AllowNewObjective() {
	if m.completionDetector != nil {
		m.completionDetector.ResetObjective()
	}
}

// GetTaskHistory returns formatted task history for display
func (m *Manager) GetTaskHistory(execution *TaskExecution) []string {
	var history []string

	for i, response := range execution.Responses {
		task := execution.Tasks[i]

		var status string
		if response.Success {
			status = "✅"
		} else {
			status = "❌"
		}

		entry := fmt.Sprintf("%s %s", status, task.Description())

		if !response.Success && response.Error != "" {
			entry += fmt.Sprintf(" - %s", response.Error)
		}

		history = append(history, entry)
	}

	return history
}

// IsTaskCompleted checks if a specific task in an execution is completed
func (execution *TaskExecution) IsTaskCompleted(taskIndex int) bool {
	if taskIndex >= len(execution.Responses) {
		return false
	}
	return execution.Responses[taskIndex].Success
}

// GetPendingTask returns the next task that requires confirmation
// REMOVED: No more confirmations - all tasks execute immediately
func (execution *TaskExecution) GetPendingTask() (*Task, *TaskResponse) {
	return nil, nil
}

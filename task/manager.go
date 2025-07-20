package task

import (
	"context"
	"fmt"
	"loom/llm"
	"strings"
	"time"
)

// Manager orchestrates task execution and recursive chat loops
type Manager struct {
	executor    *Executor
	llmAdapter  llm.LLMAdapter
	chatSession ChatSession
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
		executor:    executor,
		llmAdapter:  llmAdapter,
		chatSession: chatSession,
	}
}

// HandleLLMResponse processes an LLM response and executes any tasks found
func (m *Manager) HandleLLMResponse(llmResponse string, eventChan chan<- TaskExecutionEvent) (*TaskExecution, error) {
	// Parse tasks from LLM response
	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tasks: %w", err)
	}

	// If no tasks found, this is a regular chat response
	if taskList == nil || len(taskList.Tasks) == 0 {
		return nil, nil
	}

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

		// Send task started event
		eventChan <- TaskExecutionEvent{
			Type:      "task_started",
			Task:      &currentTask,
			Execution: execution,
			Message:   fmt.Sprintf("Executing task %d/%d: %s", i+1, len(execution.Tasks), currentTask.Description()),
		}

		// Execute the task
		response := m.executor.Execute(&currentTask)
		execution.Responses = append(execution.Responses, *response)

		if !response.Success {
			// Task failed
			eventChan <- TaskExecutionEvent{
				Type:      "task_failed",
				Task:      &currentTask,
				Response:  response,
				Execution: execution,
				Message:   fmt.Sprintf("Task failed: %s", response.Error),
			}

			// Continue with other tasks or stop based on configuration
			continue
		}

		// Check if task requires confirmation
		if currentTask.RequiresConfirmation() {
			eventChan <- TaskExecutionEvent{
				Type:          "task_completed",
				Task:          &currentTask,
				Response:      response,
				Execution:     execution,
				Message:       fmt.Sprintf("Task completed, awaiting confirmation: %s", currentTask.Description()),
				RequiresInput: true,
			}

			// Task execution is paused, waiting for user confirmation
			// The TUI will handle the confirmation and call ConfirmTask
			return execution, nil
		} else {
			// Task completed successfully
			eventChan <- TaskExecutionEvent{
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
			Content:   m.formatTaskResult(&task, response),
			Timestamp: time.Now(),
		}

		if err := m.chatSession.AddMessage(taskResultMessage); err != nil {
			fmt.Printf("Warning: failed to add task result to chat: %v\n", err)
		}
	}

	// All tasks completed
	execution.EndTime = time.Now()
	execution.Status = "completed"

	eventChan <- TaskExecutionEvent{
		Type:      "execution_completed",
		Execution: execution,
		Message:   fmt.Sprintf("All tasks completed (%d total)", len(execution.Tasks)),
	}

	return execution, nil
}

// ConfirmTask applies a confirmed destructive task
func (m *Manager) ConfirmTask(task *Task, approve bool) error {
	if !approve {
		return fmt.Errorf("task cancelled by user")
	}

	if task.Type == TaskTypeEditFile {
		return m.executor.ApplyEdit(task)
	} else if task.Type == TaskTypeRunShell {
		// For shell commands, they were already executed in the preview
		// This is just confirming the result
		return nil
	}

	return fmt.Errorf("task type %s does not require confirmation", task.Type)
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

// formatTaskResult formats a task result for the chat context
func (m *Manager) formatTaskResult(task *Task, response *TaskResponse) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("🔧 Task Result: %s\n", task.Description()))

	if response.Success {
		result.WriteString("✅ Status: Success\n")
		// Use ActualContent for LLM if available, otherwise fall back to Output
		if response.ActualContent != "" {
			result.WriteString(fmt.Sprintf("📄 Output:\n%s\n", response.ActualContent))
		} else if response.Output != "" {
			result.WriteString(fmt.Sprintf("📄 Output:\n%s\n", response.Output))
		}
		if response.Approved {
			result.WriteString("👍 User approved changes\n")
		}
	} else {
		result.WriteString("❌ Status: Failed\n")
		if response.Error != "" {
			result.WriteString(fmt.Sprintf("💥 Error: %s\n", response.Error))
		}
	}

	return result.String()
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
func (execution *TaskExecution) GetPendingTask() (*Task, *TaskResponse) {
	for i, response := range execution.Responses {
		task := execution.Tasks[i]
		if response.Success && task.RequiresConfirmation() && !response.Approved {
			// Return the updated task from the response, not the original task
			// This ensures we use the correct content that was prepared during execution
			return &response.Task, &response
		}
	}
	return nil, nil
}

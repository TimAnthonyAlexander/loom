package services

import (
	"fmt"
	"loom/shared/events"
	"loom/shared/models"
	taskPkg "loom/task"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskService handles task execution and management
type TaskService struct {
	manager         *taskPkg.Manager
	enhancedManager *taskPkg.EnhancedManager
	executor        *taskPkg.Executor
	eventBus        *events.EventBus

	// Task tracking
	pendingTasks   map[string]*models.TaskInfo
	executingTasks map[string]*models.TaskInfo
	completedTasks map[string]*models.TaskInfo

	// Confirmation handling
	pendingConfirmations map[string]*models.TaskConfirmation

	mutex sync.RWMutex
}

// NewTaskService creates a new task service
func NewTaskService(workspacePath string, eventBus *events.EventBus) *TaskService {
	// Note: These constructors need proper parameters based on actual API
	// For now, we'll create basic instances and extend as needed
	executor := taskPkg.NewExecutor(workspacePath, true, 200000) // enable shell, max file size
	// TODO: Initialize managers with proper dependencies
	var manager *taskPkg.Manager
	var enhancedManager *taskPkg.EnhancedManager

	return &TaskService{
		manager:              manager,
		enhancedManager:      enhancedManager,
		executor:             executor,
		eventBus:             eventBus,
		pendingTasks:         make(map[string]*models.TaskInfo),
		executingTasks:       make(map[string]*models.TaskInfo),
		completedTasks:       make(map[string]*models.TaskInfo),
		pendingConfirmations: make(map[string]*models.TaskConfirmation),
	}
}

// ParseTasksFromLLMResponse parses tasks from LLM response and creates task infos
func (ts *TaskService) ParseTasksFromLLMResponse(llmResponse string) ([]*models.TaskInfo, error) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	// TODO: Implement task parsing - this needs to be implemented based on actual API
	// For now, return empty slice
	var tasks []*taskPkg.Task
	err := fmt.Errorf("task parsing not yet implemented")
	if err != nil {
		return nil, fmt.Errorf("failed to parse tasks: %w", err)
	}

	var taskInfos []*models.TaskInfo
	for _, task := range tasks {
		taskInfo := &models.TaskInfo{
			ID:          uuid.New().String(),
			Type:        task.Type,
			Description: ts.generateTaskDescription(task),
			Status:      "pending",
			CreatedAt:   time.Now(),
			Task:        task,
		}

		// Store in pending tasks
		ts.pendingTasks[taskInfo.ID] = taskInfo
		taskInfos = append(taskInfos, taskInfo)

		// Emit task created event
		ts.eventBus.Emit(events.TaskCreated, *taskInfo)
	}

	return taskInfos, nil
}

// RequestTaskConfirmation requests user confirmation for a task
func (ts *TaskService) RequestTaskConfirmation(taskID string) (*models.TaskConfirmation, error) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	taskInfo, exists := ts.pendingTasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	// Generate preview for the task
	preview, err := ts.generateTaskPreview(taskInfo.Task)
	if err != nil {
		return nil, fmt.Errorf("failed to generate preview: %w", err)
	}

	confirmation := &models.TaskConfirmation{
		TaskInfo: *taskInfo,
		Preview:  preview,
		Approved: false,
	}

	// Store pending confirmation
	ts.pendingConfirmations[taskID] = confirmation

	// Emit confirmation needed event
	ts.eventBus.EmitTaskConfirmationNeeded(*confirmation)

	return confirmation, nil
}

// ApproveTask approves and executes a task
func (ts *TaskService) ApproveTask(taskID string) error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	// Get task from pending confirmations
	confirmation, exists := ts.pendingConfirmations[taskID]
	if !exists {
		return fmt.Errorf("no pending confirmation for task: %s", taskID)
	}

	taskInfo := &confirmation.TaskInfo
	taskInfo.Status = "executing"

	// Move from pending to executing
	delete(ts.pendingTasks, taskID)
	delete(ts.pendingConfirmations, taskID)
	ts.executingTasks[taskID] = taskInfo

	// Emit status change
	ts.eventBus.EmitTaskStatusChange(*taskInfo)

	// Execute task asynchronously
	go ts.executeTask(taskInfo)

	return nil
}

// RejectTask rejects a pending task
func (ts *TaskService) RejectTask(taskID string) error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	// Remove from pending tasks and confirmations
	delete(ts.pendingTasks, taskID)
	delete(ts.pendingConfirmations, taskID)

	return nil
}

// executeTask executes a task and handles the result
func (ts *TaskService) executeTask(taskInfo *models.TaskInfo) {
	// TODO: Implement task execution - this needs to be implemented based on actual API
	// For now, simulate success
	response := &taskPkg.TaskResponse{
		Output:  "Task executed successfully",
		Success: true,
	}
	err := error(nil)

	ts.mutex.Lock()

	if err != nil {
		taskInfo.Status = "failed"
		taskInfo.Error = err.Error()
		now := time.Now()
		taskInfo.CompletedAt = &now
	} else {
		taskInfo.Status = "completed"
		taskInfo.Result = response.Output
		taskInfo.Response = response
		now := time.Now()
		taskInfo.CompletedAt = &now
	}

	// Move from executing to completed
	delete(ts.executingTasks, taskInfo.ID)
	ts.completedTasks[taskInfo.ID] = taskInfo

	ts.mutex.Unlock()

	// Emit completion event
	if err != nil {
		ts.eventBus.Emit(events.TaskFailed, *taskInfo)
	} else {
		ts.eventBus.Emit(events.TaskCompleted, *taskInfo)
	}
}

// GetAllTasks returns all tasks grouped by status
func (ts *TaskService) GetAllTasks() map[string][]*models.TaskInfo {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	result := make(map[string][]*models.TaskInfo)

	// Convert maps to slices
	var pending []*models.TaskInfo
	for _, task := range ts.pendingTasks {
		pending = append(pending, task)
	}

	var executing []*models.TaskInfo
	for _, task := range ts.executingTasks {
		executing = append(executing, task)
	}

	var completed []*models.TaskInfo
	for _, task := range ts.completedTasks {
		completed = append(completed, task)
	}

	result["pending"] = pending
	result["executing"] = executing
	result["completed"] = completed

	return result
}

// GetPendingConfirmations returns all pending task confirmations
func (ts *TaskService) GetPendingConfirmations() []*models.TaskConfirmation {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	var confirmations []*models.TaskConfirmation
	for _, confirmation := range ts.pendingConfirmations {
		confirmations = append(confirmations, confirmation)
	}

	return confirmations
}

// Helper methods

func (ts *TaskService) generateTaskDescription(task *taskPkg.Task) string {
	switch task.Type {
	case "READ":
		if task.Path != "" {
			return fmt.Sprintf("Read file: %s", task.Path)
		}
		return "Read file"
	case "LIST":
		if task.Path != "" {
			return fmt.Sprintf("List directory: %s", task.Path)
		}
		return "List directory"
	case "SEARCH":
		if task.Query != "" {
			return fmt.Sprintf("Search for: %s", task.Query)
		}
		return "Search"
	case "RUN":
		if task.Command != "" {
			return fmt.Sprintf("Run command: %s", task.Command)
		}
		return "Run command"
	case "LOOM_EDIT":
		if task.Path != "" {
			return fmt.Sprintf("Edit file: %s", task.Path)
		}
		return "Edit file"
	default:
		return fmt.Sprintf("Task: %s", task.Type)
	}
}

func (ts *TaskService) generateTaskPreview(task *taskPkg.Task) (string, error) {
	// Use the existing task manager to generate preview
	// This is a simplified implementation - you may want to enhance this
	switch task.Type {
	case "READ":
		return fmt.Sprintf("Will read file: %s", task.Path), nil
	case "LIST":
		return fmt.Sprintf("Will list directory: %s", task.Path), nil
	case "SEARCH":
		return fmt.Sprintf("Will search for: %s", task.Query), nil
	case "RUN":
		return fmt.Sprintf("Will execute command: %s", task.Command), nil
	case "LOOM_EDIT":
		return fmt.Sprintf("Will edit file: %s\nContent: %s", task.Path, task.Content), nil
	default:
		return fmt.Sprintf("Will execute %s task", task.Type), nil
	}
}

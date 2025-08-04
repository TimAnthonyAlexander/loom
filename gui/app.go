package main

import (
	"context"
	"fmt"
	"log"
	"loom/config"
	"loom/indexer"
	"loom/llm"
	"loom/shared/events"
	"loom/shared/models"
	"loom/shared/services"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct contains the main application state and services
type App struct {
	ctx           context.Context
	workspacePath string
	chatService   *services.ChatService
	taskService   *services.TaskService
	fileService   *services.FileService
	eventBus      *events.EventBus
	config        *config.Config
	index         *indexer.Index
}

// NewApp creates a new App application struct
func NewApp(workspacePath string, cfg *config.Config, idx *indexer.Index) *App {
	// Initialize event bus
	eventBus := events.NewEventBus()

	// Initialize LLM adapter
	llmAdapter, err := llm.CreateAdapterFromConfig(cfg)
	if err != nil {
		log.Printf("Warning: LLM not available: %v", err)
		llmAdapter = nil
	}

	// Initialize services
	chatService := services.NewChatService(workspacePath, llmAdapter, eventBus)
	taskService := services.NewTaskService(workspacePath, eventBus)
	fileService := services.NewFileService(workspacePath, idx, eventBus)

	return &App{
		workspacePath: workspacePath,
		chatService:   chatService,
		taskService:   taskService,
		fileService:   fileService,
		eventBus:      eventBus,
		config:        cfg,
		index:         idx,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Subscribe to events and emit them to frontend
	a.subscribeToEvents()

	// Start file watching
	if err := a.fileService.WatchFiles(); err != nil {
		log.Printf("Warning: Could not start file watching: %v", err)
	}
}

// subscribeToEvents sets up event listeners to forward events to frontend
func (a *App) subscribeToEvents() {
	// Chat events
	a.eventBus.Subscribe(events.ChatMessageReceived, func(event events.Event) {
		runtime.EventsEmit(a.ctx, "chat:message", event.Data)
	})

	a.eventBus.Subscribe(events.ChatStreamChunk, func(event events.Event) {
		runtime.EventsEmit(a.ctx, "chat:stream_chunk", event.Data)
	})

	a.eventBus.Subscribe(events.ChatStreamEnded, func(event events.Event) {
		runtime.EventsEmit(a.ctx, "chat:stream_ended", event.Data)
	})

	// Task events
	a.eventBus.Subscribe(events.TaskConfirmationNeeded, func(event events.Event) {
		runtime.EventsEmit(a.ctx, "task:confirmation_needed", event.Data)
	})

	a.eventBus.Subscribe(events.TaskStatusChanged, func(event events.Event) {
		runtime.EventsEmit(a.ctx, "task:status_changed", event.Data)
	})

	// File events
	a.eventBus.Subscribe(events.FileTreeUpdated, func(event events.Event) {
		runtime.EventsEmit(a.ctx, "file:tree_updated", event.Data)
	})

	// System events
	a.eventBus.Subscribe(events.SystemError, func(event events.Event) {
		runtime.EventsEmit(a.ctx, "system:error", event.Data)
	})
}

// Frontend API Methods - these will be available to the React frontend

// SendMessage sends a user message to the chat
func (a *App) SendMessage(message string) error {
	return a.chatService.SendMessage(message)
}

// GetChatState returns the current chat state
func (a *App) GetChatState() models.ChatState {
	return a.chatService.GetChatState()
}

// GetFileTree returns the current file tree
func (a *App) GetFileTree() ([]models.FileInfo, error) {
	return a.fileService.GetFileTree()
}

// ReadFile reads a file's content
func (a *App) ReadFile(path string) (string, error) {
	return a.fileService.ReadFile(path)
}

// SearchFiles searches for files matching a pattern
func (a *App) SearchFiles(pattern string) ([]models.FileInfo, error) {
	return a.fileService.SearchFiles(pattern)
}

// GetFileAutocomplete returns file suggestions for autocomplete
func (a *App) GetFileAutocomplete(query string) ([]string, error) {
	return a.fileService.GetFileAutocompleteOptions(query)
}

// GetProjectSummary returns project summary and statistics
func (a *App) GetProjectSummary() (models.ProjectSummary, error) {
	return a.fileService.GetProjectSummary()
}

// GetAllTasks returns all tasks grouped by status
func (a *App) GetAllTasks() map[string][]*models.TaskInfo {
	return a.taskService.GetAllTasks()
}

// GetPendingConfirmations returns tasks waiting for confirmation
func (a *App) GetPendingConfirmations() []*models.TaskConfirmation {
	return a.taskService.GetPendingConfirmations()
}

// ApproveTask approves and executes a task
func (a *App) ApproveTask(taskID string) error {
	return a.taskService.ApproveTask(taskID)
}

// RejectTask rejects a pending task
func (a *App) RejectTask(taskID string) error {
	return a.taskService.RejectTask(taskID)
}

// StopStreaming cancels current LLM streaming
func (a *App) StopStreaming() {
	a.chatService.StopStreaming()
}

// ClearChat clears all chat messages
func (a *App) ClearChat() {
	a.chatService.ClearChat()
}

// GetAppInfo returns basic app information
func (a *App) GetAppInfo() map[string]interface{} {
	return map[string]interface{}{
		"workspacePath": a.workspacePath,
		"version":       "1.0.0",
		"hasLLM":        a.chatService != nil,
	}
}

// Greet returns a greeting for the given name (kept for compatibility)
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, Welcome to Loom GUI!", name)
}

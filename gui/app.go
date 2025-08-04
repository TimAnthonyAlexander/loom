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
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct contains the main application state and services
type App struct {
	ctx                  context.Context
	workspacePath        string
	chatService          *services.ChatService
	taskService          *services.TaskService
	fileService          *services.FileService
	eventBus             *events.EventBus
	config               *config.Config
	index                *indexer.Index
	workspaceInitialized bool
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

	// Initialize services with full Loom integration
	chatService := services.NewChatService(workspacePath, llmAdapter, eventBus, cfg, idx)
	taskService := services.NewTaskService(workspacePath, eventBus)
	fileService := services.NewFileService(workspacePath, idx, eventBus)

	return &App{
		workspacePath:        workspacePath,
		chatService:          chatService,
		taskService:          taskService,
		fileService:          fileService,
		eventBus:             eventBus,
		config:               cfg,
		index:                idx,
		workspaceInitialized: true,
	}
}

// NewAppWithDeferredInit creates a new App with minimal initialization
// The workspace will be selected by the user later
func NewAppWithDeferredInit() *App {
	// Initialize only the event bus initially
	eventBus := events.NewEventBus()

	return &App{
		eventBus:             eventBus,
		workspaceInitialized: false,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Subscribe to events and emit them to frontend
	a.subscribeToEvents()

	// Don't start file watching yet - wait for workspace selection
	// This will be done in SelectWorkspace()
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
	if !a.workspaceInitialized {
		return fmt.Errorf("workspace not initialized")
	}
	return a.chatService.SendMessage(message)
}

// GetChatState returns the current chat state
func (a *App) GetChatState() models.ChatState {
	if !a.workspaceInitialized {
		return models.ChatState{
			Messages:      []models.Message{},
			IsStreaming:   false,
			SessionID:     "",
			WorkspacePath: "",
		}
	}
	return a.chatService.GetChatState()
}

// GetFileTree returns the current file tree
func (a *App) GetFileTree() ([]models.FileInfo, error) {
	if !a.workspaceInitialized {
		return []models.FileInfo{}, nil
	}
	return a.fileService.GetFileTree()
}

// ReadFile reads a file's content
func (a *App) ReadFile(path string) (string, error) {
	if !a.workspaceInitialized {
		return "", fmt.Errorf("workspace not initialized")
	}
	return a.fileService.ReadFile(path)
}

// SearchFiles searches for files matching a pattern
func (a *App) SearchFiles(pattern string) ([]models.FileInfo, error) {
	if !a.workspaceInitialized {
		return []models.FileInfo{}, nil
	}
	return a.fileService.SearchFiles(pattern)
}

// GetFileAutocomplete returns file suggestions for autocomplete
func (a *App) GetFileAutocomplete(query string) ([]string, error) {
	if !a.workspaceInitialized {
		return []string{}, nil
	}
	return a.fileService.GetFileAutocompleteOptions(query)
}

// GetProjectSummary returns project summary and statistics
func (a *App) GetProjectSummary() (models.ProjectSummary, error) {
	if !a.workspaceInitialized {
		return models.ProjectSummary{}, nil
	}
	return a.fileService.GetProjectSummary()
}

// GetAllTasks returns all tasks grouped by status
func (a *App) GetAllTasks() map[string][]*models.TaskInfo {
	if !a.workspaceInitialized {
		return map[string][]*models.TaskInfo{}
	}
	return a.taskService.GetAllTasks()
}

// GetPendingConfirmations returns tasks waiting for confirmation
func (a *App) GetPendingConfirmations() []*models.TaskConfirmation {
	if !a.workspaceInitialized {
		return []*models.TaskConfirmation{}
	}
	return a.taskService.GetPendingConfirmations()
}

// ApproveTask approves and executes a task
func (a *App) ApproveTask(taskID string) error {
	if !a.workspaceInitialized {
		return fmt.Errorf("workspace not initialized")
	}
	return a.taskService.ApproveTask(taskID)
}

// RejectTask rejects a pending task
func (a *App) RejectTask(taskID string) error {
	if !a.workspaceInitialized {
		return fmt.Errorf("workspace not initialized")
	}
	return a.taskService.RejectTask(taskID)
}

// StopStreaming cancels current LLM streaming
func (a *App) StopStreaming() {
	if !a.workspaceInitialized {
		return
	}
	a.chatService.StopStreaming()
}

// ClearChat clears all chat messages
func (a *App) ClearChat() {
	if !a.workspaceInitialized {
		return
	}
	a.chatService.ClearChat()
}

// GetAppInfo returns basic app information
func (a *App) GetAppInfo() map[string]interface{} {
	return map[string]interface{}{
		"workspacePath":        a.workspacePath,
		"version":              "1.0.0",
		"hasLLM":               a.chatService != nil,
		"workspaceInitialized": a.workspaceInitialized,
	}
}

// ChangeWorkspace changes the current workspace to a new one
func (a *App) ChangeWorkspace(workspacePath string) error {
	// Stop file watching for the current workspace if initialized
	if a.workspaceInitialized && a.index != nil {
		a.index.StopWatching()
	}

	// Reset workspace state
	a.workspaceInitialized = false
	a.workspacePath = ""
	a.config = nil
	a.index = nil
	a.chatService = nil
	a.taskService = nil
	a.fileService = nil

	log.Printf("Workspace reset, initializing new workspace: %s", workspacePath)

	// Initialize the new workspace
	return a.selectWorkspaceInternal(workspacePath)
}

// SelectWorkspace initializes the workspace and all services
func (a *App) SelectWorkspace(workspacePath string) error {
	if a.workspaceInitialized {
		return fmt.Errorf("workspace already initialized")
	}

	return a.selectWorkspaceInternal(workspacePath)
}

// selectWorkspaceInternal contains the shared workspace initialization logic
func (a *App) selectWorkspaceInternal(workspacePath string) error {

	// Validate the workspace path
	if workspacePath == "" {
		return fmt.Errorf("workspace path cannot be empty")
	}

	// Safety check: Don't index from filesystem root or system directories
	if workspacePath == "/" || workspacePath == "/System" || workspacePath == "/usr" {
		return fmt.Errorf("cannot use system directory as workspace: %s", workspacePath)
	}

	// Check if directory exists
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return fmt.Errorf("workspace directory does not exist: %s", workspacePath)
	}

	log.Printf("Initializing workspace: %s", workspacePath)
	a.workspacePath = workspacePath

	// Load configuration
	cfg, err := config.LoadConfig(workspacePath)
	if err != nil {
		log.Printf("Warning: Error loading config: %v", err)
		// Use default config if loading fails
		cfg = &config.Config{MaxFileSize: 1024 * 1024} // 1MB default
	}
	a.config = cfg

	// Initialize or load index
	idx, err := a.initializeIndex(workspacePath, cfg.MaxFileSize)
	if err != nil {
		return fmt.Errorf("failed to initialize index: %v", err)
	}
	a.index = idx

	// Initialize LLM adapter
	llmAdapter, err := llm.CreateAdapterFromConfig(cfg)
	if err != nil {
		log.Printf("Warning: LLM not available: %v", err)
		llmAdapter = nil
	}

	// Initialize services with full Loom integration
	a.chatService = services.NewChatService(workspacePath, llmAdapter, a.eventBus, cfg, idx)
	a.taskService = services.NewTaskService(workspacePath, a.eventBus)
	a.fileService = services.NewFileService(workspacePath, idx, a.eventBus)

	// Start file watching
	if err := a.fileService.WatchFiles(); err != nil {
		log.Printf("Warning: Could not start file watching: %v", err)
	}

	a.workspaceInitialized = true
	log.Printf("Workspace initialized successfully: %s", workspacePath)

	// Emit workspace initialized event
	runtime.EventsEmit(a.ctx, "workspace:initialized", map[string]string{
		"workspacePath": workspacePath,
	})

	return nil
}

// initializeIndex handles index creation and loading
func (a *App) initializeIndex(workspacePath string, maxFileSize int64) (*indexer.Index, error) {
	// Try to load from cache first
	idx, err := indexer.LoadFromCache(workspacePath, maxFileSize)
	if err != nil {
		// Create new index if cache doesn't exist or is invalid
		log.Printf("Building workspace index for: %s...", workspacePath)
		idx = indexer.NewIndex(workspacePath, maxFileSize)

		// Build the index
		err = idx.BuildIndex()
		if err != nil {
			log.Printf("Warning: Error building index: %v", err)
			// Return empty index if build fails
			return indexer.NewIndex(workspacePath, maxFileSize), nil
		}

		// Save to cache
		err = idx.SaveToCache()
		if err != nil {
			log.Printf("Warning: Error saving index cache: %v", err)
		}
	}

	return idx, nil
}

// OpenDirectoryDialog opens a native directory picker dialog
func (a *App) OpenDirectoryDialog() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app context not available")
	}

	// Use Wails runtime to open directory dialog
	selectedPath, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Workspace Directory",
	})

	if err != nil {
		return "", fmt.Errorf("failed to open directory dialog: %v", err)
	}

	return selectedPath, nil
}

// Greet returns a greeting for the given name (kept for compatibility)
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, Welcome to Loom GUI!", name)
}

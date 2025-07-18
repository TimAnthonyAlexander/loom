package tui

import (
	"context"
	"fmt"
	"loom/chat"
	"loom/config"
	"loom/indexer"
	"loom/llm"
	"loom/task"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	infoStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(1, 2)

	inputStyle = lipgloss.NewStyle().
			Padding(0, 1)

	messageStyle = lipgloss.NewStyle().
			Padding(0, 1)

	fileTreeStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#A550DF")).
			Padding(1, 2).
			Height(15)

	taskStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#FFA500")).
			Padding(1, 2).
			Height(8)

	confirmStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF6B6B")).
			Background(lipgloss.Color("#2A2A2A")).
			Padding(1, 2).
			Bold(true)
)

type viewMode int

const (
	viewChat viewMode = iota
	viewFileTree
	viewTasks
)

// StreamMsg represents a streaming message chunk from LLM
type StreamMsg struct {
	Content string
	Error   error
	Done    bool
}

// StreamStartMsg indicates streaming has started
type StreamStartMsg struct {
	chunks chan llm.StreamChunk
}

// TaskEventMsg represents task execution events
type TaskEventMsg struct {
	Event task.TaskExecutionEvent
}

// TaskConfirmationMsg represents a pending task confirmation
type TaskConfirmationMsg struct {
	Task     *task.Task
	Response *task.TaskResponse
	Preview  string
}

// ContinueLLMMsg indicates that LLM should continue the conversation after task completion
type ContinueLLMMsg struct{}

// SessionOptions controls how chat sessions are loaded/created
type SessionOptions struct {
	ContinueLatest bool   // If true, continue from latest session
	SessionID      string // Specific session ID to load (overrides ContinueLatest)
}

type model struct {
	workspacePath  string
	config         *config.Config
	index          *indexer.Index
	input          string
	messages       []string
	width          int
	height         int
	currentView    viewMode
	fileTreeScroll int

	// LLM integration
	llmAdapter       llm.LLMAdapter
	chatSession      *chat.Session
	streamingContent string
	isStreaming      bool
	llmError         error
	streamChan       chan llm.StreamChunk

	// Task execution
	taskManager      *task.Manager
	taskExecutor     *task.Executor
	currentExecution *task.TaskExecution
	taskHistory      []string
	taskEventChan    chan task.TaskExecutionEvent

	// Task confirmation
	pendingConfirmation *TaskConfirmationMsg
	showingConfirmation bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle confirmation dialog
		if m.showingConfirmation {
			switch msg.String() {
			case "y", "Y":
				return m.handleTaskConfirmation(true)
			case "n", "N":
				return m.handleTaskConfirmation(false)
			case "ctrl+c":
				return m, tea.Quit
			}
			return m, nil // Ignore other keys during confirmation
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			// Switch between views
			switch m.currentView {
			case viewChat:
				m.currentView = viewFileTree
			case viewFileTree:
				m.currentView = viewTasks
			case viewTasks:
				m.currentView = viewChat
				// Refresh messages when returning to chat view to ensure sync
				m.messages = m.chatSession.GetDisplayMessages()
			}
		case "up":
			// File tree scrolling removed since we no longer show file lists
		case "down":
			// File tree scrolling removed since we no longer show file lists
		case "enter":
			if m.currentView == viewChat && strings.TrimSpace(m.input) != "" && !m.isStreaming {
				userInput := strings.TrimSpace(m.input)
				m.input = ""

				// Handle special commands
				if userInput == "/quit" {
					return m, tea.Quit
				} else if userInput == "/files" {
					stats := m.index.GetStats()
					// Add user command to chat session
					userMessage := llm.Message{
						Role:      "user",
						Content:   userInput,
						Timestamp: time.Now(),
					}
					m.chatSession.AddMessage(userMessage)

					// Add response to chat session
					response := llm.Message{
						Role:      "assistant",
						Content:   fmt.Sprintf("Indexed %d files", stats.TotalFiles),
						Timestamp: time.Now(),
					}
					m.chatSession.AddMessage(response)

					// Refresh display from chat session
					m.messages = m.chatSession.GetDisplayMessages()
				} else if userInput == "/stats" {
					// Add user command to chat session
					userMessage := llm.Message{
						Role:      "user",
						Content:   userInput,
						Timestamp: time.Now(),
					}
					m.chatSession.AddMessage(userMessage)

					// Add response to chat session
					response := llm.Message{
						Role:      "assistant",
						Content:   m.getIndexStatsMessage(),
						Timestamp: time.Now(),
					}
					m.chatSession.AddMessage(response)

					// Refresh display from chat session
					m.messages = m.chatSession.GetDisplayMessages()
				} else if userInput == "/tasks" {
					// Add user command to chat session
					userMessage := llm.Message{
						Role:      "user",
						Content:   userInput,
						Timestamp: time.Now(),
					}
					m.chatSession.AddMessage(userMessage)

					var responseContent string
					if m.currentExecution != nil {
						history := m.taskManager.GetTaskHistory(m.currentExecution)
						responseContent = fmt.Sprintf("Task history:\n%s", strings.Join(history, "\n"))
					} else {
						responseContent = "No task execution in progress"
					}

					// Add response to chat session
					response := llm.Message{
						Role:      "assistant",
						Content:   responseContent,
						Timestamp: time.Now(),
					}
					m.chatSession.AddMessage(response)

					// Refresh display from chat session
					m.messages = m.chatSession.GetDisplayMessages()
				} else {
					// Send to LLM with task execution
					if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
						return m, m.sendToLLMWithTasks(userInput)
					} else {
						// Add user message to chat session first
						userMessage := llm.Message{
							Role:      "user",
							Content:   userInput,
							Timestamp: time.Now(),
						}
						m.chatSession.AddMessage(userMessage)

						errorMsg := "LLM not available. Please configure your model and API key."
						if m.llmError != nil {
							errorMsg = fmt.Sprintf("LLM error: %v", m.llmError)
						}

						// Add error response to chat session
						errorResponse := llm.Message{
							Role:      "assistant",
							Content:   errorMsg,
							Timestamp: time.Now(),
						}
						m.chatSession.AddMessage(errorResponse)

						// Refresh display from chat session
						m.messages = m.chatSession.GetDisplayMessages()
					}
				}
			}
		case "backspace":
			if m.currentView == viewChat && len(m.input) > 0 && !m.isStreaming {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			if m.currentView == viewChat && !m.isStreaming {
				m.input += msg.String()
			}
		}

	case StreamStartMsg:
		m.isStreaming = true
		m.streamingContent = ""
		m.streamChan = msg.chunks
		// Refresh messages to show user input immediately
		m.messages = m.chatSession.GetDisplayMessages()
		return m, m.waitForStream()

	case StreamMsg:
		if msg.Error != nil {
			m.isStreaming = false
			m.streamChan = nil
			m.messages = append(m.messages, fmt.Sprintf("Loom: Error: %v", msg.Error))
			return m, nil
		}

		if msg.Done {
			m.isStreaming = false
			m.streamChan = nil
			// Add the complete response to chat history
			if m.streamingContent != "" {
				response := llm.Message{
					Role:      "assistant",
					Content:   m.streamingContent,
					Timestamp: time.Now(),
				}
				if err := m.chatSession.AddMessage(response); err != nil {
					fmt.Printf("Warning: failed to save assistant message: %v\n", err)
				}

				// Refresh display messages from chat session to ensure sync
				m.messages = m.chatSession.GetDisplayMessages()

				// Process LLM response for tasks
				return m, m.handleLLMResponseForTasks(m.streamingContent)
			}
			return m, nil
		}

		m.streamingContent += msg.Content
		return m, m.waitForStream()

	case TaskEventMsg:
		return m.handleTaskEvent(msg.Event)

	case TaskConfirmationMsg:
		m.pendingConfirmation = &msg
		m.showingConfirmation = true
		return m, nil

	case ContinueLLMMsg:
		// Continue LLM conversation after task completion
		if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
			// Refresh messages from chat session to include task results
			m.messages = m.chatSession.GetDisplayMessages()
			return m, m.continueLLMAfterTasks()
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m model) View() string {
	// Title
	title := titleStyle.Render("Loom - AI Coding Assistant")

	// Index stats for info panel
	stats := m.index.GetStats()
	langSummary := m.getLanguageSummary(stats)

	// Workspace info
	var modelStatus string
	if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
		modelStatus = m.config.Model + " ✓"
	} else {
		modelStatus = m.config.Model + " ✗"
	}

	var taskStatus string
	if m.currentExecution != nil {
		taskStatus = fmt.Sprintf("Tasks: %s", m.currentExecution.Status)
	} else {
		taskStatus = "Tasks: Ready"
	}

	info := infoStyle.Render(fmt.Sprintf(
		"Workspace: %s\nModel: %s\nShell: %t\nFiles: %d %s\n%s",
		m.workspacePath,
		modelStatus,
		m.config.EnableShell,
		stats.TotalFiles,
		langSummary,
		taskStatus,
	))

	var mainContent string

	// Show confirmation dialog if needed
	if m.showingConfirmation && m.pendingConfirmation != nil {
		confirmDialog := m.renderConfirmationDialog()
		return lipgloss.JoinVertical(lipgloss.Left, title, "", info, "", confirmDialog)
	}

	switch m.currentView {
	case viewChat:
		// Messages area - include streaming content
		allMessages := make([]string, len(m.messages))
		copy(allMessages, m.messages)

		if m.isStreaming && m.streamingContent != "" {
			allMessages = append(allMessages, fmt.Sprintf("Loom: %s", m.streamingContent))
		}

		messageText := strings.Join(allMessages, "\n")
		if messageText == "" && !m.isStreaming {
			messageText = "Welcome to Loom!\nYou can now chat with an AI assistant about your project.\nTry asking about your code, architecture, or programming questions.\n\nSpecial commands:\n/files - Show file count\n/stats - Show detailed index statistics\n/tasks - Show task execution history\n/quit - Exit the application\n\nPress Tab to view file tree or tasks.\nPress Ctrl+C to exit."
		}
		messages := messageStyle.Render(messageText)

		// Input area at the bottom
		inputPrefix := "> "
		if m.isStreaming {
			inputPrefix = "> (streaming...) "
		}
		input := inputStyle.Render(fmt.Sprintf("%s%s", inputPrefix, m.input))

		mainContent = lipgloss.JoinVertical(lipgloss.Left, messages, "", input)

	case viewFileTree:
		// File tree view
		treeContent := m.renderFileTree()
		fileTree := fileTreeStyle.Render(treeContent)
		mainContent = fileTree

	case viewTasks:
		// Task execution view
		taskContent := m.renderTaskView()
		taskView := taskStyle.Render(taskContent)
		mainContent = taskView
	}

	// Help
	var helpText string
	switch m.currentView {
	case viewChat:
		helpText = "Tab: File Tree/Tasks | Enter: Send | Ctrl+C: Quit"
	case viewFileTree:
		helpText = "Tab: Chat/Tasks | Ctrl+C: Quit"
	case viewTasks:
		helpText = "Tab: Chat/File Tree | Ctrl+C: Quit"
	}

	help := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, title, "", info, "", mainContent, "", help)
}

// renderConfirmationDialog renders the task confirmation dialog
func (m model) renderConfirmationDialog() string {
	if m.pendingConfirmation == nil {
		return ""
	}

	task := m.pendingConfirmation.Task
	response := m.pendingConfirmation.Response

	var content strings.Builder
	content.WriteString(fmt.Sprintf("⚠️  TASK CONFIRMATION REQUIRED\n\n"))
	content.WriteString(fmt.Sprintf("Task: %s\n\n", task.Description()))

	if response.Output != "" {
		content.WriteString("Preview:\n")
		content.WriteString(response.Output)
		content.WriteString("\n\n")
	}

	content.WriteString("Do you want to proceed with this task?\n")
	content.WriteString("Press 'y' to approve, 'n' to cancel")

	return confirmStyle.Render(content.String())
}

// renderTaskView renders the task execution view
func (m model) renderTaskView() string {
	var content strings.Builder
	content.WriteString("🔧 Task Execution\n\n")

	if m.currentExecution == nil {
		content.WriteString("No task execution in progress.\n")
		content.WriteString("Tasks will appear here when the AI needs to read files, make edits, or run commands.")
		return content.String()
	}

	// Show execution summary
	content.WriteString(fmt.Sprintf("Status: %s\n", m.currentExecution.Status))
	content.WriteString(fmt.Sprintf("Tasks: %d\n", len(m.currentExecution.Tasks)))
	if !m.currentExecution.StartTime.IsZero() {
		if m.currentExecution.EndTime.IsZero() {
			duration := time.Since(m.currentExecution.StartTime)
			content.WriteString(fmt.Sprintf("Duration: %v (ongoing)\n", duration.Round(time.Second)))
		} else {
			duration := m.currentExecution.EndTime.Sub(m.currentExecution.StartTime)
			content.WriteString(fmt.Sprintf("Duration: %v\n", duration.Round(time.Second)))
		}
	}
	content.WriteString("\n")

	// Show task history
	if len(m.taskHistory) > 0 {
		content.WriteString("Recent tasks:\n")
		// Show last 10 tasks
		start := len(m.taskHistory) - 10
		if start < 0 {
			start = 0
		}
		for i := start; i < len(m.taskHistory); i++ {
			content.WriteString(fmt.Sprintf("  %s\n", m.taskHistory[i]))
		}
	}

	return content.String()
}

// renderFileTree renders the file tree view (modified to hide actual structure)
func (m model) renderFileTree() string {
	files := m.index.GetFileList()

	if len(files) == 0 {
		return "No files indexed yet."
	}

	stats := m.index.GetStats()

	var lines []string
	lines = append(lines, "📁 File Tree Available")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Total files: %d", stats.TotalFiles))
	lines = append(lines, fmt.Sprintf("Total size: %.2f MB", float64(stats.TotalSize)/1024/1024))
	lines = append(lines, "")

	// Show language breakdown instead of file names
	if len(stats.LanguagePercent) > 0 {
		lines = append(lines, "Language breakdown:")

		type langPair struct {
			name    string
			percent float64
		}

		var langs []langPair
		for name, percent := range stats.LanguagePercent {
			if percent > 0 {
				langs = append(langs, langPair{name, percent})
			}
		}

		sort.Slice(langs, func(i, j int) bool {
			return langs[i].percent > langs[j].percent
		})

		for _, lang := range langs {
			lines = append(lines, fmt.Sprintf("  %s: %.1f%%", lang.name, lang.percent))
		}
	}

	lines = append(lines, "")
	lines = append(lines, "File structure is indexed and available to the AI")
	lines = append(lines, "but hidden from this view for privacy.")

	return strings.Join(lines, "\n")
}

// sendToLLMWithTasks sends a message to the LLM and sets up task execution
func (m *model) sendToLLMWithTasks(userInput string) tea.Cmd {
	return func() tea.Msg {
		// Add user message to chat session
		userMessage := llm.Message{
			Role:      "user",
			Content:   userInput,
			Timestamp: time.Now(),
		}

		if err := m.chatSession.AddMessage(userMessage); err != nil {
			return StreamMsg{Error: fmt.Errorf("failed to save user message: %w", err)}
		}

		// Refresh display messages from chat session
		m.messages = m.chatSession.GetDisplayMessages()

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

		// Get all messages for the LLM
		messages := m.chatSession.GetMessages()

		// Create a channel for streaming
		chunks := make(chan llm.StreamChunk, 10)

		// Start streaming in a goroutine
		go func() {
			defer cancel()
			if err := m.llmAdapter.Stream(ctx, messages, chunks); err != nil {
				chunks <- llm.StreamChunk{Error: err}
			}
		}()

		return StreamStartMsg{chunks: chunks}
	}
}

// continueLLMAfterTasks continues LLM conversation after task completion
func (m *model) continueLLMAfterTasks() tea.Cmd {
	return func() tea.Msg {
		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

		// Get all messages for the LLM (including task results)
		messages := m.chatSession.GetMessages()

		// Create a channel for streaming
		chunks := make(chan llm.StreamChunk, 10)

		// Start streaming in a goroutine
		go func() {
			defer cancel()
			if err := m.llmAdapter.Stream(ctx, messages, chunks); err != nil {
				chunks <- llm.StreamChunk{Error: err}
			}
		}()

		return StreamStartMsg{chunks: chunks}
	}
}

// handleLLMResponseForTasks processes the LLM response for task execution
func (m *model) handleLLMResponseForTasks(llmResponse string) tea.Cmd {
	return func() tea.Msg {
		// Handle task execution if task manager is available
		if m.taskManager != nil {
			execution, err := m.taskManager.HandleLLMResponse(llmResponse, m.taskEventChan)
			if err != nil {
				return TaskEventMsg{
					Event: task.TaskExecutionEvent{
						Type:    "execution_failed",
						Message: fmt.Sprintf("Task execution failed: %v", err),
					},
				}
			}

			if execution != nil {
				m.currentExecution = execution

				// Check if there's a pending confirmation
				if pendingTask, pendingResponse := execution.GetPendingTask(); pendingTask != nil {
					return TaskConfirmationMsg{
						Task:     pendingTask,
						Response: pendingResponse,
						Preview:  pendingResponse.Output,
					}
				}

				// If execution completed successfully without needing confirmation,
				// trigger LLM continuation to explain the results
				if execution.Status == "completed" {
					return ContinueLLMMsg{}
				}
			}
		}

		return nil
	}
}

// handleTaskEvent processes task execution events
func (m model) handleTaskEvent(event task.TaskExecutionEvent) (tea.Model, tea.Cmd) {
	// Add to task history
	if event.Message != "" {
		m.taskHistory = append(m.taskHistory, event.Message)
	}

	// Handle confirmation requests
	if event.RequiresInput && event.Task != nil && event.Response != nil {
		return m, func() tea.Msg {
			return TaskConfirmationMsg{
				Task:     event.Task,
				Response: event.Response,
				Preview:  event.Response.Output,
			}
		}
	}

	return m, nil
}

// handleTaskConfirmation handles user confirmation for destructive tasks
func (m model) handleTaskConfirmation(approved bool) (tea.Model, tea.Cmd) {
	m.showingConfirmation = false

	if m.pendingConfirmation == nil {
		return m, nil
	}

	task := m.pendingConfirmation.Task
	m.pendingConfirmation = nil

	if approved {
		// Apply the task
		if err := m.taskManager.ConfirmTask(task, true); err != nil {
			m.taskHistory = append(m.taskHistory, fmt.Sprintf("❌ Failed to apply %s: %v", task.Description(), err))
		} else {
			m.taskHistory = append(m.taskHistory, fmt.Sprintf("✅ Applied %s", task.Description()))
		}
	} else {
		m.taskHistory = append(m.taskHistory, fmt.Sprintf("❌ Cancelled %s", task.Description()))
	}

	return m, nil
}

// waitForStream waits for the next streaming chunk (unchanged from original)
func (m *model) waitForStream() tea.Cmd {
	return func() tea.Msg {
		if m.streamChan == nil {
			return StreamMsg{Done: true}
		}

		select {
		case chunk, ok := <-m.streamChan:
			if !ok {
				return StreamMsg{Done: true}
			}

			if chunk.Error != nil {
				return StreamMsg{Error: chunk.Error}
			}

			if chunk.Done {
				return StreamMsg{Done: true}
			}

			return StreamMsg{Content: chunk.Content}
		case <-time.After(100 * time.Millisecond):
			// Timeout to prevent blocking - continue waiting
			return StreamMsg{Content: ""}
		}
	}
}

// Helper functions (unchanged from original)
func (m model) getLanguageSummary(stats indexer.IndexStats) string {
	if len(stats.LanguagePercent) == 0 {
		return ""
	}

	type langPair struct {
		name    string
		percent float64
	}

	var langs []langPair
	for name, percent := range stats.LanguagePercent {
		if percent > 0 {
			langs = append(langs, langPair{name, percent})
		}
	}

	sort.Slice(langs, func(i, j int) bool {
		return langs[i].percent > langs[j].percent
	})

	var summary []string
	for i, lang := range langs {
		if i >= 3 { // Show top 3 languages
			break
		}
		summary = append(summary, fmt.Sprintf("%s %.1f%%", lang.name, lang.percent))
	}

	return fmt.Sprintf("(%s)", strings.Join(summary, ", "))
}

func (m model) getIndexStatsMessage() string {
	stats := m.index.GetStats()

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Files: %d\n", stats.TotalFiles))
	result.WriteString(fmt.Sprintf("Size: %.2f MB\n", float64(stats.TotalSize)/1024/1024))
	result.WriteString(fmt.Sprintf("Updated: %s\n", m.index.LastUpdated.Format("15:04:05")))

	if len(stats.LanguagePercent) > 0 {
		result.WriteString("\nLanguages:\n")

		type langPair struct {
			name    string
			percent float64
		}

		var langs []langPair
		for name, percent := range stats.LanguagePercent {
			if percent > 0 {
				langs = append(langs, langPair{name, percent})
			}
		}

		sort.Slice(langs, func(i, j int) bool {
			return langs[i].percent > langs[j].percent
		})

		for _, lang := range langs {
			result.WriteString(fmt.Sprintf("  %s: %.1f%%\n", lang.name, lang.percent))
		}
	}

	return result.String()
}

// StartTUI initializes and starts the TUI interface with task execution support
func StartTUI(workspacePath string, cfg *config.Config, idx *indexer.Index, options SessionOptions) error {
	// Initialize LLM adapter
	var llmAdapter llm.LLMAdapter
	var llmError error

	adapter, err := llm.CreateAdapter(cfg.Model, cfg.APIKey, cfg.BaseURL)
	if err != nil {
		llmError = err
		fmt.Printf("Warning: LLM not available: %v\n", err)
	} else {
		llmAdapter = adapter
	}

	// Initialize chat session based on options
	var chatSession *chat.Session

	if options.SessionID != "" {
		// Load specific session by ID
		session, sessionErr := chat.LoadSessionByID(workspacePath, options.SessionID, 50)
		if sessionErr != nil {
			fmt.Printf("Warning: Failed to load session %s: %v\n", options.SessionID, sessionErr)
			fmt.Println("Creating new session instead...")
			chatSession = chat.NewSession(workspacePath, 50)
		} else {
			chatSession = session
		}
	} else if options.ContinueLatest {
		// Continue from latest session
		session, sessionErr := chat.LoadLatestSession(workspacePath, 50)
		if sessionErr != nil {
			return fmt.Errorf("failed to load latest session: %w", sessionErr)
		}
		chatSession = session
	} else {
		// Create new session (default behavior)
		chatSession = chat.NewSession(workspacePath, 50)
	}

	// Initialize task system
	taskExecutor := task.NewExecutor(workspacePath, cfg.EnableShell, cfg.MaxFileSize)
	var taskManager *task.Manager
	var taskEventChan chan task.TaskExecutionEvent

	if llmAdapter != nil {
		taskManager = task.NewManager(taskExecutor, llmAdapter, chatSession)
		taskEventChan = make(chan task.TaskExecutionEvent, 10)
	}

	// Add system prompt if this is a new session (no previous messages)
	if len(chatSession.GetMessages()) == 0 {
		systemPrompt := createSystemPromptWithTasks(idx, cfg.EnableShell)
		if err := chatSession.AddMessage(systemPrompt); err != nil {
			fmt.Printf("Warning: failed to add system prompt: %v\n", err)
		}
	}

	m := model{
		workspacePath: workspacePath,
		config:        cfg,
		index:         idx,
		input:         "",
		messages:      chatSession.GetDisplayMessages(),
		currentView:   viewChat,
		llmAdapter:    llmAdapter,
		chatSession:   chatSession,
		llmError:      llmError,
		taskManager:   taskManager,
		taskExecutor:  taskExecutor,
		taskEventChan: taskEventChan,
		taskHistory:   make([]string, 0),
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// createSystemPromptWithTasks creates an enhanced system prompt that includes task capabilities
func createSystemPromptWithTasks(index *indexer.Index, enableShell bool) llm.Message {
	stats := index.GetStats()

	// Get language breakdown
	var langBreakdown []string
	type langPair struct {
		name    string
		percent float64
	}

	var langs []langPair
	for name, percent := range stats.LanguagePercent {
		if percent > 0 {
			langs = append(langs, langPair{name, percent})
		}
	}

	sort.Slice(langs, func(i, j int) bool {
		return langs[i].percent > langs[j].percent
	})

	for i, lang := range langs {
		if i >= 5 { // Show top 5 languages
			break
		}
		langBreakdown = append(langBreakdown, fmt.Sprintf("%s (%.1f%%)", lang.name, lang.percent))
	}

	shellStatus := "disabled"
	if enableShell {
		shellStatus = "enabled"
	}

	prompt := fmt.Sprintf(`You are Loom, an AI coding assistant with task execution capabilities. You can analyze code, answer questions, and perform actions on the workspace through structured tasks.

Current workspace summary:
- Total files: %d
- Total size: %.2f MB
- Last updated: %s
- Primary languages: %s
- Shell execution: %s

## Available Task Types

You can emit tasks to interact with the workspace. Use JSON code blocks like this:

`+"```"+`json
{
  "tasks": [
    {"type": "ReadFile", "path": "main.go", "max_lines": 150},
    {"type": "EditFile", "path": "main.go", "diff": "diff content here"},
    {"type": "ListDir", "path": "src/", "recursive": false},
    {"type": "RunShell", "command": "go build", "timeout": 10}
  ]
}
`+"```"+`

### Task Types:
1. **ReadFile**: Read file contents with optional line limits
   - path: File path (required)
   - max_lines: Max lines to read (default: 200)
   - start_line, end_line: Read specific line range

2. **EditFile**: Apply file changes (requires user confirmation)
   - path: File path (required) 
   - diff: Unified diff format, OR
   - content: Complete file replacement

3. **ListDir**: List directory contents
   - path: Directory path (default: ".")
   - recursive: Include subdirectories (default: false)

4. **RunShell**: Execute shell commands (requires user confirmation, %s)
   - command: Shell command (required)
   - timeout: Timeout in seconds (default: 3)

## Security & Constraints:
- All file paths must be within the workspace
- Binary files cannot be read
- Secrets are automatically redacted from file content
- EditFile and RunShell tasks require user confirmation
- File size limits apply (large files are truncated)

## Usage Guidelines:
- Use tasks when you need to examine or modify files
- Break complex operations into multiple simple tasks
- Always explain what you're doing before emitting tasks
- Check files before editing to understand current state
- Test changes incrementally

You can help with:
- Reading and analyzing code files
- Making targeted edits and improvements  
- Listing and exploring directory structures
- Running build/test commands (if shell enabled)
- Explaining code structure and architecture
- Debugging and troubleshooting issues

Start by asking what the user would like to accomplish!`,
		stats.TotalFiles,
		float64(stats.TotalSize)/1024/1024,
		index.LastUpdated.Format("15:04:05"),
		strings.Join(langBreakdown, ", "),
		shellStatus,
		shellStatus)

	return llm.Message{
		Role:      "system",
		Content:   prompt,
		Timestamp: time.Now(),
	}
}

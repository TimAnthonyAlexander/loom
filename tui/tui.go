package tui

import (
	"context"
	"fmt"
	"loom/chat"
	"loom/config"
	contextMgr "loom/context"
	"loom/indexer"
	"loom/llm"
	"loom/session"
	taskPkg "loom/task"
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

	// New styles to clearly distinguish user and assistant prefixes
	userPrefixStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00D7FF"))

	assistantPrefixStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFAF00"))
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
	Event taskPkg.TaskExecutionEvent
}

// TaskConfirmationMsg represents a pending task confirmation
type TaskConfirmationMsg struct {
	Task     *taskPkg.Task
	Response *taskPkg.TaskResponse
	Preview  string
}

type TestPromptMsg struct {
	TestCount    int
	Language     string
	EditedFiles  []string
	ShouldPrompt bool
}

// ContinueLLMMsg indicates that LLM should continue the conversation after task completion
type ContinueLLMMsg struct{}

// AutoContinueMsg triggers automatic continuation of LLM conversation
type AutoContinueMsg struct {
	Depth        int
	LastResponse string
}

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

	// Chat scrolling and display
	messageScroll int      // New field for message scrolling
	messageLines  []string // Wrapped message lines for proper scrolling

	// LLM integration
	llmAdapter       llm.LLMAdapter
	chatSession      *chat.Session
	streamingContent string
	isStreaming      bool
	llmError         error
	streamChan       chan llm.StreamChunk

	// Task execution
	taskManager       *taskPkg.Manager
	enhancedManager   *taskPkg.EnhancedManager
	sequentialManager *taskPkg.SequentialTaskManager
	taskExecutor      *taskPkg.Executor
	currentExecution  *taskPkg.TaskExecution
	taskHistory       []string
	taskEventChan     chan taskPkg.TaskExecutionEvent

	// Task confirmation
	pendingConfirmation *TaskConfirmationMsg
	showingConfirmation bool

	// Always-on recursive execution tracking
	recursiveDepth     int
	recursiveStartTime time.Time
	lastLLMResponse    string
	recentResponses    []string

	// Safety limits (hardcoded defaults)
	maxRecursiveDepth int
	maxRecursiveTime  time.Duration

	// UI preferences
	showInfoPanel bool // Hide the top info panel after the first user message
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
		case "ctrl+s":
			// Generate summary with Ctrl+S
			if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
				return m, m.generateSummary("session")
			}
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
				m.updateWrappedMessages()
			}
		case "up":
			// Scroll up in chat view
			if m.currentView == viewChat && m.messageScroll > 0 {
				m.messageScroll--
			}
		case "down":
			// Scroll down in chat view
			if m.currentView == viewChat {
				maxScroll := len(m.messageLines) - m.getAvailableMessageHeight()
				if maxScroll > 0 && m.messageScroll < maxScroll {
					m.messageScroll++
				}
			}
		case "enter":
			if m.currentView == viewChat && strings.TrimSpace(m.input) != "" && !m.isStreaming {
				userInput := strings.TrimSpace(m.input)
				m.input = ""

				// Hide the static info panel after the first message is sent
				if m.showInfoPanel {
					m.showInfoPanel = false
				}

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
					m.updateWrappedMessages()
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
					m.updateWrappedMessages()
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
					m.updateWrappedMessages()
				} else if userInput == "/summary" {
					// Add user command to chat session
					userMessage := llm.Message{
						Role:      "user",
						Content:   userInput,
						Timestamp: time.Now(),
					}
					m.chatSession.AddMessage(userMessage)

					// Generate summary if LLM is available
					if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
						return m, m.generateSummary("session")
					} else {
						// Add error response
						response := llm.Message{
							Role:      "assistant",
							Content:   "Summary feature requires LLM to be available. Please configure your model and API key.",
							Timestamp: time.Now(),
						}
						m.chatSession.AddMessage(response)
						m.messages = m.chatSession.GetDisplayMessages()
						m.updateWrappedMessages()
					}
				} else if userInput == "/rationale" {
					// Add user command to chat session
					userMessage := llm.Message{
						Role:      "user",
						Content:   userInput,
						Timestamp: time.Now(),
					}
					m.chatSession.AddMessage(userMessage)

					// Show change summaries and rationales
					return m, m.showRationale()
				} else if userInput == "/test" {
					// Add user command to chat session
					userMessage := llm.Message{
						Role:      "user",
						Content:   userInput,
						Timestamp: time.Now(),
					}
					m.chatSession.AddMessage(userMessage)

					// Show test discovery and results
					return m, m.showTestSummary()
				} else if userInput == "/debug" {
					// Toggle debug mode for task parsing
					if taskPkg.IsTaskDebugEnabled() {
						taskPkg.DisableTaskDebug()
						response := llm.Message{
							Role:      "assistant",
							Content:   "🔧 Task debug mode disabled.\n\nDebug output for task parsing is now off.",
							Timestamp: time.Now(),
						}
						m.chatSession.AddMessage(response)
					} else {
						taskPkg.EnableTaskDebug()
						response := llm.Message{
							Role:      "assistant",
							Content:   "🔧 Task debug mode enabled.\n\nYou'll now see detailed output when the LLM fails to output proper task JSON format. This helps identify when the AI suggests actions but doesn't provide executable tasks.\n\nTry asking the AI to read or edit files to see the debug output.",
							Timestamp: time.Now(),
						}
						m.chatSession.AddMessage(response)
					}

					// Add user command to chat session
					userMessage := llm.Message{
						Role:      "user",
						Content:   userInput,
						Timestamp: time.Now(),
					}
					m.chatSession.AddMessage(userMessage)

					// Refresh display
					m.messages = m.chatSession.GetDisplayMessages()
					m.updateWrappedMessages()

				} else if userInput == "/help" {
					// Show comprehensive help
					userMessage := llm.Message{
						Role:      "user",
						Content:   userInput,
						Timestamp: time.Now(),
					}
					m.chatSession.AddMessage(userMessage)

					helpContent := `🤖 **Loom Help**

**Navigation:**
• Tab - Switch between Chat, File Tree, and Tasks views
• ↑↓ - Scroll in chat view
• Enter - Send message or confirm actions
• Ctrl+S - Quick summary generation
• Ctrl+C - Exit application

**Special Commands:**
• /files - Show file count and language breakdown
• /stats - Detailed project statistics and index information
• /tasks - Task execution history and current status
• /test - Test discovery results and execution options
• /summary - AI-generated session summary
• /rationale - Change summaries and explanations
• /debug - Toggle task debugging mode (shows AI task parsing)
• /help - Show this help message
• /quit - Exit application

**Views:**
• Chat - Main conversation with AI assistant
• File Tree - Project file overview and language statistics
• Tasks - Task execution history and status

**Tips:**
• The AI can read, edit, and list files using natural language
• All file edits require your confirmation before being applied
• Use specific questions for better AI responses
• Press Tab to explore different views
• Task debug mode helps troubleshoot AI task generation

Ask me anything about your code, architecture, or programming questions!`

					response := llm.Message{
						Role:      "assistant",
						Content:   helpContent,
						Timestamp: time.Now(),
					}
					m.chatSession.AddMessage(response)

					// Refresh display
					m.messages = m.chatSession.GetDisplayMessages()
					m.updateWrappedMessages()

				} else if (userInput == "yes" || userInput == "y") && m.enhancedManager != nil {
					// Check if this is a response to a test prompt
					testDiscovery := m.enhancedManager.GetTestDiscovery()
					if testDiscovery.GetTestCount() > 0 {
						// Add user response to chat
						userMessage := llm.Message{
							Role:      "user",
							Content:   userInput,
							Timestamp: time.Now(),
						}
						m.chatSession.AddMessage(userMessage)

						// Run tests
						return m, m.runTestsFromPrompt("Go")
					}

					// Otherwise proceed with normal LLM handling
					if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
						return m, m.sendToLLMWithTasks(userInput)
					}
				} else if (userInput == "no" || userInput == "n") && m.enhancedManager != nil {
					// Check if this is a response to a test prompt
					testDiscovery := m.enhancedManager.GetTestDiscovery()
					if testDiscovery.GetTestCount() > 0 {
						// Add user response to chat
						userMessage := llm.Message{
							Role:      "user",
							Content:   userInput,
							Timestamp: time.Now(),
						}
						m.chatSession.AddMessage(userMessage)

						// Add acknowledgment message
						skipMessage := llm.Message{
							Role:      "assistant",
							Content:   "Tests skipped. You can run them later with the `/test` command.",
							Timestamp: time.Now(),
						}
						m.chatSession.AddMessage(skipMessage)

						// Refresh display
						m.messages = m.chatSession.GetDisplayMessages()
						m.updateWrappedMessages()
						return m, nil
					}

					// Otherwise proceed with normal LLM handling
					if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
						return m, m.sendToLLMWithTasks(userInput)
					}
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
						m.updateWrappedMessages()
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
		m.updateWrappedMessagesWithOptions(true) // Force auto-scroll when starting stream
		return m, m.waitForStream()

	case StreamMsg:
		if msg.Error != nil {
			m.isStreaming = false
			m.streamChan = nil
			m.messages = append(m.messages, fmt.Sprintf("Loom: Error: %v", msg.Error))
			m.updateWrappedMessages()
			return m, nil
		}

		if msg.Done {
			m.isStreaming = false
			m.streamChan = nil
			// Add the complete response to chat history
			if m.streamingContent != "" {
				// Filter potentially misleading status messages
				filteredContent := m.filterMisleadingStatusMessages(m.streamingContent)

				response := llm.Message{
					Role:      "assistant",
					Content:   filteredContent,
					Timestamp: time.Now(),
				}
				if err := m.chatSession.AddMessage(response); err != nil {
					fmt.Printf("Warning: failed to save assistant message: %v\n", err)
				}

				// Refresh display messages from chat session to ensure sync
				m.messages = m.chatSession.GetDisplayMessages()
				m.updateWrappedMessages()

				// Process LLM response for tasks (use original content for task parsing)
				return m, m.handleLLMResponseForTasks(m.streamingContent)
			}
			return m, nil
		}

		m.streamingContent += msg.Content
		return m, m.waitForStream()

	case TaskEventMsg:
		return m.handleTaskEvent(msg.Event)

	case TestPromptMsg:
		return m.handleTestPrompt(msg)

	case TaskConfirmationMsg:
		m.pendingConfirmation = &msg
		m.showingConfirmation = true
		return m, nil

	case ContinueLLMMsg:
		// Continue LLM conversation after task completion
		if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
			// Refresh messages from chat session to include task results
			m.messages = m.chatSession.GetDisplayMessages()
			m.updateWrappedMessages()
			return m, m.continueLLMAfterTasks()
		}
		return m, nil

	case AutoContinueMsg:
		// Handle always-on recursive continuation
		return m.handleAutoContinuation(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update wrapped messages when window size changes
		m.updateWrappedMessages()
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
	var changeStatus string

	if m.currentExecution != nil {
		taskStatus = fmt.Sprintf("Tasks: %s", m.currentExecution.Status)
	} else {
		taskStatus = "Tasks: Ready"
	}

	// Add change summary information if enhanced manager is available
	if m.enhancedManager != nil {
		changeSummaryMgr := m.enhancedManager.GetChangeSummaryManager()
		recentChanges := changeSummaryMgr.GetRecentSummaries(1)
		if len(recentChanges) > 0 {
			changeStatus = fmt.Sprintf("Changes: %d", len(changeSummaryMgr.GetSummaries()))
		} else {
			changeStatus = "Changes: None"
		}
	} else {
		changeStatus = "Changes: N/A"
	}

	// Build header components conditionally
	headerParts := []string{title}
	if m.showInfoPanel {
		headerParts = append(headerParts, "", infoStyle.Render(fmt.Sprintf(
			"Workspace: %s\nModel: %s\nShell: %t\nFiles: %d %s\n%s\n%s",
			m.workspacePath,
			modelStatus,
			m.config.EnableShell,
			stats.TotalFiles,
			langSummary,
			taskStatus,
			changeStatus,
		)))
	}

	header := lipgloss.JoinVertical(lipgloss.Left, headerParts...)

	var mainContent string

	// Show confirmation dialog if needed
	if m.showingConfirmation && m.pendingConfirmation != nil {
		confirmDialog := m.renderConfirmationDialog()
		return lipgloss.JoinVertical(lipgloss.Left, header, "", confirmDialog)
	}

	switch m.currentView {
	case viewChat:
		// Update wrapped messages when needed - force auto-scroll during streaming
		if m.isStreaming {
			m.updateWrappedMessagesWithOptions(true)
		} else {
			m.updateWrappedMessages()
		}

		// Calculate available height for messages
		availableHeight := m.getAvailableMessageHeight()

		// Get visible message lines based on scroll position
		var visibleLines []string
		if len(m.messageLines) > 0 {
			start := m.messageScroll
			end := m.messageScroll + availableHeight

			if start < 0 {
				start = 0
			}
			if end > len(m.messageLines) {
				end = len(m.messageLines)
			}

			if start < len(m.messageLines) {
				visibleLines = m.messageLines[start:end]
			}
		}

		// Join visible lines and apply style with width constraint
		messageText := strings.Join(visibleLines, "\n")
		messageWidth := m.width - 6 // Account for padding
		if messageWidth < 40 {
			messageWidth = 40
		}

		messages := messageStyle.Width(messageWidth).Render(messageText)

		// Show scroll indicator if there are more messages
		var scrollIndicator string
		if len(m.messageLines) > availableHeight {
			scrollIndicator = fmt.Sprintf(" [%d/%d]",
				m.messageScroll+1,
				len(m.messageLines)-availableHeight+1)
		}

		// Input area at the bottom
		inputPrefix := "> "
		if m.isStreaming {
			inputPrefix = "> (streaming...) "
		}
		input := inputStyle.Render(fmt.Sprintf("%s%s%s", inputPrefix, m.input, scrollIndicator))

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
		helpText = "Tab: Views | ↑↓: Scroll | Ctrl+S: Summary | /help: Commands | /test: Tests | /debug: Debug | Ctrl+C: Quit"
	case viewFileTree:
		helpText = "Tab: Chat/Tasks | /help: All Commands | Ctrl+C: Quit"
	case viewTasks:
		helpText = "Tab: Chat/File Tree | /help: All Commands | Ctrl+C: Quit"
	}

	help := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, header, "", mainContent, "", help)
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
	content.WriteString("🔧 Task Execution & Changes\n\n")

	// Show current execution status
	if m.currentExecution != nil {
		content.WriteString("📊 Current Execution:\n")
		content.WriteString(fmt.Sprintf("  Status: %s\n", m.currentExecution.Status))
		content.WriteString(fmt.Sprintf("  Tasks: %d\n", len(m.currentExecution.Tasks)))
		if !m.currentExecution.StartTime.IsZero() {
			if m.currentExecution.EndTime.IsZero() {
				duration := time.Since(m.currentExecution.StartTime)
				content.WriteString(fmt.Sprintf("  Duration: %v (ongoing)\n", duration.Round(time.Second)))
			} else {
				duration := m.currentExecution.EndTime.Sub(m.currentExecution.StartTime)
				content.WriteString(fmt.Sprintf("  Duration: %v\n", duration.Round(time.Second)))
			}
		}
		content.WriteString("\n")
	}

	// Show recent changes if enhanced manager is available
	if m.enhancedManager != nil {
		changeSummaryMgr := m.enhancedManager.GetChangeSummaryManager()
		recentChanges := changeSummaryMgr.GetRecentSummaries(5)

		if len(recentChanges) > 0 {
			content.WriteString("📝 Recent Changes:\n")
			for i := len(recentChanges) - 1; i >= 0; i-- {
				change := recentChanges[i]
				content.WriteString(fmt.Sprintf("  • %s\n", change.Summary))
				if change.FilePath != "" {
					content.WriteString(fmt.Sprintf("    📁 %s\n", change.FilePath))
				}
				if change.Rationale != "" && len(change.Rationale) < 80 {
					content.WriteString(fmt.Sprintf("    💭 %s\n", change.Rationale))
				}
				content.WriteString("\n")
			}
		} else {
			content.WriteString("📝 Recent Changes:\n")
			content.WriteString("  No changes recorded in this session\n\n")
		}

		// Show test status
		testDiscovery := m.enhancedManager.GetTestDiscovery()
		testCount := testDiscovery.GetTestCount()

		content.WriteString("🧪 Test Status:\n")
		if testCount > 0 {
			content.WriteString(fmt.Sprintf("  Found %d tests\n", testCount))
			goTests := testDiscovery.GetTestsByLanguage("Go")
			if len(goTests) > 0 {
				content.WriteString(fmt.Sprintf("  Go tests: %d files\n", len(goTests)))
			}
		} else {
			content.WriteString("  No tests discovered\n")
		}
		content.WriteString("\n")
	}

	// Show task history
	if len(m.taskHistory) > 0 {
		content.WriteString("📋 Task History:\n")
		// Show last 8 tasks to leave room for other info
		start := len(m.taskHistory) - 8
		if start < 0 {
			start = 0
		}
		for i := start; i < len(m.taskHistory); i++ {
			content.WriteString(fmt.Sprintf("  %s\n", m.taskHistory[i]))
		}
	} else {
		content.WriteString("📋 Task History:\n")
		content.WriteString("  No tasks executed yet\n")
		content.WriteString("  Tasks will appear here when AI performs actions\n")
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
		m.updateWrappedMessages()

		// For exploration queries, use the objective-driven exploration
		if m.sequentialManager != nil && m.isExplorationQuery(userInput) {
			// Start objective exploration
			m.sequentialManager.StartObjectiveExploration(userInput)

			// Replace the system message with objective-setting prompt
			messages := m.chatSession.GetMessages()
			if len(messages) > 0 && messages[0].Role == "system" {
				objectivePrompt := m.sequentialManager.CreateSequentialSystemMessage()
				messages[0] = objectivePrompt
			}
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

		// Get all messages for the LLM
		messages := m.chatSession.GetMessages()

		// Create a channel for streaming
		chunks := make(chan llm.StreamChunk, 10)

		// Start streaming in a goroutine
		go func() {
			defer cancel()
			// Stream method handles its own error reporting via chunks
			m.llmAdapter.Stream(ctx, messages, chunks)
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
			// Stream method handles its own error reporting via chunks
			m.llmAdapter.Stream(ctx, messages, chunks)
		}()

		return StreamStartMsg{chunks: chunks}
	}
}

// handleLLMResponseForTasks processes the LLM response for task execution
func (m *model) handleLLMResponseForTasks(llmResponse string) tea.Cmd {
	return func() tea.Msg {
		// Store the response for recursive tracking
		m.lastLLMResponse = llmResponse

		// Handle objective-driven exploration only for exploration queries
		if m.sequentialManager != nil && m.sequentialManager.IsExploring() {
			return m.handleObjectiveExploration(llmResponse)
		}

		// For non-exploration requests, use standard task management
		if m.taskManager != nil {
			execution, err := m.taskManager.HandleLLMResponse(llmResponse, m.taskEventChan)
			if err != nil {
				return StreamMsg{Error: fmt.Errorf("failed to handle LLM response: %w", err)}
			}

			if execution != nil {
				m.currentExecution = execution
			}
		}

		// No tasks found - this is a regular Q&A response
		m.recursiveDepth = 0
		return nil
	}
}

// handleObjectiveExploration manages the objective-driven exploration flow
func (m *model) handleObjectiveExploration(llmResponse string) tea.Msg {
	// Check if this is setting a new objective
	if objective := m.sequentialManager.ExtractObjective(llmResponse); objective != "" {
		m.sequentialManager.SetObjective(objective)
		// Show objective to user
		objectiveMsg := fmt.Sprintf("🎯 OBJECTIVE: %s", objective)
		m.messages = append(m.messages, objectiveMsg)
		m.updateWrappedMessages()
	}

	// Check if objective is complete
	if m.sequentialManager.IsObjectiveComplete(llmResponse) {
		m.sequentialManager.CompleteObjective()
		// This is the final synthesis - show it normally
		m.recursiveDepth = 0
		return nil
	}

	// Parse and execute task
	task, _, err := m.sequentialManager.ParseSingleTask(llmResponse)
	if err != nil {
		return StreamMsg{Error: fmt.Errorf("failed to parse task: %w", err)}
	}

	if task != nil {
		// Execute the task
		response := m.taskExecutor.Execute(task)

		// Add task result to accumulated data
		m.sequentialManager.AddTaskResult(*response)

		// Add task result to chat session for LLM to see
		taskResultMsg := m.formatTaskResultForLLM(task, response)
		if err := m.chatSession.AddMessage(taskResultMsg); err != nil {
			fmt.Printf("Warning: failed to add task result to chat: %v\n", err)
		}

		// Show minimal status during suppressed phase
		if m.sequentialManager.GetCurrentPhase() == taskPkg.PhaseSuppressedExploration {
			statusMsg := m.createMinimalTaskStatus(task)
			m.messages = append(m.messages, statusMsg)
			m.updateWrappedMessages()
		} else {
			// Refresh display normally for objective setting phase
			m.messages = m.chatSession.GetDisplayMessages()
			m.updateWrappedMessages()
		}

		// Continue the conversation for next step
		return m.continueLLMAfterTasks()()
	}

	// No task found - regular response
	m.recursiveDepth = 0
	return nil
}

// createMinimalTaskStatus creates very minimal status messages for suppressed exploration
func (m *model) createMinimalTaskStatus(task *taskPkg.Task) string {
	switch task.Type {
	case taskPkg.TaskTypeReadFile:
		// Extract just the filename from path
		filename := task.Path
		if idx := strings.LastIndex(filename, "/"); idx != -1 {
			filename = filename[idx+1:]
		}
		return fmt.Sprintf("📖 %s", filename)

	case taskPkg.TaskTypeListDir:
		dirName := task.Path
		if dirName == "." {
			dirName = "root"
		}
		if idx := strings.LastIndex(dirName, "/"); idx != -1 {
			dirName = dirName[idx+1:]
		}
		return fmt.Sprintf("📂 %s/", dirName)

	case taskPkg.TaskTypeEditFile:
		filename := task.Path
		if idx := strings.LastIndex(filename, "/"); idx != -1 {
			filename = filename[idx+1:]
		}
		return fmt.Sprintf("✏️  %s", filename)

	case taskPkg.TaskTypeRunShell:
		// Show just the command verb
		cmd := strings.Fields(task.Command)
		if len(cmd) > 0 {
			return fmt.Sprintf("🔧 %s", cmd[0])
		}
		return "🔧 shell"

	default:
		return "⚡ task"
	}
}

// detectsActionWithoutTasks checks if the LLM indicated an action but didn't provide tasks (simplified)
func (m *model) detectsActionWithoutTasks(response string) bool {
	lowerResponse := strings.ToLower(response)

	// Simple check for common action phrases
	actionPhrases := []string{"let me read", "i'll read", "reading file"}

	for _, phrase := range actionPhrases {
		if strings.Contains(lowerResponse, phrase) {
			// Check if there are actual JSON tasks
			return !strings.Contains(response, "```json")
		}
	}

	return false
}

// filterMisleadingStatusMessages filters out misleading status messages (simplified)
func (m *model) filterMisleadingStatusMessages(content string) string {
	// Simplified: just return content as-is for better transparency
	// The user can see what the AI actually said
	return content
}

// isInformationalResponse checks if the response is answering a question vs. indicating work to do
func (m *model) isInformationalResponse(response string) bool {
	lowerResponse := strings.ToLower(response)

	// Signs this is an informational/Q&A response
	qaPatterns := []string{
		"the license", "this project uses", "according to", "based on",
		"the answer is", "to answer your question", "in response to",
		"the file shows", "the code shows", "looking at", "from what i can see",
		"the current", "this appears to be", "it looks like", "it seems",
		"the project", "the workspace", "the repository", "this codebase",
		"currently configured", "currently set up", "currently using",
		"here's what", "here are the", "the status is", "the configuration",
	}

	for _, pattern := range qaPatterns {
		if strings.Contains(lowerResponse, pattern) {
			return true
		}
	}

	// If response is short and doesn't mention future work, it's likely complete
	wordCount := len(strings.Fields(response))
	if wordCount < 50 && !m.mentionsFutureWork(response) {
		return true
	}

	return false
}

// mentionsFutureWork checks if the response indicates more work to be done
func (m *model) mentionsFutureWork(response string) bool {
	lowerResponse := strings.ToLower(response)

	futureWorkPatterns := []string{
		"next", "then", "after", "should", "would", "could", "let me",
		"i'll", "i will", "going to", "plan to", "need to", "want to",
		"we should", "we could", "we need", "might want", "may want",
		"let's", "continuing", "proceeding", "moving forward",
	}

	for _, pattern := range futureWorkPatterns {
		if strings.Contains(lowerResponse, pattern) {
			return true
		}
	}

	return false
}

// handleTaskEvent processes task execution events
func (m model) handleTaskEvent(event taskPkg.TaskExecutionEvent) (tea.Model, tea.Cmd) {
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

	// Handle test discovery completion - prompt user to run tests
	if event.Type == "test_discovery_completed" && m.enhancedManager != nil {
		testDiscovery := m.enhancedManager.GetTestDiscovery()
		testCount := testDiscovery.GetTestCount()

		if testCount > 0 {
			return m, func() tea.Msg {
				return TestPromptMsg{
					TestCount:    testCount,
					Language:     "Go",       // Could be made dynamic
					EditedFiles:  []string{}, // Could extract from execution
					ShouldPrompt: true,
				}
			}
		}
	}

	// Automatically continue the LLM conversation once all tasks have
	// finished and there is no user confirmation required. This prevents
	// the UI from stalling on a status-only message like "Reading file …"
	// and allows the assistant to immediately respond with the follow-up
	// answer that uses the newly-read content.
	if event.Type == "execution_completed" && m.pendingConfirmation == nil {
		// Trigger continuation only if the LLM adapter is available so we
		// don't schedule redundant work in offline mode.
		if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
			return m, func() tea.Msg {
				return ContinueLLMMsg{}
			}
		}
	}

	return m, nil
}

// handleTestPrompt handles test execution prompts
func (m model) handleTestPrompt(msg TestPromptMsg) (tea.Model, tea.Cmd) {
	if !msg.ShouldPrompt || m.enhancedManager == nil {
		return m, nil
	}

	// Add test prompt to chat
	promptContent := fmt.Sprintf(`🧪 **Test Discovery Complete**

Found %d tests in the workspace. Would you like to run them to verify your recent changes?

You can:
• Type "yes" to run all %s tests now
• Type "no" to skip testing for now
• Use "/test" command anytime to see test details
• Tests will run automatically after future code changes

Run tests now?`, msg.TestCount, msg.Language)

	testPrompt := llm.Message{
		Role:      "assistant",
		Content:   promptContent,
		Timestamp: time.Now(),
	}

	if err := m.chatSession.AddMessage(testPrompt); err != nil {
		fmt.Printf("Warning: failed to add test prompt: %v\n", err)
	}

	// Refresh display
	m.messages = m.chatSession.GetDisplayMessages()
	m.updateWrappedMessages()

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

	// Add user confirmation decision to chat
	confirmationMessage := ""
	if approved {
		confirmationMessage = "User approved the task."
	} else {
		confirmationMessage = "User rejected the task."
	}

	userConfirmationMsg := llm.Message{
		Role:      "system",
		Content:   fmt.Sprintf("TASK_CONFIRMATION: %s Task: %s", confirmationMessage, task.Description()),
		Timestamp: time.Now(),
	}
	m.chatSession.AddMessage(userConfirmationMsg)

	var resultMessage llm.Message
	var shouldContinue bool = false

	var err error
	if approved {
		// Apply the task using the appropriate manager
		if m.enhancedManager != nil {
			// Use enhanced manager if available
			err = m.enhancedManager.ConfirmTask(task, true)
		} else if m.taskManager != nil {
			// Fall back to basic manager
			err = m.taskManager.ConfirmTask(task, true)
		} else {
			err = fmt.Errorf("no task manager available")
		}
	}

	// Format result message using the manager's formatting method
	var formattedResult string
	if m.taskManager != nil {
		formattedResult = m.taskManager.FormatConfirmationResult(task, approved, err)
	} else {
		// Fallback formatting if no manager available
		if !approved {
			formattedResult = fmt.Sprintf("Task cancelled: %s", task.Description())
		} else if err != nil {
			formattedResult = fmt.Sprintf("Task failed: %s - Error: %v", task.Description(), err)
		} else {
			formattedResult = fmt.Sprintf("Task completed: %s", task.Description())
		}
	}

	// Update task history for display
	if !approved {
		m.taskHistory = append(m.taskHistory, fmt.Sprintf("❌ Cancelled %s", task.Description()))
	} else if err != nil {
		m.taskHistory = append(m.taskHistory, fmt.Sprintf("❌ Failed to apply %s: %v", task.Description(), err))
	} else {
		m.taskHistory = append(m.taskHistory, fmt.Sprintf("✅ Applied %s", task.Description()))
	}

	// Create result message for LLM
	resultMessage = llm.Message{
		Role:      "system",
		Content:   formattedResult,
		Timestamp: time.Now(),
	}
	shouldContinue = true // Always continue to let LLM know about the result

	// Add result message to chat
	m.chatSession.AddMessage(resultMessage)

	// Update display messages
	m.messages = m.chatSession.GetDisplayMessages()
	m.updateWrappedMessages()

	// Check if the task is complete by asking the AI
	if shouldContinue && m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
		return m, func() tea.Msg {
			return AutoContinueMsg{
				Depth:        m.recursiveDepth,
				LastResponse: formattedResult,
			}
		}
	}

	return m, nil
}

// handleAutoContinuation handles simplified auto-continuation
func (m model) handleAutoContinuation(msg AutoContinueMsg) (tea.Model, tea.Cmd) {
	// Simple depth check - avoid excessive loops
	if msg.Depth >= 3 {
		m.recursiveDepth = 0
		return m, nil
	}

	// Simple time check
	if m.recursiveDepth > 0 && time.Since(m.recursiveStartTime) > 1*time.Minute {
		m.recursiveDepth = 0
		return m, nil
	}

	// Check if this looks like a completion signal
	lowerResponse := strings.ToLower(msg.LastResponse)
	if strings.Contains(lowerResponse, "complete") ||
		strings.Contains(lowerResponse, "done") ||
		strings.Contains(lowerResponse, "finished") {
		m.recursiveDepth = 0
		return m, nil
	}

	// Continue automatically
	m.recursiveDepth = msg.Depth + 1
	if m.recursiveDepth == 1 {
		m.recursiveStartTime = time.Now()
	}

	// Simple completion check prompt
	completionPrompt := "Is this task complete?"

	autoMessage := llm.Message{
		Role:      "user",
		Content:   completionPrompt,
		Timestamp: time.Now(),
	}

	if err := m.chatSession.AddMessage(autoMessage); err != nil {
		return m, func() tea.Msg {
			return StreamMsg{Error: fmt.Errorf("failed to add completion check: %w", err)}
		}
	}

	return m, m.sendToLLMWithTasks(completionPrompt)
}

// addSystemMessage helper method to add system messages to chat
func (m *model) addSystemMessage(message string) {
	systemMsg := llm.Message{
		Role:      "system",
		Content:   message,
		Timestamp: time.Now(),
	}

	m.chatSession.AddMessage(systemMsg)
	m.messages = m.chatSession.GetDisplayMessages()
	m.updateWrappedMessages()
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

// getAvailableMessageHeight calculates how many lines are available for messages
func (m model) getAvailableMessageHeight() int {
	// Calculate used height: title (3), info panel (~7), input (3), help (3), spacing (4)
	usedHeight := 20
	availableHeight := m.height - usedHeight
	if availableHeight < 5 {
		return 5 // Minimum height
	}
	return availableHeight
}

// wrapText wraps text to fit within the given width
func (m model) wrapText(text string, width int) []string {
	if width <= 0 {
		width = 80 // Default width
	}

	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if len(line) <= width {
			lines = append(lines, line)
			continue
		}

		// Wrap long lines
		for len(line) > width {
			// Try to break at word boundary
			breakPoint := width
			if spaceIndex := strings.LastIndex(line[:width], " "); spaceIndex > width/2 {
				breakPoint = spaceIndex
			}

			lines = append(lines, line[:breakPoint])
			line = line[breakPoint:]
			if strings.HasPrefix(line, " ") {
				line = line[1:]
			}
		}
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	return lines
}

// updateWrappedMessages updates the wrapped message lines when content changes
func (m *model) updateWrappedMessages() {
	m.updateWrappedMessagesWithOptions(false)
}

// updateWrappedMessagesWithOptions updates the wrapped message lines with auto-scroll options
func (m *model) updateWrappedMessagesWithOptions(forceAutoScroll bool) {
	if m.width <= 0 {
		m.width = 80 // Default width
	}

	// Remember if user was at bottom before update
	availableHeight := m.getAvailableMessageHeight()
	oldMaxScroll := len(m.messageLines) - availableHeight
	wasAtBottom := oldMaxScroll <= 0 || m.messageScroll >= oldMaxScroll-1

	// Calculate available width (accounting for padding and borders)
	messageWidth := m.width - 6 // Padding and potential borders
	if messageWidth < 40 {
		messageWidth = 40
	}

	var allLines []string

	// Wrap each message
	allMessages := make([]string, len(m.messages))
	copy(allMessages, m.messages)

	// Add streaming content if active
	if m.isStreaming && m.streamingContent != "" {
		allMessages = append(allMessages, fmt.Sprintf("Loom: %s", m.streamingContent))
	}

	// Add welcome message if no messages
	if len(allMessages) == 0 && !m.isStreaming {
		debugStatus := ""
		if taskPkg.IsTaskDebugEnabled() {
			debugStatus = "\n🔧 Task debug mode is ON"
		} else {
			debugStatus = "\n🔧 Task debug mode is OFF (use /debug to enable)"
		}

		welcomeMsg := "Welcome to Loom!\nYou can now chat with an AI assistant about your project.\nTry asking about your code, architecture, or programming questions.\n\nQuick start:\n• Type /help for all available commands\n• Press Tab to explore views (Chat, File Tree, Tasks)\n• Press Ctrl+S for quick summary\n• Use /test to discover and run tests" + debugStatus + "\n\nThe AI can read, edit, and list files using natural language.\nAll changes require your confirmation before being applied.\n\nPress Ctrl+C to exit."
		allMessages = append(allMessages, welcomeMsg)
	}

	for _, original := range allMessages {
		msg := original
		if strings.HasPrefix(msg, "You: ") {
			if parts := strings.SplitN(msg, ": ", 2); len(parts) == 2 {
				prefix := userPrefixStyle.Render(parts[0] + ":")
				msg = prefix + " " + parts[1]
			}
		} else if strings.HasPrefix(msg, "Loom: ") {
			if parts := strings.SplitN(msg, ": ", 2); len(parts) == 2 {
				prefix := assistantPrefixStyle.Render(parts[0] + ":")
				msg = prefix + " " + parts[1]
			}
		}

		wrapped := m.wrapText(msg, messageWidth)
		allLines = append(allLines, wrapped...)
	}

	m.messageLines = allLines

	// Auto-scroll to bottom logic
	newMaxScroll := len(m.messageLines) - availableHeight
	if newMaxScroll > 0 {
		if forceAutoScroll || m.isStreaming || wasAtBottom {
			// During streaming or if user was at bottom, follow the content
			m.messageScroll = newMaxScroll
		} else if m.messageScroll >= newMaxScroll-3 {
			// User is near bottom but not streaming - gentle auto-scroll
			m.messageScroll = newMaxScroll
		}

		// Ensure scroll position is within bounds
		if m.messageScroll > newMaxScroll {
			m.messageScroll = newMaxScroll
		}
	} else {
		m.messageScroll = 0
	}
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

	// Initialize session with recovery checking
	var chatSession *chat.Session

	// Check for session recovery needs if not explicitly loading a specific session
	if options.SessionID == "" && !options.ContinueLatest {
		recoveryMgr, err := session.NewRecoveryManager(workspacePath)
		if err != nil {
			fmt.Printf("Warning: Failed to initialize recovery manager: %v\n", err)
		} else {
			recoveryOptions, err := recoveryMgr.CheckForRecovery()
			if err != nil {
				fmt.Printf("Warning: Failed to check for recovery: %v\n", err)
			} else if recoveryOptions != nil {
				// Show recovery report
				fmt.Println(recoveryMgr.GenerateRecoveryReport(recoveryOptions))

				// Auto-recover if available and no corruption detected
				if recoveryOptions.AutoRecoveryAvailable && len(recoveryOptions.CorruptedSessions) == 0 {
					fmt.Println("Auto-recovering most recent session...")
					if sessionState, err := recoveryMgr.AutoRecover(); err == nil {
						// Convert SessionState to chat.Session (simplified)
						chatSession = chat.NewSession(workspacePath, 50)
						for _, msg := range sessionState.Messages {
							chatSession.AddMessage(msg)
						}

						recoveryMgr.AddRecoveryMessage(sessionState, "AUTO_RECOVERY",
							"Automatically recovered from most recent session")
						fmt.Println("✅ Session recovered successfully")
					} else {
						fmt.Printf("⚠️  Auto-recovery failed: %v\n", err)
						fmt.Println("Starting new session...")
						chatSession = chat.NewSession(workspacePath, 50)
					}
				} else {
					fmt.Println("⚠️  Manual recovery recommended - starting new session for now")
					chatSession = chat.NewSession(workspacePath, 50)
				}
			}
		}
	}

	// Fall back to original session loading logic if no recovery happened
	if chatSession == nil {
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
	}

	// Initialize task system with enhanced M6 features
	taskExecutor := taskPkg.NewExecutor(workspacePath, cfg.EnableShell, cfg.MaxFileSize)
	var taskManager *taskPkg.Manager
	var enhancedManager *taskPkg.EnhancedManager
	var sequentialManager *taskPkg.SequentialTaskManager
	var taskEventChan chan taskPkg.TaskExecutionEvent

	if llmAdapter != nil {
		enhancedManager = taskPkg.NewEnhancedManager(taskExecutor, llmAdapter, chatSession, idx)
		taskManager = enhancedManager.Manager // For compatibility
		sequentialManager = taskPkg.NewSequentialTaskManager(taskExecutor, llmAdapter, chatSession)
		taskEventChan = make(chan taskPkg.TaskExecutionEvent, 10)
	}

	// Add enhanced system prompt if this is a new session (no previous messages)
	if len(chatSession.GetMessages()) == 0 {
		promptEnhancer := llm.NewPromptEnhancer(workspacePath, idx)
		systemPrompt := promptEnhancer.CreateEnhancedSystemPrompt(cfg.EnableShell)
		if err := chatSession.AddMessage(systemPrompt); err != nil {
			fmt.Printf("Warning: failed to add system prompt: %v\n", err)
		}
	}

	m := model{
		workspacePath:     workspacePath,
		config:            cfg,
		index:             idx,
		input:             "",
		messages:          chatSession.GetDisplayMessages(),
		currentView:       viewChat,
		llmAdapter:        llmAdapter,
		chatSession:       chatSession,
		llmError:          llmError,
		taskManager:       taskManager,
		enhancedManager:   enhancedManager,
		sequentialManager: sequentialManager,
		taskExecutor:      taskExecutor,
		taskEventChan:     taskEventChan,
		taskHistory:       make([]string, 0),
		width:             80, // Default width until window size is received
		height:            24, // Default height until window size is received

		// Initialize recursive tracking with safety defaults
		recentResponses:   make([]string, 0, 5),
		maxRecursiveDepth: 15,
		maxRecursiveTime:  30 * time.Minute,
		showInfoPanel:     true,
	}

	// Initialize wrapped messages
	m.updateWrappedMessages()

	p := tea.NewProgram(m, tea.WithAltScreen())

	// Forward task execution events from the manager to the TUI update loop.
	if taskEventChan != nil {
		go func() {
			for ev := range taskEventChan {
				// Forward TaskEventMsg; Send has no return value.
				p.Send(TaskEventMsg{Event: ev})
			}
		}()
	}

	_, err = p.Run()
	return err
}

// generateSummary generates a summary of the current session
func (m *model) generateSummary(summaryType string) tea.Cmd {
	return func() tea.Msg {
		// Get messages from chat session
		messages := m.chatSession.GetMessages()

		// Create summarizer
		summarizer := contextMgr.NewSummarizer(m.llmAdapter)

		var summary *contextMgr.Summary
		var err error

		// Generate summary based on type
		switch summaryType {
		case "session":
			summary, err = summarizer.SummarizeMessages(messages, contextMgr.SummaryTypeSession)
		case "recent":
			summary, err = summarizer.SummarizeRecentHistory(messages, 5)
		case "actionplan":
			summary, err = summarizer.SummarizeActionPlans(messages)
		case "progress":
			summary, err = summarizer.SummarizeProgress(messages)
		default:
			summary, err = summarizer.SummarizeMessages(messages, contextMgr.SummaryTypeSession)
		}

		if err != nil {
			return StreamMsg{Error: fmt.Errorf("failed to generate summary: %w", err)}
		}

		// Format summary response
		summaryResponse := fmt.Sprintf("📊 **%s**\n\n%s\n\n*Summary generated from %s | %d tokens saved*",
			summary.Title,
			summary.Content,
			summary.MessageRange,
			summary.TokensSaved)

		// Add summary to chat session
		response := llm.Message{
			Role:      "assistant",
			Content:   summaryResponse,
			Timestamp: time.Now(),
		}

		if err := m.chatSession.AddMessage(response); err != nil {
			return StreamMsg{Error: fmt.Errorf("failed to save summary: %w", err)}
		}

		// Refresh display
		m.messages = m.chatSession.GetDisplayMessages()
		m.updateWrappedMessages()

		return StreamMsg{Content: "", Done: true}
	}
}

// showRationale displays change summaries and rationales
func (m *model) showRationale() tea.Cmd {
	return func() tea.Msg {
		var rationaleContent string

		// Use enhanced manager if available for real rationale data
		if m.enhancedManager != nil {
			rationaleContent = m.enhancedManager.GetChangeSummaries()
			if rationaleContent == "No change summaries available." {
				rationaleContent = `📋 Change Summaries & Rationale

No changes have been made in this session yet.

To see rationales for changes:
1. Make code edits through the AI
2. The AI will provide explanations for each change
3. Use /rationale to view the collected explanations

This helps you understand:
• Why changes were made
• What impact they have
• Testing recommendations
• Architectural decisions`
			}
		} else {
			// Fallback for when enhanced manager isn't available
			rationaleContent = `📋 Change Summaries & Rationale

Enhanced rationale features are not available in this session.

To enable:
1. Restart Loom with an LLM configured
2. Make code changes through the AI
3. View detailed explanations with /rationale`
		}

		// Add rationale response to chat session
		response := llm.Message{
			Role:      "assistant",
			Content:   rationaleContent,
			Timestamp: time.Now(),
		}

		if err := m.chatSession.AddMessage(response); err != nil {
			return StreamMsg{Error: fmt.Errorf("failed to save rationale: %w", err)}
		}

		// Refresh display
		m.messages = m.chatSession.GetDisplayMessages()
		m.updateWrappedMessages()

		return StreamMsg{Content: "", Done: true}
	}
}

// showTestSummary displays test discovery and execution information
func (m *model) showTestSummary() tea.Cmd {
	return func() tea.Msg {
		var testContent string

		// Use enhanced manager if available for real test data
		if m.enhancedManager != nil {
			testContent = m.enhancedManager.GetTestSummary()
		} else {
			// Fallback for when enhanced manager isn't available
			testContent = `🧪 Test Discovery & Execution

Enhanced test features are not available in this session.

To enable:
1. Restart Loom with an LLM configured
2. Ensure test files exist in your workspace
3. Use /test to see discovered tests and run them

Supported test patterns:
• Go: *_test.go
• JavaScript/TypeScript: *.test.js, *.spec.js
• Python: test_*.py, *_test.py`
		}

		// Add test response to chat session
		response := llm.Message{
			Role:      "assistant",
			Content:   testContent,
			Timestamp: time.Now(),
		}

		if err := m.chatSession.AddMessage(response); err != nil {
			return StreamMsg{Error: fmt.Errorf("failed to save test summary: %w", err)}
		}

		// Refresh display
		m.messages = m.chatSession.GetDisplayMessages()
		m.updateWrappedMessages()

		return StreamMsg{Content: "", Done: true}
	}
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

You can emit tasks to interact with the workspace using simple natural language commands:

🔧 READ main.go (max: 150 lines)
🔧 EDIT main.go → add error handling
🔧 LIST src/
🔧 RUN go build (timeout: 10)

### Task Types:
1. **READ**: Read file contents with optional line limits
   - 🔧 READ filename.go
   - 🔧 READ filename.go (max: 200 lines)
   - 🔧 READ filename.go (lines 50-100)

2. **EDIT**: Apply file changes (requires user confirmation)
   - 🔧 EDIT filename.go → describe changes
   - 🔧 EDIT newfile.go → create new file

3. **LIST**: List directory contents
   - 🔧 LIST .
   - 🔧 LIST src/
   - 🔧 LIST . recursive

4. **RUN**: Execute shell commands (requires user confirmation, %s)
   - 🔧 RUN go build
   - 🔧 RUN go test (timeout: 30)

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

// runTestsFromPrompt runs tests when user responds to a test prompt
func (m *model) runTestsFromPrompt(language string) tea.Cmd {
	return func() tea.Msg {
		if m.enhancedManager == nil {
			return StreamMsg{Error: fmt.Errorf("enhanced manager not available")}
		}

		// Run tests manually
		testResult, err := m.enhancedManager.RunTestsManually(language)
		if err != nil {
			errorMsg := llm.Message{
				Role:      "assistant",
				Content:   fmt.Sprintf("❌ Failed to run tests: %v", err),
				Timestamp: time.Now(),
			}
			m.chatSession.AddMessage(errorMsg)
			m.messages = m.chatSession.GetDisplayMessages()
			m.updateWrappedMessages()
			return StreamMsg{Error: err}
		}

		// Format and display test results
		resultContent := func() string {
			if testResult.Success {
				return "✅ All tests passed! Your changes look good."
			} else {
				return fmt.Sprintf("❌ %d test(s) failed. The AI can help analyze and fix these issues.", testResult.TestsFailed)
			}
		}()

		resultSummary := fmt.Sprintf(`🧪 **Test Results**

%s

%s`, testResult.Output, resultContent)

		resultMsg := llm.Message{
			Role:      "assistant",
			Content:   resultSummary,
			Timestamp: time.Now(),
		}

		if err := m.chatSession.AddMessage(resultMsg); err != nil {
			return StreamMsg{Error: fmt.Errorf("failed to save test results: %w", err)}
		}

		// If tests failed, ask AI to analyze
		if !testResult.Success && testResult.TestsFailed > 0 {
			analysisPrompt := fmt.Sprintf(`The tests failed after running them. Here are the results:

%s

Please analyze why the tests failed and suggest how to fix them.`, testResult.Output)

			analysisMsg := llm.Message{
				Role:      "user",
				Content:   analysisPrompt,
				Timestamp: time.Now(),
			}

			if err := m.chatSession.AddMessage(analysisMsg); err != nil {
				return StreamMsg{Error: fmt.Errorf("failed to add analysis request: %w", err)}
			}

			// Refresh display and trigger AI analysis
			m.messages = m.chatSession.GetDisplayMessages()
			m.updateWrappedMessages()

			// Send to AI for analysis
			if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
				return m.sendToLLMWithTasks(analysisPrompt)()
			}
		}

		// Refresh display
		m.messages = m.chatSession.GetDisplayMessages()
		m.updateWrappedMessages()

		return StreamMsg{Content: "", Done: true}
	}
}

// isExplorationQuery checks if the user query should trigger sequential exploration
func (m *model) isExplorationQuery(userInput string) bool {
	lowerInput := strings.ToLower(userInput)

	explorationPatterns := []string{
		"tell me about",
		"check out",
		"what is this",
		"explain this",
		"analyze this",
		"how does this work",
		"what does this do",
		"show me this",
		"explore this",
		"understand this",
		"architecture",
		"codebase",
		"repository",
		"repo",
		"project",
	}

	for _, pattern := range explorationPatterns {
		if strings.Contains(lowerInput, pattern) {
			return true
		}
	}

	return false
}

// extractUserQuery extracts the most recent user query from chat history
func (m *model) extractUserQuery() string {
	messages := m.chatSession.GetMessages()

	// Look for the most recent user message (not system messages)
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			// Skip common commands
			content := strings.TrimSpace(messages[i].Content)
			if !strings.HasPrefix(content, "/") {
				return content
			}
		}
	}

	return ""
}

// formatTaskResultForLLM formats task results for LLM context (hidden from user display)
func (m *model) formatTaskResultForLLM(task *taskPkg.Task, response *taskPkg.TaskResponse) llm.Message {
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
		Role:      "system",
		Content:   content.String(),
		Timestamp: time.Now(),
	}
}

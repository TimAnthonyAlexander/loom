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
	"loom/todo"
	"os"
	"path/filepath"
	"regexp"
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

	// File autocomplete style
	fileAutocompleteStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#5D9CF1")). // Blue border
				Padding(0, 1).
				Margin(0, 0, 0, 2) // Add left margin to indent it slightly

	fileAutocompleteSelectedStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("#FFAF00")). // Highlight color for selected item
					Background(lipgloss.Color("#333333"))  // Subtle background for selected item

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

// TestPromptMsg represents a pending test prompt
type TestPromptMsg struct {
	TestCount    int
	Language     string
	EditedFiles  []string
	ShouldPrompt bool
}

// FileAutocompleteMsg represents an update to file autocomplete suggestions
type FileAutocompleteMsg struct {
	Candidates []string
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

	// File mention autocomplete
	fileAutocompleteActive        bool     // Whether file autocomplete is active
	fileAutocompleteQuery         string   // The current query for file autocomplete
	fileAutocompleteCandidates    []string // List of matching files
	fileAutocompleteSelectedIndex int      // Currently selected file index
	fileAutocompleteStartPos      int      // Position in input where @ was typed

	// Command autocomplete (e.g., /help, /stats)
	commandAutocompleteActive        bool     // Whether command autocomplete is active
	commandAutocompleteQuery         string   // Current query for command autocomplete
	commandAutocompleteCandidates    []string // Matching commands
	commandAutocompleteSelectedIndex int      // Selected command index
	commandAutocompleteStartPos      int      // Position in input where / was typed

	// LLM integration
	llmAdapter       llm.LLMAdapter
	chatSession      *chat.Session
	streamingContent string
	isStreaming      bool
	llmError         error
	streamChan       chan llm.StreamChunk
	// Allows external interruption of an active LLM request
	streamCancel context.CancelFunc

	// Task execution
	taskManager       *taskPkg.Manager
	enhancedManager   *taskPkg.EnhancedManager
	sequentialManager *taskPkg.SequentialTaskManager
	taskExecutor      *taskPkg.Executor
	todoManager       *todo.TodoManager
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

	// Enhanced completion detection
	completionDetector   *taskPkg.CompletionDetector
	currentObjective     string
	objectiveExtracted   bool
	completionCheckCount int
	maxCompletionChecks  int
	debugEnabled         bool // Unified debug flag for all debug systems

	// User visible input
	userVisibleInput string

	// Cached project summary to display at top of chat
	projectSummary string
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
				// Ctrl+C no longer quits. Ignore inside confirmation dialog.
				return m, nil
			}
			return m, nil // Ignore other keys during confirmation
		}

		switch msg.String() {
		case "ctrl+c":
			// If streaming, cancel the ongoing LLM request instead of quitting.
			if m.isStreaming {
				if m.streamCancel != nil {
					m.streamCancel()
					m.streamCancel = nil
				}
				m.isStreaming = false
				m.streamChan = nil
				m.streamingContent = "" // Clear streaming content on cancellation
				// Optional: add a brief message so the user knows the stream was interrupted.
				interruptMsg := llm.Message{Role: "assistant", Content: "⏹️ Streaming interrupted.", Timestamp: time.Now()}
				m.chatSession.AddMessage(interruptMsg)
				m.messages = m.chatSession.GetDisplayMessages()
				m.updateWrappedMessagesWithOptions(false)
			}
			return m, nil
		case "ctrl+s":
			// Generate summary with Ctrl+S
			if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
				return m, m.generateSummary("session")
			}
		case "ctrl+d":
			// Toggle debug mode with Ctrl+D for easier debugging
			m.debugEnabled = !m.debugEnabled
			if m.debugEnabled {
				taskPkg.EnableTaskDebug()
				m.addDebugMessage("Debug mode enabled")
			} else {
				taskPkg.DisableTaskDebug()
			}
			return m, nil
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
			// File autocomplete navigation
			if m.commandAutocompleteActive {
				if m.commandAutocompleteSelectedIndex > 0 {
					m.commandAutocompleteSelectedIndex--
				}
				return m, nil
			} else if m.fileAutocompleteActive {
				if m.fileAutocompleteSelectedIndex > 0 {
					m.fileAutocompleteSelectedIndex--
				}
				return m, nil
			} else if m.currentView == viewChat && m.messageScroll > 0 {
				m.messageScroll--
			}
		case "down":
			// File autocomplete navigation
			if m.commandAutocompleteActive {
				if m.commandAutocompleteSelectedIndex < len(m.commandAutocompleteCandidates)-1 {
					m.commandAutocompleteSelectedIndex++
				}
				return m, nil
			} else if m.fileAutocompleteActive {
				if m.fileAutocompleteSelectedIndex < len(m.fileAutocompleteCandidates)-1 {
					m.fileAutocompleteSelectedIndex++
				}
				return m, nil
			} else if m.currentView == viewChat {
				maxScroll := len(m.messageLines) - m.getAvailableMessageHeight()
				if maxScroll > 0 && m.messageScroll < maxScroll {
					m.messageScroll++
				}
			}
		case "esc":
			// Cancel autocompletes
			if m.fileAutocompleteActive {
				m.fileAutocompleteActive = false
			}
			if m.commandAutocompleteActive {
				m.commandAutocompleteActive = false
				return m, nil
			}
		case "enter":
			// Handle command autocomplete selection (immediate execution)
			if m.commandAutocompleteActive {
				if len(m.commandAutocompleteCandidates) > 0 {
					selectedCmd := m.commandAutocompleteCandidates[m.commandAutocompleteSelectedIndex]
					// Reset autocomplete state
					m.commandAutocompleteActive = false
					m.input = ""
					// Immediately execute the command
					return m, m.executeSlashCommand(selectedCmd)
				}
				return m, nil
			} else if m.fileAutocompleteActive {
				m.selectFileAutocomplete()
				return m, nil
			} else if m.currentView == viewChat && strings.TrimSpace(m.input) != "" && !m.isStreaming {
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
					// Toggle unified debug mode for both task parsing and completion detection
					m.debugEnabled = !m.debugEnabled

					// Also toggle task debug mode to match
					if m.debugEnabled {
						taskPkg.EnableTaskDebug()
					} else {
						taskPkg.DisableTaskDebug()
					}

					var status string
					if m.debugEnabled {
						status = "enabled"
					} else {
						status = "disabled"
					}

					debugInfo := fmt.Sprintf(`🔍 **Unified Debug Mode %s**

This mode shows debug information for:
• Task parsing (JSON extraction from LLM responses)
• Completion detection (objective tracking and analysis)
• Auto-continuation decision logic
• Loop detection and prevention

Current Status:
• Objective extracted: %t
• Current objective: "%s"
• Completion checks sent: %d/%d
• Recent responses tracked: %d

%s`,
						status,
						m.objectiveExtracted,
						m.currentObjective,
						m.completionCheckCount,
						m.maxCompletionChecks,
						len(m.recentResponses),
						func() string {
							if m.debugEnabled {
								return "Debug messages will now appear in chat."
							} else {
								return "Debug messages are now hidden."
							}
						}())

					response := llm.Message{
						Role:      "assistant",
						Content:   debugInfo,
						Timestamp: time.Now(),
					}
					m.chatSession.AddMessage(response)

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
• Ctrl+C - Cancel streaming
• /quit - Exit application

**Special Commands:**
• /files - Show file count and language breakdown
• /stats - Detailed project statistics and index information
• /tasks - Task execution history and current status
• /test - Test discovery results and execution options
• /summary - AI-generated session summary
• /rationale - Change summaries and explanations
• /debug - Toggle unified debug mode (task parsing and completion detection)
• /help - Show this help message
• /quit - Exit application

**File Mentions:**
• Type @ to activate file autocomplete
• Navigate suggestions with arrow keys
• Press Enter to select a file
• Press Esc to cancel
• Selected files are automatically included in your messages

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
				// Handle command autocomplete backspace
				if m.commandAutocompleteActive {
					if m.commandAutocompleteStartPos == len(m.input)-1 {
						m.commandAutocompleteActive = false
					} else {
						m.input = m.input[:len(m.input)-1]
						m.commandAutocompleteQuery = m.input[m.commandAutocompleteStartPos+1:]
						m.commandAutocompleteCandidates = m.getCommandCandidates(m.commandAutocompleteQuery)
						if m.commandAutocompleteSelectedIndex >= len(m.commandAutocompleteCandidates) {
							m.commandAutocompleteSelectedIndex = len(m.commandAutocompleteCandidates) - 1
						} else if m.commandAutocompleteSelectedIndex < 0 {
							// Clamp negative index to zero to avoid out-of-range panics
							m.commandAutocompleteSelectedIndex = 0
						}
						return m, nil
					}
				}
				// Handle file autocomplete backspace
				if m.fileAutocompleteActive {
					// If we're right after the @ symbol, cancel autocomplete
					if m.fileAutocompleteStartPos == len(m.input)-1 {
						m.fileAutocompleteActive = false
					} else {
						m.input = m.input[:len(m.input)-1]
						m.fileAutocompleteQuery = m.input[m.fileAutocompleteStartPos+1:]
						return m, m.updateFileAutocompleteCandidates()
					}
				}

				m.input = m.input[:len(m.input)-1]
			}
		case "@":
			if m.currentView == viewChat && !m.isStreaming {
				m.input += "@"
				m.fileAutocompleteActive = true
				m.fileAutocompleteQuery = ""
				m.fileAutocompleteCandidates = []string{}
				m.fileAutocompleteSelectedIndex = 0
				m.fileAutocompleteStartPos = len(m.input) - 1

				// Debug logging
				if m.debugEnabled {
					m.addDebugMessage("File autocomplete activated")
				}

				return m, m.updateFileAutocompleteCandidates()
			}
		case "/":
			if m.currentView == viewChat && !m.isStreaming {
				m.input += "/"
				// Trigger command autocomplete only if this is the first character (start of input)
				if len(m.input) == 1 {
					m.commandAutocompleteActive = true
					m.commandAutocompleteQuery = ""
					m.commandAutocompleteCandidates = m.getCommandCandidates("")
					m.commandAutocompleteSelectedIndex = 0
					m.commandAutocompleteStartPos = len(m.input) - 1
				}
				return m, nil
			}
		default:
			if m.currentView == viewChat && !m.isStreaming {
				// Handle characters during command autocomplete first
				if m.commandAutocompleteActive {
					char := msg.String()
					// Space ends autocomplete
					if char == " " {
						m.commandAutocompleteActive = false
						m.input += char
						return m, nil
					}

					m.input += char
					m.commandAutocompleteQuery = m.input[m.commandAutocompleteStartPos+1:]
					m.commandAutocompleteCandidates = m.getCommandCandidates(m.commandAutocompleteQuery)
					if m.commandAutocompleteSelectedIndex >= len(m.commandAutocompleteCandidates) {
						m.commandAutocompleteSelectedIndex = len(m.commandAutocompleteCandidates) - 1
					} else if m.commandAutocompleteSelectedIndex < 0 {
						// Clamp negative index to zero to avoid out-of-range panics
						m.commandAutocompleteSelectedIndex = 0
					}
					return m, nil
				} else if m.fileAutocompleteActive {
					// Handle characters typed during autocomplete
					char := msg.String()

					// Space ends autocomplete
					if char == " " {
						m.fileAutocompleteActive = false
						m.input += char
						return m, nil
					}

					// Update autocomplete query
					m.input += char
					m.fileAutocompleteQuery = m.input[m.fileAutocompleteStartPos+1:]
					return m, m.updateFileAutocompleteCandidates()
				} else {
					m.input += msg.String()
				}
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
			m.streamingContent = "" // Clear streaming content on error
			m.messages = append(m.messages, fmt.Sprintf("Loom: Error: %v", msg.Error))
			m.updateWrappedMessagesWithOptions(false)
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

				// Save streaming content for task processing before clearing
				finalStreamingContent := m.streamingContent
				m.streamingContent = "" // Clear streaming content to prevent artifacts

				// Update wrapped messages with the final state
				m.updateWrappedMessagesWithOptions(false)

				// Process LLM response for tasks (use original content for task parsing)
				return m, m.handleLLMResponseForTasks(finalStreamingContent)
			}

			// Clear streaming content even if empty to ensure clean state
			m.streamingContent = ""
			m.updateWrappedMessagesWithOptions(false)
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

	case FileAutocompleteMsg:
		// Update file autocomplete candidates
		m.fileAutocompleteCandidates = msg.Candidates
		m.fileAutocompleteSelectedIndex = 0
		// Force re-render
		return m, nil

	case ContinueLLMMsg:
		// Continue LLM conversation after task completion
		if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
			// Refresh messages from chat session to include task results
			m.messages = m.chatSession.GetDisplayMessages()
			m.updateWrappedMessages()

			// Always continue with LLM conversation to allow completion message
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
	// Calculate heights first
	headerHeight := 1 // Title
	if m.showInfoPanel {
		headerHeight += 9 // Info panel + border + spacing
	}
	navHeight := 1 // Navigation at bottom

	// Available height for content (messages + input)
	contentHeight := m.height - headerHeight - navHeight
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Title with enhanced styling
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#7D56F4"}).
		Padding(0, 2).
		Width(m.width).
		Align(lipgloss.Center).
		Render("✨ Loom - AI Coding Assistant")

	// Header section
	var header string
	if m.showInfoPanel {
		// Index stats for info panel
		stats := m.index.GetStats()
		langSummary := m.getLanguageSummary(stats)

		// Workspace info
		var modelStatus string
		if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
			modelStatus = "🟢 " + m.config.Model
		} else {
			modelStatus = "🔴 " + m.config.Model
		}

		// Base info content
		infoContent := fmt.Sprintf(
			" %s\n %s  |  Shell Access: %t\n %d files %s",
			m.workspacePath,
			modelStatus,
			m.config.EnableShell,
			stats.TotalFiles,
			langSummary,
		)

		// Add todo list if active
		if m.todoManager != nil && m.todoManager.HasActiveTodoList() {
			todoDisplay := m.renderTodoPanel()
			infoContent += "\n" + todoDisplay
		}

		infoPanel := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#874BFD"}).
			Padding(1, 2).
			Width(m.width - 4).
			Render(infoContent)

		header = lipgloss.JoinVertical(lipgloss.Left, title, infoPanel)
	} else {
		header = title
	}

	// Show confirmation dialog if needed (full screen override)
	if m.showingConfirmation && m.pendingConfirmation != nil {
		confirmDialog := m.renderConfirmationDialog()
		// Navigation for confirmation
		nav := lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "#f0f0f0", Dark: "#2a2a2a"}).
			Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}).
			Width(m.width).
			Align(lipgloss.Center).
			Render("Press 'y' to approve, 'n' to cancel, Ctrl+C to quit")

		content := lipgloss.NewStyle().Height(contentHeight).Render(confirmDialog)
		return lipgloss.JoinVertical(lipgloss.Left, header, content, nav)
	}

	var mainContent string

	switch m.currentView {
	case viewChat:
		// Update wrapped messages consistently
		m.updateWrappedMessagesWithOptions(m.isStreaming)

		// Calculate message area height (subtract 2 for input + spacing)
		messageHeight := contentHeight - 2
		if messageHeight < 3 {
			messageHeight = 3
		}

		// Get visible message lines based on scroll position
		var visibleLines []string
		if len(m.messageLines) > 0 {
			start := m.messageScroll
			end := m.messageScroll + messageHeight

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

		// Message area with proper height
		messageText := strings.Join(visibleLines, "\n")
		messageWidth := m.width - 4 // Account for padding
		if messageWidth < 40 {
			messageWidth = 40
		}

		messages := lipgloss.NewStyle().
			Width(messageWidth).
			Height(messageHeight).
			Padding(0, 1).
			Render(messageText)

		// Show scroll indicator if there are more messages
		var scrollIndicator string
		if len(m.messageLines) > messageHeight {
			scrollIndicator = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}).
				Render(fmt.Sprintf(" [%d/%d]",
					m.messageScroll+1,
					len(m.messageLines)-messageHeight+1))
		}

		// Input area with enhanced styling
		inputPrefix := "💬 "
		if m.isStreaming {
			inputPrefix = "🧠 "
		}

		inputContent := fmt.Sprintf("%s%s%s", inputPrefix, m.input, scrollIndicator)
		input := lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#dddddd", Dark: "#444444"}).
			Padding(0, 1).
			Width(m.width - 2).
			Render(inputContent)

		// Add autocomplete dropdowns if active
		if m.fileAutocompleteActive {
			autocompleteView := m.renderFileAutocomplete()
			// Style the autocomplete view to ensure it's clearly visible
			styledAutocomplete := lipgloss.NewStyle().
				Margin(1, 0, 0, 2). // Add margin for visibility (top, right, bottom, left)
				MaxWidth(m.width - 10).
				Render(autocompleteView)

			mainContent = lipgloss.JoinVertical(lipgloss.Left, messages, input, styledAutocomplete)
		} else if m.commandAutocompleteActive {
			autocompleteView := m.renderCommandAutocomplete()
			styledAutocomplete := lipgloss.NewStyle().
				Margin(1, 0, 0, 2).
				MaxWidth(m.width - 10).
				Render(autocompleteView)

			mainContent = lipgloss.JoinVertical(lipgloss.Left, messages, input, styledAutocomplete)
		} else {
			mainContent = lipgloss.JoinVertical(lipgloss.Left, messages, input)
		}

	case viewFileTree:
		// File tree view with proper height
		treeContent := m.renderFileTree()
		fileTree := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#A550DF", Dark: "#A550DF"}).
			Padding(1, 2).
			Width(m.width - 4).
			Height(contentHeight - 2).
			Render(treeContent)
		mainContent = fileTree

	case viewTasks:
		// Task execution view with proper height
		taskContent := m.renderTaskView()
		taskView := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#FFA500", Dark: "#FFA500"}).
			Padding(1, 2).
			Width(m.width - 4).
			Height(contentHeight - 2).
			Render(taskContent)
		mainContent = taskView
	}

	// Navigation bar at the very bottom with enhanced styling
	var helpText string
	switch m.currentView {
	case viewChat:
		helpText = "Tab: Views  •  ↑↓: Scroll  •  Ctrl+S: Summary  •  /help: Commands  •  Ctrl+C: Cancel"
	case viewFileTree:
		helpText = "Tab: Chat/Tasks  •  /help: All Commands  •  Ctrl+C: Cancel"
	case viewTasks:
		helpText = "Tab: Chat/File Tree  •  /help: All Commands  •  Ctrl+C: Cancel"
	}

	nav := lipgloss.NewStyle().
		Background(lipgloss.AdaptiveColor{Light: "#f8f9fa", Dark: "#1a1a1a"}).
		Foreground(lipgloss.AdaptiveColor{Light: "#6c757d", Dark: "#adb5bd"}).
		Width(m.width).
		Align(lipgloss.Center).
		Padding(0, 1).
		Render(helpText)

	// Ensure content takes up exactly the right height
	content := lipgloss.NewStyle().Height(contentHeight).Render(mainContent)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, nav)
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

// renderTodoPanel renders the todo list for the info panel
func (m model) renderTodoPanel() string {
	if m.todoManager == nil {
		return ""
	}

	currentList := m.todoManager.GetCurrentList()
	if currentList == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("📝 TODO:")

	// Show only first 3 items to keep info panel compact
	itemsToShow := len(currentList.Items)
	if itemsToShow > 3 {
		itemsToShow = 3
	}

	for i := 0; i < itemsToShow; i++ {
		item := currentList.Items[i]
		status := "⬜"
		if item.Checked {
			status = "✅"
		} else if i > 0 && !currentList.Items[i-1].Checked {
			status = "🔒" // Locked because previous item not checked
		}

		// Truncate long titles for compact display
		title := item.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}

		sb.WriteString(fmt.Sprintf(" %s %s", status, title))
	}

	// Show "..." if there are more items
	if len(currentList.Items) > 3 {
		remaining := len(currentList.Items) - 3
		sb.WriteString(fmt.Sprintf(" (+%d more)", remaining))
	}

	return sb.String()
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
			if langs[i].percent != langs[j].percent {
				return langs[i].percent > langs[j].percent
			}
			return langs[i].name < langs[j].name
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
		// Process @file mentions to include file content
		processedInput := m.processFileMentions(userInput)

		// Determine what to display in chat (user visible version if available)
		displayInput := userInput
		if m.userVisibleInput != "" {
			displayInput = m.userVisibleInput
			m.userVisibleInput = "" // Reset for next message
		}

		// Add user message to chat session first with user-friendly version for display
		userDisplayMessage := llm.Message{
			Role:      "user",
			Content:   displayInput,
			Timestamp: time.Now(),
		}

		if err := m.chatSession.AddMessage(userDisplayMessage); err != nil {
			return StreamMsg{Error: fmt.Errorf("failed to save user message: %w", err)}
		}

		// Also save the processed version for LLM (with file contents) internally
		userLLMMessage := llm.Message{
			Role:      "user",
			Content:   processedInput,
			Timestamp: time.Now(),
		}

		// Reset completion detection state for new user queries (not completion checks)
		if !strings.HasPrefix(userInput, "COMPLETION_CHECK:") {
			m.currentObjective = ""
			m.objectiveExtracted = false
			m.completionCheckCount = 0
			m.recursiveDepth = 0
			// Reset context tracking for the completion detector
			if m.completionDetector != nil {
				m.completionDetector.ResetContext()
			}
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
		// Save cancel func so we can interrupt with Ctrl+C
		m.streamCancel = cancel

		// Get messages for the LLM with optimized context
		var messages []llm.Message
		var err error

		// Get max context tokens from config or use default
		maxContextTokens := 6000 // Default
		if tokenValue, err := m.config.Get("max_context_tokens"); err == nil {
			if tokenInt, ok := tokenValue.(int); ok && tokenInt > 0 {
				maxContextTokens = tokenInt
			}
		}

		// Create a context manager if needed
		contextManager := contextMgr.NewContextManager(m.index, maxContextTokens)

		// Use optimized context instead of full history
		messages, err = m.chatSession.GetOptimizedContextMessages(contextManager, maxContextTokens)
		if err != nil {
			defer cancel()
			return StreamMsg{Error: fmt.Errorf("context optimization error: %w", err)}
		}

		// Replace the last user message with the processed version containing file contents
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "user" {
				messages[i] = userLLMMessage
				break
			}
		}

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
		m.streamCancel = cancel

		// Get messages for the LLM with optimized context
		var messages []llm.Message
		var err error

		// Get max context tokens from config or use default
		maxContextTokens := 6000 // Default
		if tokenValue, err := m.config.Get("max_context_tokens"); err == nil {
			if tokenInt, ok := tokenValue.(int); ok && tokenInt > 0 {
				maxContextTokens = tokenInt
			}
		}

		// Create a context manager if needed
		contextManager := contextMgr.NewContextManager(m.index, maxContextTokens)

		// Use optimized context instead of full history
		messages, err = m.chatSession.GetOptimizedContextMessages(contextManager, maxContextTokens)
		if err != nil {
			defer cancel()
			return StreamMsg{Error: fmt.Errorf("context optimization error: %w", err)}
		}

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

		// Add to recent responses for loop detection
		m.recentResponses = append(m.recentResponses, llmResponse)
		if len(m.recentResponses) > 5 {
			m.recentResponses = m.recentResponses[1:]
		}

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
				// If tasks were executed, wait for completion before checking
				return nil
			}
		}

		// No tasks found - this is a regular Q&A response
		// ALWAYS trigger completion checking for objective-based workflow
		if !strings.HasPrefix(llmResponse, "COMPLETION_CHECK:") {
			return AutoContinueMsg{
				LastResponse: llmResponse,
				Depth:        m.recursiveDepth,
			}
		}

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

// updateFileAutocompleteCandidates updates the file autocomplete candidates based on current query
func (m *model) updateFileAutocompleteCandidates() tea.Cmd {
	return func() tea.Msg {
		query := m.fileAutocompleteQuery
		candidates := m.index.SearchFiles(query, 10) // Limit to 10 results

		// Debug logging
		debugMsg := fmt.Sprintf("File autocomplete query: '%s', found %d candidates", query, len(candidates))
		if m.debugEnabled {
			m.addDebugMessage(debugMsg)
		}

		m.fileAutocompleteCandidates = candidates
		m.fileAutocompleteSelectedIndex = 0

		return FileAutocompleteMsg{Candidates: candidates}
	}
}

// selectFileAutocomplete selects the currently highlighted file in autocomplete
func (m *model) selectFileAutocomplete() {
	if len(m.fileAutocompleteCandidates) == 0 {
		m.fileAutocompleteActive = false
		return
	}

	// Replace @query with @filename and add a space after it
	selectedFile := m.fileAutocompleteCandidates[m.fileAutocompleteSelectedIndex]
	beforeAt := m.input[:m.fileAutocompleteStartPos+1]
	afterQuery := m.input[m.fileAutocompleteStartPos+1+len(m.fileAutocompleteQuery):]

	// Add a space after the selected filename to allow continuing with the next word
	m.input = beforeAt + selectedFile + " " + afterQuery
	m.fileAutocompleteActive = false
}

// readFileSnippet reads the first N lines of a file
func (m *model) readFileSnippet(filename string, lineCount int) (string, error) {
	filePath := filepath.Join(m.workspacePath, filename)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) > lineCount {
		lines = lines[:lineCount]
	}

	return strings.Join(lines, "\n"), nil
}

// processFileMentions processes @file mentions in user input to include file content
func (m *model) processFileMentions(input string) string {
	re := regexp.MustCompile(`@([^\s]+)`)
	matches := re.FindAllStringSubmatch(input, -1)

	result := input

	// If no matches, just return the input
	if len(matches) == 0 {
		return input
	}

	// Create user visible version with shortened content
	userVisibleVersion := input

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		filename := match[1]
		content, err := m.readFileSnippet(filename, 50)
		if err != nil {
			continue
		}

		// Create full version for LLM (with full content)
		fullReplacement := fmt.Sprintf("@%s\n```%s (first 50 lines):\n%s\n```",
			filename, filename, content)
		result = strings.Replace(result, match[0], fullReplacement, 1)

		// Create shortened version for user display (with truncated content)
		shortContent := content
		if len(shortContent) > 100 {
			shortContent = shortContent[:100] + "..."
		}
		userReplacement := fmt.Sprintf("@%s [file attached, %d lines]",
			filename, strings.Count(content, "\n")+1)
		userVisibleVersion = strings.Replace(userVisibleVersion, match[0], userReplacement, 1)
	}

	// Save the user visible version for display
	m.userVisibleInput = userVisibleVersion

	return result
}

// renderFileAutocomplete renders the file autocomplete dropdown
func (m model) renderFileAutocomplete() string {
	if !m.fileAutocompleteActive {
		return ""
	}

	// If no candidates but autocomplete is active, show a message
	if len(m.fileAutocompleteCandidates) == 0 {
		return fileAutocompleteStyle.
			Width(40).
			Render("📁 No matching files found\n\nType to search or press Esc to cancel")
	}

	var sb strings.Builder
	maxWidth := 0

	// Calculate max width needed
	for _, file := range m.fileAutocompleteCandidates {
		if len(file) > maxWidth {
			maxWidth = len(file)
		}
	}
	maxWidth += 6 // Add padding and for the selection marker

	// Title row
	sb.WriteString("📁 Files matching: " + m.fileAutocompleteQuery + "\n\n")

	// Build list with styled items
	for i, file := range m.fileAutocompleteCandidates {
		if i == m.fileAutocompleteSelectedIndex {
			// Selected item gets special styling
			highlightedFile := fileAutocompleteSelectedStyle.Render(fmt.Sprintf(" %s ", file))
			sb.WriteString("▶ " + highlightedFile + "\n")
		} else {
			sb.WriteString("  " + file + "\n")
		}
	}

	// Footer with help
	sb.WriteString("\n↑↓: Select • Enter: Choose • Esc: Cancel")

	// Apply the main container style
	return fileAutocompleteStyle.
		Width(maxWidth).
		Render(sb.String())
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

	// Handle objective change with auto-continuation
	if event.Type == "objective_change_auto_continue" {
		// Add the warning message to the display
		warningMessage := fmt.Sprintf("🚨 %s", event.Message)

		// Add warning as a system message visible to user
		systemMsg := llm.Message{
			Role:      "assistant",
			Content:   warningMessage,
			Timestamp: time.Now(),
		}
		m.chatSession.AddMessage(systemMsg)

		// Refresh display to show the warning
		m.messages = m.chatSession.GetDisplayMessages()
		m.updateWrappedMessages()

		// Automatically continue the LLM conversation
		if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
			return m, func() tea.Msg {
				return ContinueLLMMsg{}
			}
		}
		return m, nil
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

	// Use the response task which contains the properly combined content for targeted edits
	task := m.pendingConfirmation.Response.Task
	// Save the response before clearing pendingConfirmation
	taskResponse := m.pendingConfirmation.Response
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
			err = m.enhancedManager.ConfirmTask(&task, taskResponse, true)
		} else if m.taskManager != nil {
			// Fall back to basic manager
			err = m.taskManager.ConfirmTask(&task, taskResponse, true)
		} else {
			err = fmt.Errorf("no task manager available")
		}
	}

	// Format result message using the manager's formatting method
	var formattedResult string
	if m.taskManager != nil {
		formattedResult = m.taskManager.FormatConfirmationResult(&task, approved, err)
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
	// Check for infinite loop patterns first
	if m.completionDetector.HasInfiniteLoopPattern(m.recentResponses) {
		if m.debugEnabled {
			m.addDebugMessage("🔄 Loop pattern detected, stopping auto-continuation")
		}
		m.recursiveDepth = 0
		m.completionCheckCount = 0
		return m, nil
	}

	// Check completion check count to prevent excessive checking
	if m.completionCheckCount >= m.maxCompletionChecks {
		if m.debugEnabled {
			m.addDebugMessage(fmt.Sprintf("🛑 Maximum completion checks reached (%d), stopping", m.maxCompletionChecks))
		}
		m.recursiveDepth = 0
		m.completionCheckCount = 0
		return m, nil
	}

	// Check if the response is a text-only message with no commands
	if isTextOnlyResponse(msg.LastResponse) {
		if m.debugEnabled {
			m.addDebugMessage("✅ Work appears complete - received text-only response")
		}
		m.recursiveDepth = 0
		m.completionCheckCount = 0
		return m, nil
	}

	// Continue with next iteration
	m.recursiveDepth = msg.Depth + 1
	m.completionCheckCount++

	if m.recursiveDepth == 1 {
		m.recursiveStartTime = time.Now()
	}

	// Simple continuation prompt
	continuationPrompt := "Continue."

	if m.debugEnabled {
		m.addDebugMessage(fmt.Sprintf("🔄 Sending continuation prompt #%d", m.completionCheckCount))
	}

	autoMessage := llm.Message{
		Role:      "user",
		Content:   continuationPrompt,
		Timestamp: time.Now(),
	}

	if err := m.chatSession.AddMessage(autoMessage); err != nil {
		return m, func() tea.Msg {
			return StreamMsg{Error: fmt.Errorf("failed to add continuation prompt: %w", err)}
		}
	}

	return m, m.sendToLLMWithTasks(continuationPrompt)
}

// isTextOnlyResponse checks if a response contains no tasks or commands
func isTextOnlyResponse(response string) bool {
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

// addDebugMessage helper method to add debug messages to chat when debug mode is enabled
func (m *model) addDebugMessage(message string) {
	debugMsg := llm.Message{
		Role:      "assistant",
		Content:   fmt.Sprintf("🔍 **DEBUG**: %s", message),
		Timestamp: time.Now(),
	}

	m.chatSession.AddMessage(debugMsg)
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
		if langs[i].percent != langs[j].percent {
			return langs[i].percent > langs[j].percent
		}
		return langs[i].name < langs[j].name
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
	// Use the same logic as the View() function
	headerHeight := 1 // Title
	if m.showInfoPanel {
		headerHeight += 9 // Info panel + border + spacing
	}
	navHeight := 1 // Navigation at bottom

	// Available height for content (messages + input)
	contentHeight := m.height - headerHeight - navHeight
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Message area height (subtract 2 for input + its border)
	messageHeight := contentHeight - 2
	if messageHeight < 3 {
		messageHeight = 3
	}

	return messageHeight
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
	// Ensure project summary appears at the top of chat view
	if m.projectSummary != "" {
		if len(m.messages) == 0 || m.messages[0] != m.projectSummary {
			m.messages = append([]string{m.projectSummary, ""}, m.messages...)
		}
	}
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
			debugStatus = "\nDEBUGGING ENABLED"
		} else {
			debugStatus = ""
		}

		welcomeMsg := "Welcome to Loom!\nYour AI assistant is here to help you code, understand, and improve your project." + debugStatus
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
			if langs[i].percent != langs[j].percent {
				return langs[i].percent > langs[j].percent
			}
			return langs[i].name < langs[j].name
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

	adapter, err := llm.CreateAdapterFromConfig(cfg)
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

	// Initialize todo manager
	todoManager := todo.NewTodoManager(workspacePath)

	// Configure validation settings
	if cfg.Validation.EnableVerification {
		taskExecutor.SetValidationConfigFromMainConfig(cfg)
	}
	var taskManager *taskPkg.Manager
	var enhancedManager *taskPkg.EnhancedManager
	var sequentialManager *taskPkg.SequentialTaskManager
	var taskEventChan chan taskPkg.TaskExecutionEvent

	if llmAdapter != nil {
		enhancedManager = taskPkg.NewEnhancedManager(taskExecutor, llmAdapter, chatSession, idx)
		taskManager = enhancedManager.Manager // For compatibility
		sequentialManager = taskPkg.NewSequentialTaskManager(taskExecutor, llmAdapter, chatSession)

		// Get max context tokens from config or use default
		maxContextTokens := 6000 // Default
		if tokenValue, err := cfg.Get("max_context_tokens"); err == nil {
			if tokenInt, ok := tokenValue.(int); ok && tokenInt > 0 {
				maxContextTokens = tokenInt
			}
		}

		// Set context manager for optimized context
		sequentialManager.SetContextManager(idx, maxContextTokens)

		taskEventChan = make(chan taskPkg.TaskExecutionEvent, 10)
	}

	// -----------------------------------------------------------
	// Project summary (generate once per workspace and cache)
	// -----------------------------------------------------------
	projectSummary := ""
	if summary, err := loadOrGenerateProjectSummary(workspacePath, idx, llmAdapter); err == nil {
		projectSummary = strings.TrimSpace(summary)
	} else {
		fmt.Printf("Warning: failed to load/generate project summary: %v\n", err)
	}

	// Add enhanced system prompt if this is a new session (no previous messages)
	if len(chatSession.GetMessages()) == 0 {
		promptEnhancer := llm.NewPromptEnhancer(workspacePath, idx)
		// Set memory store for memory integration in system prompt
		if taskExecutor != nil {
			promptEnhancer.SetMemoryStore(taskExecutor.GetMemoryStore())
		}
		// Set todo manager for todo integration in system prompt
		promptEnhancer.SetTodoManager(todoManager)
		systemPrompt := promptEnhancer.CreateEnhancedSystemPrompt(cfg.EnableShell)
		if err := chatSession.AddMessage(systemPrompt); err != nil {
			fmt.Printf("Warning: failed to add system prompt: %v\n", err)
		}
	}

	m := model{
		workspacePath: workspacePath,
		config:        cfg,
		index:         idx,
		input:         "",
		messages: func() []string {
			msgs := chatSession.GetDisplayMessages()
			if projectSummary != "" {
				summaryDisplay := fmt.Sprintf("%s", projectSummary)
				msgs = append([]string{summaryDisplay, ""}, msgs...)
			}
			return msgs
		}(),
		currentView:       viewChat,
		llmAdapter:        llmAdapter,
		chatSession:       chatSession,
		llmError:          llmError,
		taskManager:       taskManager,
		enhancedManager:   enhancedManager,
		sequentialManager: sequentialManager,
		taskExecutor:      taskExecutor,
		todoManager:       todoManager,
		taskEventChan:     taskEventChan,
		taskHistory:       make([]string, 0),
		width:             80, // Default width until window size is received
		height:            24, // Default height until window size is received

		// Initialize recursive tracking with safety defaults
		recentResponses:   make([]string, 0, 5),
		maxRecursiveDepth: 15,
		maxRecursiveTime:  30 * time.Minute,
		showInfoPanel:     true,

		// Initialize completion detection
		completionDetector:  taskPkg.NewCompletionDetector(),
		maxCompletionChecks: 5, // Allow more thorough completion checking
		projectSummary:      projectSummary,
	}

	// Set up unified debug handler to send debug messages to chat instead of breaking TUI
	taskPkg.SetDebugHandler(func(message string) {
		if m.debugEnabled {
			debugMsg := llm.Message{
				Role:      "assistant",
				Content:   fmt.Sprintf("🔍 **DEBUG**: %s", message),
				Timestamp: time.Now(),
			}
			m.chatSession.AddMessage(debugMsg)
			m.messages = m.chatSession.GetDisplayMessages()
			m.updateWrappedMessages()
		}
	})

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

// selectCommandAutocomplete selects the highlighted command suggestion
func (m *model) selectCommandAutocomplete() {
	if len(m.commandAutocompleteCandidates) == 0 {
		m.commandAutocompleteActive = false
		return
	}

	selected := m.commandAutocompleteCandidates[m.commandAutocompleteSelectedIndex]

	beforeSlash := m.input[:m.commandAutocompleteStartPos]
	afterQuery := m.input[m.commandAutocompleteStartPos+1+len(m.commandAutocompleteQuery):]

	// Insert the full selected command (which already includes the leading '/')
	m.input = beforeSlash + selected + " " + afterQuery
	m.commandAutocompleteActive = false
}

// getCommandCandidates returns commands matching the query prefix (case-insensitive)
func (m *model) getCommandCandidates(query string) []string {
	commands := []string{"/files", "/stats", "/tasks", "/test", "/summary", "/rationale", "/debug", "/help", "/quit", "/todo"}

	if query == "" {
		return commands
	}

	lower := strings.ToLower(query)
	var res []string
	for _, cmd := range commands {
		withoutSlash := strings.TrimPrefix(cmd, "/")
		if strings.HasPrefix(strings.ToLower(withoutSlash), lower) {
			res = append(res, cmd)
		}
	}
	return res
}

// renderCommandAutocomplete renders the command autocomplete dropdown
func (m model) renderCommandAutocomplete() string {
	if !m.commandAutocompleteActive {
		return ""
	}

	if len(m.commandAutocompleteCandidates) == 0 {
		return fileAutocompleteStyle.
			Width(40).
			Render("⌨️ No matching commands\n\nType to search or press Esc to cancel")
	}

	var sb strings.Builder
	maxWidth := 0
	for _, cmd := range m.commandAutocompleteCandidates {
		if len(cmd) > maxWidth {
			maxWidth = len(cmd)
		}
	}
	maxWidth += 6

	sb.WriteString("⌨️ Commands matching: " + m.commandAutocompleteQuery + "\n\n")

	for i, cmd := range m.commandAutocompleteCandidates {
		if i == m.commandAutocompleteSelectedIndex {
			highlighted := fileAutocompleteSelectedStyle.Render(" " + cmd + " ")
			sb.WriteString("▶ " + highlighted + "\n")
		} else {
			sb.WriteString("  " + cmd + "\n")
		}
	}

	sb.WriteString("\n↑↓: Select • Enter: Choose • Esc: Cancel")

	return fileAutocompleteStyle.Width(maxWidth).Render(sb.String())
}

// executeSlashCommand processes built-in slash commands immediately
func (m *model) executeSlashCommand(cmdStr string) tea.Cmd {
	switch cmdStr {
	case "/quit":
		return tea.Quit

	case "/files":
		stats := m.index.GetStats()
		// Log to chat
		userMsg := llm.Message{Role: "user", Content: cmdStr, Timestamp: time.Now()}
		m.chatSession.AddMessage(userMsg)
		resp := llm.Message{Role: "assistant", Content: fmt.Sprintf("Indexed %d files", stats.TotalFiles), Timestamp: time.Now()}
		m.chatSession.AddMessage(resp)
		m.messages = m.chatSession.GetDisplayMessages()
		m.updateWrappedMessages()
		return nil

	case "/stats":
		userMsg := llm.Message{Role: "user", Content: cmdStr, Timestamp: time.Now()}
		m.chatSession.AddMessage(userMsg)
		resp := llm.Message{Role: "assistant", Content: m.getIndexStatsMessage(), Timestamp: time.Now()}
		m.chatSession.AddMessage(resp)
		m.messages = m.chatSession.GetDisplayMessages()
		m.updateWrappedMessages()
		return nil

	case "/tasks":
		userMsg := llm.Message{Role: "user", Content: cmdStr, Timestamp: time.Now()}
		m.chatSession.AddMessage(userMsg)

		var respContent string
		if m.currentExecution != nil {
			history := m.taskManager.GetTaskHistory(m.currentExecution)
			respContent = fmt.Sprintf("Task history:\n%s", strings.Join(history, "\n"))
		} else {
			respContent = "No task execution in progress"
		}
		resp := llm.Message{Role: "assistant", Content: respContent, Timestamp: time.Now()}
		m.chatSession.AddMessage(resp)
		m.messages = m.chatSession.GetDisplayMessages()
		m.updateWrappedMessages()
		return nil

	case "/summary":
		// summary requires LLM
		userMsg := llm.Message{Role: "user", Content: cmdStr, Timestamp: time.Now()}
		m.chatSession.AddMessage(userMsg)
		if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
			m.messages = m.chatSession.GetDisplayMessages()
			m.updateWrappedMessages()
			return m.generateSummary("session")
		}
		errResp := llm.Message{Role: "assistant", Content: "Summary feature requires LLM to be available. Please configure your model and API key.", Timestamp: time.Now()}
		m.chatSession.AddMessage(errResp)
		m.messages = m.chatSession.GetDisplayMessages()
		m.updateWrappedMessages()
		return nil

	case "/rationale":
		userMsg := llm.Message{Role: "user", Content: cmdStr, Timestamp: time.Now()}
		m.chatSession.AddMessage(userMsg)
		m.messages = m.chatSession.GetDisplayMessages()
		m.updateWrappedMessages()
		return m.showRationale()

	case "/test":
		userMsg := llm.Message{Role: "user", Content: cmdStr, Timestamp: time.Now()}
		m.chatSession.AddMessage(userMsg)
		m.messages = m.chatSession.GetDisplayMessages()
		m.updateWrappedMessages()
		return m.showTestSummary()

	case "/debug":
		// toggle debug
		m.debugEnabled = !m.debugEnabled
		if m.debugEnabled {
			taskPkg.EnableTaskDebug()
		} else {
			taskPkg.DisableTaskDebug()
		}
		status := "disabled"
		if m.debugEnabled {
			status = "enabled"
		}
		debugInfo := fmt.Sprintf("🔍 **Unified Debug Mode %s**", status)
		userMsg := llm.Message{Role: "user", Content: cmdStr, Timestamp: time.Now()}
		m.chatSession.AddMessage(userMsg)
		resp := llm.Message{Role: "assistant", Content: debugInfo, Timestamp: time.Now()}
		m.chatSession.AddMessage(resp)
		m.messages = m.chatSession.GetDisplayMessages()
		m.updateWrappedMessages()
		return nil

	case "/todo":
		userMsg := llm.Message{Role: "user", Content: cmdStr, Timestamp: time.Now()}
		m.chatSession.AddMessage(userMsg)

		var todoContent string
		if m.todoManager != nil {
			todoContent = m.todoManager.GetTodoStatus()
		} else {
			todoContent = "Todo manager not available"
		}

		resp := llm.Message{Role: "assistant", Content: todoContent, Timestamp: time.Now()}
		m.chatSession.AddMessage(resp)
		m.messages = m.chatSession.GetDisplayMessages()
		m.updateWrappedMessages()
		return nil

	case "/help":
		userMsg := llm.Message{Role: "user", Content: cmdStr, Timestamp: time.Now()}
		m.chatSession.AddMessage(userMsg)
		// Reuse helpContent string from earlier path (simplified call)
		helpContent := `🤖 **Loom Help**\n\nUse /files, /stats, /tasks, /test, /summary, /rationale, /debug, /help, /quit, /todo` // shorter help here
		resp := llm.Message{Role: "assistant", Content: helpContent, Timestamp: time.Now()}
		m.chatSession.AddMessage(resp)
		m.messages = m.chatSession.GetDisplayMessages()
		m.updateWrappedMessages()
		return nil
	}
	return nil
}

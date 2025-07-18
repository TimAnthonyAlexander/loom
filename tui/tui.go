package tui

import (
	"context"
	"fmt"
	"loom/chat"
	"loom/config"
	"loom/indexer"
	"loom/llm"
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
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#04B575")).
			Padding(0, 1)

	messageStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#F25D94")).
			Padding(1, 2).
			Height(10)

	fileTreeStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#A550DF")).
			Padding(1, 2).
			Height(15)
)

type viewMode int

const (
	viewChat viewMode = iota
	viewFileTree
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
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			// Switch between views
			if m.currentView == viewChat {
				m.currentView = viewFileTree
			} else {
				m.currentView = viewChat
			}
		case "up":
			if m.currentView == viewFileTree && m.fileTreeScroll > 0 {
				m.fileTreeScroll--
			}
		case "down":
			if m.currentView == viewFileTree {
				files := m.index.GetFileList()
				maxScroll := len(files) - 10 // Show 10 files at a time
				if maxScroll < 0 {
					maxScroll = 0
				}
				if m.fileTreeScroll < maxScroll {
					m.fileTreeScroll++
				}
			}
		case "enter":
			if m.currentView == viewChat && strings.TrimSpace(m.input) != "" && !m.isStreaming {
				userInput := strings.TrimSpace(m.input)
				m.input = ""

				// Handle special commands
				if userInput == "/quit" {
					return m, tea.Quit
				} else if userInput == "/files" {
					stats := m.index.GetStats()
					m.messages = append(m.messages, fmt.Sprintf("> %s", userInput))
					m.messages = append(m.messages, fmt.Sprintf("Indexed %d files", stats.TotalFiles))
				} else if userInput == "/stats" {
					m.messages = append(m.messages, fmt.Sprintf("> %s", userInput))
					m.messages = append(m.messages, m.getIndexStatsMessage())
				} else {
					// Send to LLM
					m.messages = append(m.messages, fmt.Sprintf("> %s", userInput))

					if m.llmAdapter != nil && m.llmAdapter.IsAvailable() {
						return m, m.sendToLLM(userInput)
					} else {
						errorMsg := "LLM not available. Please configure your model and API key."
						if m.llmError != nil {
							errorMsg = fmt.Sprintf("LLM error: %v", m.llmError)
						}
						m.messages = append(m.messages, fmt.Sprintf("Loom: %s", errorMsg))
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
					// Log error but continue
					fmt.Printf("Warning: failed to save assistant message: %v\n", err)
				}
				m.messages = append(m.messages, fmt.Sprintf("Loom: %s", m.streamingContent))
				m.streamingContent = ""
			}
			return m, nil
		}

		m.streamingContent += msg.Content
		return m, m.waitForStream()

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

	info := infoStyle.Render(fmt.Sprintf(
		"Workspace: %s\nModel: %s\nShell: %t\nFiles: %d %s",
		m.workspacePath,
		modelStatus,
		m.config.EnableShell,
		stats.TotalFiles,
		langSummary,
	))

	var mainContent string

	if m.currentView == viewChat {
		// Input area
		inputPrefix := "Input: "
		if m.isStreaming {
			inputPrefix = "Input (streaming...): "
		}
		input := inputStyle.Render(fmt.Sprintf("%s%s", inputPrefix, m.input))

		// Messages area - include streaming content
		allMessages := make([]string, len(m.messages))
		copy(allMessages, m.messages)

		if m.isStreaming && m.streamingContent != "" {
			allMessages = append(allMessages, fmt.Sprintf("Loom: %s", m.streamingContent))
		}

		messageText := strings.Join(allMessages, "\n")
		if messageText == "" && !m.isStreaming {
			messageText = "Welcome to Loom!\nYou can now chat with an AI assistant about your project.\nTry asking about your code, architecture, or programming questions.\n\nSpecial commands:\n/files - Show file count\n/stats - Show detailed index statistics\n/quit - Exit the application\n\nPress Tab to view file tree.\nPress Ctrl+C to exit."
		}
		messages := messageStyle.Render(messageText)

		mainContent = lipgloss.JoinVertical(lipgloss.Left, input, "", messages)
	} else {
		// File tree view
		treeContent := m.renderFileTree()
		fileTree := fileTreeStyle.Render(treeContent)
		mainContent = fileTree
	}

	// Help
	var helpText string
	if m.currentView == viewChat {
		if m.isStreaming {
			helpText = "Press Tab for file tree • Ctrl+C to quit • Streaming response..."
		} else {
			helpText = "Press Tab for file tree • Ctrl+C or '/quit' to quit • Enter to send message"
		}
	} else {
		helpText = "Press Tab for chat • ↑↓ to scroll • Ctrl+C to quit"
	}

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(helpText)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		info,
		"",
		mainContent,
		"",
		help,
	)
}

func (m model) getLanguageSummary(stats indexer.IndexStats) string {
	if len(stats.LanguagePercent) == 0 {
		return ""
	}

	// Get top 3 languages
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
		if i >= 3 {
			break
		}
		summary = append(summary, fmt.Sprintf("%s %.0f%%", lang.name, lang.percent))
	}

	if len(summary) > 0 {
		return "(" + strings.Join(summary, ", ") + ")"
	}

	return ""
}

func (m model) getIndexStatsMessage() string {
	stats := m.index.GetStats()

	var lines []string
	lines = append(lines, fmt.Sprintf("📊 Index Statistics"))
	lines = append(lines, fmt.Sprintf("Total files: %d", stats.TotalFiles))
	lines = append(lines, fmt.Sprintf("Total size: %.2f MB", float64(stats.TotalSize)/1024/1024))
	lines = append(lines, fmt.Sprintf("Last updated: %s", m.index.LastUpdated.Format("15:04:05")))

	if len(stats.LanguageBreakdown) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Language breakdown:")

		// Sort languages by count
		type langPair struct {
			name  string
			count int
		}

		var langs []langPair
		for name, count := range stats.LanguageBreakdown {
			if count > 0 {
				langs = append(langs, langPair{name, count})
			}
		}

		sort.Slice(langs, func(i, j int) bool {
			return langs[i].count > langs[j].count
		})

		for _, lang := range langs {
			percent := stats.LanguagePercent[lang.name]
			lines = append(lines, fmt.Sprintf("  %s: %d files (%.1f%%)",
				lang.name, lang.count, percent))
		}
	}

	return strings.Join(lines, "\n")
}

func (m model) renderFileTree() string {
	files := m.index.GetFileList()

	if len(files) == 0 {
		return "No files indexed yet."
	}

	sort.Strings(files)

	var lines []string
	lines = append(lines, "📁 Indexed Files:")
	lines = append(lines, "")

	// Show files with pagination
	start := m.fileTreeScroll
	end := start + 10
	if end > len(files) {
		end = len(files)
	}

	for i := start; i < end; i++ {
		file := files[i]
		meta := m.index.Files[file]

		// Format file size
		var sizeStr string
		if meta.Size < 1024 {
			sizeStr = fmt.Sprintf("%dB", meta.Size)
		} else if meta.Size < 1024*1024 {
			sizeStr = fmt.Sprintf("%.1fKB", float64(meta.Size)/1024)
		} else {
			sizeStr = fmt.Sprintf("%.1fMB", float64(meta.Size)/1024/1024)
		}

		lines = append(lines, fmt.Sprintf("  %s (%s, %s)",
			file, sizeStr, meta.Language))
	}

	if len(files) > 10 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Showing %d-%d of %d files",
			start+1, end, len(files)))
	}

	return strings.Join(lines, "\n")
}

// sendToLLM sends a message to the LLM and returns a command to start streaming
func (m *model) sendToLLM(userInput string) tea.Cmd {
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

// waitForStream waits for the next streaming chunk
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

// StartTUI initializes and starts the TUI interface
func StartTUI(workspacePath string, cfg *config.Config, idx *indexer.Index) error {
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

	// Initialize chat session
	chatSession, err := chat.LoadLatestSession(workspacePath, 50)
	if err != nil {
		return fmt.Errorf("failed to initialize chat session: %w", err)
	}

	// Add system prompt if this is a new session (no previous messages)
	if len(chatSession.GetMessages()) == 0 {
		systemPrompt := chatSession.CreateSystemPrompt(idx)
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
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

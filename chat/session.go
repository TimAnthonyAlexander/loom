package chat

import (
	"bufio"
	"encoding/json"
	"fmt"
	"loom/indexer"
	"loom/llm"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TaskAuditEntry represents a task execution in the audit trail
type TaskAuditEntry struct {
	TaskID      string    `json:"task_id"`
	TaskType    string    `json:"task_type"`
	Description string    `json:"description"`
	Success     bool      `json:"success"`
	Output      string    `json:"output,omitempty"`
	Error       string    `json:"error,omitempty"`
	Approved    bool      `json:"approved,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// Session represents a chat session with message history
type Session struct {
	workspacePath string
	messages      []llm.Message
	maxMessages   int
	historyFile   string
	taskAudit     []TaskAuditEntry // Task execution audit trail
}

// NewSession creates a new chat session
func NewSession(workspacePath string, maxMessages int) *Session {
	if maxMessages <= 0 {
		maxMessages = 50 // Default to keeping 50 messages
	}

	// Create history filename with current timestamp
	timestamp := time.Now().Format("2006-01-02-1504")
	historyFile := filepath.Join(workspacePath, ".loom", "history", fmt.Sprintf("%s.jsonl", timestamp))

	return &Session{
		workspacePath: workspacePath,
		messages:      make([]llm.Message, 0),
		maxMessages:   maxMessages,
		historyFile:   historyFile,
		taskAudit:     make([]TaskAuditEntry, 0),
	}
}

// LoadLatestSession loads the most recent chat session or creates a new one
func LoadLatestSession(workspacePath string, maxMessages int) (*Session, error) {
	historyDir := filepath.Join(workspacePath, ".loom", "history")

	// Ensure history directory exists
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}

	// Find the latest history file
	files, err := os.ReadDir(historyDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}

	var latestFile string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".jsonl") {
			if latestFile == "" || file.Name() > latestFile {
				latestFile = file.Name()
			}
		}
	}

	session := &Session{
		workspacePath: workspacePath,
		messages:      make([]llm.Message, 0),
		maxMessages:   maxMessages,
		taskAudit:     make([]TaskAuditEntry, 0),
	}

	if latestFile != "" {
		// Load from existing file
		session.historyFile = filepath.Join(historyDir, latestFile)
		if err := session.loadFromFile(); err != nil {
			// If loading fails, create a new session
			fmt.Printf("Warning: failed to load history from %s: %v\n", latestFile, err)
			session = NewSession(workspacePath, maxMessages)
		}
	} else {
		// No history exists, create new session
		session = NewSession(workspacePath, maxMessages)
	}

	return session, nil
}

// LoadSessionByID loads a specific session by ID (timestamp format)
func LoadSessionByID(workspacePath string, sessionID string, maxMessages int) (*Session, error) {
	historyDir := filepath.Join(workspacePath, ".loom", "history")

	// Ensure history directory exists
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}

	// Construct the expected filename
	historyFile := filepath.Join(historyDir, fmt.Sprintf("%s.jsonl", sessionID))

	// Check if the file exists
	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	session := &Session{
		workspacePath: workspacePath,
		messages:      make([]llm.Message, 0),
		maxMessages:   maxMessages,
		historyFile:   historyFile,
		taskAudit:     make([]TaskAuditEntry, 0),
	}

	if err := session.loadFromFile(); err != nil {
		return nil, fmt.Errorf("failed to load session %s: %w", sessionID, err)
	}

	return session, nil
}

// ListAvailableSessions returns a list of available session IDs
func ListAvailableSessions(workspacePath string) ([]string, error) {
	historyDir := filepath.Join(workspacePath, ".loom", "history")

	// Ensure history directory exists
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}

	// Read all files in the history directory
	files, err := os.ReadDir(historyDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}

	var sessions []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".jsonl") {
			// Remove the .jsonl extension to get the session ID
			sessionID := strings.TrimSuffix(file.Name(), ".jsonl")
			sessions = append(sessions, sessionID)
		}
	}

	// Sort sessions (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i] > sessions[j]
	})

	return sessions, nil
}

// AddMessage adds a message to the session
func (s *Session) AddMessage(message llm.Message) error {
	s.messages = append(s.messages, message)

	// Trim messages if we exceed the limit
	if len(s.messages) > s.maxMessages {
		// Keep the system message (first message) and trim from the beginning of user/assistant messages
		systemMessages := []llm.Message{}
		userAssistantMessages := []llm.Message{}

		for _, msg := range s.messages {
			if msg.Role == "system" {
				systemMessages = append(systemMessages, msg)
			} else {
				userAssistantMessages = append(userAssistantMessages, msg)
			}
		}

		// Trim user/assistant messages to maxMessages - len(systemMessages)
		maxUserAssistant := s.maxMessages - len(systemMessages)
		if len(userAssistantMessages) > maxUserAssistant {
			userAssistantMessages = userAssistantMessages[len(userAssistantMessages)-maxUserAssistant:]
		}

		s.messages = append(systemMessages, userAssistantMessages...)
	}

	// Save to file
	return s.saveToFile(message)
}

// AddTaskAuditEntry adds a task execution entry to the audit trail
func (s *Session) AddTaskAuditEntry(entry TaskAuditEntry) error {
	s.taskAudit = append(s.taskAudit, entry)

	// Also save as a special message for persistence
	auditMessage := llm.Message{
		Role:      "system",
		Content:   fmt.Sprintf("TASK_AUDIT: %s", s.formatTaskAuditEntry(entry)),
		Timestamp: entry.Timestamp,
	}

	return s.saveToFile(auditMessage)
}

// formatTaskAuditEntry formats a task audit entry for storage
func (s *Session) formatTaskAuditEntry(entry TaskAuditEntry) string {
	data, _ := json.Marshal(entry)
	return string(data)
}

// GetTaskAuditTrail returns the task execution audit trail
func (s *Session) GetTaskAuditTrail() []TaskAuditEntry {
	return s.taskAudit
}

// GetMessages returns all messages in the session
func (s *Session) GetMessages() []llm.Message {
	return s.messages
}

// GetDisplayMessages returns messages formatted for display (excluding system messages)
func (s *Session) GetDisplayMessages() []string {
	var display []string
	for _, msg := range s.messages {
		if msg.Role != "system" {
			role := "You"
			if msg.Role == "assistant" {
				role = "Loom"
			}

			// Skip task audit messages in display
			if strings.HasPrefix(msg.Content, "TASK_AUDIT:") {
				continue
			}

			// Filter task result messages to show only status, not actual content
			content := s.filterTaskResultForDisplay(msg.Content)

			display = append(display, fmt.Sprintf("%s: %s", role, content))
		}
	}
	return display
}

// filterTaskResultForDisplay filters task result messages to show only status messages to users
func (s *Session) filterTaskResultForDisplay(content string) string {
	// Check if this is a task result message
	if !strings.HasPrefix(content, "🔧 Task Result:") {
		return content // Not a task result, return as is
	}

	lines := strings.Split(content, "\n")
	var filteredLines []string

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Always keep these status lines
		if strings.HasPrefix(line, "🔧 Task Result:") ||
			strings.HasPrefix(line, "✅ Status:") ||
			strings.HasPrefix(line, "❌ Status:") ||
			strings.HasPrefix(line, "💥 Error:") ||
			strings.HasPrefix(line, "👍 User approved") {
			filteredLines = append(filteredLines, line)
		} else if strings.HasPrefix(line, "📄 Output:") {
			// Check if the next line contains a user-friendly status message
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				if strings.HasPrefix(nextLine, "Reading file:") ||
					strings.HasPrefix(nextLine, "Reading folder structure:") ||
					strings.HasPrefix(nextLine, "Editing file:") ||
					strings.HasPrefix(nextLine, "File read successfully") ||
					strings.HasPrefix(nextLine, "Directory listing completed") ||
					strings.HasPrefix(nextLine, "File edit prepared") {
					// This is a user-friendly status, keep it
					filteredLines = append(filteredLines, line)
					filteredLines = append(filteredLines, nextLine)
					i++ // Skip the next line as we already processed it
				} else {
					// This contains actual content, replace with generic message
					filteredLines = append(filteredLines, line)

					// Determine what type of task this was based on the task description
					taskDescription := ""
					for _, prevLine := range filteredLines {
						if strings.HasPrefix(prevLine, "🔧 Task Result:") {
							taskDescription = prevLine
							break
						}
					}

					if strings.Contains(taskDescription, "Read ") {
						filteredLines = append(filteredLines, "File content read successfully")
					} else if strings.Contains(taskDescription, "List directory") {
						filteredLines = append(filteredLines, "Directory structure read successfully")
					} else if strings.Contains(taskDescription, "Edit ") {
						filteredLines = append(filteredLines, "File changes prepared successfully")
					} else {
						filteredLines = append(filteredLines, "Task completed successfully")
					}
					// Skip all remaining lines as they contain actual content
					break
				}
			}
		}
		// Skip other lines that might contain actual content
	}

	return strings.Join(filteredLines, "\n")
}

// CreateSystemPrompt creates a system prompt with project information
func (s *Session) CreateSystemPrompt(index *indexer.Index) llm.Message {
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

	prompt := fmt.Sprintf(`You are Loom, an AI coding assistant. You have read-only access to the workspace file index and can answer programming questions and discuss project structure.

Current workspace summary:
- Total files: %d
- Total size: %.2f MB
- Last updated: %s
- Primary languages: %s

You can help with:
- Explaining code structure and architecture
- Discussing programming concepts and best practices
- Answering questions about the project
- Providing coding advice and suggestions

Note: In this milestone, you have read-only access to the file index. You cannot modify files or execute tasks yet.`,
		stats.TotalFiles,
		float64(stats.TotalSize)/1024/1024,
		index.LastUpdated.Format("15:04:05"),
		strings.Join(langBreakdown, ", "))

	return llm.Message{
		Role:      "system",
		Content:   prompt,
		Timestamp: time.Now(),
	}
}

// loadFromFile loads messages from the history file
func (s *Session) loadFromFile() error {
	file, err := os.Open(s.historyFile)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var message llm.Message
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			continue // Skip invalid lines
		}

		s.messages = append(s.messages, message)

		// Extract task audit entries from system messages
		if message.Role == "system" && strings.HasPrefix(message.Content, "TASK_AUDIT:") {
			auditData := strings.TrimPrefix(message.Content, "TASK_AUDIT: ")
			var entry TaskAuditEntry
			if err := json.Unmarshal([]byte(auditData), &entry); err == nil {
				s.taskAudit = append(s.taskAudit, entry)
			}
		}
	}

	return scanner.Err()
}

// saveToFile appends a message to the history file
func (s *Session) saveToFile(message llm.Message) error {
	// Ensure history directory exists
	historyDir := filepath.Dir(s.historyFile)
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// Open file for appending
	file, err := os.OpenFile(s.historyFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open history file: %w", err)
	}
	defer file.Close()

	// Marshal and write the message
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	_, err = file.Write(append(data, '\n'))
	return err
}

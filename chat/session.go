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

// Session represents a chat session with message history
type Session struct {
	workspacePath string
	messages      []llm.Message
	maxMessages   int
	historyFile   string
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
			display = append(display, fmt.Sprintf("%s: %s", role, msg.Content))
		}
	}
	return display
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

package chat

import (
	"bufio"
	"encoding/json"
	"fmt"
	"loom/indexer"
	"loom/llm"
	"loom/paths"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Constants for text truncation limits
const (
	MaxActionDescriptionLength = 40 // Maximum length for action descriptions in display
	MaxCommandLength           = 50 // Maximum length for commands in display
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
	projectPaths  *paths.ProjectPaths
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

	// Get project paths
	projectPaths, err := paths.NewProjectPaths(workspacePath)
	if err != nil {
		// Fallback to old behavior if paths creation fails
		timestamp := time.Now().Format("2006-01-02-1504")
		historyFile := filepath.Join(workspacePath, ".loom", "history", fmt.Sprintf("%s.jsonl", timestamp))

		return &Session{
			workspacePath: workspacePath,
			projectPaths:  nil,
			messages:      make([]llm.Message, 0),
			maxMessages:   maxMessages,
			historyFile:   historyFile,
			taskAudit:     make([]TaskAuditEntry, 0),
		}
	}

	// Ensure project directories exist
	if err := projectPaths.EnsureProjectDir(); err != nil {
		fmt.Printf("Warning: failed to create project directories: %v\n", err)
	}

	// Create history filename with current timestamp
	timestamp := time.Now().Format("2006-01-02-1504")
	historyFile := filepath.Join(projectPaths.HistoryDir(), fmt.Sprintf("%s.jsonl", timestamp))

	return &Session{
		workspacePath: workspacePath,
		projectPaths:  projectPaths,
		messages:      make([]llm.Message, 0),
		maxMessages:   maxMessages,
		historyFile:   historyFile,
		taskAudit:     make([]TaskAuditEntry, 0),
	}
}

// LoadLatestSession loads the most recent chat session or creates a new one
func LoadLatestSession(workspacePath string, maxMessages int) (*Session, error) {
	// Get project paths
	projectPaths, err := paths.NewProjectPaths(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create project paths: %w", err)
	}

	// Ensure project directories exist
	if err := projectPaths.EnsureProjectDir(); err != nil {
		return nil, fmt.Errorf("failed to create project directories: %w", err)
	}

	historyDir := projectPaths.HistoryDir()

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
		projectPaths:  projectPaths,
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
	// Get project paths
	projectPaths, err := paths.NewProjectPaths(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create project paths: %w", err)
	}

	// Ensure project directories exist
	if err := projectPaths.EnsureProjectDir(); err != nil {
		return nil, fmt.Errorf("failed to create project directories: %w", err)
	}

	historyDir := projectPaths.HistoryDir()

	// Construct the expected filename
	historyFile := filepath.Join(historyDir, fmt.Sprintf("%s.jsonl", sessionID))

	// Check if the file exists
	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	session := &Session{
		workspacePath: workspacePath,
		projectPaths:  projectPaths,
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
	// Get project paths
	projectPaths, err := paths.NewProjectPaths(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create project paths: %w", err)
	}

	// Ensure project directories exist
	if err := projectPaths.EnsureProjectDir(); err != nil {
		return nil, fmt.Errorf("failed to create project directories: %w", err)
	}

	historyDir := projectPaths.HistoryDir()

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

			// CRITICAL FIX: Skip internal TASK_RESULT messages that are meant for LLM only
			if strings.HasPrefix(msg.Content, "TASK_RESULT:") {
				continue
			}

			var content string
			if msg.Role == "assistant" {
				// Only filter assistant messages, not user messages
				content = s.filterTaskResultForDisplay(msg.Content)
			} else {
				// User messages should be displayed as-is without any task filtering
				content = msg.Content
			}

			// Skip empty content (like completion detector interactions)
			if strings.TrimSpace(content) == "" {
				continue
			}

			display = append(display, fmt.Sprintf("%s: %s", role, content))
		}
	}
	return display
}

// filterTaskResultForDisplay filters task result messages to show only status messages to users
func (s *Session) filterTaskResultForDisplay(content string) string {
	// First check if this contains JSON task blocks - filter those out
	content = s.filterJSONTaskBlocks(content)

	// Check if this is a completion detector interaction - hide those entirely
	if s.isCompletionDetectorInteraction(content) {
		return "" // Return empty to hide these interactions
	}

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

// filterJSONTaskBlocks removes JSON task blocks from LLM responses and replaces with clean descriptions
func (s *Session) filterJSONTaskBlocks(content string) string {
	// First check for natural language tasks (much simpler)
	lines := strings.Split(content, "\n")
	var filteredLines []string
	var taskDescriptions []string

	taskPattern := regexp.MustCompile(`^🔧\s+(READ|EDIT|LIST|RUN)\s+(.+)`)
	// Remove the overly broad simplePattern that incorrectly matches natural language
	// Only match the explicit task format with 🔧 prefix

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip LOOM_EDIT commands - they should be shown as-is, not filtered
		if strings.Contains(trimmed, "LOOM_EDIT") {
			filteredLines = append(filteredLines, line)
			continue
		}

		// Check for natural language tasks (only the explicit 🔧 format)
		if matches := taskPattern.FindStringSubmatch(trimmed); len(matches) == 3 {
			taskType := strings.ToUpper(matches[1])
			taskArgs := strings.TrimSpace(matches[2])
			desc := formatNaturalLanguageTaskDescription(taskType, taskArgs)
			taskDescriptions = append(taskDescriptions, desc)
			continue // Skip this line in output
		}

		filteredLines = append(filteredLines, line)
	}

	// If we found natural language tasks, create summary
	if len(taskDescriptions) > 0 {
		taskSummary := ""
		if len(taskDescriptions) == 1 {
			taskSummary = taskDescriptions[0]
		} else {
			taskSummary = fmt.Sprintf("🔧 Executing %d tasks:\n%s", len(taskDescriptions), strings.Join(taskDescriptions, "\n"))
		}

		// Add task summary to filtered content
		filteredContent := strings.Join(filteredLines, "\n")
		if strings.TrimSpace(filteredContent) == "" {
			return taskSummary
		}
		return filteredContent + "\n\n" + taskSummary
	}

	// Fall back to JSON block filtering for backward compatibility
	content = strings.Join(filteredLines, "\n")

	// Find JSON code blocks using regex
	re := regexp.MustCompile("(?s)```(?:json)?\n?(.*?)\n?```")
	matches := re.FindAllStringSubmatch(content, -1)

	if len(matches) == 0 {
		return content // No JSON blocks found
	}

	// Try to extract task information from JSON blocks
	var jsonTaskDescriptions []string

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		jsonStr := strings.TrimSpace(match[1])
		if jsonStr == "" {
			continue
		}

		// Parse the JSON to extract task information
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			continue // Skip invalid JSON
		}

		// Look for tasks array
		tasksData, ok := data["tasks"]
		if !ok {
			continue
		}

		tasks, ok := tasksData.([]interface{})
		if !ok {
			continue
		}

		// Extract task descriptions
		for _, taskInterface := range tasks {
			taskMap, ok := taskInterface.(map[string]interface{})
			if !ok {
				continue
			}

			taskType, hasType := taskMap["type"].(string)
			path, hasPath := taskMap["path"].(string)
			command, hasCommand := taskMap["command"].(string)

			if !hasType {
				continue
			}

			// Generate clean description based on task type
			switch taskType {
			case "ReadFile":
				if hasPath && path != "" {
					jsonTaskDescriptions = append(jsonTaskDescriptions, "📖 Reading file: "+path)
				} else {
					jsonTaskDescriptions = append(jsonTaskDescriptions, "📖 Reading file")
				}
			case "EditFile":
				if hasPath && path != "" {
					jsonTaskDescriptions = append(jsonTaskDescriptions, "✏️ Editing file: "+path)
				} else {
					jsonTaskDescriptions = append(jsonTaskDescriptions, "✏️ Editing file")
				}
			case "ListDir":
				if hasPath && path != "" && path != "." {
					jsonTaskDescriptions = append(jsonTaskDescriptions, "📁 Listing directory: "+path)
				} else {
					jsonTaskDescriptions = append(jsonTaskDescriptions, "📁 Listing current directory")
				}
			case "RunShell":
				if hasCommand && command != "" {
					jsonTaskDescriptions = append(jsonTaskDescriptions, "⚡ Running command: "+command)
				} else {
					jsonTaskDescriptions = append(jsonTaskDescriptions, "⚡ Running shell command")
				}
			default:
				jsonTaskDescriptions = append(jsonTaskDescriptions, "🔧 Executing task: "+taskType)
			}
		}
	}

	// If we found tasks, create a clean summary
	if len(jsonTaskDescriptions) > 0 {
		taskSummary := ""
		if len(jsonTaskDescriptions) == 1 {
			taskSummary = jsonTaskDescriptions[0]
		} else {
			taskSummary = fmt.Sprintf("🔧 Executing %d tasks:\n%s", len(jsonTaskDescriptions), strings.Join(jsonTaskDescriptions, "\n"))
		}

		// Replace all JSON blocks with the clean task summary
		filteredContent := re.ReplaceAllString(content, "\n"+taskSummary)
		return filteredContent
	}

	// If no valid tasks found, just remove the JSON blocks
	return re.ReplaceAllString(content, "\n🔧 Executing tasks...")
}

// formatNaturalLanguageTaskDescription creates clean descriptions for natural language tasks
func formatNaturalLanguageTaskDescription(taskType, args string) string {
	switch taskType {
	case "READ":
		// Extract just the filename for cleaner display
		parts := strings.Fields(args)
		if len(parts) > 0 {
			filename := parts[0]
			// Remove path prefixes for cleaner display
			if idx := strings.LastIndex(filename, "/"); idx != -1 {
				filename = filename[idx+1:]
			}
			return "📖 Reading file: " + filename
		}
		return "📖 Reading file"

	case "EDIT":
		// Check for arrow notation
		if strings.Contains(args, "→") || strings.Contains(args, "->") {
			var parts []string
			if strings.Contains(args, "→") {
				parts = strings.Split(args, "→")
			} else {
				parts = strings.Split(args, "->")
			}
			if len(parts) >= 2 {
				filename := strings.TrimSpace(parts[0])
				if idx := strings.LastIndex(filename, "/"); idx != -1 {
					filename = filename[idx+1:]
				}
				action := strings.TrimSpace(parts[1])
				// Truncate long descriptions for display
				if len(action) > MaxActionDescriptionLength {
					action = action[:MaxActionDescriptionLength-3] + "..."
				}
				return fmt.Sprintf("✏️ Editing %s -> %s", filename, action)
			}
		}

		// Simple filename
		parts := strings.Fields(args)
		if len(parts) > 0 {
			filename := parts[0]
			if idx := strings.LastIndex(filename, "/"); idx != -1 {
				filename = filename[idx+1:]
			}
			return "✏️ Editing file: " + filename
		}
		return "✏️ Editing file"

	case "LIST":
		dirName := strings.Fields(args)[0]
		if dirName == "." {
			dirName = "current directory"
		} else if idx := strings.LastIndex(dirName, "/"); idx != -1 {
			dirName = dirName[idx+1:] + "/"
		}
		if strings.Contains(strings.ToLower(args), "recursive") {
			return "📁 Listing directory: " + dirName + " (recursive)"
		}
		return "📁 Listing directory: " + dirName

	case "RUN":
		// Extract command, limit length for display
		command := args
		if strings.Contains(command, "(timeout:") {
			command = strings.Split(command, "(timeout:")[0]
		}
		command = strings.TrimSpace(command)
		if len(command) > MaxCommandLength {
			command = command[:MaxCommandLength-3] + "..."
		}
		return "⚡ Running command: " + command

	default:
		return "🔧 Executing task: " + taskType
	}
}

// isCompletionDetectorInteraction checks if content is from completion detector
func (s *Session) isCompletionDetectorInteraction(content string) bool {
	// Don't hide debug messages - they should always be shown
	if strings.Contains(content, "🔍 **DEBUG**:") {
		return false
	}

	// Don't hide LOOM_EDIT commands - they should always be shown even if they contain completion patterns
	if (strings.Contains(content, ">>LOOM_EDIT") || strings.Contains(content, "🔧 LOOM_EDIT")) && strings.Contains(content, "<<LOOM_EDIT") {
		return false
	}

	// Check for explicit completion check prefix
	if strings.HasPrefix(content, "COMPLETION_CHECK:") {
		return true
	}

	lowerContent := strings.ToLower(content)

	// Enhanced completion detector question patterns
	completionQuestions := []string{
		"is this task complete?",
		"are you finished with this work?",
		"is there anything else you need to do?",
		"have you completed everything that was requested?",
		"is this implementation finished?",
		"are you done, or is there more work to do?",
		"is the task fully complete?",
		"do you need to do anything else?",
		"has your stated objective been fully achieved?",
		"is your current work complete?",
		"please answer clearly:",
	}

	for _, question := range completionQuestions {
		if strings.Contains(lowerContent, question) {
			return true
		}
	}

	// Enhanced completion detector response patterns
	completionResponses := []string{
		"yes, the task is complete",
		"yes, i'm finished",
		"yes, everything is done",
		"no, i still need",
		"not yet, i should",
		"there's more work",
		"objective_complete:",
		"task_complete:",
		"exploration_complete:",
		"analysis_complete:",
		"yes if the objective is complete",
		"no if more work is required",
		"additional work needed:",
		"objective partially complete",
		"ready for next phase",
	}

	for _, response := range completionResponses {
		if strings.Contains(lowerContent, response) {
			return true
		}
	}

	return false
}

// CreateSystemPrompt creates an enhanced system prompt with project information and conventions
func (s *Session) CreateSystemPrompt(index *indexer.Index, enableShell bool) llm.Message {
	promptEnhancer := llm.NewPromptEnhancer(s.workspacePath, index)
	return promptEnhancer.CreateEnhancedSystemPrompt(enableShell)
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

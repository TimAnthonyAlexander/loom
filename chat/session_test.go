package chat

import (
	"loom/indexer"
	"loom/llm"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-chat-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a .loom directory for history
	loomDir := filepath.Join(tempDir, ".loom", "history")
	if err := os.MkdirAll(loomDir, 0755); err != nil {
		t.Fatalf("Failed to create .loom directory: %v", err)
	}

	// Create a new session
	session := NewSession(tempDir, 50)
	if session == nil {
		t.Fatalf("Expected non-nil session")
	}

	if session.workspacePath != tempDir {
		t.Errorf("Expected workspace path %s, got %s", tempDir, session.workspacePath)
	}

	if session.maxMessages != 50 {
		t.Errorf("Expected max messages 50, got %d", session.maxMessages)
	}

	if len(session.messages) != 0 {
		t.Errorf("Expected empty messages, got %d", len(session.messages))
	}

	if len(session.taskAudit) != 0 {
		t.Errorf("Expected empty task audit, got %d", len(session.taskAudit))
	}

	// Check that history file contains 'history' in its path
	if !strings.Contains(filepath.ToSlash(session.historyFile), "/history/") {
		t.Errorf("Expected history file path to contain 'history' directory: %s", session.historyFile)
	}

	// Check that history filename matches timestamp format YYYY-MM-DD-HHMM.jsonl
	historyFileName := filepath.Base(session.historyFile)
	filenamePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-\d{4}\.jsonl$`)
	if !filenamePattern.MatchString(historyFileName) {
		t.Errorf("Expected history filename to match pattern YYYY-MM-DD-HHMM.jsonl, got %s", historyFileName)
	}
}

func TestAddMessage(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-chat-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a .loom directory for history
	loomDir := filepath.Join(tempDir, ".loom", "history")
	if err := os.MkdirAll(loomDir, 0755); err != nil {
		t.Fatalf("Failed to create .loom directory: %v", err)
	}

	// Create a new session
	session := NewSession(tempDir, 10)

	// Add some messages
	for i := 0; i < 5; i++ {
		message := llm.Message{
			Role:      "user",
			Content:   "Test message",
			Timestamp: time.Now(),
		}
		err := session.AddMessage(message)
		if err != nil {
			t.Fatalf("Failed to add message: %v", err)
		}
	}

	// Check that messages were added
	messages := session.GetMessages()
	if len(messages) != 5 {
		t.Errorf("Expected 5 messages, got %d", len(messages))
	}

	// Add a system message
	systemMessage := llm.Message{
		Role:      "system",
		Content:   "System message",
		Timestamp: time.Now(),
	}
	err = session.AddMessage(systemMessage)
	if err != nil {
		t.Fatalf("Failed to add system message: %v", err)
	}

	// Check that system message is included in messages
	messages = session.GetMessages()
	if len(messages) != 6 {
		t.Errorf("Expected 6 messages, got %d", len(messages))
	}

	// But system message should be excluded from display messages (the first system message is allowed)
	displayMessages := session.GetDisplayMessages()
	if len(displayMessages) != 6 {
		t.Errorf("Expected 6 display messages, got %d", len(displayMessages))
	}

	// Add messages to exceed maxMessages
	for i := 0; i < 5; i++ {
		message := llm.Message{
			Role:      "assistant",
			Content:   "Response message",
			Timestamp: time.Now(),
		}
		err := session.AddMessage(message)
		if err != nil {
			t.Fatalf("Failed to add message: %v", err)
		}
	}

	// Check that messages were trimmed to maxMessages
	messages = session.GetMessages()
	if len(messages) != 10 {
		t.Errorf("Expected 10 messages after trimming, got %d", len(messages))
	}
}

func TestTaskAudit(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-chat-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a .loom directory for history
	loomDir := filepath.Join(tempDir, ".loom", "history")
	if err := os.MkdirAll(loomDir, 0755); err != nil {
		t.Fatalf("Failed to create .loom directory: %v", err)
	}

	// Create a new session
	session := NewSession(tempDir, 50)

	// Add task audit entry
	taskEntry := TaskAuditEntry{
		TaskID:      "task1",
		TaskType:    "ReadFile",
		Description: "Read test file",
		Success:     true,
		Output:      "File content",
		Approved:    true,
		Timestamp:   time.Now(),
	}

	err = session.AddTaskAuditEntry(taskEntry)
	if err != nil {
		t.Fatalf("Failed to add task audit entry: %v", err)
	}

	// Check that task audit entry was added
	audit := session.GetTaskAuditTrail()
	if len(audit) != 1 {
		t.Errorf("Expected 1 task audit entry, got %d", len(audit))
	}

	if audit[0].TaskID != "task1" {
		t.Errorf("Expected task ID 'task1', got '%s'", audit[0].TaskID)
	}

	if audit[0].TaskType != "ReadFile" {
		t.Errorf("Expected task type 'ReadFile', got '%s'", audit[0].TaskType)
	}

	if audit[0].Description != "Read test file" {
		t.Errorf("Expected description 'Read test file', got '%s'", audit[0].Description)
	}

	if !audit[0].Success {
		t.Errorf("Expected success to be true")
	}

	if audit[0].Output != "File content" {
		t.Errorf("Expected output 'File content', got '%s'", audit[0].Output)
	}

	if !audit[0].Approved {
		t.Errorf("Expected approved to be true")
	}
}

func TestFilterTaskResultForDisplay(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-chat-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a new session
	session := NewSession(tempDir, 50)

	// Test filtering of task result messages
	taskResult := "🔧 Task Result: Read file\n✅ Status: Success\n📄 Output: File read successfully"
	filtered := session.filterTaskResultForDisplay(taskResult)

	// Check that the filtered result still contains status information
	if filtered == "" {
		t.Errorf("Expected non-empty filtered task result")
	}

	if !strings.Contains(filtered, "🔧 Task Result:") {
		t.Errorf("Expected filtered result to contain task result header")
	}

	if !strings.Contains(filtered, "✅ Status:") {
		t.Errorf("Expected filtered result to contain status")
	}

	// Test filtering of completion detector interactions
	completionCheck := "COMPLETION_CHECK: Is this task complete?"
	filtered = session.filterTaskResultForDisplay(completionCheck)
	if filtered != "" {
		t.Errorf("Expected empty filtered result for completion check, got '%s'", filtered)
	}

	completionResponse := "Yes, the task is complete"
	filtered = session.filterTaskResultForDisplay(completionResponse)
	if filtered != "" {
		t.Errorf("Expected empty filtered result for completion response, got '%s'", filtered)
	}
}

func TestFilterJSONTaskBlocks(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom-chat-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a new session
	session := NewSession(tempDir, 50)

	// Test filtering of JSON task blocks
	jsonBlock := "I'll read the file:\n```json\n{\"tasks\": [{\"type\": \"ReadFile\", \"path\": \"main.go\"}]}\n```"
	filtered := session.filterJSONTaskBlocks(jsonBlock)

	// Check that the filtered result contains a clean description
	if filtered == "" {
		t.Errorf("Expected non-empty filtered JSON task block")
	}

	if strings.Contains(filtered, "```json") {
		t.Errorf("Expected filtered result not to contain JSON block markers")
	}

	if !strings.Contains(filtered, "Reading file") {
		t.Errorf("Expected filtered result to contain task description")
	}

	// Test filtering natural language tasks
	naturalTask := "🔧 READ main.go (max: 100 lines)"
	filtered = session.filterJSONTaskBlocks(naturalTask)
	if !strings.Contains(filtered, "📖 Reading file") {
		t.Errorf("Expected filtered result to contain reading description for natural language task")
	}
}

func TestFormatNaturalLanguageTaskDescription(t *testing.T) {
	// Test task description formatting
	readDesc := formatNaturalLanguageTaskDescription("READ", "main.go (max: 100 lines)")
	if !strings.Contains(readDesc, "📖 Reading file") {
		t.Errorf("Expected read description to contain file icon, got '%s'", readDesc)
	}

	listDesc := formatNaturalLanguageTaskDescription("LIST", "src/ recursive")
	if !strings.Contains(listDesc, "📁 Listing directory") {
		t.Errorf("Expected list description to contain directory icon, got '%s'", listDesc)
	}

	editDesc := formatNaturalLanguageTaskDescription("EDIT", "config.json")
	if !strings.Contains(editDesc, "✏️ Editing file") {
		t.Errorf("Expected edit description to contain edit icon, got '%s'", editDesc)
	}

	runDesc := formatNaturalLanguageTaskDescription("RUN", "go test")
	if !strings.Contains(runDesc, "⚡ Running command") {
		t.Errorf("Expected run description to contain command icon, got '%s'", runDesc)
	}
}

// Mock index for testing
type mockIndex struct {
	fileCount int
	languages map[string]float64
}

func (m *mockIndex) GetStats() indexer.IndexStats {
	return indexer.IndexStats{
		TotalFiles:      m.fileCount,
		TotalSize:       1024 * 1024,
		LanguagePercent: m.languages,
	}
}

// Skip TestCreateSystemPrompt for now as it would need more extensive mocking
// A real implementation would need to mock the prompt enhancer

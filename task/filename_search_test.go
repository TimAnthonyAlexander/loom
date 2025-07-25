package task

import (
	"io/ioutil"
	"loom/llm"
	"path/filepath"
	"strings"
	"testing"
)

// TestFilenameSearchBug demonstrates the bug where filename search results
// are not properly preserved in the LLM context
func TestFilenameSearchBug(t *testing.T) {
	// Create a temp directory with a sample.json file
	tempDir := t.TempDir()

	// Create sample.json file
	sampleFile := filepath.Join(tempDir, "sample.json")
	err := ioutil.WriteFile(sampleFile, []byte(`{"name": "test"}`), 0644)
	if err != nil {
		t.Fatalf("Failed to create sample file: %v", err)
	}

	// Create executor with the temp dir as workspace
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Create a mock chat session to capture messages
	mockChat := &FilenameSearchMockChat{}

	// Create sequential manager
	manager := NewSequentialTaskManager(executor, nil, mockChat)

	// 1. APPROACH 1: Directly create a search task rather than parsing it
	task := &Task{
		Type:        TaskTypeSearch,
		Path:        ".",
		Query:       "sample.json",
		SearchNames: true,
	}

	// 2. Execute the search task
	taskResponse := executor.Execute(task)

	// Verify task was successful
	if !taskResponse.Success {
		t.Errorf("Search task failed: %s", taskResponse.Error)
	}

	// Verify task response contains FOUND FILES MATCHING NAME
	if !strings.Contains(taskResponse.ActualContent, "FOUND FILES MATCHING NAME") {
		t.Errorf("Expected task response to contain 'FOUND FILES MATCHING NAME', got: %s",
			truncateString(taskResponse.ActualContent, 200))
	}

	// 3. Format the task result as the sequential manager would
	taskResultMsg := manager.formatTaskResultForExploration(task, taskResponse)

	// Verify the message role is "system"
	if taskResultMsg.Role != "system" {
		t.Errorf("Expected message role to be 'system', got '%s'", taskResultMsg.Role)
	}

	// Verify message content contains FOUND FILES MATCHING NAME
	if !strings.Contains(taskResultMsg.Content, "FOUND FILES MATCHING NAME") {
		t.Errorf("Expected message content to contain 'FOUND FILES MATCHING NAME', got: %s",
			truncateString(taskResultMsg.Content, 200))
	}

	// 4. Add the message to exploration context and chat session
	manager.addToExplorationContext(taskResultMsg)

	// THIS IS WHERE THE BUG HAPPENS:
	// In a real situation, the message with filename results should be added
	// to the chat session, but it sometimes isn't
	if err := mockChat.AddMessage(taskResultMsg); err != nil {
		t.Fatalf("Failed to add message to chat: %v", err)
	}

	// 5. Check that the message is in the exploration context
	explorationContext := manager.GetExplorationContext()

	foundInContext := false
	for _, msg := range explorationContext {
		if strings.Contains(msg.Content, "TASK_RESULT:") &&
			strings.Contains(msg.Content, "Search for") &&
			strings.Contains(msg.Content, "sample.json") {

			if strings.Contains(msg.Content, "FOUND FILES MATCHING NAME") {
				foundInContext = true
				break
			}
		}
	}

	if !foundInContext {
		t.Error("Bug confirmed: Filename search results not properly marked in exploration context")
	}

	// 6. Check that the message was added to the chat session
	foundInChat := false
	for _, msg := range mockChat.messages {
		if strings.Contains(msg.Content, "TASK_RESULT:") &&
			strings.Contains(msg.Content, "Search for") &&
			strings.Contains(msg.Content, "sample.json") {

			if strings.Contains(msg.Content, "FOUND FILES MATCHING NAME") {
				foundInChat = true
				break
			}
		}
	}

	if !foundInChat {
		t.Error("Bug confirmed: Filename search results not properly added to chat session")
	}
}

// FilenameSearchMockChat implements ChatSession for testing
type FilenameSearchMockChat struct {
	messages []llm.Message
}

func (m *FilenameSearchMockChat) AddMessage(message llm.Message) error {
	m.messages = append(m.messages, message)
	return nil
}

func (m *FilenameSearchMockChat) GetMessages() []llm.Message {
	return m.messages
}

// Helper function to truncate strings for error messages
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

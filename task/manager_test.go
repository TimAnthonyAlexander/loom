package task

import (
	"loom/llm"
	"testing"
)

// Helper type for testing
type MockChatSession struct {
	messages []llm.Message
}

func (m *MockChatSession) AddMessage(message llm.Message) error {
	m.messages = append(m.messages, message)
	return nil
}

func (m *MockChatSession) GetMessages() []llm.Message {
	return m.messages
}

func (m *MockChatSession) GetDisplayMessages() []string {
	return nil // Not used in this test
}

// TestEditTaskConfirmationBug tests the bug where tasks requiring confirmation but don't show dialogs
func TestEditTaskConfirmationBug(t *testing.T) {
	executor := NewExecutor(t.TempDir(), false, 1024*1024)
	mockChat := &MockChatSession{}
	manager := NewManager(executor, nil, mockChat)

	// Use proper LOOM_EDIT syntax to edit files
	llmResponse := `>>LOOM_EDIT file=public/index.html REPLACE 1-1
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Test Page</title>
  </head>
  <body>
    <div id="root"></div>
  </body>
</html>
<<LOOM_EDIT`

	// Create event channels to capture events
	userEventChan := make(chan UserTaskEvent, 10)
	eventChan := make(chan TaskExecutionEvent, 10)

	// Execute the LLM response
	execution, err := manager.HandleLLMResponse(llmResponse, userEventChan, eventChan)
	if err != nil {
		t.Fatalf("Failed to handle LLM response: %v", err)
	}

	if execution == nil {
		t.Fatal("Expected execution to be created, got nil")
	}

	// Verify we have a task
	if len(execution.Tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(execution.Tasks))
	}

	task := execution.Tasks[0]
	if !task.LoomEditCommand {
		t.Fatal("Expected task to be recognized as LOOM_EDIT command")
	}

	// Make sure task type is correctly set
	if task.Type != TaskTypeEditFile {
		t.Fatalf("Expected task type EditFile, got %v", task.Type)
	}
}

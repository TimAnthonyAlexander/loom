package task

import (
	"fmt"
	"loom/llm"
	"os"
	"strings"
	"testing"
)

// MockChatSession for testing
type MockChatSession struct {
	messages []llm.Message
}

func (m *MockChatSession) AddMessage(msg llm.Message) error {
	m.messages = append(m.messages, msg)
	return nil
}

func (m *MockChatSession) GetMessages() []llm.Message {
	return m.messages
}

func (m *MockChatSession) GetDisplayMessages() []string {
	var display []string
	for _, msg := range m.messages {
		display = append(display, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
	}
	return display
}

// TestEditTaskConfirmationBug tests the exact bug reported by the user
func TestEditTaskConfirmationBug(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create executor and manager
	executor := NewExecutor(tempDir, false, 1024*1024)
	mockChat := &MockChatSession{}
	manager := NewManager(executor, nil, mockChat)

	// The exact LLM response that was failing
	llmResponse := `🔧 EDIT public/index.html
` + "```" + `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Fatih Secilmis Dentist Office</title>
</head>
<body>
    <h1>Welcome to Fatih Secilmis Dentist Office</h1>
    <p>Your oral health is our priority.</p>
</body>
</html>
` + "```"

	// Create event channel to capture events
	eventChan := make(chan TaskExecutionEvent, 10)

	// Execute the LLM response
	execution, err := manager.HandleLLMResponse(llmResponse, eventChan)
	if err != nil {
		t.Fatalf("Failed to handle LLM response: %v", err)
	}

	if execution == nil {
		t.Fatal("Expected execution to be created, got nil")
	}

	if len(execution.Tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(execution.Tasks))
	}

	if len(execution.Responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(execution.Responses))
	}

	task := execution.Tasks[0]
	response := execution.Responses[0]

	// Verify the task is correctly parsed
	if task.Type != TaskTypeEditFile {
		t.Errorf("Expected TaskTypeEditFile, got %s", task.Type)
	}

	if task.Path != "public/index.html" {
		t.Errorf("Expected path 'public/index.html', got '%s'", task.Path)
	}

	// UPDATED: Tasks no longer require confirmation - they execute immediately
	if task.RequiresConfirmation() {
		t.Errorf("Task should NOT require confirmation after removing confirmation system")
	}

	// UPDATED: Response task also should not require confirmation
	if response.Task.RequiresConfirmation() {
		t.Errorf("Response task should NOT require confirmation after removing confirmation system")
	}

	// UPDATED: Response should be success AND file should be created immediately
	if !response.Success {
		t.Errorf("Expected response.Success to be true, got false. Error: %s", response.Error)
	}

	// UPDATED: File SHOULD exist immediately after success (no confirmation needed)
	filePath := tempDir + "/public/index.html"
	if !fileExists(filePath) {
		t.Errorf("File SHOULD exist immediately after success: %s", filePath)
	}

	// Capture events from the channel
	var events []TaskExecutionEvent
	close(eventChan)
	for event := range eventChan {
		events = append(events, event)
	}

	// UPDATED: Verify we got completion events but NO confirmation events
	var foundConfirmationEvent bool
	var foundCompletionEvent bool
	for _, event := range events {
		t.Logf("Event: Type=%s, RequiresInput=%t, Message=%s", event.Type, event.RequiresInput, event.Message)
		if event.Type == "task_completed" && event.RequiresInput {
			foundConfirmationEvent = true
		}
		if event.Type == "task_completed" && !event.RequiresInput {
			foundCompletionEvent = true
		}
	}

	if foundConfirmationEvent {
		t.Errorf("Found confirmation event when none should exist after removing confirmation system")
	}

	if !foundCompletionEvent {
		t.Errorf("Expected to find task_completed event with RequiresInput=false")
		t.Logf("All events:")
		for i, event := range events {
			t.Logf("  [%d] Type: %s, RequiresInput: %t", i, event.Type, event.RequiresInput)
		}
	}

	// Test the confirmation flow
	t.Run("ConfirmAndApply", func(t *testing.T) {
		// Now confirm the task
		err := manager.ConfirmTask(&response.Task, &response, true)
		if err != nil {
			t.Fatalf("Failed to confirm task: %v", err)
		}

		// NOW the file should exist
		if !fileExists(filePath) {
			t.Errorf("File should exist after confirmation, but it doesn't: %s", filePath)
		}

		// Verify file content
		content, err := readFileContent(filePath)
		if err != nil {
			t.Fatalf("Failed to read file content: %v", err)
		}

		if !strings.Contains(content, "Fatih Secilmis Dentist Office") {
			t.Errorf("File content doesn't contain expected text. Content: %s", content)
		}
	})
}

// Helper functions
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readFileContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

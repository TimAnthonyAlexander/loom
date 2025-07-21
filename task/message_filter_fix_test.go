package task

import (
	"loom/chat"
	"loom/llm"
	"strings"
	"testing"
	"time"
)

// TestMessageFilteringFix tests that internal TASK_RESULT messages are properly filtered out
func TestMessageFilteringFix(t *testing.T) {
	// Create a chat session
	session := chat.NewSession("test_session", 100)

	// Add a user message
	userMsg := llm.Message{
		Role:      "user",
		Content:   "Please edit the file",
		Timestamp: time.Now(),
	}
	session.AddMessage(userMsg)

	// Add an internal TASK_RESULT message (this should be hidden)
	internalMsg := llm.Message{
		Role:      "system",
		Content:   "TASK_RESULT: Edit public/index.html (replace content)\nSTATUS: Success\nCONTENT:\nContent replacement preview for public/index.html:\n\n{\"content\":\"<html>...</html>\"}",
		Timestamp: time.Now(),
	}
	session.AddMessage(internalMsg)

	// Add a proper assistant response
	assistantMsg := llm.Message{
		Role:      "assistant",
		Content:   "I'll help you edit the file. Please confirm the changes.",
		Timestamp: time.Now(),
	}
	session.AddMessage(assistantMsg)

	// Get display messages
	displayMessages := session.GetDisplayMessages()

	// Verify the internal message is filtered out
	foundInternalMessage := false
	for _, msg := range displayMessages {
		if strings.Contains(msg, "TASK_RESULT:") || strings.Contains(msg, "STATUS: Success") {
			foundInternalMessage = true
			t.Errorf("CRITICAL BUG: Internal TASK_RESULT message leaked to display: %s", msg)
		}
	}

	if foundInternalMessage {
		t.Errorf("Message filtering FAILED - internal messages are still leaking")
	} else {
		t.Logf("SUCCESS: Internal messages properly filtered out")
	}

	// Verify we still see the user and assistant messages
	if len(displayMessages) != 2 {
		t.Errorf("Expected 2 display messages (user + assistant), got %d", len(displayMessages))
	}

	// Verify the messages are properly formatted
	expectedMessages := []string{
		"You: Please edit the file",
		"Loom: I'll help you edit the file. Please confirm the changes.",
	}

	for i, expected := range expectedMessages {
		if i >= len(displayMessages) {
			t.Errorf("Missing expected message: %s", expected)
			continue
		}

		if displayMessages[i] != expected {
			t.Errorf("Message %d mismatch.\nExpected: %s\nGot:      %s", i, expected, displayMessages[i])
		}
	}
}

// TestRealWorldScenario tests the exact scenario the user experienced
func TestRealWorldScenario(t *testing.T) {
	tempDir := t.TempDir()

	// Create chat session like TUI would
	session := chat.NewSession("test_workspace", 100)

	// Create executor and manager
	executor := NewExecutor(tempDir, false, 1024*1024)
	manager := NewManager(executor, nil, session)

	// User's exact input
	userInput := `EDIT public/index.html
{"content":"<!DOCTYPE html>\n<html lang=\"en\">\n  <head>\n    <meta charset=\"UTF-8\" />\n    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\" />\n    <title>Fatih Secilmis Dentist Office</title>\n  </head>\n  <body>\n    <div id=\"root\"></div>\n  </body>\n</html>"}`

	// Add user message to session
	userMsg := llm.Message{
		Role:      "user",
		Content:   userInput,
		Timestamp: time.Now(),
	}
	session.AddMessage(userMsg)

	// Execute the task through manager
	eventChan := make(chan TaskExecutionEvent, 10)
	execution, err := manager.HandleLLMResponse(userInput, eventChan)
	if err != nil {
		t.Fatalf("Failed to handle LLM response: %v", err)
	}

	if execution == nil || len(execution.Tasks) == 0 {
		t.Fatal("No execution or tasks created")
	}

	task := execution.Tasks[0]
	response := execution.Responses[0]

	// Simulate what TUI would do - add internal message to session
	internalTaskMsg := llm.Message{
		Role:      "system",
		Content:   formatTaskResultForLLMTest(&task, &response),
		Timestamp: time.Now(),
	}
	session.AddMessage(internalTaskMsg)

	// Get display messages (this is what user sees)
	displayMessages := session.GetDisplayMessages()

	// Verify no internal messages leak through
	for _, msg := range displayMessages {
		if strings.Contains(msg, "TASK_RESULT:") || strings.Contains(msg, "STATUS: Success") {
			t.Errorf("CRITICAL: Internal message leaked to user display: %s", msg)
		}
	}

	t.Logf("Display messages user sees:")
	for i, msg := range displayMessages {
		t.Logf("  %d: %s", i+1, msg)
	}

	// UPDATED: File SHOULD exist immediately (no confirmation needed)
	filePath := tempDir + "/public/index.html"
	if !fileExistsHelper(filePath) {
		t.Errorf("File should exist immediately after execution")
	}

	// UPDATED: Confirmation is no longer required
	if task.RequiresConfirmation() {
		t.Errorf("Task should NOT require confirmation after removing confirmation system")
	}

	t.Logf("SUCCESS: User sees proper messages, no internal leak, confirmation required")
}

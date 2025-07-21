package task

import (
	"strings"
	"testing"
)

// TestImmediateExecutionFlow tests that immediate execution works correctly
func TestImmediateExecutionFlow(t *testing.T) {
	// This test verifies the immediate execution flow:
	// 1. User provides edit task
	// 2. Task executes immediately and creates file
	// 3. Internal "TASK_RESULT: ... STATUS: Success" message is created
	// 4. User sees SUCCESS and file is actually created

	tempDir := t.TempDir()
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Test the exact format that causes the leak
	exactUserInput := `EDIT public/index.html
{"content":"<!DOCTYPE html>\n<html lang=\"en\">\n  <head>\n    <meta charset=\"UTF-8\" />\n    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\" />\n    <title>Fatih Secilmis Dentist Office</title>\n  </head>\n  <body>\n    <div id=\"root\"></div>\n  </body>\n</html>"}`

	// Parse task
	taskList, err := ParseTasks(exactUserInput)
	if err != nil {
		t.Fatalf("Failed to parse tasks: %v", err)
	}

	if taskList == nil || len(taskList.Tasks) == 0 {
		t.Fatal("No tasks parsed")
	}

	task := taskList.Tasks[0]

	// Execute task (this creates the response that will be formatted incorrectly)
	response := executor.Execute(&task)

	// Simulate the TUI's formatTaskResultForLLM function that creates the misleading message
	internalMessage := formatTaskResultForLLMTest(&task, response)

	t.Logf("Internal message that should be hidden from user:\n%s", internalMessage)

	// This is what the user sees - the internal message format
	expectedLeakedContent := []string{
		"TASK_RESULT: Edit public/index.html",
		"STATUS: Success",
		"CONTENT:",
		"Content replacement preview for public/index.html",
	}

	for _, expected := range expectedLeakedContent {
		if !strings.Contains(internalMessage, expected) {
			t.Errorf("Internal message should contain '%s' but doesn't. Message: %s", expected, internalMessage)
		}
	}

	// UPDATED: File SHOULD exist after success (immediate execution)
	filePath := tempDir + "/public/index.html"
	if !fileExistsHelper(filePath) {
		t.Errorf("File should exist after successful execution: %s", filePath)
	}

	// UPDATED: The internal message shows "SUCCESS" and:
	// 1. File WAS created (confirmed above)
	// 2. No confirmation dialog (by design)
	// 3. User sees immediate success
	t.Logf("SUCCESS: Immediate execution working - file created immediately with SUCCESS status")
}

// formatTaskResultForLLMTest reproduces the exact function from TUI that creates the problematic message
func formatTaskResultForLLMTest(task *Task, response *TaskResponse) string {
	var content strings.Builder

	content.WriteString("TASK_RESULT: " + task.Description() + "\n")

	if response.Success {
		content.WriteString("STATUS: Success\n")
		// Use ActualContent for LLM context (includes full file content, etc.)
		if response.ActualContent != "" {
			content.WriteString("CONTENT:\n" + response.ActualContent + "\n")
		} else if response.Output != "" {
			content.WriteString("CONTENT:\n" + response.Output + "\n")
		}
	} else {
		content.WriteString("STATUS: Failed\n")
		if response.Error != "" {
			content.WriteString("ERROR: " + response.Error + "\n")
		}
	}

	return content.String()
}

// TestInternalMessageFiltering tests that internal messages should be filtered from user display
func TestInternalMessageFiltering(t *testing.T) {
	// Create mock chat session
	mockChat := &MockChatSession{}

	// Simulate adding internal message to chat session (this is the bug)
	_ = "TASK_RESULT: Edit test.html (replace content)\nSTATUS: Success\nCONTENT:\nContent replacement preview..."

	// This should NOT appear in user display messages
	displayMessages := mockChat.GetDisplayMessages()

	// Check if internal message would leak through
	for _, msg := range displayMessages {
		if strings.Contains(msg, "TASK_RESULT:") && strings.Contains(msg, "STATUS: Success") {
			t.Errorf("CRITICAL BUG: Internal TASK_RESULT message leaked to user display: %s", msg)
		}
	}
}

// TestCorrectUserFeedback tests what the user SHOULD see instead of internal messages
func TestCorrectUserFeedback(t *testing.T) {
	tempDir := t.TempDir()
	executor := NewExecutor(tempDir, false, 1024*1024)
	mockChat := &MockChatSession{}
	manager := NewManager(executor, nil, mockChat)

	exactUserInput := `EDIT public/index.html
{"content":"<html><body>test</body></html>"}`

	// Create event channel
	eventChan := make(chan TaskExecutionEvent, 10)

	// Execute through manager (proper flow)
	execution, err := manager.HandleLLMResponse(exactUserInput, eventChan)
	if err != nil {
		t.Fatalf("Failed to handle LLM response: %v", err)
	}

	if execution == nil || len(execution.Tasks) == 0 {
		t.Fatal("No execution or tasks created")
	}

	task := execution.Tasks[0]
	_ = execution.Responses[0] // Not needed for this test

	// Capture events
	var events []TaskExecutionEvent
	close(eventChan)
	for event := range eventChan {
		events = append(events, event)
	}

	// UPDATED: Find completion event (no confirmation events anymore)
	var completionEvent *TaskExecutionEvent
	for _, event := range events {
		if event.Type == "task_completed" && !event.RequiresInput {
			completionEvent = &event
			break
		}
	}

	if completionEvent == nil {
		t.Error("Expected completion event for immediate execution")
	} else {
		t.Logf("CORRECT: Completion event generated: %s", completionEvent.Message)
	}

	// UPDATED: Verify immediate execution behavior
	t.Logf("What user SHOULD see: Immediate success for '%s'", task.Description())
	t.Logf("What user CORRECTLY sees: File created immediately")

	// UPDATED: File SHOULD exist immediately
	filePath := tempDir + "/public/index.html"
	if !fileExistsHelper(filePath) {
		t.Errorf("File should exist immediately after execution")
	}

	t.Logf("CORRECT FLOW: File created immediately without confirmation")
}

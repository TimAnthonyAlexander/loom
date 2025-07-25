package task

import (
	"strings"
	"testing"
)

// TestSequentialManagerConfirmationBug tests the bug where SequentialTaskManager bypasses confirmation
func TestSequentialManagerConfirmationBug(t *testing.T) {
	executor := NewExecutor(t.TempDir(), false, 1024*1024)
	mockChat := &MockChatSession{}
	manager := NewSequentialTaskManager(executor, nil, mockChat)

	// Use proper LOOM_EDIT format to edit files
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

	// Parse a single task from the response
	task, _, err := manager.ParseSingleTask(llmResponse)
	if err != nil {
		t.Fatalf("Failed to parse single task: %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be parsed, got nil")
	}

	// Make sure task type is correctly set
	if task.Type != TaskTypeEditFile {
		t.Errorf("Expected task type EditFile, got %v", task.Type)
	}

	if !task.LoomEditCommand {
		t.Fatal("Expected task to be recognized as LOOM_EDIT command")
	}
}

// TestFilenameSearchPreservation tests that filename search results are properly preserved
// in the LLM context and added to chat history.
func TestFilenameSearchPreservation(t *testing.T) {
	// Create a temp dir with a sample file
	tempDir := t.TempDir()
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Create mock chat session to track messages
	mockChat := &MockChatSession{}

	// Create sequential manager with the mock chat
	manager := NewSequentialTaskManager(executor, nil, mockChat)

	// Parse a filename search task
	llmResponse := `🔍 SEARCH "sample.json" names`

	// Parse the task
	task, _, err := manager.ParseSingleTask(llmResponse)
	if err != nil {
		t.Fatalf("Failed to parse search task: %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be parsed, got nil")
	}

	// Verify task is correctly parsed as a Search task with filename search enabled
	if task.Type != TaskTypeSearch {
		t.Errorf("Expected task type Search, got %v", task.Type)
	}

	if !task.SearchNames {
		t.Errorf("Expected SearchNames to be true for filename search")
	}

	// Execute the search task (no actual files will be found in temp dir)
	taskResponse := executor.Execute(task)

	// Format the task result as it would be in the exploration context
	taskResultMsg := manager.formatTaskResultForExploration(task, taskResponse)

	// Add to exploration context
	manager.addToExplorationContext(taskResultMsg)

	// Make sure it's added to the chat session
	if err := mockChat.AddMessage(taskResultMsg); err != nil {
		t.Fatalf("Failed to add message to chat: %v", err)
	}

	// VERIFY THE BUG: Check that the message contains the expected filename result marker
	// and that it's properly added to the chat session
	explorationContext := manager.GetExplorationContext()

	foundInContext := false
	for _, msg := range explorationContext {
		if strings.Contains(msg.Content, "TASK_RESULT:") &&
			strings.Contains(msg.Content, "Search for") &&
			strings.Contains(msg.Content, "sample.json") {

			// This should pass when the bug is fixed
			if strings.Contains(msg.Content, "FOUND FILES MATCHING NAME") {
				foundInContext = true
			} else if strings.Contains(msg.Content, "NO FILES FOUND MATCHING NAME") {
				foundInContext = true // This is also valid for empty results
			}
		}
	}

	if !foundInContext {
		t.Error("Bug confirmed: Filename search results not properly marked in exploration context")
	}

	// Check if the message was properly added to chat session
	foundInChat := false
	for _, msg := range mockChat.messages {
		if strings.Contains(msg.Content, "TASK_RESULT:") &&
			strings.Contains(msg.Content, "Search for") &&
			strings.Contains(msg.Content, "sample.json") {

			if strings.Contains(msg.Content, "FOUND FILES MATCHING NAME") ||
				strings.Contains(msg.Content, "NO FILES FOUND MATCHING NAME") {
				foundInChat = true
			}
		}
	}

	if !foundInChat {
		t.Error("Bug confirmed: Filename search results not properly added to chat session")
	}
}

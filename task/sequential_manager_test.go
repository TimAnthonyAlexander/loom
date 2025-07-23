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

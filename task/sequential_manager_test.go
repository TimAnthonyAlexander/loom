package task

import (
	"strings"
	"testing"
)

// TestSequentialManagerConfirmationBug tests the bug where SequentialTaskManager bypasses confirmation
func TestSequentialManagerConfirmationBug(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create executor and sequential manager
	executor := NewExecutor(tempDir, false, 1024*1024)
	mockChat := &MockChatSession{}
	sequentialManager := NewSequentialTaskManager(executor, nil, mockChat)

	// The exact LLM response that was causing the bug
	llmResponse := `I'll create the HTML file for you.

🔧 EDIT public/index.html -> create HTML file
` + "```html" + `
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

	// Parse the single task
	task, explorationContent, err := sequentialManager.ParseSingleTask(llmResponse)
	if err != nil {
		t.Fatalf("Failed to parse task: %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be parsed, got nil")
	}

	// Verify task details
	if task.Type != TaskTypeEditFile {
		t.Errorf("Expected TaskTypeEditFile, got %s", task.Type)
	}

	if task.Path != "public/index.html" {
		t.Errorf("Expected path 'public/index.html', got '%s'", task.Path)
	}

	// UPDATED: Tasks no longer require confirmation
	if task.RequiresConfirmation() {
		t.Errorf("Task should NOT require confirmation after removing confirmation system")
	}

	// UPDATED: Execute the task (this now writes the file immediately)
	taskResponse := executor.Execute(task)

	if !taskResponse.Success {
		t.Errorf("Expected task execution to succeed, got error: %s", taskResponse.Error)
	}

	// UPDATED: Check that file WAS created immediately
	filePath := tempDir + "/public/index.html"
	if !fileExists(filePath) {
		t.Errorf("File SHOULD exist immediately after execution: %s", filePath)
	}

	// The critical bug: SequentialTaskManager would format this as success and continue
	// without ever asking for confirmation or writing the file
	taskResultMsg := sequentialManager.formatTaskResultForExploration(task, taskResponse)

	// Verify the misleading success message
	if !strings.Contains(taskResultMsg.Content, "STATUS: Success") {
		t.Errorf("Expected task result to contain 'STATUS: Success', got: %s", taskResultMsg.Content)
	}

	if !strings.Contains(taskResultMsg.Content, "TASK_RESULT: Edit public/index.html") {
		t.Errorf("Expected task result to contain task description, got: %s", taskResultMsg.Content)
	}

	// UPDATED: Now this behavior is correct - "STATUS: Success" means file was created
	t.Logf("Task result message correctly shows success and file was created:\n%s", taskResultMsg.Content)

	// UPDATED: No confirmation checks needed - immediate execution is the new design
	if taskResponse.Success {
		t.Logf("Task executed successfully and file was created immediately")
	} else {
		t.Errorf("Task should have executed successfully")
	}

	// UPDATED: File should exist since task was executed immediately
	if !fileExists(filePath) {
		t.Errorf("File should exist since task was executed immediately: %s", filePath)
	}

	_ = explorationContent // Suppress unused variable warning
}

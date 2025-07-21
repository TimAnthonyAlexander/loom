package task

import (
	"os"
	"path/filepath"
	"testing"
)

// TestImmediateExecutionSingleFile tests that single file operations work immediately
func TestImmediateExecutionSingleFile(t *testing.T) {
	tempDir := t.TempDir()
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Test the exact user input format
	userInput := `EDIT src/index.js
{"content":"import React from 'react';\nimport ReactDOM from 'react-dom/client';\n\nfunction App() {\n  return <h1>Hello World</h1>;\n}\n\nconst root = ReactDOM.createRoot(document.getElementById('root'));\nroot.render(<App />);"}`

	// Parse task
	taskList, err := ParseTasks(userInput)
	if err != nil {
		t.Fatalf("Failed to parse tasks: %v", err)
	}

	if taskList == nil || len(taskList.Tasks) == 0 {
		t.Fatal("No tasks parsed")
	}

	task := taskList.Tasks[0]

	// Verify task doesn't require confirmation anymore
	if task.RequiresConfirmation() {
		t.Errorf("Task should NOT require confirmation after fix")
	}

	// Execute task
	response := executor.Execute(&task)

	// Verify response
	if !response.Success {
		t.Fatalf("Task execution failed: %s", response.Error)
	}

	// CRITICAL TEST: File must actually exist after Success=true
	filePath := filepath.Join(tempDir, "src", "index.js")
	if !fileExistsHelper(filePath) {
		t.Fatalf("CRITICAL FAILURE: File does not exist despite Success=true: %s", filePath)
	}

	// Verify file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read created file: %v", err)
	}

	expectedContent := "import React from 'react';\nimport ReactDOM from 'react-dom/client';\n\nfunction App() {\n  return <h1>Hello World</h1>;\n}\n\nconst root = ReactDOM.createRoot(document.getElementById('root'));\nroot.render(<App />);"
	if string(content) != expectedContent {
		t.Errorf("File content mismatch.\nExpected: %s\nGot: %s", expectedContent, string(content))
	}

	t.Logf("SUCCESS: Single file created immediately: %s", filePath)
}

// TestImmediateExecutionMultipleFiles tests that multiple file operations work correctly
func TestImmediateExecutionMultipleFiles(t *testing.T) {
	tempDir := t.TempDir()
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Test multiple file creation in one operation
	userInputs := []string{
		`EDIT src/App.js
{"content":"import React from 'react';\n\nfunction App() {\n  return (\n    <div>\n      <h1>My App</h1>\n      <p>Welcome to the app!</p>\n    </div>\n  );\n}\n\nexport default App;"}`,

		`EDIT src/index.html
{"content":"<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n  <meta charset=\"UTF-8\">\n  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n  <title>My App</title>\n</head>\n<body>\n  <div id=\"root\"></div>\n</body>\n</html>"}`,

		`EDIT package.json
{"content":"{\n  \"name\": \"my-app\",\n  \"version\": \"1.0.0\",\n  \"dependencies\": {\n    \"react\": \"^18.2.0\",\n    \"react-dom\": \"^18.2.0\"\n  },\n  \"scripts\": {\n    \"start\": \"react-scripts start\"\n  }\n}"}`,
	}

	expectedFiles := []string{
		"src/App.js",
		"src/index.html",
		"package.json",
	}

	// Execute each task
	for i, userInput := range userInputs {
		t.Logf("Processing file %d: %s", i+1, expectedFiles[i])

		// Parse task
		taskList, err := ParseTasks(userInput)
		if err != nil {
			t.Fatalf("Failed to parse task %d: %v", i+1, err)
		}

		if taskList == nil || len(taskList.Tasks) == 0 {
			t.Fatalf("No tasks parsed for input %d", i+1)
		}

		task := taskList.Tasks[0]

		// Verify no confirmation required
		if task.RequiresConfirmation() {
			t.Errorf("Task %d should NOT require confirmation", i+1)
		}

		// Execute task
		response := executor.Execute(&task)

		// Verify response
		if !response.Success {
			t.Fatalf("Task %d execution failed: %s", i+1, response.Error)
		}

		// CRITICAL TEST: File must exist immediately
		filePath := filepath.Join(tempDir, expectedFiles[i])
		if !fileExistsHelper(filePath) {
			t.Fatalf("CRITICAL FAILURE: File %d does not exist despite Success=true: %s", i+1, filePath)
		}

		t.Logf("✓ File %d created successfully: %s", i+1, filePath)
	}

	// Verify all files exist
	for i, expectedFile := range expectedFiles {
		filePath := filepath.Join(tempDir, expectedFile)
		if !fileExistsHelper(filePath) {
			t.Errorf("File %d missing after all operations: %s", i+1, filePath)
		}
	}

	t.Logf("SUCCESS: All %d files created immediately without confirmations", len(expectedFiles))
}

// TestManagerWithMultipleFiles tests that the manager handles multiple files correctly
func TestManagerWithMultipleFiles(t *testing.T) {
	tempDir := t.TempDir()
	executor := NewExecutor(tempDir, false, 1024*1024)
	mockChat := &MockChatSession{}
	manager := NewManager(executor, nil, mockChat)

	// Simulate the user's scenario - multiple files in response
	llmResponse := `I'll create the React app structure for you.

🔧 EDIT src/App.js
{"content":"import React from 'react';\n\nfunction App() {\n  return <h1>Fatih Secilmis Dentist Office</h1>;\n}\n\nexport default App;"}

🔧 EDIT src/index.js  
{"content":"import React from 'react';\nimport ReactDOM from 'react-dom/client';\nimport App from './App';\n\nconst root = ReactDOM.createRoot(document.getElementById('root'));\nroot.render(<App />);"}

🔧 EDIT public/index.html
{"content":"<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n  <meta charset=\"UTF-8\">\n  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n  <title>Fatih Secilmis Dentist Office</title>\n</head>\n<body>\n  <div id=\"root\"></div>\n</body>\n</html>"}`

	// Execute through manager
	eventChan := make(chan TaskExecutionEvent, 10)
	execution, err := manager.HandleLLMResponse(llmResponse, eventChan)
	if err != nil {
		t.Fatalf("Manager failed to handle LLM response: %v", err)
	}

	if execution == nil {
		t.Fatal("No execution returned from manager")
	}

	// Should have 3 tasks
	if len(execution.Tasks) != 3 {
		t.Fatalf("Expected 3 tasks, got %d", len(execution.Tasks))
	}

	expectedFiles := []string{
		"src/App.js",
		"src/index.js",
		"public/index.html",
	}

	// Verify all tasks completed successfully
	for i, task := range execution.Tasks {
		response := execution.Responses[i]

		if !response.Success {
			t.Errorf("Task %d failed: %s", i+1, response.Error)
			continue
		}

		// CRITICAL TEST: File must exist immediately
		filePath := filepath.Join(tempDir, expectedFiles[i])
		if !fileExistsHelper(filePath) {
			t.Errorf("CRITICAL FAILURE: File %d does not exist despite Success=true: %s", i+1, filePath)
		} else {
			t.Logf("✓ File %d created: %s", i+1, filePath)
		}

		// Verify no confirmation required
		if task.RequiresConfirmation() {
			t.Errorf("Task %d should NOT require confirmation", i+1)
		}
	}

	// Capture and check events - should be completion events, not confirmation events
	close(eventChan)
	var events []TaskExecutionEvent
	for event := range eventChan {
		events = append(events, event)
	}

	// Should have completion events for each task, but NO confirmation events
	for _, event := range events {
		if event.RequiresInput {
			t.Errorf("Found confirmation event when none should exist: %s", event.Message)
		}

		if event.Type == "task_completed" && !event.RequiresInput {
			t.Logf("✓ Proper completion event: %s", event.Message)
		}
	}

	t.Logf("SUCCESS: Manager processed %d files immediately without confirmations", len(expectedFiles))
}

// Helper function is already defined in other test files as fileExistsHelper

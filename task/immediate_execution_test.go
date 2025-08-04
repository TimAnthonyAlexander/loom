package task

import (
	"os"
	"path/filepath"
	"testing"
)

// fileExists is a helper function to check if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// TestImmediateExecutionSingleFile tests that single file operations work immediately
func TestImmediateExecutionSingleFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create executor and manager
	executor := NewExecutor(tempDir, false, 1024*1024)
	mockChat := &MockChatSession{}
	manager := NewManager(executor, nil, mockChat)

	// Use proper LOOM_EDIT format to edit files
	llmResponse := `>>LOOM_EDIT file=src/App.js REPLACE 1-1
import React from 'react';
import './App.css';

function App() {
  return (
    <div className="App">
      <header className="App-header">
        <h1>Hello World</h1>
      </header>
    </div>
  );
}

export default App;
<<LOOM_EDIT`

	// Execute the LLM response
	userEventChan := make(chan UserTaskEvent, 10)
	eventChan := make(chan TaskExecutionEvent, 10)
	execution, err := manager.HandleLLMResponse(llmResponse, userEventChan, eventChan)
	if err != nil {
		t.Fatalf("Failed to handle LLM response: %v", err)
	}

	if execution == nil {
		t.Fatal("No tasks parsed")
	}

	// Verify we have a task
	if len(execution.Tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(execution.Tasks))
	}

	if len(execution.Responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(execution.Responses))
	}

	task := execution.Tasks[0]
	response := execution.Responses[0]

	// Verify it's a LOOM_EDIT command
	if !task.LoomEditCommand {
		t.Fatal("Task was not recognized as a LOOM_EDIT command")
	}

	// Verify the task is correctly parsed
	if task.Type != TaskTypeEditFile {
		t.Errorf("Expected TaskTypeEditFile, got %s", task.Type)
	}

	if task.Path != "src/App.js" {
		t.Errorf("Expected path 'src/App.js', got '%s'", task.Path)
	}

	// Verify execution was successful
	if !response.Success {
		t.Errorf("Task execution failed: %s", response.Error)
	}
}

// TestImmediateExecutionMultipleFiles tests that multiple file operations work correctly
func TestImmediateExecutionMultipleFiles(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create executor and manager
	executor := NewExecutor(tempDir, false, 1024*1024)
	mockChat := &MockChatSession{}
	manager := NewManager(executor, nil, mockChat)

	// Use proper LOOM_EDIT format for multiple files
	inputs := []string{
		`>>LOOM_EDIT file=src/App.js REPLACE 1-1
import React from 'react';
import './App.css';

function App() {
  return (
    <div className="App">
      <header className="App-header">
        <h1>Hello World</h1>
      </header>
    </div>
  );
}

export default App;
<<LOOM_EDIT`,

		`>>LOOM_EDIT file=src/index.js REPLACE 1-1
import React from 'react';
import ReactDOM from 'react-dom/client';
import './index.css';
import App from './App';

const root = ReactDOM.createRoot(document.getElementById('root'));
root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
<<LOOM_EDIT`,

		`>>LOOM_EDIT file=public/index.html REPLACE 1-1
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>React App</title>
  </head>
  <body>
    <div id="root"></div>
  </body>
</html>
<<LOOM_EDIT`,
	}

	fileNames := []string{
		"src/App.js",
		"src/index.js",
		"public/index.html",
	}

	for i, input := range inputs {
		t.Logf("Processing file %d: %s", i+1, fileNames[i])

		// Create event channels for each task
		userEventChan := make(chan UserTaskEvent, 10)
		eventChan := make(chan TaskExecutionEvent, 10)

		// Execute the LLM response
		execution, err := manager.HandleLLMResponse(input, userEventChan, eventChan)
		if err != nil {
			t.Fatalf("Failed to handle LLM response %d: %v", i+1, err)
		}

		if execution == nil {
			t.Fatalf("No tasks parsed for input %d", i+1)
		}

		// Verify we have a task
		if len(execution.Tasks) != 1 {
			t.Fatalf("Expected 1 task for input %d, got %d", i+1, len(execution.Tasks))
		}

		if len(execution.Responses) != 1 {
			t.Fatalf("Expected 1 response for input %d, got %d", i+1, len(execution.Responses))
		}

		task := execution.Tasks[0]
		response := execution.Responses[0]

		// Verify it's a LOOM_EDIT command
		if !task.LoomEditCommand {
			t.Fatalf("Task %d was not recognized as a LOOM_EDIT command", i+1)
		}

		// Verify execution was successful
		if !response.Success {
			t.Errorf("Task %d execution failed: %s", i+1, response.Error)
		}
	}

	// Verify all files were created
	files := []string{
		"src/App.js",
		"src/index.js",
		"public/index.html",
	}

	for _, file := range files {
		fullPath := filepath.Join(tempDir, file)
		if !fileExists(fullPath) {
			t.Errorf("File should exist but doesn't: %s", fullPath)
		}
	}
}

// TestManagerWithMultipleFiles tests that the manager handles multiple files correctly
func TestManagerWithMultipleFiles(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create executor and manager
	executor := NewExecutor(tempDir, false, 1024*1024)
	mockChat := &MockChatSession{}
	manager := NewManager(executor, nil, mockChat)

	// Use proper LOOM_EDIT format for multiple files in a single response
	llmResponse := `Let me create the basic React app structure:

>>LOOM_EDIT file=src/App.js REPLACE 1-1
import React from 'react';
import './App.css';

function App() {
  return (
    <div className="App">
      <header className="App-header">
        <h1>Hello World</h1>
      </header>
    </div>
  );
}

export default App;
<<LOOM_EDIT

>>LOOM_EDIT file=src/index.js REPLACE 1-1
import React from 'react';
import ReactDOM from 'react-dom/client';
import './index.css';
import App from './App';

const root = ReactDOM.createRoot(document.getElementById('root'));
root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
<<LOOM_EDIT

>>LOOM_EDIT file=public/index.html REPLACE 1-1
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>React App</title>
  </head>
  <body>
    <div id="root"></div>
  </body>
</html>
<<LOOM_EDIT`

	// Create event channels for all tasks
	userEventChan := make(chan UserTaskEvent, 10)
	eventChan := make(chan TaskExecutionEvent, 10)

	// Execute the LLM response
	execution, err := manager.HandleLLMResponse(llmResponse, userEventChan, eventChan)
	if err != nil {
		t.Fatalf("Failed to handle LLM response: %v", err)
	}

	if execution == nil {
		t.Fatal("No execution returned from manager")
	}

	// Verify we got 3 tasks
	if len(execution.Tasks) != 3 {
		t.Fatalf("Expected 3 tasks, got %d", len(execution.Tasks))
	}

	// Verify all tasks were LOOM_EDIT commands
	for i, task := range execution.Tasks {
		if !task.LoomEditCommand {
			t.Errorf("Task %d not recognized as LOOM_EDIT command", i+1)
		}
	}

	// Verify all responses were successful
	for i, response := range execution.Responses {
		if !response.Success {
			t.Errorf("Task %d execution failed: %s", i+1, response.Error)
		}
	}

	// Verify all files were created
	files := []string{
		"src/App.js",
		"src/index.js",
		"public/index.html",
	}

	for _, file := range files {
		fullPath := filepath.Join(tempDir, file)
		if !fileExists(fullPath) {
			t.Errorf("File should exist but doesn't: %s", fullPath)
		}
	}
}

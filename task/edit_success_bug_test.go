package task

import (
	"os"
	"testing"
)

// TestEditTaskFalseSuccessBug tests the critical bug where edit tasks report SUCCESS
// but don't actually create files or show confirmation dialogs
func TestEditTaskFalseSuccessBug(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Use proper LOOM_EDIT format to edit files
	// Note: There is no EDIT task, only LOOM_EDIT format is supported
	exactUserInput := `>>LOOM_EDIT file=public/index.html REPLACE 1-1
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Fatih Secilmis Dentist Office</title>
  </head>
  <body>
    <div id="root"></div>
  </body>
</html>
<<LOOM_EDIT`

	// Test with Manager (normal flow)
	t.Run("Manager_Normal_Flow", func(t *testing.T) {
		executor := NewExecutor(tempDir+"/normal", false, 1024*1024)
		mockChat := &MockChatSession{}
		manager := NewManager(executor, nil, mockChat)

		// Create event channel to capture events
		eventChan := make(chan TaskExecutionEvent, 10)

		// Execute the LLM response
		execution, err := manager.HandleLLMResponse(exactUserInput, eventChan)
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
		if len(execution.Responses) != 1 {
			t.Fatalf("Expected 1 response, got %d", len(execution.Responses))
		}

		response := execution.Responses[0]

		// CRITICAL TEST: If response shows Success, the file MUST exist OR confirmation MUST be required
		filePath := tempDir + "/normal/public/index.html"
		fileExists := fileExistsHelper(filePath)

		t.Logf("Task: %s", task.Description())
		t.Logf("Response Success: %t", response.Success)
		t.Logf("File exists: %t", fileExists)
		t.Logf("Requires confirmation: %t", task.RequiresConfirmation())

		if response.Success {
			if task.RequiresConfirmation() {
				// If task requires confirmation, file should NOT exist yet
				if fileExists {
					t.Errorf("CRITICAL BUG: Task reports Success and requires confirmation, but file already exists! This means confirmation was bypassed.")
				}

				// Should have generated confirmation event
				var foundConfirmationEvent bool
				close(eventChan)
				for event := range eventChan {
					if event.Type == "task_completed" && event.RequiresInput {
						foundConfirmationEvent = true
						break
					}
				}

				if !foundConfirmationEvent {
					t.Errorf("CRITICAL BUG: Task requires confirmation but no confirmation event was generated")
				}

			} else {
				// If task doesn't require confirmation, file MUST exist after success
				if !fileExists {
					t.Errorf("CRITICAL BUG: Task reports Success and doesn't require confirmation, but file doesn't exist! Success is false.")
				}
			}
		} else {
			// If task failed, file should not exist
			if fileExists {
				t.Errorf("CRITICAL BUG: Task failed but file was still created")
			}
		}
	})

	// Test with Executor directly (bypass manager)
	t.Run("Executor_Direct", func(t *testing.T) {
		executor := NewExecutor(tempDir+"/direct", false, 1024*1024)

		// Parse the task
		taskList, err := ParseTasks(exactUserInput)
		if err != nil {
			t.Fatalf("Failed to parse tasks: %v", err)
		}

		if taskList == nil {
			t.Fatal("No tasks parsed from user input")
		}

		// Verify we have a task
		if len(taskList.Tasks) != 1 {
			t.Fatalf("Expected 1 task, got %d", len(taskList.Tasks))
		}

		task := taskList.Tasks[0]

		// Execute the task
		response := executor.Execute(&task)

		// CRITICAL TEST: If response shows Success, the file MUST exist
		filePath := tempDir + "/direct/public/index.html"
		fileExists := fileExistsHelper(filePath)

		t.Logf("Task: %s", task.Description())
		t.Logf("Response Success: %t", response.Success)
		t.Logf("File exists: %t", fileExists)

		if response.Success {
			if !fileExists {
				t.Errorf("CRITICAL BUG: Task reports Success directly through executor but file doesn't exist!")
			}
		} else {
			// If task failed, file should not exist
			if fileExists {
				t.Errorf("CRITICAL BUG: Task failed but file was still created")
			}
		}
	})

	// Test with Sequential Manager
	t.Run("SequentialManager", func(t *testing.T) {
		executor := NewExecutor(tempDir+"/sequential", false, 1024*1024)
		mockChat := &MockChatSession{}
		manager := NewSequentialTaskManager(executor, nil, mockChat)

		// Parse a single task
		task, _, err := manager.ParseSingleTask(exactUserInput)
		if err != nil {
			t.Fatalf("Failed to parse task: %v", err)
		}

		if task == nil {
			t.Fatal("No task parsed")
		}

		// Execute the task
		response := executor.Execute(task)

		// CRITICAL TEST: If response shows Success, the file MUST exist
		filePath := tempDir + "/sequential/public/index.html"
		fileExists := fileExistsHelper(filePath)

		t.Logf("Task: %s", task.Description())
		t.Logf("Response Success: %t", response.Success)
		t.Logf("File exists: %t", fileExists)

		if response.Success {
			if !fileExists {
				t.Errorf("CRITICAL BUG: Sequential task reports Success but file doesn't exist!")
			}
		} else {
			// If task failed, file should not exist
			if fileExists {
				t.Errorf("CRITICAL BUG: Task failed but file was still created")
			}
		}
	})
}

// TestFalseSuccessPatterns tests various patterns that might cause false success
func TestFalseSuccessPatterns(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "Natural_Language_With_JSON",
			input: `🔧 EDIT test.html -> create file\n{"content":"<html></html>"}`,
		},
		{
			name: "JSON_After_Task_Line",
			input: `EDIT test.html
{"content":"<html><body>test</body></html>"}`,
		},
		{
			name:  "Escaped_JSON_Content",
			input: `EDIT test.html\n{\"content\":\"<html>\\n<body>test</body>\\n</html>\"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			executor := NewExecutor(tempDir+"/"+tc.name, false, 1024*1024)

			// Parse and execute
			taskList, err := ParseTasks(tc.input)
			if err != nil {
				t.Logf("Parse error (might be expected): %v", err)
				return
			}

			if taskList == nil || len(taskList.Tasks) == 0 {
				t.Logf("No tasks parsed from input: %s", tc.input)
				return
			}

			task := taskList.Tasks[0]
			response := executor.Execute(&task)

			filePath := tempDir + "/" + tc.name + "/" + task.Path
			fileExists := fileExistsHelper(filePath)

			t.Logf("Pattern %s - Success: %t, File exists: %t, Requires confirmation: %t",
				tc.name, response.Success, fileExists, task.RequiresConfirmation())

			// Apply the same rules: SUCCESS should only be true if file exists OR proper confirmation flow
			if response.Success {
				if task.RequiresConfirmation() {
					// File should NOT exist yet - should be prepared for confirmation
					if fileExists {
						t.Errorf("Pattern %s: SUCCESS reported but file exists without confirmation", tc.name)
					}
				} else {
					// File MUST exist if no confirmation required
					if !fileExists {
						t.Errorf("Pattern %s: SUCCESS reported but file doesn't exist (no confirmation required)", tc.name)
					}
				}
			}
		})
	}
}

// Helper function to check if file exists
func fileExistsHelper(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

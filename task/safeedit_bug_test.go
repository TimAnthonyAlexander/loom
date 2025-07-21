package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeEditBugReproduction(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom_safeedit_bug_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an executor
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Create a README.md file with the exact content the user mentioned
	testFile := filepath.Join(tempDir, "README.md")
	originalContent := `# Loom
**Advanced AI-Driven Coding Assistant**
A sophisticated terminal-based AI coding assistant written in Go that provides a conversational interface for understanding, modifying, and extending codebases. Features autonomous task execution, intelligent context management, comprehensive security, and seamless project integration.`

	err = os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Parse the exact task format the user provided
	llmResponse := `🔧 EDIT README.md:2 -> append "(Test Edit)" to the tagline

--- BEFORE ---
# Loom
**Advanced AI-Driven Coding Assistant**
--- CHANGE ---
EDIT_LINES:2
**Advanced AI-Driven Coding Assistant (Test Edit)**
--- AFTER ---
A sophisticated terminal-based AI coding assistant written in Go that provides a conversational interface for understanding, modifying, and extending codebases. Features autonomous task execution, intelligent context management, comprehensive security, and seamless project integration.`

	// Parse the task
	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		t.Fatalf("Failed to parse task: %v", err)
	}

	if taskList == nil || len(taskList.Tasks) == 0 {
		t.Fatalf("No tasks parsed from LLM response")
	}

	task := &taskList.Tasks[0]
	
	// Log what was parsed
	t.Logf("Parsed task:")
	t.Logf("  Type: %s", task.Type)
	t.Logf("  Path: %s", task.Path)
	t.Logf("  Intent: %s", task.Intent)
	t.Logf("  TargetLine: %d", task.TargetLine)
	t.Logf("  SafeEditMode: %t", task.SafeEditMode)
	t.Logf("  BeforeContext: %q", task.BeforeContext)
	t.Logf("  AfterContext: %q", task.AfterContext)
	t.Logf("  Content: %q", task.Content)

	// Execute the task
	response := executor.Execute(task)
	
	t.Logf("Execution result:")
	t.Logf("  Success: %t", response.Success)
	t.Logf("  Error: %s", response.Error)
	t.Logf("  Output: %s", response.Output)
	
	if !response.Success {
		t.Errorf("Task execution failed: %s", response.Error)
		return
	}

	// Check if the task requires confirmation (it should for SafeEdit)
	requiresConfirmation := task.RequiresConfirmation()
	t.Logf("  RequiresConfirmation: %t", requiresConfirmation)
	
	if !requiresConfirmation {
		t.Errorf("SafeEdit task should require confirmation, but it doesn't")
	}

	// Apply the edit to see what the final content would be
	err = executor.ApplyEdit(&response.Task)
	if err != nil {
		t.Fatalf("Failed to apply edit: %v", err)
	}

	// Read the file and verify the change was made
	actualContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file after edit: %v", err)
	}

	actualContentStr := string(actualContent)
	expectedContent := `# Loom
**Advanced AI-Driven Coding Assistant (Test Edit)**
A sophisticated terminal-based AI coding assistant written in Go that provides a conversational interface for understanding, modifying, and extending codebases. Features autonomous task execution, intelligent context management, comprehensive security, and seamless project integration.`

	if actualContentStr != expectedContent {
		t.Errorf("File content after edit is incorrect.")
		t.Errorf("Expected:\n%s", expectedContent)
		t.Errorf("Got:\n%s", actualContentStr)
		
		// Show the differences line by line
		expectedLines := strings.Split(expectedContent, "\n")
		actualLines := strings.Split(actualContentStr, "\n")
		
		t.Logf("Line by line comparison:")
		maxLines := len(expectedLines)
		if len(actualLines) > maxLines {
			maxLines = len(actualLines)
		}
		
		for i := 0; i < maxLines; i++ {
			var expectedLine, actualLine string
			if i < len(expectedLines) {
				expectedLine = expectedLines[i]
			}
			if i < len(actualLines) {
				actualLine = actualLines[i]
			}
			
			if expectedLine != actualLine {
				t.Logf("  Line %d DIFF:", i+1)
				t.Logf("    Expected: %q", expectedLine)
				t.Logf("    Actual:   %q", actualLine)
			} else {
				t.Logf("  Line %d OK: %q", i+1, expectedLine)
			}
		}
	}
} 
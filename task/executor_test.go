package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTargetedEditReplaceBug(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom_executor_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an executor
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Create a test file with content that includes "Loom"
	testFile := filepath.Join(tempDir, "README.md")
	originalContent := "# Loom\n\nThis is a test file."
	err = os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a targeted edit task to replace "Loom" with "Spoon"
	task := &Task{
		Type:         TaskTypeEditFile,
		Path:         "README.md",
		Content:      "# Spoon",
		StartContext: "# Loom",
		InsertMode:   "replace",
	}

	// Execute the edit (prepare stage)
	response := executor.executeEditFile(task)
	if !response.Success {
		t.Fatalf("Edit execution failed: %s", response.Error)
	}

	// Verify that response.Task.Content contains the correct new content
	expectedContent := "# Spoon\n\nThis is a test file."
	if response.Task.Content != expectedContent {
		t.Errorf("Expected content:\n%s\nGot:\n%s", expectedContent, response.Task.Content)
	}

	// Apply the edit (this simulates the confirmation step)
	err = executor.ApplyEdit(&response.Task)
	if err != nil {
		t.Fatalf("Failed to apply edit: %v", err)
	}

	// Read the file and verify it contains the correct content
	actualContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file after edit: %v", err)
	}

	actualContentStr := string(actualContent)
	if actualContentStr != expectedContent {
		t.Errorf("File content after edit is incorrect.\nExpected:\n%s\nGot:\n%s", expectedContent, actualContentStr)
	}

	// Specifically check that no diff syntax is present in the file
	if strings.Contains(actualContentStr, "- #") || strings.Contains(actualContentStr, "+ #") {
		t.Errorf("File contains diff syntax, which indicates the bug is still present:\n%s", actualContentStr)
	}
}

func TestReplaceContentBug(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom_executor_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an executor
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	originalContent := "Hello World"
	err = os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a content replacement task
	newContent := "Hello Universe"
	task := &Task{
		Type:    TaskTypeEditFile,
		Path:    "test.txt",
		Content: newContent,
	}

	// Execute the edit (prepare stage)
	response := executor.executeEditFile(task)
	if !response.Success {
		t.Fatalf("Edit execution failed: %s", response.Error)
	}

	// Verify that response.Task.Content is set correctly (this was the first bug)
	if response.Task.Content != newContent {
		t.Errorf("Expected response.Task.Content to be %q, got %q", newContent, response.Task.Content)
	}

	// Apply the edit
	err = executor.ApplyEdit(&response.Task)
	if err != nil {
		t.Fatalf("Failed to apply edit: %v", err)
	}

	// Read the file and verify it contains the correct content
	actualContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file after edit: %v", err)
	}

	if string(actualContent) != newContent {
		t.Errorf("File content after edit is incorrect.\nExpected: %s\nGot: %s", newContent, string(actualContent))
	}
}

func TestDiffContentInContentField(t *testing.T) {
	// This test reproduces the exact issue: LLM provides diff-formatted content
	// in the content field instead of either final content or using the diff field

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom_executor_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an executor
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Create a test file with content that includes "Loom"
	testFile := filepath.Join(tempDir, "README.md")
	originalContent := "# Loom\n\nThis is a test file."
	err = os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a task where LLM incorrectly provides diff format in content field
	// This should now be automatically detected and processed correctly
	diffFormattedContent := "- # Loom\n+ # Spoon\n\nThis is a test file."
	task := &Task{
		Type:    TaskTypeEditFile,
		Path:    "README.md",
		Content: diffFormattedContent, // This should now be detected as diff format
	}

	// Execute the edit (prepare stage)
	response := executor.executeEditFile(task)
	if !response.Success {
		t.Fatalf("Edit execution failed: %s", response.Error)
	}

	// Check that the response task content was processed correctly
	expectedContent := "# Spoon\n\nThis is a test file."
	if response.Task.Content != expectedContent {
		t.Errorf("Response task content not processed correctly.\nExpected:\n%s\nGot:\n%s", expectedContent, response.Task.Content)
	}

	// Apply the edit
	err = executor.ApplyEdit(&response.Task)
	if err != nil {
		t.Fatalf("Failed to apply edit: %v", err)
	}

	// Read the file and verify what actually got written
	actualContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file after edit: %v", err)
	}

	actualContentStr := string(actualContent)

	// Verify the file does NOT contain diff syntax
	if strings.Contains(actualContentStr, "- #") || strings.Contains(actualContentStr, "+ #") {
		t.Errorf("BUG STILL EXISTS: File contains diff syntax:\n%s", actualContentStr)
	}

	// Verify the file contains the correct final content
	if actualContentStr != expectedContent {
		t.Errorf("File content is incorrect.\nExpected:\n%s\nGot:\n%s", expectedContent, actualContentStr)
	} else {
		t.Logf("SUCCESS: File contains correct content without diff syntax")
	}
}

func TestReplaceAllOccurrences(t *testing.T) {
	// Test the new replace_all functionality for global find-and-replace

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom_executor_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an executor
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Create a test file with multiple occurrences of "loom"
	testFile := filepath.Join(tempDir, "README.md")
	originalContent := `# Loom AI Assistant

Loom is a powerful AI coding assistant that helps with various tasks.

## Features

- Loom can read files
- Loom can edit files  
- Loom provides intelligent assistance

## Installation

To install loom, run:
` + "```bash" + `
npm install loom
` + "```" + `

## Usage

Start loom by running the loom command.

Welcome to loom!`

	err = os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test various replace all patterns
	testCases := []struct {
		name        string
		description string
		findText    string
		replaceText string
		expectCount int // Expected number of replacements
	}{
		{
			name:        "replace all occurrences pattern",
			description: "replace all occurrences of \"loom\" with \"spoon\"",
			findText:    "loom",
			replaceText: "spoon",
			expectCount: 5, // Count of "loom" instances in the content (case sensitive)
		},
		{
			name:        "replace all simple pattern",
			description: "replace all \"Loom\" with \"Spoon\"",
			findText:    "Loom",
			replaceText: "Spoon",
			expectCount: 5, // Count of "Loom" instances (case sensitive)
		},
		{
			name:        "find and replace pattern",
			description: "find and replace \"npm install loom\" with \"npm install spoon\"",
			findText:    "npm install loom",
			replaceText: "npm install spoon",
			expectCount: 1, // Should find and replace the npm install line
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset file content for each test
			err = os.WriteFile(testFile, []byte(originalContent), 0644)
			if err != nil {
				t.Fatalf("Failed to reset test file: %v", err)
			}

			// Create a replace_all task
			task := &Task{
				Type:   TaskTypeEditFile,
				Path:   "README.md",
				Intent: tc.description,
			}

			// Parse the edit context
			parseEditContext(task, tc.description)

			// Verify it was parsed correctly
			if task.InsertMode != "replace_all" {
				t.Errorf("Expected InsertMode 'replace_all', got '%s'", task.InsertMode)
			}

			if task.StartContext != tc.findText {
				t.Errorf("Expected StartContext '%s', got '%s'", tc.findText, task.StartContext)
			}

			if task.EndContext != tc.replaceText {
				t.Errorf("Expected EndContext '%s', got '%s'", tc.replaceText, task.EndContext)
			}

			// Execute the edit (prepare stage)
			response := executor.executeEditFile(task)
			if !response.Success {
				t.Fatalf("Edit execution failed: %s", response.Error)
			}

			// Verify the content was replaced correctly
			expectedContent := strings.ReplaceAll(originalContent, tc.findText, tc.replaceText)
			if response.Task.Content != expectedContent {
				t.Errorf("Content replacement incorrect.\nExpected:\n%s\nGot:\n%s", expectedContent, response.Task.Content)
			}

			// Apply the edit
			err = executor.ApplyEdit(&response.Task)
			if err != nil {
				t.Fatalf("Failed to apply edit: %v", err)
			}

			// Read and verify the final file content
			actualContent, err := os.ReadFile(testFile)
			if err != nil {
				t.Fatalf("Failed to read file after edit: %v", err)
			}

			actualContentStr := string(actualContent)
			if actualContentStr != expectedContent {
				t.Errorf("File content after edit is incorrect.\nExpected:\n%s\nGot:\n%s", expectedContent, actualContentStr)
			}

			// Verify the replacements were made
			originalCount := strings.Count(originalContent, tc.findText)
			newCount := strings.Count(actualContentStr, tc.findText)
			replacementsMade := originalCount - newCount

			if replacementsMade != tc.expectCount {
				t.Errorf("Expected %d replacements, but made %d replacements", tc.expectCount, replacementsMade)
			}

			// Verify the new text appears the expected number of times
			newTextCount := strings.Count(actualContentStr, tc.replaceText)
			expectedNewCount := strings.Count(originalContent, tc.replaceText) + tc.expectCount

			if newTextCount < expectedNewCount {
				t.Errorf("Expected at least %d occurrences of '%s', but found %d", expectedNewCount, tc.replaceText, newTextCount)
			}

			t.Logf("SUCCESS: Replaced %d occurrences of '%s' with '%s'", replacementsMade, tc.findText, tc.replaceText)
		})
	}
}

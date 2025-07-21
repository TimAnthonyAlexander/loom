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

// TestSafeEditFormatParsing tests parsing of the SafeEdit format
func TestSafeEditFormatParsing(t *testing.T) {
	// Test valid SafeEdit format
	llmResponse := `🔧 EDIT main.go:15-17 → replace error handling

BEFORE_CONTEXT:
    if err != nil {
        log.Fatal(err)
    }

EDIT_LINES: 15-17
    if err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }

AFTER_CONTEXT:
    
    return result`

	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		t.Fatalf("Failed to parse SafeEdit format: %v", err)
	}

	if taskList == nil || len(taskList.Tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(taskList.Tasks))
	}

	task := taskList.Tasks[0]

	// Verify task type and basic fields
	if task.Type != TaskTypeEditFile {
		t.Errorf("Expected TaskTypeEditFile, got %v", task.Type)
	}

	if task.Path != "main.go" {
		t.Errorf("Expected path 'main.go', got '%s'", task.Path)
	}

	// Verify SafeEdit mode is enabled
	if !task.SafeEditMode {
		t.Error("Expected SafeEditMode to be true")
	}

	// Verify line range
	if task.TargetStartLine != 15 || task.TargetEndLine != 17 {
		t.Errorf("Expected lines 15-17, got %d-%d", task.TargetStartLine, task.TargetEndLine)
	}

	// Verify context content
	expectedBefore := `    if err != nil {
        log.Fatal(err)
    }
`
	if task.BeforeContext != expectedBefore {
		t.Errorf("Expected BeforeContext:\n%q\nGot:\n%q", expectedBefore, task.BeforeContext)
	}

	expectedAfter := `    
    return result`
	if task.AfterContext != expectedAfter {
		t.Errorf("Expected AfterContext:\n%s\nGot:\n%s", expectedAfter, task.AfterContext)
	}

	expectedContent := `    if err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }
`
	if task.Content != expectedContent {
		t.Errorf("Expected Content:\n%q\nGot:\n%q", expectedContent, task.Content)
	}
}

// TestSafeEditContextValidationSuccess tests successful context validation
func TestSafeEditContextValidationSuccess(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom_safeedit_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an executor
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Create a test file with specific content
	testFile := filepath.Join(tempDir, "main.go")
	originalContent := `package main

import "fmt"

func main() {
    if err != nil {
        log.Fatal(err)
    }
    
    return result
}`
	err = os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a SafeEdit task with correct context
	task := &Task{
		Type:            TaskTypeEditFile,
		Path:            "main.go",
		SafeEditMode:    true,
		TargetStartLine: 6,
		TargetEndLine:   8,
		BeforeContext: `import "fmt"

func main() {`,
		Content: `    if err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }`,
		AfterContext: `    
    return result`,
	}

	// Execute the SafeEdit
	response := executor.executeEditFile(task)
	if !response.Success {
		t.Fatalf("SafeEdit execution failed: %s", response.Error)
	}

	// Verify the content was changed correctly
	expectedNewContent := `package main

import "fmt"

func main() {
    if err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }
    
    return result
}`

	if response.Task.Content != expectedNewContent {
		t.Errorf("Expected new content:\n%s\nGot:\n%s", expectedNewContent, response.Task.Content)
	}

	// Verify context validation message
	if !strings.Contains(response.ActualContent, "Context validation: PASSED ✓") {
		t.Error("Expected context validation success message")
	}
}

// TestSafeEditContextValidationFailure tests context validation failures
func TestSafeEditContextValidationFailure(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom_safeedit_fail_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an executor
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Create a test file with specific content
	testFile := filepath.Join(tempDir, "main.go")
	originalContent := `package main

import "fmt"

func main() {
    if err != nil {
        log.Fatal(err)
    }
    
    return result
}`
	err = os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test case 1: Incorrect before context
	task1 := &Task{
		Type:            TaskTypeEditFile,
		Path:            "main.go",
		SafeEditMode:    true,
		TargetStartLine: 6,
		TargetEndLine:   8,
		BeforeContext: `import "wrong"

func main() {`, // Wrong context
		Content: `    if err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }`,
		AfterContext: `    
    return result`,
	}

	response1 := executor.executeEditFile(task1)
	if response1.Success {
		t.Error("Expected SafeEdit to fail with incorrect before context")
	}
	if !strings.Contains(response1.Error, "CONTEXT VALIDATION FAILED") {
		t.Errorf("Expected context validation error, got: %s", response1.Error)
	}

	// Test case 2: Incorrect after context
	task2 := &Task{
		Type:            TaskTypeEditFile,
		Path:            "main.go",
		SafeEditMode:    true,
		TargetStartLine: 6,
		TargetEndLine:   8,
		BeforeContext: `import "fmt"

func main() {`,
		Content: `    if err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }`,
		AfterContext: `    
    return wrong`, // Wrong context
	}

	response2 := executor.executeEditFile(task2)
	if response2.Success {
		t.Error("Expected SafeEdit to fail with incorrect after context")
	}
	if !strings.Contains(response2.Error, "CONTEXT VALIDATION FAILED") {
		t.Errorf("Expected context validation error, got: %s", response2.Error)
	}
}

// TestSafeEditMissingContext tests validation when context is missing
func TestSafeEditMissingContext(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom_safeedit_missing_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an executor
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Create a test file
	testFile := filepath.Join(tempDir, "main.go")
	originalContent := `package main

func main() {
    fmt.Println("hello")
}`
	err = os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test missing BeforeContext
	task1 := &Task{
		Type:            TaskTypeEditFile,
		Path:            "main.go",
		SafeEditMode:    true,
		TargetStartLine: 4,
		TargetEndLine:   4,
		// BeforeContext missing
		Content:      `    fmt.Println("world")`,
		AfterContext: `}`,
	}

	response1 := executor.executeEditFile(task1)
	if response1.Success {
		t.Error("Expected SafeEdit to fail with missing BeforeContext")
	}
	if !strings.Contains(response1.Error, "BeforeContext is required") {
		t.Errorf("Expected BeforeContext required error, got: %s", response1.Error)
	}

	// Test missing AfterContext
	task2 := &Task{
		Type:            TaskTypeEditFile,
		Path:            "main.go",
		SafeEditMode:    true,
		TargetStartLine: 4,
		TargetEndLine:   4,
		BeforeContext:   `func main() {`,
		Content:         `    fmt.Println("world")`,
		// AfterContext missing
	}

	response2 := executor.executeEditFile(task2)
	if response2.Success {
		t.Error("Expected SafeEdit to fail with missing AfterContext")
	}
	if !strings.Contains(response2.Error, "AfterContext is required") {
		t.Errorf("Expected AfterContext required error, got: %s", response2.Error)
	}
}

// TestSafeEditSingleLine tests SafeEdit with single line edits
func TestSafeEditSingleLine(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom_safeedit_single_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an executor
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Create a test file
	testFile := filepath.Join(tempDir, "main.go")
	originalContent := `package main

func main() {
    userName := "john"
    fmt.Println(userName)
}`
	err = os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a single-line SafeEdit task
	task := &Task{
		Type:          TaskTypeEditFile,
		Path:          "main.go",
		SafeEditMode:  true,
		TargetLine:    4, // Single line edit
		BeforeContext: `func main() {`,
		Content:       `    username := "john"`,
		AfterContext:  `    fmt.Println(userName)`,
	}

	// Execute the SafeEdit
	response := executor.executeEditFile(task)
	if !response.Success {
		t.Fatalf("SafeEdit single line execution failed: %s", response.Error)
	}

	// Verify the content was changed correctly
	expectedNewContent := `package main

func main() {
    username := "john"
    fmt.Println(userName)
}`

	if response.Task.Content != expectedNewContent {
		t.Errorf("Expected new content:\n%s\nGot:\n%s", expectedNewContent, response.Task.Content)
	}
}

// TestSafeEditVsLegacyComparison tests that SafeEdit is safer than legacy methods
func TestSafeEditVsLegacyComparison(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "loom_safeedit_comparison_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an executor
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Create a test file that has been modified since AI last saw it
	testFile := filepath.Join(tempDir, "main.go")
	originalContent := `package main

func main() {
    // This line was changed after AI saw the file
    fmt.Println("modified content")
    return
}`
	err = os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test legacy line-based edit (without context validation)
	// AI thinks it's editing the old content but file has changed
	legacyTask := &Task{
		Type:            TaskTypeEditFile,
		Path:            "main.go",
		TargetStartLine: 4,
		TargetEndLine:   4,
		Content:         `    fmt.Println("hello world")`, // AI expects old content here
	}

	legacyResponse := executor.executeEditFile(legacyTask)
	// Legacy edit succeeds even though the context is wrong
	if !legacyResponse.Success {
		t.Logf("Legacy edit failed (which is actually good): %s", legacyResponse.Error)
	} else {
		t.Logf("Legacy edit succeeded without context validation (potentially unsafe)")
	}

	// Test SafeEdit with outdated context (should fail safely)
	safeTask := &Task{
		Type:            TaskTypeEditFile,
		Path:            "main.go",
		SafeEditMode:    true,
		TargetStartLine: 4,
		TargetEndLine:   4,
		BeforeContext:   `func main() {`,
		Content:         `    fmt.Println("hello world")`,
		AfterContext:    `    fmt.Println("old content")`, // AI expects old content
	}

	safeResponse := executor.executeEditFile(safeTask)
	// SafeEdit should fail because context doesn't match
	if safeResponse.Success {
		t.Error("SafeEdit should have failed due to context mismatch, but it succeeded")
	}

	if !strings.Contains(safeResponse.Error, "CONTEXT VALIDATION FAILED") {
		t.Errorf("Expected context validation failure, got: %s", safeResponse.Error)
	}

	t.Logf("SafeEdit correctly prevented unsafe edit due to context mismatch")
}

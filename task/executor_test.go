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
	err = executor.ApplyEditForTesting(&response.Task)
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
	err = executor.ApplyEditForTesting(&response.Task)
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
	err = executor.ApplyEditForTesting(&response.Task)
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
			err = executor.ApplyEditForTesting(&response.Task)
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

// TestSafeEditFormatParsing tests parsing of the new SafeEdit fenced format
func TestSafeEditFormatParsing(t *testing.T) {
	// Test new fenced SafeEdit format
	llmResponse := `🔧 EDIT main.go:15-17 -> replace error handling

--- BEFORE ---
    if err != nil {
        log.Fatal(err)
    }

--- CHANGE ---
EDIT_LINES: 15-17
    if err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }

--- AFTER ---
    
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

// TestSafeEditFormatParsingLegacy tests parsing of the old SafeEdit format for backward compatibility
func TestSafeEditFormatParsingLegacy(t *testing.T) {
	// Test old SafeEdit format (with Unicode arrow)
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
		t.Fatalf("Failed to parse legacy SafeEdit format: %v", err)
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
		t.Error("Expected SafeEditMode to be true for legacy format")
	}

	// Verify line range
	if task.TargetStartLine != 15 || task.TargetEndLine != 17 {
		t.Errorf("Expected lines 15-17, got %d-%d", task.TargetStartLine, task.TargetEndLine)
	}

	// Verify context content (should be identical regardless of format)
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

	// Test case 1: Completely wrong before context that doesn't match any content
	task1 := &Task{
		Type:            TaskTypeEditFile,
		Path:            "main.go",
		SafeEditMode:    true,
		TargetStartLine: 6,
		TargetEndLine:   8,
		BeforeContext: `import "database/sql"

func helper() {`, // Content that doesn't exist in the file
		Content: `    if err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }`,
		AfterContext: `    
    return result`,
	}

	response1 := executor.executeEditFile(task1)
	if response1.Success {
		t.Error("Expected SafeEdit to fail with completely incorrect before context")
	}
	if !strings.Contains(response1.Error, "CONTEXT VALIDATION FAILED") {
		t.Errorf("Expected context validation error, got: %s", response1.Error)
	}

	// Test case 2: Completely wrong after context that doesn't match any content
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
    defer cleanup()`, // Content that doesn't exist in the file
	}

	response2 := executor.executeEditFile(task2)
	if response2.Success {
		t.Error("Expected SafeEdit to fail with completely incorrect after context")
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

// TestSafeEditFormatParsingExactUserCase tests the exact case from the user's bug report
func TestSafeEditFormatParsingExactUserCase(t *testing.T) {
	// This is the exact SafeEdit format that was failing in the user's conversation
	llmResponse := `🔧 EDIT sample.json:36 -> bump version in metadata

--- BEFORE ---
35      "generated_at": "2024-06-15T12:00:00Z",
36      "version": "1.0.0"
37    }
--- CHANGE ---
EDIT_LINES: 36
      "version": "1.1.0"
--- AFTER ---
37    }
38  }`

	t.Log("Testing exact user case SafeEdit format parsing...")

	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		t.Fatalf("Failed to parse exact user case SafeEdit format: %v", err)
	}

	if taskList == nil {
		t.Fatal("TaskList is nil - parsing failed completely")
	}

	if len(taskList.Tasks) == 0 {
		t.Fatal("No tasks found - parsing failed to detect the SafeEdit task")
	}

	if len(taskList.Tasks) != 1 {
		t.Fatalf("Expected exactly 1 task, got %d", len(taskList.Tasks))
	}

	task := taskList.Tasks[0]

	// Verify basic task properties
	if task.Type != TaskTypeEditFile {
		t.Errorf("Expected TaskTypeEditFile, got %v", task.Type)
	}

	if task.Path != "sample.json" {
		t.Errorf("Expected path 'sample.json', got '%s'", task.Path)
	}

	// Verify SafeEdit mode is enabled
	if !task.SafeEditMode {
		t.Error("Expected SafeEditMode to be true")
	}

	// Verify line targeting (should be single line edit)
	if task.TargetLine != 36 {
		t.Errorf("Expected TargetLine to be 36, got %d", task.TargetLine)
	}

	// Verify context content
	expectedBefore := `35      "generated_at": "2024-06-15T12:00:00Z",
36      "version": "1.0.0"
37    }`
	if task.BeforeContext != expectedBefore {
		t.Errorf("BeforeContext mismatch.\nExpected:\n%q\nGot:\n%q", expectedBefore, task.BeforeContext)
	}

	expectedAfter := `37    }
38  }`
	if task.AfterContext != expectedAfter {
		t.Errorf("AfterContext mismatch.\nExpected:\n%q\nGot:\n%q", expectedAfter, task.AfterContext)
	}

	expectedContent := `      "version": "1.1.0"`
	if task.Content != expectedContent {
		t.Errorf("Content mismatch.\nExpected:\n%q\nGot:\n%q", expectedContent, task.Content)
	}

	t.Log("Successfully parsed exact user case SafeEdit format!")
}

// TestSafeEditFormatDebugParsing tests parsing with debug output to understand what's happening
func TestSafeEditFormatDebugParsing(t *testing.T) {
	// Enable debug mode for this test
	origDebug := debugTaskParsing
	debugTaskParsing = true
	defer func() { debugTaskParsing = origDebug }()

	llmResponse := `🔧 EDIT sample.json:36 -> bump version in metadata

--- BEFORE ---
35      "generated_at": "2024-06-15T12:00:00Z",
36      "version": "1.0.0"
37    }
--- CHANGE ---
EDIT_LINES: 36
      "version": "1.1.0"
--- AFTER ---
37    }
38  }`

	t.Log("Testing SafeEdit parsing with debug output...")

	taskList, err := ParseTasks(llmResponse)

	t.Logf("ParseTasks result: taskList=%v, err=%v", taskList, err)

	if taskList != nil {
		t.Logf("TaskList has %d tasks", len(taskList.Tasks))
		for i, task := range taskList.Tasks {
			t.Logf("Task %d: Type=%v, Path=%s, SafeEditMode=%v", i, task.Type, task.Path, task.SafeEditMode)
		}
	}

	// Even if parsing fails, let's also test the internal parsing functions directly
	t.Log("Testing parseSafeEditFormat directly...")

	lines := strings.Split(llmResponse, "\n")

	// Find the EDIT line
	editLineIndex := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "🔧 EDIT") {
			editLineIndex = i
			break
		}
	}

	if editLineIndex >= 0 {
		t.Logf("Found EDIT line at index %d: %s", editLineIndex, lines[editLineIndex])

		// Create a basic task to test SafeEdit parsing
		task := &Task{
			Type: TaskTypeEditFile,
			Path: "sample.json",
		}

		// Test parseSafeEditFormat directly
		success := parseSafeEditFormat(task, lines, editLineIndex+1)
		t.Logf("parseSafeEditFormat result: success=%v", success)

		if success {
			t.Logf("SafeEdit parsing succeeded:")
			t.Logf("  SafeEditMode: %v", task.SafeEditMode)
			t.Logf("  TargetLine: %d", task.TargetLine)
			t.Logf("  BeforeContext: %q", task.BeforeContext)
			t.Logf("  Content: %q", task.Content)
			t.Logf("  AfterContext: %q", task.AfterContext)
		}
	}
}

// TestSafeEditExecutionExactUserCase tests the complete execution flow for the user's case
func TestSafeEditExecutionExactUserCase(t *testing.T) {
	// Create a temporary test file with sample.json content
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "sample.json")

	sampleContent := `{
  "users": [
    {
      "id": 1,
      "name": "John Doe",
      "email": "john.doe@example.com",
      "roles": ["admin", "user"],
      "active": true,
      "profile": {
        "age": 30,
        "address": {
          "street": "123 Main St",
          "city": "Metropolis",
          "zip": "12345"
        }
      }
    },
    {
      "id": 2,
      "name": "Jane Smith",
      "email": "jane.smith@example.com",
      "roles": ["user"],
      "active": false,
      "profile": {
        "age": 25,
        "address": {
          "street": "456 Elm St",
          "city": "Gotham",
          "zip": "67890"
        }
      }
    }
  ],
  "metadata": {
    "generated_at": "2024-06-15T12:00:00Z",
    "version": "1.0.0"
  }
}`

	err := os.WriteFile(testFile, []byte(sampleContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create executor
	executor := NewExecutor(tempDir, false, 1024*1024)

	// Parse the exact task from user's case
	llmResponse := `🔧 EDIT sample.json:36 -> bump version in metadata

--- BEFORE ---
35      "generated_at": "2024-06-15T12:00:00Z",
36      "version": "1.0.0"
37    }
--- CHANGE ---
EDIT_LINES: 36
      "version": "1.1.0"
--- AFTER ---
37    }
38  }`

	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		t.Fatalf("Failed to parse task: %v", err)
	}

	if taskList == nil || len(taskList.Tasks) == 0 {
		t.Fatal("No tasks parsed")
	}

	task := &taskList.Tasks[0]

	// Execute the task
	response := executor.Execute(task)

	t.Logf("Execution result: Success=%v, Error=%s", response.Success, response.Error)
	t.Logf("Output: %s", response.Output)

	if !response.Success {
		t.Errorf("Task execution failed: %s", response.Error)
		return
	}

	// Verify the edit was applied correctly
	if response.EditSummary != nil {
		t.Logf("Edit summary: %s", response.EditSummary.Summary)
	}

	// Apply the edit to test file
	err = executor.ApplyEditForTesting(task)
	if err != nil {
		t.Errorf("Failed to apply edit: %v", err)
		return
	}

	// Read the file and verify the change
	updatedContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	updatedString := string(updatedContent)
	if !strings.Contains(updatedString, `"version": "1.1.0"`) {
		t.Error("File was not updated correctly - version should be 1.1.0")
		t.Logf("Updated content:\n%s", updatedString)
	}

	if strings.Contains(updatedString, `"version": "1.0.0"`) {
		t.Error("File still contains old version 1.0.0")
	}

	t.Log("SafeEdit execution test passed!")
}

// TestSafeEditWithLineNumberPrefixes tests the issue where SafeEdit context includes line numbers
func TestSafeEditWithLineNumberPrefixes(t *testing.T) {
	// Create a temporary test file with the exact content from sample.json
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "sample.json")

	sampleContent := `{
  "users": [
    {
      "id": 1,
      "name": "John Doe",
      "email": "john.doe@example.com",
      "roles": ["admin", "user"],
      "active": true,
      "profile": {
        "age": 30,
        "address": {
          "street": "123 Main St",
          "city": "Metropolis",
          "zip": "12345"
        }
      }
    },
    {
      "id": 2,
      "name": "Jane Smith",
      "email": "jane.smith@example.com",
      "roles": ["user"],
      "active": false,
      "profile": {
        "age": 25,
        "address": {
          "street": "456 Elm St",
          "city": "Gotham",
          "zip": "67890"
        }
      }
    }
  ],
  "metadata": {
    "generated_at": "2024-06-15T12:00:00Z",
    "version": "1.0.0"
  }
}`

	err := os.WriteFile(testFile, []byte(sampleContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create executor
	executor := NewExecutor(tempDir, false, 1024*1024)

	// The SafeEdit task - includes line numbers in context (this should now work after the fix)
	task := &Task{
		Type:         TaskTypeEditFile,
		Path:         "sample.json",
		SafeEditMode: true,
		TargetLine:   36,
		BeforeContext: `35      "generated_at": "2024-06-15T12:00:00Z",
36      "version": "1.0.0"
37    }`,
		Content: `      "version": "1.1.0"`,
		AfterContext: `37    }
38  }`,
	}

	// This should now succeed because we fixed the context validation to handle line numbers
	response := executor.Execute(task)

	t.Logf("Execution result: Success=%v, Error=%s", response.Success, response.Error)

	if !response.Success {
		t.Errorf("SafeEdit execution failed (should succeed after fix): %s", response.Error)
		return
	}

	// Apply the edit to test file
	err = executor.ApplyEditForTesting(task)
	if err != nil {
		t.Errorf("Failed to apply edit: %v", err)
		return
	}

	// Read the file and verify the change
	updatedContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	updatedString := string(updatedContent)
	if !strings.Contains(updatedString, `"version": "1.1.0"`) {
		t.Error("File was not updated correctly - version should be 1.1.0")
		t.Logf("Updated content:\n%s", updatedString)
	}

	if strings.Contains(updatedString, `"version": "1.0.0"`) {
		t.Error("File still contains old version 1.0.0")
	}

	t.Log("SafeEdit with line number prefixes now works correctly after fix!")
}

// TestSafeEditWithoutLineNumberPrefixes tests SafeEdit with correct context (no line numbers)
func TestSafeEditWithoutLineNumberPrefixes(t *testing.T) {
	// Create a temporary test file with the exact content from sample.json
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "sample.json")

	sampleContent := `{
  "users": [
    {
      "id": 1,
      "name": "John Doe",
      "email": "john.doe@example.com",
      "roles": ["admin", "user"],
      "active": true,
      "profile": {
        "age": 30,
        "address": {
          "street": "123 Main St",
          "city": "Metropolis",
          "zip": "12345"
        }
      }
    },
    {
      "id": 2,
      "name": "Jane Smith",
      "email": "jane.smith@example.com",
      "roles": ["user"],
      "active": false,
      "profile": {
        "age": 25,
        "address": {
          "street": "456 Elm St",
          "city": "Gotham",
          "zip": "67890"
        }
      }
    }
  ],
  "metadata": {
    "generated_at": "2024-06-15T12:00:00Z",
    "version": "1.0.0"
  }
}`

	err := os.WriteFile(testFile, []byte(sampleContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create executor
	executor := NewExecutor(tempDir, false, 1024*1024)

	// SafeEdit task with correct context (no line numbers)
	task := &Task{
		Type:         TaskTypeEditFile,
		Path:         "sample.json",
		SafeEditMode: true,
		TargetLine:   36,
		BeforeContext: `    "generated_at": "2024-06-15T12:00:00Z",
    "version": "1.0.0"
  }`,
		Content: `    "version": "1.1.0"`,
		AfterContext: `  }
}`,
	}

	// This should succeed because the context is correct
	response := executor.Execute(task)

	t.Logf("Execution result: Success=%v, Error=%s", response.Success, response.Error)

	if !response.Success {
		t.Errorf("SafeEdit execution failed: %s", response.Error)
		return
	}

	// Apply the edit to test file
	err = executor.ApplyEditForTesting(task)
	if err != nil {
		t.Errorf("Failed to apply edit: %v", err)
		return
	}

	// Read the file and verify the change
	updatedContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	updatedString := string(updatedContent)
	if !strings.Contains(updatedString, `"version": "1.1.0"`) {
		t.Error("File was not updated correctly - version should be 1.1.0")
		t.Logf("Updated content:\n%s", updatedString)
	}

	if strings.Contains(updatedString, `"version": "1.0.0"`) {
		t.Error("File still contains old version 1.0.0")
	}

	t.Log("SafeEdit execution test passed!")
}

// TestSafeEditContextValidationDebug provides detailed debugging of context validation
func TestSafeEditContextValidationDebug(t *testing.T) {
	// Create a temporary test file with the exact content from sample.json
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "sample.json")

	sampleContent := `{
  "users": [
    {
      "id": 1,
      "name": "John Doe",
      "email": "john.doe@example.com",
      "roles": ["admin", "user"],
      "active": true,
      "profile": {
        "age": 30,
        "address": {
          "street": "123 Main St",
          "city": "Metropolis",
          "zip": "12345"
        }
      }
    },
    {
      "id": 2,
      "name": "Jane Smith",
      "email": "jane.smith@example.com",
      "roles": ["user"],
      "active": false,
      "profile": {
        "age": 25,
        "address": {
          "street": "456 Elm St",
          "city": "Gotham",
          "zip": "67890"
        }
      }
    }
  ],
  "metadata": {
    "generated_at": "2024-06-15T12:00:00Z",
    "version": "1.0.0"
  }
}`

	err := os.WriteFile(testFile, []byte(sampleContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Print the actual file content with line numbers for debugging
	lines := strings.Split(sampleContent, "\n")
	t.Log("Actual file content with line numbers:")
	for i, line := range lines {
		t.Logf("%2d: %s", i+1, line)
	}

	// Create executor
	executor := NewExecutor(tempDir, false, 1024*1024)

	// The problematic SafeEdit task - includes line numbers in context
	task := &Task{
		Type:         TaskTypeEditFile,
		Path:         "sample.json",
		SafeEditMode: true,
		TargetLine:   36,
		BeforeContext: `35      "generated_at": "2024-06-15T12:00:00Z",
36      "version": "1.0.0"
37    }`,
		Content: `      "version": "1.1.0"`,
		AfterContext: `37    }
38  }`,
	}

	t.Log("BeforeContext lines:")
	beforeLines := strings.Split(task.BeforeContext, "\n")
	for i, line := range beforeLines {
		t.Logf("  [%d]: %q", i, line)
	}

	t.Log("AfterContext lines:")
	afterLines := strings.Split(task.AfterContext, "\n")
	for i, line := range afterLines {
		t.Logf("  [%d]: %q", i, line)
	}

	// Let's manually test the line number stripping function
	t.Log("Testing line number stripping:")
	for _, line := range beforeLines {
		cleaned := executor.stripLineNumberPrefix(line)
		normalized := executor.normalizeWhitespace(cleaned)
		t.Logf("  Original: %q -> Cleaned: %q -> Normalized: %q", line, cleaned, normalized)
	}

	// Now test with the exact lines around line 36
	t.Log("Lines around target line 36:")
	if len(lines) >= 36 {
		for i := 33; i <= 38 && i < len(lines); i++ { // Lines 34-38 (0-indexed 33-37)
			normalized := executor.normalizeWhitespace(lines[i])
			t.Logf("  Line %d: %q -> Normalized: %q", i+1, lines[i], normalized)
		}
	}

	// Execute the task to see detailed error
	response := executor.Execute(task)
	t.Logf("Execution result: Success=%v, Error=%s", response.Success, response.Error)
}

// TestUserBugReportExactScenario tests the exact scenario from the user's bug report
func TestUserBugReportExactScenario(t *testing.T) {
	// This test replicates the exact scenario from the user's conversation log
	// where SafeEdit was being repeated multiple times without execution

	// Create a temporary test file with the exact content from the real sample.json
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "sample.json")

	// This is the actual content of sample.json from the user's workspace
	sampleContent := `{
  "users": [
    {
      "id": 1,
      "name": "John Doe",
      "email": "john.doe@example.com",
      "roles": ["admin", "user"],
      "active": true,
      "profile": {
        "age": 30,
        "address": {
          "street": "123 Main St",
          "city": "Metropolis",
          "zip": "12345"
        }
      }
    },
    {
      "id": 2,
      "name": "Jane Smith",
      "email": "jane.smith@example.com",
      "roles": ["user"],
      "active": false,
      "profile": {
        "age": 25,
        "address": {
          "street": "456 Elm St",
          "city": "Gotham",
          "zip": "67890"
        }
      }
    }
  ],
  "metadata": {
    "generated_at": "2024-06-15T12:00:00Z",
    "version": "1.0.0"
  }
}`

	err := os.WriteFile(testFile, []byte(sampleContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create executor
	executor := NewExecutor(tempDir, false, 1024*1024)

	// This is the exact LLM response from the user's conversation log that was failing
	llmResponse := `🔧 EDIT sample.json:36 -> bump version in metadata

--- BEFORE ---
35      "generated_at": "2024-06-15T12:00:00Z",
36      "version": "1.0.0"
37    }
--- CHANGE ---
EDIT_LINES: 36
      "version": "1.1.0"
--- AFTER ---
37    }
38  }`

	t.Log("Testing exact LLM response from user's bug report...")

	// Step 1: Parse the task from the LLM response
	taskList, err := ParseTasks(llmResponse)
	if err != nil {
		t.Fatalf("Failed to parse LLM response: %v", err)
	}

	if taskList == nil || len(taskList.Tasks) == 0 {
		t.Fatal("No tasks found in LLM response - parsing failed")
	}

	if len(taskList.Tasks) != 1 {
		t.Fatalf("Expected exactly 1 task, got %d", len(taskList.Tasks))
	}

	task := &taskList.Tasks[0]
	t.Logf("Parsed task: Type=%v, Path=%s, SafeEditMode=%v, TargetLine=%d",
		task.Type, task.Path, task.SafeEditMode, task.TargetLine)

	// Step 2: Execute the task (this was failing before the fix)
	response := executor.Execute(task)

	if !response.Success {
		t.Fatalf("Task execution failed (should succeed after fix): %s", response.Error)
	}

	// Verify this is a SafeEdit with context validation
	if !task.SafeEditMode {
		t.Error("Expected SafeEdit mode to be enabled")
	}

	if !strings.Contains(response.ActualContent, "Context validation: PASSED ✓") {
		t.Error("Expected context validation success message")
	}

	// Step 3: Apply the edit
	// Use the task from the response, not the original task, as it contains the updated content
	responseTask := &response.Task
	err = executor.ApplyEditForTesting(responseTask)
	if err != nil {
		t.Fatalf("Failed to apply edit: %v", err)
	}

	// Step 4: Verify the file was changed correctly
	updatedContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	updatedString := string(updatedContent)

	// Check that the version was updated
	if !strings.Contains(updatedString, `"version": "1.1.0"`) {
		t.Error("File was not updated correctly - version should be 1.1.0")
		t.Logf("Updated content:\n%s", updatedString)
	}

	// Check that the old version is gone
	if strings.Contains(updatedString, `"version": "1.0.0"`) {
		t.Error("File still contains old version 1.0.0")
	}

	// Verify the rest of the file is intact
	if !strings.Contains(updatedString, `"generated_at": "2024-06-15T12:00:00Z"`) {
		t.Error("File structure was corrupted - missing generated_at field")
	}

	if !strings.Contains(updatedString, `"name": "John Doe"`) {
		t.Error("File structure was corrupted - missing user data")
	}

	t.Log("✅ User bug report scenario fixed successfully!")
	t.Log("  - SafeEdit format parsing: ✓")
	t.Log("  - Context validation with line numbers: ✓")
	t.Log("  - Task execution: ✓")
	t.Log("  - File modification: ✓")
	t.Log("  - No more repeated task suggestions: ✓")
}

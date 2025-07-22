package task

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEditingCases tests editing functionality by executing commands from test cases
func TestEditingCases(t *testing.T) {
	// Base directory for editing tests
	baseDir := "../editing_tests"
	originalFile := filepath.Join(baseDir, "thefile.md")
	
	// Read original file content
	originalContent, err := os.ReadFile(originalFile)
	if err != nil {
		t.Fatalf("Failed to read original file %s: %v", originalFile, err)
	}

	// Find all test case directories
	testCases, err := discoverTestCases(baseDir)
	if err != nil {
		t.Fatalf("Failed to discover test cases: %v", err)
	}

	if len(testCases) == 0 {
		t.Fatal("No test cases found in editing_tests directory")
	}

	t.Logf("Found %d test cases", len(testCases))

	// Create executor for testing
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "thefile.md")
	executor := NewExecutor(tempDir, false, 1024*1024)

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			// Reset test file to original content
			err := os.WriteFile(testFile, originalContent, 0644)
			if err != nil {
				t.Fatalf("Failed to reset test file: %v", err)
			}

			// Parse command from command.txt
			task, err := parseCommandFile(testCase.CommandPath)
			if err != nil {
				t.Fatalf("Failed to parse command file: %v", err)
			}

			// Set the correct path for our test file
			task.Path = "thefile.md"

			t.Logf("Executing task: %s", task.Description())
			t.Logf("Task type: %s", task.Type)
			if task.SafeEditMode {
				t.Logf("SafeEdit mode enabled")
				t.Logf("Target line: %d, Range: %d-%d", task.TargetLine, task.TargetStartLine, task.TargetEndLine)
				t.Logf("Before context: %q", task.BeforeContext)
				t.Logf("Content: %q", task.Content)
				t.Logf("After context: %q", task.AfterContext)
			}

			// Execute the task
			response := executor.Execute(task)
			if !response.Success {
				t.Fatalf("Task execution failed: %s", response.Error)
			}

			// Read the result
			actualContent, err := os.ReadFile(testFile)
			if err != nil {
				t.Fatalf("Failed to read result file: %v", err)
			}

			// Read expected output
			expectedContent, err := os.ReadFile(testCase.OutputPath)
			if err != nil {
				t.Fatalf("Failed to read expected output file: %v", err)
			}

			// Compare results
			actualStr := string(actualContent)
			expectedStr := string(expectedContent)

			if actualStr != expectedStr {
				t.Errorf("Result does not match expected output")
				t.Errorf("Expected:\n%s", expectedStr)
				t.Errorf("Actual:\n%s", actualStr)
				
				// Show detailed diff
				t.Errorf("Expected lines: %d, Actual lines: %d", 
					len(strings.Split(expectedStr, "\n")), 
					len(strings.Split(actualStr, "\n")))
				
				// Show line-by-line comparison for debugging
				expectedLines := strings.Split(expectedStr, "\n")
				actualLines := strings.Split(actualStr, "\n")
				maxLines := len(expectedLines)
				if len(actualLines) > maxLines {
					maxLines = len(actualLines)
				}
				
				for i := 0; i < maxLines; i++ {
					expectedLine := ""
					actualLine := ""
					if i < len(expectedLines) {
						expectedLine = expectedLines[i]
					}
					if i < len(actualLines) {
						actualLine = actualLines[i]
					}
					
					if expectedLine != actualLine {
						t.Errorf("Line %d differs:", i+1)
						t.Errorf("  Expected: %q", expectedLine)
						t.Errorf("  Actual:   %q", actualLine)
					}
				}
			}
		})
	}
}

// TestCase represents a single editing test case
type TestCase struct {
	Name        string
	Dir         string
	CommandPath string
	OutputPath  string
}

// discoverTestCases finds all test case directories in the base directory
func discoverTestCases(baseDir string) ([]TestCase, error) {
	var testCases []TestCase

	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		// Skip the base directory itself
		if path == baseDir {
			return nil
		}

		// Check if this directory has command.txt and output.txt
		commandPath := filepath.Join(path, "command.txt")
		outputPath := filepath.Join(path, "output.txt")

		if fileExistsHelper(commandPath) && fileExistsHelper(outputPath) {
			// Extract case name from directory name
			caseName := filepath.Base(path)
			
			testCase := TestCase{
				Name:        caseName,
				Dir:         path,
				CommandPath: commandPath,
				OutputPath:  outputPath,
			}
			testCases = append(testCases, testCase)
		}

		return nil
	})

	return testCases, err
}

// parseCommandFile parses a command.txt file and returns a Task
func parseCommandFile(commandPath string) (*Task, error) {
	content, err := os.ReadFile(commandPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read command file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	
	// Try to parse as natural language task format first
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for task commands like "🔧 EDIT" or "EDIT"
		if strings.HasPrefix(line, "🔧 EDIT") || strings.HasPrefix(line, "EDIT") {
			// Parse the edit command
			task := parseEditCommand(line)
			if task != nil {
				// Look for SafeEdit format in remaining lines
				if parseSafeEditFormat(task, lines, i+1) {
					return task, nil
				}
				
				// Look for content in the remaining lines
				contentLines := lines[i+1:]
				content := extractContentFromLines(contentLines)
				if content != "" {
					task.Content = content
				}
				
				return task, nil
			}
		}
		
		// Look for standalone EDIT_LINES commands
		if strings.HasPrefix(line, "EDIT_LINES:") {
			task := &Task{Type: TaskTypeEditFile}
			
			// Extract line range
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				lineRange := strings.TrimSpace(parts[1])
				parseEditLineRange(task, lineRange)
			}
			
			// Get content from remaining lines until "--- AFTER ---" or end
			contentLines := []string{}
			for j := i + 1; j < len(lines); j++ {
				nextLine := strings.TrimSpace(lines[j])
				if nextLine == "--- AFTER ---" {
					break
				}
				if nextLine != "" || len(contentLines) > 0 { // Include empty lines if we've started collecting content
					contentLines = append(contentLines, lines[j])
				}
			}
			
			if len(contentLines) > 0 {
				task.Content = strings.Join(contentLines, "\n")
				task.Content = strings.TrimSpace(task.Content) // Remove trailing newlines
			}
			
			return task, nil
		}
	}

	return nil, fmt.Errorf("no valid task command found in command file")
}

// parseEditCommand parses an edit command line
func parseEditCommand(line string) *Task {
	// Remove task emoji if present
	line = strings.TrimPrefix(line, "🔧 ")
	line = strings.TrimSpace(line)

	// Look for EDIT command with arrow notation
	if strings.HasPrefix(line, "EDIT ") {
		args := strings.TrimPrefix(line, "EDIT ")
		return parseEditTask(args)
	}

	return nil
}

// extractContentFromLines extracts content from lines, handling code blocks
func extractContentFromLines(lines []string) string {
	var contentLines []string
	inCodeBlock := false
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip empty lines at the beginning
		if line == "" && len(contentLines) == 0 {
			continue
		}
		
		// Handle code block markers
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		
		// Stop at SafeEdit markers
		if line == "--- BEFORE ---" || line == "--- CHANGE ---" || line == "--- AFTER ---" {
			break
		}
		
		// Stop at other task commands
		if strings.HasPrefix(line, "🔧") || strings.HasPrefix(line, "OBJECTIVE_COMPLETE:") {
			break
		}
		
		// Collect content lines
		if inCodeBlock || line != "" {
			contentLines = append(contentLines, line)
		}
	}
	
	return strings.Join(contentLines, "\n")
}

 
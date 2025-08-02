package validation

import (
	"loom/loom_edit"
	"os"
	"strings"
	"testing"
)

func TestExtractEditContext(t *testing.T) {
	// Create a temporary test file
	testContent := `package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
    fmt.Println("This is a test")
    fmt.Println("Another line")
    fmt.Println("Final line")
}`

	tmpFile, err := os.CreateTemp("", "test_edit_*.go")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tmpFile.Close()

	// Test REPLACE operation
	editCmd := &loom_edit.EditCommand{
		File:    tmpFile.Name(),
		Action:  "REPLACE",
		Start:   6, // Line with "This is a test"
		End:     6,
		NewText: `    fmt.Println("This line was replaced")`,
	}

	// Apply the edit first
	err = loom_edit.ApplyEdit(tmpFile.Name(), editCmd)
	if err != nil {
		t.Fatalf("Failed to apply edit: %v", err)
	}

	// Now extract context
	context, err := ExtractEditContext(tmpFile.Name(), editCmd, testContent)
	if err != nil {
		t.Fatalf("Failed to extract edit context: %v", err)
	}

	// Verify context
	if context.StartLine != 6 {
		t.Errorf("Expected StartLine 6, got %d", context.StartLine)
	}

	if context.LanguageID != "go" {
		t.Errorf("Expected language 'go', got '%s'", context.LanguageID)
	}

	if context.EditAction != "REPLACE" {
		t.Errorf("Expected action 'REPLACE', got '%s'", context.EditAction)
	}

	// Check that we have context lines
	if len(context.ContextBefore) == 0 {
		t.Error("Expected context before edit")
	}

	if len(context.ContextAfter) == 0 {
		t.Error("Expected context after edit")
	}

	// Check that we have modified lines
	if len(context.ModifiedLines) != 1 {
		t.Errorf("Expected 1 modified line, got %d", len(context.ModifiedLines))
	}

	if !strings.Contains(context.ModifiedLines[0], "This line was replaced") {
		t.Errorf("Expected modified line to contain replacement text, got: %s", context.ModifiedLines[0])
	}
}

func TestFormatVerificationForLLM(t *testing.T) {
	context := &EditContext{
		FilePath:      "test.go",
		StartLine:     5,
		EndLine:       5,
		LanguageID:    "go",
		EditAction:    "REPLACE",
		OriginalLines: []string{`    fmt.Println("Original line")`},
		ModifiedLines: []string{`    fmt.Println("Modified line")`},
		ContextBefore: []string{
			"package main",
			"",
			"func main() {",
		},
		ContextAfter: []string{
			"    return",
			"}",
		},
	}

	verification := FormatVerificationForLLM(context, nil)

	// Check that verification contains expected sections
	expectedSections := []string{
		"📝 EDIT VERIFICATION:",
		"Context before edit:",
		"Original content (replaced):",
		"Your changes:",
		"Context after edit:",
		"Please review this verification",
	}

	for _, section := range expectedSections {
		if !strings.Contains(verification, section) {
			t.Errorf("Expected verification to contain '%s'", section)
		}
	}

	// Check that it contains the file info
	if !strings.Contains(verification, "test.go") {
		t.Error("Expected verification to contain file path")
	}

	if !strings.Contains(verification, "REPLACE go") {
		t.Error("Expected verification to contain action and language")
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"test.go", "go"},
		{"script.js", "javascript"},
		{"component.tsx", "typescript"},
		{"style.css", "css"},
		{"config.json", "json"},
		{"README.md", "markdown"},
		{"Dockerfile", "dockerfile"},
		{"unknown.xyz", "text"},
	}

	for _, test := range tests {
		result := DetectLanguage(test.filename)
		if result != test.expected {
			t.Errorf("For file '%s', expected language '%s', got '%s'",
				test.filename, test.expected, result)
		}
	}
}

func TestValidationConfig(t *testing.T) {
	config := DefaultValidationConfig()

	if !config.EnableVerification {
		t.Error("Expected EnableVerification to be true by default")
	}

	if config.EnableLSP {
		t.Error("Expected EnableLSP to be false by default")
	}

	if config.ContextLines != 8 {
		t.Errorf("Expected ContextLines to be 8, got %d", config.ContextLines)
	}

	if len(config.SupportedLanguages) == 0 {
		t.Error("Expected some supported languages by default")
	}
}

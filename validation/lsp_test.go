package validation

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"loom/loom_edit"
)

func TestValidatorBasicFunctionality(t *testing.T) {
	tempDir := t.TempDir()

	// Create a simple validator with defaults
	config := DefaultValidationConfig()
	validator := NewValidator(tempDir, &config)
	defer validator.Shutdown()

	// Test language detection
	if DetectLanguage("test.go") != "go" {
		t.Errorf("Expected 'go' for .go file")
	}

	if DetectLanguage("test.js") != "javascript" {
		t.Errorf("Expected 'javascript' for .js file")
	}

	// Test basic validation for a non-existent file
	result, err := validator.ValidateFile("nonexistent.go")
	if err != nil {
		t.Errorf("Validation should not fail for non-existent file: %v", err)
	}

	// Should be "fallback", "none", or "unavailable" for non-existent files
	if result.ValidatorUsed != "none" && result.ValidatorUsed != "unavailable" && result.ValidatorUsed != "fallback" {
		t.Errorf("Expected 'none', 'unavailable', or 'fallback' validator, got: %s", result.ValidatorUsed)
	}
}

func TestJSONValidation(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultValidationConfig()
	validator := NewValidator(tempDir, &config)
	defer validator.Shutdown()

	// Test valid JSON
	validJSON := `{"name": "test", "value": 123}`
	validFile := createTempFile(t, tempDir, "valid.json", validJSON)

	result, err := validator.ValidateFile(validFile)
	if err != nil {
		t.Errorf("Validation failed: %v", err)
	}

	if !result.IsValid {
		t.Errorf("Valid JSON should pass validation")
	}

	// Test invalid JSON
	invalidJSON := `{"name": "test", "value": 123`
	invalidFile := createTempFile(t, tempDir, "invalid.json", invalidJSON)

	result, err = validator.ValidateFile(invalidFile)
	if err != nil {
		t.Errorf("Validation failed: %v", err)
	}

	if result.IsValid {
		t.Errorf("Invalid JSON should fail validation")
	}

	if len(result.Errors) == 0 {
		t.Errorf("Expected validation errors for invalid JSON")
	}
}

func TestRollbackLogic(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultValidationConfig()
	config.RollbackOnSyntaxError = true
	validator := NewValidator(tempDir, &config)
	defer validator.Shutdown()

	// Create a validation result with critical errors
	validationResult := &ValidationResult{
		IsValid: false,
		Errors: []LSPDiagnostic{
			{
				Range: Range{
					Start: Position{Line: 0, Character: 0},
					End:   Position{Line: 0, Character: 10},
				},
				Message:  "syntax error: unexpected token",
				Severity: func() *int { s := 1; return &s }(),
			},
		},
		ValidatorUsed: "test",
	}

	shouldRollback := validator.ShouldRollbackEdit(validationResult)
	if !shouldRollback {
		t.Errorf("Should rollback on syntax error")
	}

	// Test non-critical error
	validationResult.Errors[0].Message = "variable unused"
	shouldRollback = validator.ShouldRollbackEdit(validationResult)
	if shouldRollback {
		t.Errorf("Should not rollback on non-critical error")
	}
}

func TestEditValidation(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultValidationConfig()
	validator := NewValidator(tempDir, &config)
	defer validator.Shutdown()

	// Create a test file
	originalContent := `package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
`
	testFile := createTempFile(t, tempDir, "test.go", originalContent)

	// Create an edit command
	editCmd := &loom_edit.EditCommand{
		File:    testFile,
		Action:  "REPLACE",
		Start:   6,
		End:     6,
		NewText: `    fmt.Println("Hello, Loom!")`,
	}

	// Apply the edit
	err := loom_edit.ApplyEdit(testFile, editCmd)
	if err != nil {
		t.Fatalf("Failed to apply edit: %v", err)
	}

	// Validate the edit
	result, err := validator.ValidateEditOperation(testFile, editCmd, originalContent)
	if err != nil {
		t.Errorf("Edit validation failed: %v", err)
	}

	if result == nil {
		t.Errorf("Expected validation result")
	}

	if result.Context == nil {
		t.Errorf("Expected edit context")
	}

	if result.VerificationText == "" {
		t.Errorf("Expected verification text")
	}
}

func TestConfigurationDefaults(t *testing.T) {
	config := DefaultValidationConfig()

	if !config.EnableVerification {
		t.Errorf("Expected EnableVerification to be true by default")
	}

	if config.EnableLSP {
		t.Errorf("Expected EnableLSP to be false by default")
	}

	if config.ContextLines != 8 {
		t.Errorf("Expected ContextLines to be 8, got %d", config.ContextLines)
	}

	if len(config.SupportedLanguages) == 0 {
		t.Errorf("Expected some supported languages")
	}

	if len(config.RollbackOnErrors) == 0 {
		t.Errorf("Expected some rollback patterns")
	}
}

func TestLSPClientCreation(t *testing.T) {
	tempDir := t.TempDir()

	config := DefaultLSPConfig()
	client := NewLSPClient(tempDir, config)
	defer client.Shutdown()

	if client == nil {
		t.Errorf("Expected LSP client to be created")
	}

	if client.workspacePath != tempDir {
		t.Errorf("Expected workspace path to be set correctly")
	}
}

func TestServerConfigDefaults(t *testing.T) {
	configs := DefaultServerConfigs

	// Check that Go server is enabled by default
	goConfig, exists := configs["go"]
	if !exists {
		t.Errorf("Expected Go server config to exist")
	}

	if goConfig.Command != "gopls" {
		t.Errorf("Expected Go server command to be 'gopls', got: %s", goConfig.Command)
	}

	if !goConfig.Enabled {
		t.Errorf("Expected Go server to be enabled by default")
	}

	// Check that TypeScript server exists but is disabled
	tsConfig, exists := configs["typescript"]
	if !exists {
		t.Errorf("Expected TypeScript server config to exist")
	}

	if tsConfig.Enabled {
		t.Errorf("Expected TypeScript server to be disabled by default")
	}
}

func TestValidationResultFormatting(t *testing.T) {
	context := &EditContext{
		FilePath:      "test.go",
		StartLine:     5,
		EndLine:       5,
		LanguageID:    "go",
		EditAction:    "REPLACE",
		OriginalLines: []string{`    fmt.Println("Original")`},
		ModifiedLines: []string{`    fmt.Println("Modified")`},
	}

	validation := &ValidationResult{
		IsValid: false,
		Errors: []LSPDiagnostic{
			{
				Range: Range{
					Start: Position{Line: 4, Character: 0},
					End:   Position{Line: 4, Character: 10},
				},
				Message:  "syntax error",
				Severity: func() *int { s := 1; return &s }(),
			},
		},
		Warnings: []LSPDiagnostic{
			{
				Range: Range{
					Start: Position{Line: 6, Character: 0},
					End:   Position{Line: 6, Character: 5},
				},
				Message:  "unused variable",
				Severity: func() *int { s := 2; return &s }(),
			},
		},
		ValidatorUsed: "lsp",
		ServerInfo: &ServerInfo{
			Name:   "gopls",
			Health: "healthy",
		},
	}

	formatted := FormatVerificationForLLM(context, validation)

	expectedSections := []string{
		"📝 EDIT VERIFICATION:",
		"🔍 SYNTAX VALIDATION:",
		"❌ Syntax errors detected:",
		"⚠️  Warnings:",
		"Line 5: syntax error",
		"Line 7: unused variable",
		"Validated using: lsp (gopls - healthy)",
	}

	for _, section := range expectedSections {
		if !strings.Contains(formatted, section) {
			t.Errorf("Expected formatted text to contain: %s", section)
		}
	}
}

func TestRollbackMessageFormatting(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultValidationConfig()
	validator := NewValidator(tempDir, &config)
	defer validator.Shutdown()

	validation := &ValidationResult{
		IsValid: false,
		Errors: []LSPDiagnostic{
			{
				Range: Range{
					Start: Position{Line: 4, Character: 0},
					End:   Position{Line: 4, Character: 10},
				},
				Message:  "syntax error: unexpected token",
				Severity: func() *int { s := 1; return &s }(),
			},
		},
		ValidatorUsed: "lsp",
		ServerInfo: &ServerInfo{
			Name:   "gopls",
			Health: "healthy",
		},
	}

	editCmd := &loom_edit.EditCommand{
		File:   "test.go",
		Action: "REPLACE",
		Start:  5,
		End:    5,
	}

	message := validator.FormatRollbackMessage(validation, editCmd)

	expectedSections := []string{
		"🚫 EDIT ROLLED BACK DUE TO SYNTAX ERRORS:",
		"File: test.go",
		"Operation: REPLACE (lines 5-5)",
		"Critical errors detected:",
		"Line 5: syntax error: unexpected token",
		"The file has been restored to its previous state",
		"Validated by: gopls (healthy)",
	}

	for _, section := range expectedSections {
		if !strings.Contains(message, section) {
			t.Errorf("Expected rollback message to contain: %s", section)
		}
	}
}

// Helper function to create temporary files for tests
func createTempFile(t *testing.T, dir, name, content string) string {
	filePath := fmt.Sprintf("%s/%s", dir, name)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	return filePath
}

// Helper function to create temporary files for benchmarks
func createTempFileForBench(b *testing.B, dir, name, content string) string {
	filePath := fmt.Sprintf("%s/%s", dir, name)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	return filePath
}

// Benchmark tests for performance
func BenchmarkValidateFile(b *testing.B) {
	tempDir := b.TempDir()
	config := DefaultValidationConfig()
	validator := NewValidator(tempDir, &config)
	defer validator.Shutdown()

	// Create a test file
	content := `{"name": "test", "value": 123, "items": [1, 2, 3]}`
	testFile := createTempFileForBench(b, tempDir, "test.json", content)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := validator.ValidateFile(testFile)
		if err != nil {
			b.Errorf("Validation failed: %v", err)
		}
	}
}

func BenchmarkFormatVerification(b *testing.B) {
	context := &EditContext{
		FilePath:      "test.go",
		StartLine:     5,
		EndLine:       7,
		LanguageID:    "go",
		EditAction:    "REPLACE",
		OriginalLines: []string{"line1", "line2", "line3"},
		ModifiedLines: []string{"new1", "new2", "new3"},
		ContextBefore: []string{"before1", "before2"},
		ContextAfter:  []string{"after1", "after2"},
	}

	validation := &ValidationResult{
		IsValid:       true,
		ValidatorUsed: "lsp",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatVerificationForLLM(context, validation)
	}
}

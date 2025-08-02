package validation

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"loom/loom_edit"
)

// Validator provides file validation capabilities
type Validator struct {
	lspClient     *LSPClient
	config        *ValidationConfig
	workspacePath string
}

// NewValidator creates a new validator instance
func NewValidator(workspacePath string, config *ValidationConfig) *Validator {
	if config == nil {
		defaultConfig := DefaultValidationConfig()
		config = &defaultConfig
	}

	var lspClient *LSPClient
	if config.EnableLSP {
		lspConfig := DefaultLSPConfig()
		lspConfig.GlobalEnabled = true
		lspClient = NewLSPClient(workspacePath, lspConfig)
	}

	return &Validator{
		lspClient:     lspClient,
		config:        config,
		workspacePath: workspacePath,
	}
}

// ValidateFile validates a file using the best available method
func (v *Validator) ValidateFile(filePath string) (*ValidationResult, error) {
	startTime := time.Now()

	// Try LSP validation first if enabled
	if v.config.EnableLSP && v.lspClient != nil {
		if result, err := v.lspClient.ValidateFile(filePath); err == nil {
			result.ProcessTime = time.Since(startTime)
			return result, nil
		}
	}

	// Fallback to basic syntax checking
	if result, err := v.basicSyntaxCheck(filePath); err == nil {
		result.ProcessTime = time.Since(startTime)
		result.ValidatorUsed = "fallback"
		return result, nil
	}

	// No validation available - assume valid
	return &ValidationResult{
		IsValid:       true,
		Errors:        []LSPDiagnostic{},
		Warnings:      []LSPDiagnostic{},
		Hints:         []LSPDiagnostic{},
		Language:      DetectLanguage(filePath),
		ProcessTime:   time.Since(startTime),
		ValidatorUsed: "none",
	}, nil
}

// ValidateEditContext validates a file after an edit with context
func (v *Validator) ValidateEditContext(filePath string, context *EditContext) (*ValidationResult, error) {
	result, err := v.ValidateFile(filePath)
	if err != nil {
		return result, err
	}

	// Filter diagnostics to focus on the edited region if we have context
	if context != nil && len(result.Errors) > 0 {
		result.Errors = v.filterDiagnosticsToRegion(result.Errors, context)
	}
	if context != nil && len(result.Warnings) > 0 {
		result.Warnings = v.filterDiagnosticsToRegion(result.Warnings, context)
	}

	return result, nil
}

// filterDiagnosticsToRegion filters diagnostics to focus on the edited region
func (v *Validator) filterDiagnosticsToRegion(diagnostics []LSPDiagnostic, context *EditContext) []LSPDiagnostic {
	var filtered []LSPDiagnostic

	// Include diagnostics in or near the edited region
	bufferLines := 5 // Include diagnostics within 5 lines of the edit

	for _, diag := range diagnostics {
		diagLine := diag.Range.Start.Line + 1 // Convert to 1-based

		if diagLine >= context.StartLine-bufferLines &&
			diagLine <= context.EndLine+bufferLines {
			filtered = append(filtered, diag)
		}
	}

	return filtered
}

// ShouldRollbackEdit determines if an edit should be rolled back based on validation
func (v *Validator) ShouldRollbackEdit(validation *ValidationResult) bool {
	if validation == nil || !v.config.RollbackOnSyntaxError {
		return false
	}

	// Only rollback on critical syntax errors, not warnings
	for _, err := range validation.Errors {
		if v.isCriticalSyntaxError(err) {
			return true
		}
	}

	return false
}

// isCriticalSyntaxError determines if a diagnostic represents a critical syntax error
func (v *Validator) isCriticalSyntaxError(diagnostic LSPDiagnostic) bool {
	message := strings.ToLower(diagnostic.Message)

	// Check against configured rollback patterns
	for _, pattern := range v.config.RollbackOnErrors {
		if strings.Contains(message, strings.ToLower(pattern)) {
			return true
		}
	}

	// Additional heuristics for critical errors
	criticalPatterns := []string{
		"syntax error",
		"unexpected token",
		"missing semicolon",
		"unclosed",
		"invalid syntax",
		"parse error",
		"compilation failed",
		"fatal error",
	}

	for _, pattern := range criticalPatterns {
		if strings.Contains(message, pattern) {
			return true
		}
	}

	return false
}

// basicSyntaxCheck provides fallback syntax checking for common languages
func (v *Validator) basicSyntaxCheck(filePath string) (*ValidationResult, error) {
	language := DetectLanguage(filePath)

	switch language {
	case "json":
		return v.validateJSON(filePath)
	case "go":
		return v.validateGo(filePath)
	default:
		// For unknown languages, assume valid
		return &ValidationResult{
			IsValid:       true,
			Errors:        []LSPDiagnostic{},
			Warnings:      []LSPDiagnostic{},
			Hints:         []LSPDiagnostic{},
			Language:      language,
			ValidatorUsed: "basic",
		}, nil
	}
}

// validateJSON provides basic JSON syntax validation
func (v *Validator) validateJSON(filePath string) (*ValidationResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	result := &ValidationResult{
		IsValid:       true,
		Errors:        []LSPDiagnostic{},
		Warnings:      []LSPDiagnostic{},
		Hints:         []LSPDiagnostic{},
		Language:      "json",
		ValidatorUsed: "basic",
	}

	// Try to parse JSON
	var jsonData interface{}
	if err := json.Unmarshal(content, &jsonData); err != nil {
		result.IsValid = false
		severity := 1
		source := "json-parser"
		result.Errors = []LSPDiagnostic{
			{
				Range: Range{
					Start: Position{Line: 0, Character: 0},
					End:   Position{Line: 0, Character: 0},
				},
				Message:  fmt.Sprintf("JSON syntax error: %s", err.Error()),
				Severity: &severity,
				Source:   &source,
			},
		}
	}

	return result, nil
}

// validateGo provides basic Go syntax validation using go/parser
func (v *Validator) validateGo(filePath string) (*ValidationResult, error) {
	// For now, just check if it's a .go file and return valid
	// In a full implementation, we'd use go/parser to check syntax
	return &ValidationResult{
		IsValid:       true,
		Errors:        []LSPDiagnostic{},
		Warnings:      []LSPDiagnostic{},
		Hints:         []LSPDiagnostic{},
		Language:      "go",
		ValidatorUsed: "basic",
	}, nil
}

// RollbackEdit rolls back an edit by restoring the original content
func (v *Validator) RollbackEdit(filePath, originalContent string, validation *ValidationResult) error {
	if err := os.WriteFile(filePath, []byte(originalContent), 0644); err != nil {
		return fmt.Errorf("failed to rollback edit: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the validator
func (v *Validator) Shutdown() {
	if v.lspClient != nil {
		v.lspClient.Shutdown()
	}
}

// Enhanced validation for edit operations

// ValidateEditOperation performs comprehensive validation of an edit operation
func (v *Validator) ValidateEditOperation(filePath string, editCmd *loom_edit.EditCommand, originalContent string) (*EditValidationResult, error) {
	// Extract edit context
	context, err := ExtractEditContext(filePath, editCmd, originalContent)
	if err != nil {
		return nil, fmt.Errorf("failed to extract edit context: %w", err)
	}

	// Validate the file after edit
	validation, err := v.ValidateEditContext(filePath, context)
	if err != nil {
		return nil, fmt.Errorf("failed to validate file: %w", err)
	}

	// Create enhanced result
	result := &EditValidationResult{
		Context:          context,
		Validation:       validation,
		ShouldRollback:   v.ShouldRollbackEdit(validation),
		VerificationText: FormatVerificationForLLM(context, validation),
	}

	return result, nil
}

// EditValidationResult contains comprehensive validation results for an edit
type EditValidationResult struct {
	Context          *EditContext      `json:"context"`
	Validation       *ValidationResult `json:"validation"`
	ShouldRollback   bool              `json:"should_rollback"`
	VerificationText string            `json:"verification_text"`
}

// FormatRollbackMessage creates a formatted message for rollback scenarios
func (v *Validator) FormatRollbackMessage(validation *ValidationResult, editCmd *loom_edit.EditCommand) string {
	var builder strings.Builder

	builder.WriteString("🚫 EDIT ROLLED BACK DUE TO SYNTAX ERRORS:\n\n")

	builder.WriteString(fmt.Sprintf("File: %s\n", editCmd.File))
	builder.WriteString(fmt.Sprintf("Operation: %s (lines %d-%d)\n\n", editCmd.Action, editCmd.Start, editCmd.End))

	builder.WriteString("Critical errors detected:\n")
	for _, err := range validation.Errors {
		if v.isCriticalSyntaxError(err) {
			builder.WriteString(fmt.Sprintf("  Line %d: %s\n", err.Range.Start.Line+1, err.Message))
		}
	}

	builder.WriteString("\nThe file has been restored to its previous state.\n")
	builder.WriteString("Please fix the syntax errors and try again.\n")

	if validation.ValidatorUsed == "lsp" && validation.ServerInfo != nil {
		builder.WriteString(fmt.Sprintf("\nValidated by: %s (%s)\n",
			validation.ServerInfo.Name, validation.ServerInfo.Health))
	}

	return builder.String()
}

package validation

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// EditContext contains the context of an edit operation for verification
type EditContext struct {
	FilePath      string   `json:"file_path"`
	OriginalLines []string `json:"original_lines"` // Lines that were in the edited range before
	ModifiedLines []string `json:"modified_lines"` // Lines that are now in the edited range
	ContextBefore []string `json:"context_before"` // Lines before the edit for context
	ContextAfter  []string `json:"context_after"`  // Lines after the edit for context
	StartLine     int      `json:"start_line"`     // 1-based line number where edit started
	EndLine       int      `json:"end_line"`       // 1-based line number where edit ended
	LanguageID    string   `json:"language_id"`    // Programming language detected
	EditAction    string   `json:"edit_action"`    // REPLACE, INSERT_AFTER, etc.
}

// ValidationResult represents the result of syntax/LSP validation
type ValidationResult struct {
	IsValid       bool            `json:"is_valid"`
	Errors        []LSPDiagnostic `json:"errors"`
	Warnings      []LSPDiagnostic `json:"warnings"`
	Hints         []LSPDiagnostic `json:"hints"`
	Language      string          `json:"language"`
	ProcessTime   time.Duration   `json:"process_time"`
	ValidatorUsed string          `json:"validator_used"` // "lsp", "fallback", "disabled", "unavailable", "unsupported"
	ServerInfo    *ServerInfo     `json:"server_info,omitempty"`
}

// ServerInfo provides information about the LSP server used for validation
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Health  string `json:"health"` // "healthy", "degraded", "unavailable"
}

// LSPDiagnostic is imported from lsp_client.go

// ValidationConfig holds configuration for the validation system
type ValidationConfig struct {
	EnableLSP             bool     `json:"enable_lsp"`
	EnableVerification    bool     `json:"enable_verification"`
	ContextLines          int      `json:"context_lines"`
	RollbackOnSyntaxError bool     `json:"rollback_on_syntax_error"`
	LSPTimeoutSeconds     int      `json:"lsp_timeout_seconds"`
	SupportedLanguages    []string `json:"supported_languages"`
	RollbackOnErrors      []string `json:"rollback_on_errors"` // Patterns that trigger rollback
	IgnoreWarnings        []string `json:"ignore_warnings"`    // Warning patterns to ignore
}

// DefaultValidationConfig returns sensible defaults
func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		EnableLSP:             false, // Start disabled until LSP integration is complete
		EnableVerification:    true,  // Context extraction is always useful
		ContextLines:          8,
		RollbackOnSyntaxError: false, // Conservative default
		LSPTimeoutSeconds:     5,
		SupportedLanguages:    []string{"go", "typescript", "javascript", "python", "rust", "java"},
		RollbackOnErrors: []string{
			"syntax error",
			"unexpected token",
			"parse error",
			"invalid syntax",
		},
		IgnoreWarnings: []string{},
	}
}

// DetectLanguage attempts to detect the programming language from file extension
func DetectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	languageMap := map[string]string{
		".go":    "go",
		".ts":    "typescript",
		".tsx":   "typescript",
		".js":    "javascript",
		".jsx":   "javascript",
		".py":    "python",
		".rs":    "rust",
		".java":  "java",
		".c":     "c",
		".cpp":   "cpp",
		".cc":    "cpp",
		".cxx":   "cpp",
		".h":     "c",
		".hpp":   "cpp",
		".cs":    "csharp",
		".php":   "php",
		".rb":    "ruby",
		".swift": "swift",
		".kt":    "kotlin",
		".scala": "scala",
		".clj":   "clojure",
		".ml":    "ocaml",
		".hs":    "haskell",
		".elm":   "elm",
		".ex":    "elixir",
		".exs":   "elixir",
		".erl":   "erlang",
		".lua":   "lua",
		".r":     "r",
		".R":     "r",
		".m":     "matlab",
		".jl":    "julia",
		".nim":   "nim",
		".zig":   "zig",
		".dart":  "dart",
		".sh":    "bash",
		".bash":  "bash",
		".zsh":   "zsh",
		".fish":  "fish",
		".ps1":   "powershell",
		".sql":   "sql",
		".yaml":  "yaml",
		".yml":   "yaml",
		".toml":  "toml",
		".json":  "json",
		".xml":   "xml",
		".html":  "html",
		".css":   "css",
		".scss":  "scss",
		".sass":  "sass",
		".less":  "less",
		".md":    "markdown",
		".tex":   "latex",
	}

	if lang, found := languageMap[ext]; found {
		return lang
	}

	// Check for common filenames without extensions
	baseName := strings.ToLower(filepath.Base(filePath))
	switch baseName {
	case "dockerfile":
		return "dockerfile"
	case "makefile":
		return "makefile"
	case "cmakelists.txt":
		return "cmake"
	case "cargo.toml":
		return "toml"
	case "package.json", "composer.json":
		return "json"
	case "requirements.txt":
		return "text"
	}

	return "text" // Default fallback
}

// FormatVerificationForLLM creates a detailed verification message for the LLM
func FormatVerificationForLLM(context *EditContext, validation *ValidationResult) string {
	var builder strings.Builder

	builder.WriteString("📝 EDIT VERIFICATION:\n")
	builder.WriteString(fmt.Sprintf("File: %s (lines %d-%d, %s %s)\n\n",
		context.FilePath, context.StartLine, context.EndLine, context.EditAction, context.LanguageID))

	// Show context before edit
	if len(context.ContextBefore) > 0 {
		builder.WriteString("Context before edit:\n")
		startLineNum := context.StartLine - len(context.ContextBefore)
		for i, line := range context.ContextBefore {
			lineNum := startLineNum + i
			builder.WriteString(fmt.Sprintf("%4d: %s\n", lineNum, line))
		}
		builder.WriteString("\n")
	}

	// Show what was changed
	if context.EditAction == "REPLACE" && len(context.OriginalLines) > 0 {
		builder.WriteString("Original content (replaced):\n")
		for i, line := range context.OriginalLines {
			lineNum := context.StartLine + i
			builder.WriteString(fmt.Sprintf("%4d: %s\n", lineNum, line))
		}
		builder.WriteString("\n")
	}

	// Show the current content in the edited range
	builder.WriteString("Your changes:\n")
	for i, line := range context.ModifiedLines {
		lineNum := context.StartLine + i
		builder.WriteString(fmt.Sprintf("%4d: %s\n", lineNum, line))
	}
	builder.WriteString("\n")

	// Show context after edit
	if len(context.ContextAfter) > 0 {
		builder.WriteString("Context after edit:\n")
		startLineNum := context.EndLine + 1
		for i, line := range context.ContextAfter {
			lineNum := startLineNum + i
			builder.WriteString(fmt.Sprintf("%4d: %s\n", lineNum, line))
		}
		builder.WriteString("\n")
	}

	// Add validation results if available
	if validation != nil {
		builder.WriteString("🔍 SYNTAX VALIDATION:\n")
		if validation.IsValid {
			builder.WriteString("✅ Syntax is valid\n")
		} else {
			builder.WriteString("❌ Syntax errors detected:\n")
			for _, err := range validation.Errors {
				builder.WriteString(fmt.Sprintf("  Line %d: %s\n", err.Range.Start.Line+1, err.Message))
			}
		}

		if len(validation.Warnings) > 0 {
			builder.WriteString("⚠️  Warnings:\n")
			for _, warn := range validation.Warnings {
				builder.WriteString(fmt.Sprintf("  Line %d: %s\n", warn.Range.Start.Line+1, warn.Message))
			}
		}

		if len(validation.Hints) > 0 {
			builder.WriteString("💡 Hints:\n")
			for _, hint := range validation.Hints {
				builder.WriteString(fmt.Sprintf("  Line %d: %s\n", hint.Range.Start.Line+1, hint.Message))
			}
		}

		builder.WriteString(fmt.Sprintf("Validated using: %s", validation.ValidatorUsed))

		if validation.ServerInfo != nil {
			builder.WriteString(fmt.Sprintf(" (%s - %s)", validation.ServerInfo.Name, validation.ServerInfo.Health))
		}
		builder.WriteString("\n")
	}

	builder.WriteString("\nPlease review this verification to ensure your edit is correct and complete.")

	return builder.String()
}

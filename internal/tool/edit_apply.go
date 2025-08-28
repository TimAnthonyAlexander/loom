package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/loom/loom/internal/editor"
)

// ApplyEditArgs represents the arguments for applying an edit that was previously approved.
type ApplyEditArgs struct {
	Path      string `json:"path"`
	Action    string `json:"action"`
	Content   string `json:"content,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Line      int    `json:"line,omitempty"`
	OldString string `json:"old_string,omitempty"`
	NewString string `json:"new_string,omitempty"`
	// Anchored replace parameters (for ANCHOR_REPLACE)
	AnchorBefore        string  `json:"anchor_before,omitempty"`
	Target              string  `json:"target,omitempty"`
	AnchorAfter         string  `json:"anchor_after,omitempty"`
	NormalizeWhitespace bool    `json:"normalize_whitespace,omitempty"`
	FuzzyThreshold      float64 `json:"fuzzy_threshold,omitempty"`
	Occurrence          int     `json:"occurrence,omitempty"`        // backward compatibility
	OccurrenceBefore    int     `json:"occurrence_before,omitempty"` // independent control for anchor_before
	OccurrenceAfter     int     `json:"occurrence_after,omitempty"`  // independent control for anchor_after
}

// RegisterApplyEdit registers the apply_edit tool with the registry.
// This is an internal tool that will be called after approval.
func RegisterApplyEdit(registry *Registry, workspacePath string) error {
	return registry.Register(Definition{
		Name:        "apply_edit",
		Description: "Apply a previously approved file edit using advanced actions, including ANCHOR_REPLACE",
		Safe:        true, // This is safe since it's only called after explicit approval
		JSONSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file, relative to the workspace root",
				},
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Action to perform",
					"enum":        []string{"CREATE", "REPLACE", "INSERT_AFTER", "INSERT_BEFORE", "DELETE", "SEARCH_REPLACE", "ANCHOR_REPLACE"},
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content used for CREATE/REPLACE/INSERT actions",
				},
				"start_line": map[string]interface{}{
					"type":        "integer",
					"description": "Start line (1-indexed) for REPLACE/DELETE",
				},
				"end_line": map[string]interface{}{
					"type":        "integer",
					"description": "End line (1-indexed, inclusive) for REPLACE/DELETE",
				},
				"line": map[string]interface{}{
					"type":        "integer",
					"description": "Line (1-indexed) for INSERT_BEFORE/INSERT_AFTER",
				},
				"old_string": map[string]interface{}{
					"type":        "string",
					"description": "String to search for during SEARCH_REPLACE",
				},
				"new_string": map[string]interface{}{
					"type":        "string",
					"description": "Replacement string for SEARCH_REPLACE",
				},
				// Anchored replace fields
				"anchor_before": map[string]interface{}{
					"type":        "string",
					"description": "Text immediately before the region to replace (not included)",
				},
				"target": map[string]interface{}{
					"type":        "string",
					"description": "Existing block to be replaced. Optional when replacing the span between anchors.",
				},
				"anchor_after": map[string]interface{}{
					"type":        "string",
					"description": "Text immediately after the region to replace (not included)",
				},
				"normalize_whitespace": map[string]interface{}{
					"type":        "boolean",
					"description": "Normalize whitespace during matching (collapse spaces, ignore tabs vs spaces)",
				},
				"fuzzy_threshold": map[string]interface{}{
					"type":        "number",
					"description": "0..1: higher prefers stricter match when using fuzzy search fallback",
				},
				"occurrence": map[string]interface{}{
					"type":        "integer",
					"description": "1-based occurrence of the anchors to use (default 1, for backward compatibility)",
				},
				"occurrence_before": map[string]interface{}{
					"type":        "integer",
					"description": "1-based occurrence of anchor_before to use (default 1). Overrides 'occurrence' for anchor_before.",
				},
				"occurrence_after": map[string]interface{}{
					"type":        "integer",
					"description": "1-based occurrence of anchor_after to use (default 1). Overrides 'occurrence' for anchor_after. Searched relative to anchor_before position.",
				},
			},
			"required": []string{"path", "action"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args ApplyEditArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("failed to parse arguments: %w", err)
			}

			return applyEdit(ctx, workspacePath, args)
		},
	})
}

// applyEdit applies a file edit that has been approved.
func applyEdit(ctx context.Context, workspacePath string, args ApplyEditArgs) (*ExecutionResult, error) {
	// First recreate the edit plan (this also validates the edit again)
	plan, err := editor.ProposeAdvancedEdit(workspacePath, editor.AdvancedEditRequest{
		FilePath:            args.Path,
		Action:              editor.ActionType(args.Action),
		Content:             args.Content,
		StartLine:           args.StartLine,
		EndLine:             args.EndLine,
		Line:                args.Line,
		OldString:           args.OldString,
		NewString:           args.NewString,
		AnchorBefore:        args.AnchorBefore,
		Target:              args.Target,
		AnchorAfter:         args.AnchorAfter,
		NormalizeWhitespace: args.NormalizeWhitespace,
		FuzzyThreshold:      args.FuzzyThreshold,
		Occurrence:          args.Occurrence,
		OccurrenceBefore:    args.OccurrenceBefore,
		OccurrenceAfter:     args.OccurrenceAfter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to recreate edit plan: %w", err)
	}

	// Store the original content before applying for verification
	originalContent := plan.OldContent

	// Apply the edit
	if err := editor.ApplyEdit(plan); err != nil {
		return nil, fmt.Errorf("failed to apply edit: %w", err)
	}

	// Read the actual file content after applying to verify what was written
	actualContent, err := readFileForVerification(plan.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file for verification: %w", err)
	}

	// Generate verification diff to show what actually changed
	verificationDiff, err := generateVerificationDiff(originalContent, actualContent, args.Path, plan.ChangedLines)
	if err != nil {
		// If diff generation fails, still return success but note the issue
		verificationDiff = fmt.Sprintf("Edit applied successfully, but couldn't generate verification diff: %v", err)
	}

	// Create a message describing what happened
	var message string
	switch editor.ActionType(args.Action) {
	case editor.ActionCreate:
		message = fmt.Sprintf("✅ Created file: %s", args.Path)
	case editor.ActionDeleteLines:
		message = fmt.Sprintf("✅ Edited file (DELETE lines %d-%d): %s", args.StartLine, args.EndLine, args.Path)
	case editor.ActionReplaceLines:
		message = fmt.Sprintf("✅ Edited file (REPLACE lines %d-%d): %s", args.StartLine, args.EndLine, args.Path)
	case editor.ActionInsertBefore:
		message = fmt.Sprintf("✅ Edited file (INSERT_BEFORE line %d): %s", args.Line, args.Path)
	case editor.ActionInsertAfter:
		message = fmt.Sprintf("✅ Edited file (INSERT_AFTER line %d): %s", args.Line, args.Path)
	case editor.ActionSearchReplace:
		message = fmt.Sprintf("✅ Edited file (SEARCH_REPLACE): %s", args.Path)
	case editor.ActionAnchorReplace:
		message = fmt.Sprintf("✅ Edited file (ANCHOR_REPLACE): %s", args.Path)
	default:
		message = fmt.Sprintf("✅ Edited file: %s", args.Path)
	}

	// Include verification in the message
	message += "\n\n" + verificationDiff

	return &ExecutionResult{
		Content: message,
		Diff:    verificationDiff,
		Safe:    true,
	}, nil
}

// readFileForVerification reads the file content after an edit for verification purposes.
func readFileForVerification(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// generateVerificationDiff creates a focused diff showing the changes with context.
func generateVerificationDiff(originalContent, actualContent, filePath string, changedLines editor.LineRange) (string, error) {
	// For file creation, show a preview of the created content
	if originalContent == "" {
		lines := strings.Split(actualContent, "\n")
		preview := "📄 **File Created Successfully**\n\nContent preview:\n"

		// Show first 10 lines or all lines if fewer
		maxLines := 10
		if len(lines) <= maxLines {
			for i, line := range lines {
				preview += fmt.Sprintf("%4d: %s\n", i+1, line)
			}
		} else {
			for i := 0; i < maxLines-1; i++ {
				preview += fmt.Sprintf("%4d: %s\n", i+1, lines[i])
			}
			preview += fmt.Sprintf("... (%d more lines)\n", len(lines)-maxLines+1)
		}
		return preview, nil
	}

	// Generate a contextual diff focusing on the changed region
	originalLines := strings.Split(originalContent, "\n")
	actualLines := strings.Split(actualContent, "\n")

	// Calculate context window around changes
	contextBefore := 3
	contextAfter := 3

	startLine := changedLines.StartLine - contextBefore
	if startLine < 1 {
		startLine = 1
	}

	endLine := changedLines.EndLine + contextAfter
	if endLine > len(actualLines) {
		endLine = len(actualLines)
	}

	// Build verification result showing before/after with context
	result := "🔍 **Edit Verification - Changes Applied Successfully**\n\n"
	result += fmt.Sprintf("File: %s\n", filepath.Base(filePath))
	result += fmt.Sprintf("Changed lines: %d-%d\n", changedLines.StartLine, changedLines.EndLine)
	result += "Context:\n\n"

	// Show the edited region with context
	for i := startLine - 1; i < endLine && i < len(actualLines); i++ {
		lineNum := i + 1
		line := actualLines[i]

		// Mark changed lines
		if lineNum >= changedLines.StartLine && lineNum <= changedLines.EndLine {
			result += fmt.Sprintf("→ %4d: %s\n", lineNum, line)
		} else {
			result += fmt.Sprintf("  %4d: %s\n", lineNum, line)
		}
	}

	// Add summary
	totalOriginalLines := len(originalLines)
	totalActualLines := len(actualLines)
	linesDiff := totalActualLines - totalOriginalLines

	result += "\n📊 **Summary:**\n"
	if linesDiff > 0 {
		result += fmt.Sprintf("- Added %d line(s)\n", linesDiff)
	} else if linesDiff < 0 {
		result += fmt.Sprintf("- Removed %d line(s)\n", -linesDiff)
	} else {
		result += "- Modified content (same line count)\n"
	}
	result += fmt.Sprintf("- Total lines: %d → %d\n", totalOriginalLines, totalActualLines)

	return result, nil
}

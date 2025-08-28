package tool

import (
	"context"
	"encoding/json"
	"fmt"

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

	// Apply the edit
	if err := editor.ApplyEdit(plan); err != nil {
		return nil, fmt.Errorf("failed to apply edit: %w", err)
	}

	// Create a message describing what happened
	var message string
	switch editor.ActionType(args.Action) {
	case editor.ActionCreate:
		message = fmt.Sprintf("Created file: %s", args.Path)
	case editor.ActionDeleteLines:
		message = fmt.Sprintf("Edited file (DELETE lines %d-%d): %s", args.StartLine, args.EndLine, args.Path)
	case editor.ActionReplaceLines:
		message = fmt.Sprintf("Edited file (REPLACE lines %d-%d): %s", args.StartLine, args.EndLine, args.Path)
	case editor.ActionInsertBefore:
		message = fmt.Sprintf("Edited file (INSERT_BEFORE line %d): %s", args.Line, args.Path)
	case editor.ActionInsertAfter:
		message = fmt.Sprintf("Edited file (INSERT_AFTER line %d): %s", args.Line, args.Path)
	case editor.ActionSearchReplace:
		message = fmt.Sprintf("Edited file (SEARCH_REPLACE): %s", args.Path)
	case editor.ActionAnchorReplace:
		message = fmt.Sprintf("Edited file (ANCHOR_REPLACE): %s", args.Path)
	default:
		message = fmt.Sprintf("Edited file: %s", args.Path)
	}

	return &ExecutionResult{
		Content: message,
		Safe:    true,
	}, nil
}

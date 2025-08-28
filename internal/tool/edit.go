package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/loom/loom/internal/editor"
)

// EditFileArgs represents the arguments for the edit_file tool (advanced actions).
type EditFileArgs struct {
    Path   string `json:"path"`
    Action string `json:"action"`
    // Content for create/replace/insert operations
    Content string `json:"content,omitempty"`
    // Line-based parameters (1-indexed)
    StartLine int `json:"start_line,omitempty"`
    EndLine   int `json:"end_line,omitempty"`
    Line      int `json:"line,omitempty"`
    // Search/replace parameters
    OldString string `json:"old_string,omitempty"`
    NewString string `json:"new_string,omitempty"`
    // Anchored replace parameters (for ANCHOR_REPLACE)
    AnchorBefore        string  `json:"anchor_before,omitempty"`
    Target              string  `json:"target,omitempty"`
    AnchorAfter         string  `json:"anchor_after,omitempty"`
    NormalizeWhitespace bool    `json:"normalize_whitespace,omitempty"`
    FuzzyThreshold      float64 `json:"fuzzy_threshold,omitempty"`
    Occurrence          int     `json:"occurrence,omitempty"`
}

// EditFileResult represents the result of the edit_file tool.
type EditFileResult struct {
	Path    string `json:"path"`
	Success bool   `json:"success"`
	Diff    string `json:"diff"`
	Message string `json:"message,omitempty"`
}

// RegisterEditFile registers the edit_file tool with the registry.
func RegisterEditFile(registry *Registry, workspacePath string) error {
	return registry.Register(Definition{
		Name:        "edit_file",
        Description: "Edit a file with actions: CREATE, REPLACE (line range), INSERT_BEFORE/INSERT_AFTER (line), DELETE (line range), SEARCH_REPLACE, or ANCHOR_REPLACE (content-anchored). Prefer ANCHOR_REPLACE over line numbers when possible.",
		Safe:        false, // Editing files requires approval
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
                    "description": "1-based occurrence of the anchors to use (default 1)",
                },
            },
            // We cannot express conditional requirements here; runtime will validate
            "required": []string{"path", "action"},
        },
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args EditFileArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("failed to parse arguments: %w", err)
			}

			return editFile(ctx, workspacePath, args)
		},
	})
}

// editFile implements the file editing logic.
func editFile(ctx context.Context, workspacePath string, args EditFileArgs) (*ExecutionResult, error) {
	// Map args to advanced request
    adv := editor.AdvancedEditRequest{
        FilePath:  args.Path,
        Action:    editor.ActionType(args.Action),
        Content:   args.Content,
        StartLine: args.StartLine,
        EndLine:   args.EndLine,
        Line:      args.Line,
        OldString: args.OldString,
        NewString: args.NewString,
        AnchorBefore:        args.AnchorBefore,
        Target:              args.Target,
        AnchorAfter:         args.AnchorAfter,
        NormalizeWhitespace: args.NormalizeWhitespace,
        FuzzyThreshold:      args.FuzzyThreshold,
        Occurrence:          args.Occurrence,
    }

	// Create an edit plan first
	plan, err := editor.ProposeAdvancedEdit(workspacePath, adv)
	if err != nil {
		return nil, fmt.Errorf("failed to create edit plan: %w", err)
	}

	// Perform additional safety checks
	if err := editor.ValidateEditSafety(plan); err != nil {
		return nil, fmt.Errorf("safety validation failed: %w", err)
	}

	// Generate a better diff using git if available
	diff, err := editor.GenerateGitDiff(plan.OldContent, plan.NewContent, plan.FilePath)
	if err != nil {
		// Fallback to the basic diff if git diff fails
		diff = plan.Diff
	} else {
		// Update the plan with the better diff
		plan.Diff = diff
	}

	// Create a descriptive message based on edit type
	var message string
    switch editor.ActionType(args.Action) {
    case editor.ActionCreate:
        message = fmt.Sprintf("File will be created: %s", args.Path)
    case editor.ActionDeleteLines:
        message = fmt.Sprintf("File will be edited (DELETE lines %d-%d): %s", args.StartLine, args.EndLine, args.Path)
    case editor.ActionReplaceLines:
        message = fmt.Sprintf("File will be edited (REPLACE lines %d-%d): %s", args.StartLine, args.EndLine, args.Path)
    case editor.ActionInsertBefore:
        message = fmt.Sprintf("File will be edited (INSERT_BEFORE line %d): %s", args.Line, args.Path)
    case editor.ActionInsertAfter:
        message = fmt.Sprintf("File will be edited (INSERT_AFTER line %d): %s", args.Line, args.Path)
    case editor.ActionSearchReplace:
        message = fmt.Sprintf("File will be edited (SEARCH_REPLACE): %s", args.Path)
    case editor.ActionAnchorReplace:
        message = fmt.Sprintf("File will be edited (ANCHOR_REPLACE): %s", args.Path)
    default:
        message = fmt.Sprintf("File will be edited: %s", args.Path)
    }

	// Create a result that fits the ExecutionResult interface
	result := &ExecutionResult{
		Content: message,
		Diff:    diff,
		Safe:    false, // Always require approval for edits
	}

	return result, nil
}

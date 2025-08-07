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
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
	CreateNew bool   `json:"create_new,omitempty"`
}

// RegisterApplyEdit registers the apply_edit tool with the registry.
// This is an internal tool that will be called after approval.
func RegisterApplyEdit(registry *Registry, workspacePath string) error {
	return registry.Register(Definition{
		Name:        "apply_edit",
		Description: "Apply a previously approved file edit",
		Safe:        true, // This is safe since it's only called after explicit approval
		JSONSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file, relative to the workspace root",
				},
				"old_string": map[string]interface{}{
					"type":        "string",
					"description": "The text to replace (must be present in the file unless creating a new file)",
				},
				"new_string": map[string]interface{}{
					"type":        "string",
					"description": "The new text to insert",
				},
				"create_new": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, creates a new file (old_string should be empty in this case)",
				},
			},
			"required": []string{"path", "old_string", "new_string"},
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
	plan, err := editor.ProposeEdit(workspacePath, args.Path, args.OldString, args.NewString)
	if err != nil {
		return nil, fmt.Errorf("failed to recreate edit plan: %w", err)
	}

	// Apply the edit
	if err := editor.ApplyEdit(plan); err != nil {
		return nil, fmt.Errorf("failed to apply edit: %w", err)
	}

	// Create a message describing what happened
	var message string
	if plan.IsCreation {
		message = fmt.Sprintf("Created file: %s", args.Path)
	} else if plan.IsDeletion {
		message = fmt.Sprintf("Deleted file: %s", args.Path)
	} else {
		message = fmt.Sprintf("Edited file: %s", args.Path)
	}

	return &ExecutionResult{
		Content: message,
		Safe:    true,
	}, nil
}

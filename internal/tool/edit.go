package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/loom/loom/internal/editor"
)

// EditFileArgs represents the arguments for the edit_file tool.
type EditFileArgs struct {
	Path      string `json:"path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
	CreateNew bool   `json:"create_new,omitempty"`
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
		Description: "Edit a file in the workspace by replacing text or creating a new file",
		Safe:        false, // Editing files requires approval
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
	// For new file creation
	if args.CreateNew {
		if args.OldString != "" {
			return nil, fmt.Errorf("old_string must be empty when creating a new file")
		}

		// Use empty old string for new file
		args.OldString = ""
	}

	// Create an edit plan first
	plan, err := editor.ProposeEdit(workspacePath, args.Path, args.OldString, args.NewString)
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
	if plan.IsCreation {
		message = fmt.Sprintf("File will be created: %s", args.Path)
	} else if plan.IsDeletion {
		message = fmt.Sprintf("File will be deleted: %s", args.Path)
	} else {
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

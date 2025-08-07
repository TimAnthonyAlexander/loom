package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
func editFile(ctx context.Context, workspacePath string, args EditFileArgs) (*EditFileResult, error) {
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

	// Create result to return (with diff from the plan)
	result := &EditFileResult{
		Path: args.Path,
		Diff: plan.Diff,
	}

	// Ensure parent directories exist
	dir := filepath.Dir(plan.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to create directory: %s", err)
		return result, nil
	}

	// Apply the edit
	if plan.IsDeletion {
		// Delete the file
		if err := os.Remove(plan.FilePath); err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("Failed to delete file: %s", err)
			return result, nil
		}
	} else {
		// Create or update the file
		if err := os.WriteFile(plan.FilePath, []byte(plan.NewContent), 0644); err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("Failed to write file: %s", err)
			return result, nil
		}
	}

	// Success
	result.Success = true
	if plan.IsCreation {
		result.Message = "File created successfully"
	} else if plan.IsDeletion {
		result.Message = "File deleted successfully"
	} else {
		result.Message = "File edited successfully"
	}

	return result, nil
}

package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
)

// RunShellArgs describes a shell command proposal.
// This tool DOES NOT execute the command. It only proposes it for approval.
type RunShellArgs struct {
	// If Shell is true, the command will be executed via the system shell using "sh -c".
	Shell bool `json:"shell,omitempty"`
	// Command is either the binary to execute (when shell=false) or the full shell command string (when shell=true).
	Command string `json:"command"`
	// Args are positional arguments for the command when shell=false. Ignored when shell=true.
	Args []string `json:"args,omitempty"`
	// Cwd is the working directory. If relative, it is resolved within the workspace. Defaults to workspace root.
	Cwd string `json:"cwd,omitempty"`
	// TimeoutSeconds is the maximum time to allow the command to run. Defaults to 60 seconds.
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
}

// RegisterRunShell registers the run_shell tool which proposes a shell command for approval.
func RegisterRunShell(registry *Registry, workspacePath string) error {
	return registry.Register(Definition{
		Name:        "run_shell",
		Description: "Propose running a shell command. Requires approval. After approval, call apply_shell with the same arguments to execute.",
		Safe:        false, // Requires approval
		JSONSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"shell": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, run via system shell using 'sh -c' with the given command string.",
				},
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Binary to execute (shell=false) or full command string (shell=true)",
				},
				"args": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Arguments to pass to the binary (ignored when shell=true)",
				},
				"cwd": map[string]interface{}{
					"type":        "string",
					"description": "Working directory. If relative, resolved within the workspace. Defaults to workspace root.",
				},
				"timeout_seconds": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum execution time in seconds (default 60)",
				},
			},
			"required": []string{"command"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args RunShellArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("failed to parse arguments: %w", err)
			}

			// Normalize CWD for display using the same validation used at execution time
			absCwd := workspacePath
			if args.Cwd != "" {
				var err error
				// validatePath ensures path remains within workspace
				absCwd, err = validatePath(workspacePath, args.Cwd)
				if err != nil {
					// For proposals, just show the intended cwd even if invalid; execution will fail later
					absCwd = filepath.Clean(filepath.Join(workspacePath, args.Cwd))
				}
			}

			// Create a diff-like summary for approval UI
			summary := "Propose running command"
			var content string
			if args.Shell {
				content = fmt.Sprintf("Will run via shell:\n  cwd: %s\n  timeout: %ds\n+ $ %s", absCwd, normalizeTimeout(args.TimeoutSeconds), args.Command)
			} else {
				content = fmt.Sprintf("Will exec binary:\n  cwd: %s\n  timeout: %ds\n+ $ %s %v", absCwd, normalizeTimeout(args.TimeoutSeconds), args.Command, args.Args)
			}

			return &ExecutionResult{
				Content: summary,
				Diff:    content,
				Safe:    false,
			}, nil
		},
	})
}

// normalizeTimeout returns default when unset or invalid
func normalizeTimeout(seconds int) int {
	if seconds <= 0 {
		return 60
	}
	if seconds > 600 {
		// clamp to 10 minutes for safety
		return 600
	}
	return seconds
}

package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ApplyShellArgs are the same as RunShellArgs; duplicated to keep schema explicit.
type ApplyShellArgs struct {
	Shell          bool     `json:"shell,omitempty"`
	Command        string   `json:"command"`
	Args           []string `json:"args,omitempty"`
	Cwd            string   `json:"cwd,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
}

// ShellResult captures stdout, stderr and exit code.
type ShellResult struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int    `json:"duration_ms"`
	Cwd        string `json:"cwd"`
}

// RegisterApplyShell registers the apply_shell tool that actually executes commands after approval.
func RegisterApplyShell(registry *Registry, workspacePath string) error {
	return registry.Register(Definition{
		Name:        "apply_shell",
		Description: "Execute a shell command previously proposed via run_shell. Returns stdout, stderr, and exit code.",
		Safe:        true, // Called only after explicit approval
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
					"description": "Maximum execution time in seconds (default 60, max 600)",
				},
			},
			"required": []string{"command"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			var args ApplyShellArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("failed to parse arguments: %w", err)
			}
			return applyShell(ctx, workspacePath, args)
		},
	})
}

func applyShell(ctx context.Context, workspacePath string, args ApplyShellArgs) (*ShellResult, error) {
	if strings.TrimSpace(args.Command) == "" {
		return nil, errors.New("command is required")
	}

	// Resolve CWD and ensure it's inside the workspace
	absCwd := workspacePath
	if args.Cwd != "" {
		var err error
		absCwd, err = validatePath(workspacePath, args.Cwd)
		if err != nil {
			return nil, fmt.Errorf("invalid cwd: %w", err)
		}
	}

	// Prepare the command
	var cmd *exec.Cmd
	if args.Shell {
		// Use 'sh -c' for POSIX shells; we're on darwin per user env
		cmd = exec.CommandContext(ctx, "sh", "-c", args.Command)
	} else {
		// Execute binary directly with args
		cmd = exec.CommandContext(ctx, args.Command, args.Args...)
	}
	cmd.Dir = absCwd

	// Capture output
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Apply timeout using context
	timeout := time.Duration(normalizeTimeout(args.TimeoutSeconds)) * time.Second
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd = withContext(cmd, ctx)

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	// Determine exit code
	exitCode := 0
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			// Non-exit errors (e.g., context deadline)
			exitCode = -1
		}
	}

	result := &ShellResult{
		Stdout:     stdoutBuf.String(),
		Stderr:     stderrBuf.String(),
		ExitCode:   exitCode,
		DurationMs: int(duration / time.Millisecond),
		Cwd:        absCwd,
	}
	return result, nil
}

// withContext rebinds cmd to use the provided context if not already constructed with it.
func withContext(cmd *exec.Cmd, ctx context.Context) *exec.Cmd {
	// exec.CommandContext already set the context; nothing to do here.
	// Function kept for parity/clarity.
	return cmd
}

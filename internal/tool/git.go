package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// GitStatusParams represents parameters for git status
type GitStatusParams struct {
	Short bool `json:"short,omitempty"` // Use short format
}

// GitAddParams represents parameters for git add
type GitAddParams struct {
	Files []string `json:"files"`           // Files to add (can include patterns)
	All   bool     `json:"all,omitempty"`   // Add all modified files
	Force bool     `json:"force,omitempty"` // Force add ignored files
}

// GitCommitParams represents parameters for git commit
type GitCommitParams struct {
	Message string   `json:"message"`           // Commit message
	Files   []string `json:"files,omitempty"`   // Specific files to commit
	All     bool     `json:"all,omitempty"`     // Commit all staged files
	Amend   bool     `json:"amend,omitempty"`   // Amend the last commit
	NoEdit  bool     `json:"no_edit,omitempty"` // Don't open editor for amend
	Author  string   `json:"author,omitempty"`  // Override author
}

// GitPullParams represents parameters for git pull
type GitPullParams struct {
	Remote string `json:"remote,omitempty"` // Remote name (default: origin)
	Branch string `json:"branch,omitempty"` // Branch name
	Rebase bool   `json:"rebase,omitempty"` // Use rebase instead of merge
	Force  bool   `json:"force,omitempty"`  // Force pull
}

// GitPushParams represents parameters for git push
type GitPushParams struct {
	Remote      string `json:"remote,omitempty"`       // Remote name (default: origin)
	Branch      string `json:"branch,omitempty"`       // Branch name
	Force       bool   `json:"force,omitempty"`        // Force push
	SetUpstream bool   `json:"set_upstream,omitempty"` // Set upstream branch
	All         bool   `json:"all,omitempty"`          // Push all branches
	Tags        bool   `json:"tags,omitempty"`         // Push tags
}

// GitLogParams represents parameters for git log
type GitLogParams struct {
	MaxCount int    `json:"max_count,omitempty"` // Maximum number of commits (default: 10)
	Oneline  bool   `json:"oneline,omitempty"`   // One line per commit
	Graph    bool   `json:"graph,omitempty"`     // Show graph
	Since    string `json:"since,omitempty"`     // Show commits since date
	Author   string `json:"author,omitempty"`    // Filter by author
	File     string `json:"file,omitempty"`      // Show commits affecting specific file
}

// GitDiffParams represents parameters for git diff
type GitDiffParams struct {
	Staged   bool     `json:"staged,omitempty"`    // Show staged changes
	Files    []string `json:"files,omitempty"`     // Specific files to diff
	Cached   bool     `json:"cached,omitempty"`    // Show cached/staged changes
	CommitA  string   `json:"commit_a,omitempty"`  // First commit for comparison
	CommitB  string   `json:"commit_b,omitempty"`  // Second commit for comparison
	NameOnly bool     `json:"name_only,omitempty"` // Show only file names
}

// GitBranchParams represents parameters for git branch
type GitBranchParams struct {
	List   bool   `json:"list,omitempty"`   // List branches (default)
	Create string `json:"create,omitempty"` // Create new branch
	Delete string `json:"delete,omitempty"` // Delete branch
	Remote bool   `json:"remote,omitempty"` // Show remote branches
	All    bool   `json:"all,omitempty"`    // Show all branches
}

// GitCheckoutParams represents parameters for git checkout
type GitCheckoutParams struct {
	Branch    string   `json:"branch"`               // Branch or commit to checkout
	CreateNew bool     `json:"create_new,omitempty"` // Create new branch
	Force     bool     `json:"force,omitempty"`      // Force checkout
	Files     []string `json:"files,omitempty"`      // Specific files to checkout
	Track     bool     `json:"track,omitempty"`      // Set up tracking
}

// GitMergeParams represents parameters for git merge
type GitMergeParams struct {
	Branch   string `json:"branch"`              // Branch to merge
	NoCommit bool   `json:"no_commit,omitempty"` // Don't auto-commit
	NoFF     bool   `json:"no_ff,omitempty"`     // Create merge commit even for fast-forward
	Squash   bool   `json:"squash,omitempty"`    // Squash commits
	Message  string `json:"message,omitempty"`   // Merge commit message
	Abort    bool   `json:"abort,omitempty"`     // Abort merge
	Continue bool   `json:"continue,omitempty"`  // Continue merge after resolving conflicts
}

// RegisterGitTools registers all git-related tools with the registry
func RegisterGitTools(registry *Registry, workspacePath string) error {
	tools := []Definition{
		{
			Name:        "git_status",
			Description: "Show the working tree status using git status",
			JSONSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"short": map[string]interface{}{
						"type":        "boolean",
						"description": "Use short format output",
						"default":     false,
					},
				},
			},
			Safe:    true, // Read-only operation
			Handler: createGitHandler(workspacePath, handleGitStatus),
		},
		{
			Name:        "git_add",
			Description: "Add files to the staging area using git add",
			JSONSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"files": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Files or patterns to add",
					},
					"all": map[string]interface{}{
						"type":        "boolean",
						"description": "Add all modified and new files",
						"default":     false,
					},
					"force": map[string]interface{}{
						"type":        "boolean",
						"description": "Force add ignored files",
						"default":     false,
					},
				},
			},
			Safe:    false, // Modifies repository state
			Handler: createGitHandler(workspacePath, handleGitAdd),
		},
		{
			Name:        "git_commit",
			Description: "Create a commit using git commit",
			JSONSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": map[string]interface{}{
						"type":        "string",
						"description": "Commit message",
					},
					"files": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Specific files to commit",
					},
					"all": map[string]interface{}{
						"type":        "boolean",
						"description": "Commit all staged changes",
						"default":     false,
					},
					"amend": map[string]interface{}{
						"type":        "boolean",
						"description": "Amend the last commit",
						"default":     false,
					},
					"no_edit": map[string]interface{}{
						"type":        "boolean",
						"description": "Don't open editor when amending",
						"default":     true,
					},
					"author": map[string]interface{}{
						"type":        "string",
						"description": "Override commit author (format: 'Name <email>')",
					},
				},
				"required": []string{"message"},
			},
			Safe:    false, // Creates commits
			Handler: createGitHandler(workspacePath, handleGitCommit),
		},
		{
			Name:        "git_pull",
			Description: "Fetch and integrate changes from remote repository using git pull",
			JSONSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"remote": map[string]interface{}{
						"type":        "string",
						"description": "Remote repository name",
						"default":     "origin",
					},
					"branch": map[string]interface{}{
						"type":        "string",
						"description": "Branch name",
					},
					"rebase": map[string]interface{}{
						"type":        "boolean",
						"description": "Use rebase instead of merge",
						"default":     false,
					},
					"force": map[string]interface{}{
						"type":        "boolean",
						"description": "Force pull (may overwrite local changes)",
						"default":     false,
					},
				},
			},
			Safe:    false, // Network operation that modifies repository
			Handler: createGitHandler(workspacePath, handleGitPull),
		},
		{
			Name:        "git_push",
			Description: "Push changes to remote repository using git push",
			JSONSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"remote": map[string]interface{}{
						"type":        "string",
						"description": "Remote repository name",
						"default":     "origin",
					},
					"branch": map[string]interface{}{
						"type":        "string",
						"description": "Branch name",
					},
					"force": map[string]interface{}{
						"type":        "boolean",
						"description": "Force push (may overwrite remote history)",
						"default":     false,
					},
					"set_upstream": map[string]interface{}{
						"type":        "boolean",
						"description": "Set upstream tracking branch",
						"default":     false,
					},
					"all": map[string]interface{}{
						"type":        "boolean",
						"description": "Push all branches",
						"default":     false,
					},
					"tags": map[string]interface{}{
						"type":        "boolean",
						"description": "Push tags",
						"default":     false,
					},
				},
			},
			Safe:    false, // Network operation that modifies remote repository
			Handler: createGitHandler(workspacePath, handleGitPush),
		},
		{
			Name:        "git_log",
			Description: "Show commit history using git log",
			JSONSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"max_count": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of commits to show",
						"default":     10,
						"minimum":     1,
						"maximum":     100,
					},
					"oneline": map[string]interface{}{
						"type":        "boolean",
						"description": "Show one line per commit",
						"default":     false,
					},
					"graph": map[string]interface{}{
						"type":        "boolean",
						"description": "Show commit graph",
						"default":     false,
					},
					"since": map[string]interface{}{
						"type":        "string",
						"description": "Show commits since date (e.g., '2 weeks ago', '2023-01-01')",
					},
					"author": map[string]interface{}{
						"type":        "string",
						"description": "Filter commits by author",
					},
					"file": map[string]interface{}{
						"type":        "string",
						"description": "Show commits affecting specific file",
					},
				},
			},
			Safe:    true, // Read-only operation
			Handler: createGitHandler(workspacePath, handleGitLog),
		},
		{
			Name:        "git_diff",
			Description: "Show changes using git diff",
			JSONSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"staged": map[string]interface{}{
						"type":        "boolean",
						"description": "Show staged changes",
						"default":     false,
					},
					"cached": map[string]interface{}{
						"type":        "boolean",
						"description": "Show cached/staged changes (alias for staged)",
						"default":     false,
					},
					"files": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Specific files to diff",
					},
					"commit_a": map[string]interface{}{
						"type":        "string",
						"description": "First commit for comparison",
					},
					"commit_b": map[string]interface{}{
						"type":        "string",
						"description": "Second commit for comparison",
					},
					"name_only": map[string]interface{}{
						"type":        "boolean",
						"description": "Show only file names",
						"default":     false,
					},
				},
			},
			Safe:    true, // Read-only operation
			Handler: createGitHandler(workspacePath, handleGitDiff),
		},
		{
			Name:        "git_branch",
			Description: "List, create, or delete branches using git branch",
			JSONSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"list": map[string]interface{}{
						"type":        "boolean",
						"description": "List branches (default operation)",
						"default":     true,
					},
					"create": map[string]interface{}{
						"type":        "string",
						"description": "Create new branch with given name",
					},
					"delete": map[string]interface{}{
						"type":        "string",
						"description": "Delete branch with given name",
					},
					"remote": map[string]interface{}{
						"type":        "boolean",
						"description": "Show remote branches",
						"default":     false,
					},
					"all": map[string]interface{}{
						"type":        "boolean",
						"description": "Show all branches (local and remote)",
						"default":     false,
					},
				},
			},
			Safe:    true, // Read-only by default, unsafe operations require parameters
			Handler: createGitHandler(workspacePath, handleGitBranch),
		},
		{
			Name:        "git_checkout",
			Description: "Switch branches or restore files using git checkout",
			JSONSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"branch": map[string]interface{}{
						"type":        "string",
						"description": "Branch or commit to checkout",
					},
					"create_new": map[string]interface{}{
						"type":        "boolean",
						"description": "Create new branch",
						"default":     false,
					},
					"force": map[string]interface{}{
						"type":        "boolean",
						"description": "Force checkout (may discard local changes)",
						"default":     false,
					},
					"files": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Specific files to checkout",
					},
					"track": map[string]interface{}{
						"type":        "boolean",
						"description": "Set up tracking when creating new branch",
						"default":     false,
					},
				},
				"required": []string{"branch"},
			},
			Safe:    false, // Can modify working directory
			Handler: createGitHandler(workspacePath, handleGitCheckout),
		},
		{
			Name:        "git_merge",
			Description: "Merge branches using git merge",
			JSONSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"branch": map[string]interface{}{
						"type":        "string",
						"description": "Branch to merge",
					},
					"no_commit": map[string]interface{}{
						"type":        "boolean",
						"description": "Don't auto-commit merge",
						"default":     false,
					},
					"no_ff": map[string]interface{}{
						"type":        "boolean",
						"description": "Create merge commit even for fast-forward",
						"default":     false,
					},
					"squash": map[string]interface{}{
						"type":        "boolean",
						"description": "Squash commits",
						"default":     false,
					},
					"message": map[string]interface{}{
						"type":        "string",
						"description": "Merge commit message",
					},
					"abort": map[string]interface{}{
						"type":        "boolean",
						"description": "Abort ongoing merge",
						"default":     false,
					},
					"continue": map[string]interface{}{
						"type":        "boolean",
						"description": "Continue merge after resolving conflicts",
						"default":     false,
					},
				},
			},
			Safe:    false, // Modifies repository state
			Handler: createGitHandler(workspacePath, handleGitMerge),
		},
	}

	for _, tool := range tools {
		if err := registry.Register(tool); err != nil {
			return fmt.Errorf("failed to register %s: %w", tool.Name, err)
		}
	}

	return nil
}

// createGitHandler creates a handler function for git operations
func createGitHandler(workspacePath string, handlerFunc func(workspacePath string, params json.RawMessage) (interface{}, error)) func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
	return func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
		return handlerFunc(workspacePath, raw)
	}
}

// runGitCommand executes a git command in the workspace directory
func runGitCommand(workspacePath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = workspacePath

	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	if err != nil {
		return outputStr, fmt.Errorf("git command failed: %w\nOutput: %s", err, outputStr)
	}

	return outputStr, nil
}

// Git command handlers

func handleGitStatus(workspacePath string, params json.RawMessage) (interface{}, error) {
	var p GitStatusParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	args := []string{"status"}
	if p.Short {
		args = append(args, "--short")
	}

	output, err := runGitCommand(workspacePath, args...)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"output": output,
		"short":  p.Short,
	}, nil
}

func handleGitAdd(workspacePath string, params json.RawMessage) (interface{}, error) {
	var p GitAddParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	args := []string{"add"}

	if p.Force {
		args = append(args, "--force")
	}

	if p.All {
		args = append(args, "--all")
	} else if len(p.Files) > 0 {
		args = append(args, p.Files...)
	} else {
		return nil, fmt.Errorf("either 'all' must be true or 'files' must be specified")
	}

	output, err := runGitCommand(workspacePath, args...)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"output": output,
		"files":  p.Files,
		"all":    p.All,
	}, nil
}

func handleGitCommit(workspacePath string, params json.RawMessage) (interface{}, error) {
	var p GitCommitParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	args := []string{"commit", "-m", p.Message}

	if p.All {
		args = append(args, "--all")
	}

	if p.Amend {
		args = append(args, "--amend")
		if p.NoEdit {
			args = append(args, "--no-edit")
		}
	}

	if p.Author != "" {
		args = append(args, "--author", p.Author)
	}

	if len(p.Files) > 0 {
		args = append(args, p.Files...)
	}

	output, err := runGitCommand(workspacePath, args...)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"output":  output,
		"message": p.Message,
		"amend":   p.Amend,
	}, nil
}

func handleGitPull(workspacePath string, params json.RawMessage) (interface{}, error) {
	var p GitPullParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	args := []string{"pull"}

	if p.Rebase {
		args = append(args, "--rebase")
	}

	if p.Force {
		args = append(args, "--force")
	}

	if p.Remote != "" {
		args = append(args, p.Remote)
	}

	if p.Branch != "" {
		args = append(args, p.Branch)
	}

	output, err := runGitCommand(workspacePath, args...)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"output": output,
		"remote": p.Remote,
		"branch": p.Branch,
		"rebase": p.Rebase,
	}, nil
}

func handleGitPush(workspacePath string, params json.RawMessage) (interface{}, error) {
	var p GitPushParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	args := []string{"push"}

	if p.Force {
		args = append(args, "--force")
	}

	if p.SetUpstream {
		args = append(args, "--set-upstream")
	}

	if p.All {
		args = append(args, "--all")
	}

	if p.Tags {
		args = append(args, "--tags")
	}

	if p.Remote != "" {
		args = append(args, p.Remote)
	}

	if p.Branch != "" {
		args = append(args, p.Branch)
	}

	output, err := runGitCommand(workspacePath, args...)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"output": output,
		"remote": p.Remote,
		"branch": p.Branch,
		"force":  p.Force,
	}, nil
}

func handleGitLog(workspacePath string, params json.RawMessage) (interface{}, error) {
	var p GitLogParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	args := []string{"log"}

	maxCount := p.MaxCount
	if maxCount <= 0 {
		maxCount = 10
	}
	args = append(args, fmt.Sprintf("--max-count=%d", maxCount))

	if p.Oneline {
		args = append(args, "--oneline")
	}

	if p.Graph {
		args = append(args, "--graph")
	}

	if p.Since != "" {
		args = append(args, "--since", p.Since)
	}

	if p.Author != "" {
		args = append(args, "--author", p.Author)
	}

	if p.File != "" {
		args = append(args, "--", p.File)
	}

	output, err := runGitCommand(workspacePath, args...)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"output":    output,
		"max_count": maxCount,
		"oneline":   p.Oneline,
		"graph":     p.Graph,
	}, nil
}

func handleGitDiff(workspacePath string, params json.RawMessage) (interface{}, error) {
	var p GitDiffParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	args := []string{"diff"}

	if p.Staged || p.Cached {
		args = append(args, "--staged")
	}

	if p.NameOnly {
		args = append(args, "--name-only")
	}

	if p.CommitA != "" && p.CommitB != "" {
		args = append(args, p.CommitA, p.CommitB)
	} else if p.CommitA != "" {
		args = append(args, p.CommitA)
	}

	if len(p.Files) > 0 {
		args = append(args, "--")
		args = append(args, p.Files...)
	}

	output, err := runGitCommand(workspacePath, args...)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"output":    output,
		"staged":    p.Staged || p.Cached,
		"name_only": p.NameOnly,
		"files":     p.Files,
	}, nil
}

func handleGitBranch(workspacePath string, params json.RawMessage) (interface{}, error) {
	var p GitBranchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	args := []string{"branch"}

	if p.Delete != "" {
		args = append(args, "-d", p.Delete)
	} else if p.Create != "" {
		args = append(args, p.Create)
	} else {
		// List branches
		if p.Remote {
			args = append(args, "-r")
		} else if p.All {
			args = append(args, "-a")
		}
	}

	output, err := runGitCommand(workspacePath, args...)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"output": output,
		"create": p.Create,
		"delete": p.Delete,
		"remote": p.Remote,
		"all":    p.All,
	}, nil
}

func handleGitCheckout(workspacePath string, params json.RawMessage) (interface{}, error) {
	var p GitCheckoutParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	args := []string{"checkout"}

	if p.CreateNew {
		args = append(args, "-b")
	}

	if p.Force {
		args = append(args, "--force")
	}

	if p.Track {
		args = append(args, "--track")
	}

	args = append(args, p.Branch)

	if len(p.Files) > 0 {
		args = append(args, "--")
		args = append(args, p.Files...)
	}

	output, err := runGitCommand(workspacePath, args...)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"output":     output,
		"branch":     p.Branch,
		"create_new": p.CreateNew,
		"force":      p.Force,
		"files":      p.Files,
	}, nil
}

func handleGitMerge(workspacePath string, params json.RawMessage) (interface{}, error) {
	var p GitMergeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Validate that at least one operation is specified
	if !p.Abort && !p.Continue && p.Branch == "" {
		return nil, fmt.Errorf("either 'branch' must be specified, or 'abort' or 'continue' must be true")
	}

	args := []string{"merge"}

	if p.Abort {
		args = append(args, "--abort")
	} else if p.Continue {
		args = append(args, "--continue")
	} else {
		if p.NoCommit {
			args = append(args, "--no-commit")
		}

		if p.NoFF {
			args = append(args, "--no-ff")
		}

		if p.Squash {
			args = append(args, "--squash")
		}

		if p.Message != "" {
			args = append(args, "-m", p.Message)
		}

		if p.Branch != "" {
			args = append(args, p.Branch)
		}
	}

	output, err := runGitCommand(workspacePath, args...)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"output":    output,
		"branch":    p.Branch,
		"no_commit": p.NoCommit,
		"squash":    p.Squash,
		"abort":     p.Abort,
		"continue":  p.Continue,
	}, nil
}

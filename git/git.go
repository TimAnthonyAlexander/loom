package git

import (
	"fmt"
	"strings"
	"time"
)

// Status represents the Git status of a file
type FileStatus struct {
	Path     string `json:"path"`
	Status   string `json:"status"`    // "untracked", "modified", "staged", "deleted", etc.
	Staged   bool   `json:"staged"`    // Is the file staged for commit
	WorkTree bool   `json:"work_tree"` // Has changes in working tree
}

// Repository represents Git repository information
type Repository struct {
	workspacePath string
	isGitRepo     bool
	currentBranch string
	remoteURL     string
}

// RepositoryStatus represents the overall Git status
type RepositoryStatus struct {
	IsGitRepo      bool          `json:"is_git_repo"`
	CurrentBranch  string        `json:"current_branch"`
	RemoteURL      string        `json:"remote_url"`
	Files          []*FileStatus `json:"files"`
	StagedCount    int           `json:"staged_count"`
	ModifiedCount  int           `json:"modified_count"`
	UntrackedCount int           `json:"untracked_count"`
	IsClean        bool          `json:"is_clean"`
	AheadCount     int           `json:"ahead_count"`
	BehindCount    int           `json:"behind_count"`
}

// CommitInfo represents information about a commit
type CommitInfo struct {
	Hash    string    `json:"hash"`
	Message string    `json:"message"`
	Author  string    `json:"author"`
	Date    time.Time `json:"date"`
	Files   []string  `json:"files"`
}

// NewRepository creates a new Git repository interface

// Check if this is a Git repository

// Return non-Git repo (isGitRepo will be false)

// Get current branch and remote

// checkGitRepo checks if the workspace is a Git repository

// loadRepositoryInfo loads basic repository information

// GetStatus returns the current Git status

// getAheadBehindCount gets the number of commits ahead/behind remote

// StageFiles stages the specified files for commit

// UnstageFiles unstages the specified files

// Commit creates a new commit with the specified message

// GetLastCommit returns information about the last commit

// IsFileIgnored checks if a file is ignored by Git

// ValidatePreConditions checks if Git preconditions are met for an operation

// GetDiff returns the diff for specified files

// GetFileAtCommit returns the content of a file at a specific commit

// GetBranches returns a list of all branches

// IsGitRepository returns whether this is a Git repository

// GetCurrentBranch returns the current branch name

// GetRemoteURL returns the remote URL

// FormatStatus returns a human-readable status summary
func (status *RepositoryStatus) FormatStatus() string {
	if !status.IsGitRepo {
		return "Not a Git repository"
	}

	var parts []string

	if status.CurrentBranch != "" {
		parts = append(parts, fmt.Sprintf("Branch: %s", status.CurrentBranch))
	}

	if status.IsClean {
		parts = append(parts, "Clean")
	} else {
		if status.ModifiedCount > 0 {
			parts = append(parts, fmt.Sprintf("%d modified", status.ModifiedCount))
		}
		if status.StagedCount > 0 {
			parts = append(parts, fmt.Sprintf("%d staged", status.StagedCount))
		}
		if status.UntrackedCount > 0 {
			parts = append(parts, fmt.Sprintf("%d untracked", status.UntrackedCount))
		}
	}

	if status.AheadCount > 0 {
		parts = append(parts, fmt.Sprintf("%d ahead", status.AheadCount))
	}
	if status.BehindCount > 0 {
		parts = append(parts, fmt.Sprintf("%d behind", status.BehindCount))
	}

	return strings.Join(parts, ", ")
}

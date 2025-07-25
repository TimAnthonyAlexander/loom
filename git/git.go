package git

import (
	"bufio"
	"fmt"
	"os/exec"
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
func (r *Repository) checkGitRepo() error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = r.workspacePath

	_, err := cmd.Output()
	if err != nil {
		return err
	}

	r.isGitRepo = true
	return nil
}

// loadRepositoryInfo loads basic repository information
func (r *Repository) loadRepositoryInfo() error {
	// Get current branch
	branchCmd := exec.Command("git", "branch", "--show-current")
	branchCmd.Dir = r.workspacePath

	if output, err := branchCmd.Output(); err == nil {
		r.currentBranch = strings.TrimSpace(string(output))
	}

	// Get remote URL
	remoteCmd := exec.Command("git", "remote", "get-url", "origin")
	remoteCmd.Dir = r.workspacePath

	if output, err := remoteCmd.Output(); err == nil {
		r.remoteURL = strings.TrimSpace(string(output))
	}

	return nil
}

// GetStatus returns the current Git status
func (r *Repository) GetStatus() (*RepositoryStatus, error) {
	status := &RepositoryStatus{
		IsGitRepo:     r.isGitRepo,
		CurrentBranch: r.currentBranch,
		RemoteURL:     r.remoteURL,
		Files:         make([]*FileStatus, 0),
	}

	if !r.isGitRepo {
		return status, nil
	}

	// Get file status using git status --porcelain
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = r.workspacePath

	output, err := cmd.Output()
	if err != nil {
		return status, fmt.Errorf("failed to get git status: %w", err)
	}

	// Parse git status output
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 3 {
			continue
		}

		indexStatus := line[0]
		workTreeStatus := line[1]
		filePath := line[3:]

		fileStatus := &FileStatus{
			Path:     filePath,
			Staged:   indexStatus != ' ' && indexStatus != '?',
			WorkTree: workTreeStatus != ' ',
		}

		// Determine overall status
		switch {
		case indexStatus == '?' && workTreeStatus == '?':
			fileStatus.Status = "untracked"
			status.UntrackedCount++
		case indexStatus == 'A':
			fileStatus.Status = "added"
			status.StagedCount++
		case indexStatus == 'M':
			fileStatus.Status = "modified_staged"
			status.StagedCount++
		case indexStatus == 'D':
			fileStatus.Status = "deleted_staged"
			status.StagedCount++
		case workTreeStatus == 'M':
			fileStatus.Status = "modified"
			status.ModifiedCount++
		case workTreeStatus == 'D':
			fileStatus.Status = "deleted"
			status.ModifiedCount++
		default:
			fileStatus.Status = "unknown"
		}

		status.Files = append(status.Files, fileStatus)
	}

	// Check if repository is clean
	status.IsClean = status.StagedCount == 0 && status.ModifiedCount == 0 && status.UntrackedCount == 0

	// Get ahead/behind count
	if ahead, behind, err := r.getAheadBehindCount(); err == nil {
		status.AheadCount = ahead
		status.BehindCount = behind
	}

	return status, nil
}

// getAheadBehindCount gets the number of commits ahead/behind remote
func (r *Repository) getAheadBehindCount() (int, int, error) {
	if r.currentBranch == "" {
		return 0, 0, nil
	}

	// Get ahead/behind count
	cmd := exec.Command("git", "rev-list", "--left-right", "--count",
		fmt.Sprintf("origin/%s...HEAD", r.currentBranch))
	cmd.Dir = r.workspacePath

	output, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	parts := strings.Fields(strings.TrimSpace(string(output)))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected git rev-list output: %s", output)
	}

	var behind, ahead int
	fmt.Sscanf(parts[0], "%d", &behind)
	fmt.Sscanf(parts[1], "%d", &ahead)

	return ahead, behind, nil
}

// StageFiles stages the specified files for commit
func (r *Repository) StageFiles(filePaths []string) error {
	if !r.isGitRepo {
		return fmt.Errorf("not a git repository")
	}

	if len(filePaths) == 0 {
		return nil
	}

	args := append([]string{"add"}, filePaths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = r.workspacePath

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stage files: %s", string(output))
	}

	return nil
}

// UnstageFiles unstages the specified files
func (r *Repository) UnstageFiles(filePaths []string) error {
	if !r.isGitRepo {
		return fmt.Errorf("not a git repository")
	}

	if len(filePaths) == 0 {
		return nil
	}

	args := append([]string{"reset", "HEAD", "--"}, filePaths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = r.workspacePath

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to unstage files: %s", string(output))
	}

	return nil
}

// Commit creates a new commit with the specified message
func (r *Repository) Commit(message string) (*CommitInfo, error) {
	if !r.isGitRepo {
		return nil, fmt.Errorf("not a git repository")
	}

	if message == "" {
		return nil, fmt.Errorf("commit message cannot be empty")
	}

	// Create the commit
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = r.workspacePath

	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to commit: %s", string(output))
	}

	// Get the commit info
	return r.GetLastCommit()
}

// GetLastCommit returns information about the last commit
func (r *Repository) GetLastCommit() (*CommitInfo, error) {
	if !r.isGitRepo {
		return nil, fmt.Errorf("not a git repository")
	}

	// Get commit info
	cmd := exec.Command("git", "log", "-1", "--pretty=format:%H|%s|%an|%ad", "--date=iso")
	cmd.Dir = r.workspacePath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get last commit: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) != 4 {
		return nil, fmt.Errorf("unexpected git log output format")
	}

	date, err := time.Parse("2006-01-02 15:04:05 -0700", parts[3])
	if err != nil {
		date = time.Now() // Fallback
	}

	commit := &CommitInfo{
		Hash:    parts[0],
		Message: parts[1],
		Author:  parts[2],
		Date:    date,
	}

	// Get files changed in this commit
	filesCmd := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", commit.Hash)
	filesCmd.Dir = r.workspacePath

	if filesOutput, err := filesCmd.Output(); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(filesOutput)))
		for scanner.Scan() {
			if file := strings.TrimSpace(scanner.Text()); file != "" {
				commit.Files = append(commit.Files, file)
			}
		}
	}

	return commit, nil
}

// IsFileIgnored checks if a file is ignored by Git
func (r *Repository) IsFileIgnored(filePath string) (bool, error) {
	if !r.isGitRepo {
		return false, nil
	}

	cmd := exec.Command("git", "check-ignore", filePath)
	cmd.Dir = r.workspacePath

	err := cmd.Run()
	if err != nil {
		// File is not ignored if git check-ignore returns an error
		return false, nil
	}

	return true, nil
}

// ValidatePreConditions checks if Git preconditions are met for an operation
func (r *Repository) ValidatePreConditions(preConditions []string) error {
	if !r.isGitRepo {
		return nil // Skip Git validation for non-Git repos
	}

	status, err := r.GetStatus()
	if err != nil {
		return fmt.Errorf("failed to get git status: %w", err)
	}

	for _, condition := range preConditions {
		switch condition {
		case "clean":
			if !status.IsClean {
				return fmt.Errorf("git repository is not clean (has %d modified, %d staged, %d untracked files)",
					status.ModifiedCount, status.StagedCount, status.UntrackedCount)
			}
		case "no_staged":
			if status.StagedCount > 0 {
				return fmt.Errorf("git repository has %d staged files", status.StagedCount)
			}
		case "no_modified":
			if status.ModifiedCount > 0 {
				return fmt.Errorf("git repository has %d modified files", status.ModifiedCount)
			}
		default:
			return fmt.Errorf("unknown git precondition: %s", condition)
		}
	}

	return nil
}

// GetDiff returns the diff for specified files
func (r *Repository) GetDiff(filePaths []string, staged bool) (string, error) {
	if !r.isGitRepo {
		return "", fmt.Errorf("not a git repository")
	}

	args := []string{"diff"}
	if staged {
		args = append(args, "--staged")
	}

	if len(filePaths) > 0 {
		args = append(args, "--")
		args = append(args, filePaths...)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = r.workspacePath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	return string(output), nil
}

// GetFileAtCommit returns the content of a file at a specific commit
func (r *Repository) GetFileAtCommit(filePath, commitHash string) (string, error) {
	if !r.isGitRepo {
		return "", fmt.Errorf("not a git repository")
	}

	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", commitHash, filePath))
	cmd.Dir = r.workspacePath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get file at commit: %w", err)
	}

	return string(output), nil
}

// GetBranches returns a list of all branches
func (r *Repository) GetBranches() ([]string, error) {
	if !r.isGitRepo {
		return nil, fmt.Errorf("not a git repository")
	}

	cmd := exec.Command("git", "branch", "-a")
	cmd.Dir = r.workspacePath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get branches: %w", err)
	}

	var branches []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		branch := strings.TrimSpace(scanner.Text())
		if branch != "" && !strings.HasPrefix(branch, "*") {
			branches = append(branches, branch)
		}
	}

	return branches, nil
}

// IsGitRepository returns whether this is a Git repository
func (r *Repository) IsGitRepository() bool {
	return r.isGitRepo
}

// GetCurrentBranch returns the current branch name
func (r *Repository) GetCurrentBranch() string {
	return r.currentBranch
}

// GetRemoteURL returns the remote URL
func (r *Repository) GetRemoteURL() string {
	return r.remoteURL
}

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

func NewRepository(workspacePath string) (*Repository, error) {
	repo := &Repository{
		workspacePath: workspacePath,
	}

	// Check if this is a Git repository
	if err := repo.checkGitRepo(); err != nil {
		return repo, nil // Not a Git repo, return empty repo
	}

	// Load basic repository info
	if err := repo.loadRepositoryInfo(); err != nil {
		return nil, fmt.Errorf("failed to load repository info: %w", err)
	}

	return repo, nil
}

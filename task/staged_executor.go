package task

import (
	"crypto/sha256"
	"fmt"
	"loom/context"
	"loom/paths"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// StagedExecutor handles staging and batch execution of action plans
type StagedExecutor struct {
	*Executor      // Embed the base executor
	contextManager *context.ContextManager
	projectPaths   *paths.ProjectPaths
}

// NewStagedExecutor creates a new staged executor

// Get project paths

// Ensure project directories exist

// PrepareActionPlan prepares an action plan by staging all edits
func (se *StagedExecutor) PrepareActionPlan(plan *ActionPlan) (*ActionPlanExecution, error) {
	execution := &ActionPlanExecution{
		Plan:          plan,
		TaskResponses: make([]TaskResponse, 0, len(plan.Tasks)),
		StagedEdits:   make([]*StagedEdit, 0),
		BackupPaths:   make([]string, 0),
		StartTime:     time.Now(),
		Status:        "preparing",
	}

	// Execute non-destructive tasks first (reads, lists)
	for i, task := range plan.Tasks {
		if !task.IsDestructive() {
			response := se.Execute(&task)
			execution.TaskResponses = append(execution.TaskResponses, *response)

			if !response.Success {
				execution.Status = "failed"
				return execution, fmt.Errorf("task %d failed: %s", i, response.Error)
			}
		}
	}

	// Stage destructive edits
	for i, task := range plan.Tasks {
		if task.Type == TaskTypeEditFile {
			stagedEdit, err := se.stageEdit(&task)
			if err != nil {
				execution.Status = "failed"
				return execution, fmt.Errorf("failed to stage edit for task %d: %w", i, err)
			}

			execution.StagedEdits = append(execution.StagedEdits, stagedEdit)

			// Create task response for the staged edit
			response := &TaskResponse{
				Task:    task,
				Success: true,
				Output:  fmt.Sprintf("Staged edit for %s:\n\n%s", task.Path, stagedEdit.DiffPreview),
			}
			execution.TaskResponses = append(execution.TaskResponses, *response)
		}
	}

	// Set approval requirement
	execution.ApprovalNeeded = plan.RequiresApproval()
	execution.Status = "staged"

	return execution, nil
}

// stageEdit prepares a file edit without applying it
func (se *StagedExecutor) stageEdit(task *Task) (*StagedEdit, error) {
	fullPath, err := se.Executor.securePath(task.Path)
	if err != nil {
		return nil, err
	}

	// Read current file content
	var originalContent string
	var originalHash string

	if _, err := os.Stat(fullPath); err == nil {
		// File exists, read it
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read existing file: %w", err)
		}
		originalContent = string(data)
		originalHash = se.calculateContentHash(originalContent)
	} else {
		// New file
		originalContent = ""
		originalHash = se.calculateContentHash("")
	}

	// Apply the edit to get new content
	var newContent string

	if task.Diff != "" {
		// Apply diff
		dmp := diffmatchpatch.New()
		patches, err := dmp.PatchFromText(task.Diff)
		if err != nil {
			return nil, fmt.Errorf("invalid diff format: %w", err)
		}

		result, success := dmp.PatchApply(patches, originalContent)
		for i, applied := range success {
			if !applied {
				return nil, fmt.Errorf("failed to apply patch %d", i)
			}
		}
		newContent = result
	} else if task.Content != "" {
		// Replace content entirely
		newContent = task.Content
	} else {
		return nil, fmt.Errorf("edit task requires either diff or content")
	}

	// Generate diff preview
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(originalContent, newContent, false)
	diffPreview := dmp.DiffPrettyText(diffs)

	// Create backup file
	backupPath := se.createBackupPath(task.Path)
	if originalContent != "" {
		if err := os.WriteFile(backupPath, []byte(originalContent), 0644); err != nil {
			return nil, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	stagedEdit := &StagedEdit{
		FilePath:     task.Path,
		OriginalHash: originalHash,
		NewContent:   newContent,
		DiffPreview:  diffPreview,
		BackupPath:   backupPath,
		Task:         task,
	}

	return stagedEdit, nil
}

// ApplyActionPlan applies a staged action plan
func (se *StagedExecutor) ApplyActionPlan(execution *ActionPlanExecution) error {
	if execution.Status != "staged" {
		return fmt.Errorf("action plan is not staged for application")
	}

	execution.Status = "applying"

	// Apply staged edits
	for _, edit := range execution.StagedEdits {
		if err := se.applyStagedEdit(edit); err != nil {
			execution.Status = "failed"
			return fmt.Errorf("failed to apply edit to %s: %w", edit.FilePath, err)
		}
	}

	// Execute remaining destructive tasks (shell commands)
	for _, task := range execution.Plan.Tasks {
		if task.Type == TaskTypeRunShell {
			response := se.Execute(&task)
			if !response.Success {
				execution.Status = "failed"
				return fmt.Errorf("shell command failed: %s", response.Error)
			}
		}
	}

	execution.EndTime = time.Now()
	execution.Status = "completed"

	return nil
}

// applyStagedEdit writes a staged edit to disk
func (se *StagedExecutor) applyStagedEdit(edit *StagedEdit) error {
	fullPath, err := se.Executor.securePath(edit.FilePath)
	if err != nil {
		return err
	}

	// Verify file hasn't changed since staging
	if _, err := os.Stat(fullPath); err == nil {
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("failed to verify file state: %w", err)
		}

		currentHash := se.calculateContentHash(string(data))
		if currentHash != edit.OriginalHash {
			return fmt.Errorf("file %s has been modified since staging", edit.FilePath)
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write the new content
	if err := os.WriteFile(fullPath, []byte(edit.NewContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// GetStagingPreview generates a comprehensive preview of all staged changes
func (se *StagedExecutor) GetStagingPreview(execution *ActionPlanExecution) string {
	var preview strings.Builder

	preview.WriteString(fmt.Sprintf("=== Action Plan: %s ===\n", execution.Plan.Title))
	preview.WriteString(fmt.Sprintf("Description: %s\n", execution.Plan.Description))
	preview.WriteString(fmt.Sprintf("Status: %s\n", execution.Status))
	preview.WriteString(fmt.Sprintf("Files to modify: %d\n", len(execution.StagedEdits)))

	if execution.Plan.TestFirst {
		preview.WriteString("⚠️  Test-first policy enabled\n")
	}

	preview.WriteString("\n")

	// Show staged edits
	for i, edit := range execution.StagedEdits {
		preview.WriteString(fmt.Sprintf("--- File %d: %s ---\n", i+1, edit.FilePath))
		preview.WriteString("Changes prepared for approval\n")
		preview.WriteString("\n")
	}

	// Show shell commands that will be executed
	shellCommands := 0
	for _, task := range execution.Plan.Tasks {
		if task.Type == TaskTypeRunShell {
			shellCommands++
			preview.WriteString(fmt.Sprintf("Shell Command %d: %s\n", shellCommands, task.Command))
		}
	}

	if shellCommands > 0 {
		preview.WriteString("\n")
	}

	preview.WriteString("=== End Preview ===\n")

	return preview.String()
}

// UndoActionPlan reverts the changes made by an action plan
func (se *StagedExecutor) UndoActionPlan(execution *ActionPlanExecution) error {
	if execution.Status != "completed" {
		return fmt.Errorf("can only undo completed action plans")
	}

	// Restore files from backups
	for _, edit := range execution.StagedEdits {
		if err := se.restoreFromBackup(edit); err != nil {
			return fmt.Errorf("failed to restore %s: %w", edit.FilePath, err)
		}
	}

	execution.Status = "undone"
	return nil
}

// restoreFromBackup restores a file from its backup
func (se *StagedExecutor) restoreFromBackup(edit *StagedEdit) error {
	fullPath, err := se.Executor.securePath(edit.FilePath)
	if err != nil {
		return err
	}

	// Check if backup exists
	if _, err := os.Stat(edit.BackupPath); os.IsNotExist(err) {
		// No backup means this was a new file, so delete it
		return os.Remove(fullPath)
	}

	// Restore from backup
	backupData, err := os.ReadFile(edit.BackupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	if err := os.WriteFile(fullPath, backupData, 0644); err != nil {
		return fmt.Errorf("failed to restore file: %w", err)
	}

	return nil
}

// CleanupActionPlan removes staging and backup files for an action plan
func (se *StagedExecutor) CleanupActionPlan(execution *ActionPlanExecution) error {
	// Remove backup files
	for _, edit := range execution.StagedEdits {
		if edit.BackupPath != "" {
			os.Remove(edit.BackupPath) // Ignore errors
		}
	}

	return nil
}

// calculateContentHash calculates a hash of content for change detection
func (se *StagedExecutor) calculateContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// createBackupPath creates a unique backup file path
func (se *StagedExecutor) createBackupPath(filePath string) string {
	// Clean the file path for safe backup naming
	cleanPath := strings.ReplaceAll(filePath, "/", "_")
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "_")

	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("%s_%s.backup", cleanPath, timestamp)

	return filepath.Join(se.projectPaths.BackupsDir(), backupName)
}

// ValidateStagedEdits validates that staged edits are still applicable
func (se *StagedExecutor) ValidateStagedEdits(execution *ActionPlanExecution) error {
	for _, edit := range execution.StagedEdits {
		fullPath, err := se.Executor.securePath(edit.FilePath)
		if err != nil {
			return fmt.Errorf("invalid path %s: %w", edit.FilePath, err)
		}

		// Check if file still matches the original hash
		if _, err := os.Stat(fullPath); err == nil {
			data, err := os.ReadFile(fullPath)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", edit.FilePath, err)
			}

			currentHash := se.calculateContentHash(string(data))
			if currentHash != edit.OriginalHash {
				return fmt.Errorf("file %s has been modified since staging", edit.FilePath)
			}
		}
	}

	return nil
}

// GetActionPlanSummary returns a brief summary of an action plan execution
func (se *StagedExecutor) GetActionPlanSummary(execution *ActionPlanExecution) string {
	duration := ""
	if !execution.EndTime.IsZero() {
		duration = fmt.Sprintf(" (%v)", execution.EndTime.Sub(execution.StartTime).Round(time.Second))
	}

	return fmt.Sprintf("Action Plan '%s': %s%s - %d files, %d tasks",
		execution.Plan.Title,
		execution.Status,
		duration,
		len(execution.StagedEdits),
		len(execution.Plan.Tasks))
}

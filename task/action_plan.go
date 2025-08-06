package task

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ActionPlan represents a coordinated set of tasks that should be executed together
type ActionPlan struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Tasks         []Task    `json:"tasks"`
	TestFirst     bool      `json:"test_first"` // Whether to require tests before implementation
	CreatedAt     time.Time `json:"created_at"`
	Status        string    `json:"status"`                   // "planned", "staged", "approved", "applied", "cancelled"
	PreConditions []string  `json:"pre_conditions,omitempty"` // Git status requirements, etc.
}

// ActionPlanExecution represents the execution state of an action plan
type ActionPlanExecution struct {
	Plan           *ActionPlan    `json:"plan"`
	TaskResponses  []TaskResponse `json:"task_responses"`
	StagedEdits    []*StagedEdit  `json:"staged_edits"`
	BackupPaths    []string       `json:"backup_paths"`
	ApprovalNeeded bool           `json:"approval_needed"`
	StartTime      time.Time      `json:"start_time"`
	EndTime        time.Time      `json:"end_time"`
	Status         string         `json:"status"` // "preparing", "staged", "applying", "completed", "failed"
}

// StagedEdit represents a file edit that has been prepared but not yet applied
type StagedEdit struct {
	FilePath     string `json:"file_path"`
	OriginalHash string `json:"original_hash"`
	NewContent   string `json:"new_content"`
	DiffPreview  string `json:"diff_preview"`
	BackupPath   string `json:"backup_path"`
	Task         *Task  `json:"task"`
}

// TestRequirement represents a test that must exist before implementing changes
type TestRequirement struct {
	TestFile    string `json:"test_file"`
	TestName    string `json:"test_name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ActionPlanParser extracts action plans from LLM responses
type ActionPlanParser struct {
	taskRegex *regexp.Regexp
	planRegex *regexp.Regexp
}

// NewActionPlanParser creates a new action plan parser

// ParseActionPlan extracts an action plan from LLM response text

// inferPlanDescription creates a description based on the tasks in the plan

// ValidateActionPlan validates an action plan for consistency and safety

// RequiresApproval checks if the action plan requires user approval
func (plan *ActionPlan) RequiresApproval() bool {
	for _, task := range plan.Tasks {
		if task.RequiresConfirmation() {
			return true
		}
	}
	return false
}

// GetEditedFiles returns a list of files that will be modified by this plan
func (plan *ActionPlan) GetEditedFiles() []string {
	files := make(map[string]bool)
	for _, task := range plan.Tasks {
		if task.Type == TaskTypeEditFile && task.Path != "" {
			files[task.Path] = true
		}
	}

	result := make([]string, 0, len(files))
	for file := range files {
		result = append(result, file)
	}
	return result
}

// GetCommandCount returns the number of shell commands in this plan
func (plan *ActionPlan) GetCommandCount() int {
	count := 0
	for _, task := range plan.Tasks {
		if task.Type == TaskTypeRunShell {
			count++
		}
	}
	return count
}

// Summary returns a human-readable summary of the action plan
func (plan *ActionPlan) Summary() string {
	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("Action Plan: %s\n", plan.Title))
	if plan.Description != "" {
		summary.WriteString(fmt.Sprintf("Description: %s\n", plan.Description))
	}

	summary.WriteString(fmt.Sprintf("Tasks: %d\n", len(plan.Tasks)))

	editFiles := plan.GetEditedFiles()
	if len(editFiles) > 0 {
		summary.WriteString(fmt.Sprintf("Files to edit: %s\n", strings.Join(editFiles, ", ")))
	}

	commands := plan.GetCommandCount()
	if commands > 0 {
		summary.WriteString(fmt.Sprintf("Shell commands: %d\n", commands))
	}

	if plan.TestFirst {
		summary.WriteString("Test-first policy: Enabled\n")
	}

	return summary.String()
}

// generatePlanID generates a unique ID for an action plan

// DetectTestFirst analyzes the plan to determine if test-first approach should be used
func (plan *ActionPlan) DetectTestFirst() bool {
	// Look for test files in the tasks
	hasTestFiles := false
	hasSourceFiles := false

	for _, task := range plan.Tasks {
		if task.Type == TaskTypeEditFile && task.Path != "" {
			if strings.Contains(task.Path, "_test.") ||
				strings.Contains(task.Path, ".test.") ||
				strings.Contains(task.Path, "/test/") {
				hasTestFiles = true
			} else {
				hasSourceFiles = true
			}
		}
	}

	// If we have both test files and source files, suggest test-first
	return hasTestFiles && hasSourceFiles
}

// GetTestTasks returns tasks that involve test files
func (plan *ActionPlan) GetTestTasks() []Task {
	var testTasks []Task

	for _, task := range plan.Tasks {
		if task.Type == TaskTypeEditFile && task.Path != "" {
			if strings.Contains(task.Path, "_test.") ||
				strings.Contains(task.Path, ".test.") ||
				strings.Contains(task.Path, "/test/") {
				testTasks = append(testTasks, task)
			}
		}
	}

	return testTasks
}

// GetImplementationTasks returns tasks that involve non-test files
func (plan *ActionPlan) GetImplementationTasks() []Task {
	var implTasks []Task

	for _, task := range plan.Tasks {
		if task.Type == TaskTypeEditFile && task.Path != "" {
			if !strings.Contains(task.Path, "_test.") &&
				!strings.Contains(task.Path, ".test.") &&
				!strings.Contains(task.Path, "/test/") {
				implTasks = append(implTasks, task)
			}
		} else if task.Type != TaskTypeEditFile {
			// Include non-edit tasks (reads, shell commands) with implementation
			implTasks = append(implTasks, task)
		}
	}

	return implTasks
}

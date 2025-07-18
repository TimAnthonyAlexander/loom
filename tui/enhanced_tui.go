package tui

import (
	"fmt"
	"loom/context"
	"loom/git"
	"loom/security"
	"loom/task"
	"loom/undo"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Enhanced view modes that include new functionality
type enhancedViewMode int

const (
	viewEnhancedChat enhancedViewMode = iota
	viewEnhancedFileTree
	viewEnhancedActionPlan
	viewEnhancedBatchApproval
	viewEnhancedGitStatus
	viewEnhancedUndo
)

// EnhancedModel extends the base model with Milestone 5 features
type EnhancedModel struct {
	*model // Embed the base model

	// Enhanced context and staging
	contextManager *context.ContextManager
	stagedExecutor *task.StagedExecutor
	gitRepo        *git.Repository
	undoManager    *undo.UndoManager
	secretDetector *security.SecretDetector

	// Action plan management
	currentActionPlan *task.ActionPlan
	planExecution     *task.ActionPlanExecution

	// Enhanced view state
	enhancedView         enhancedViewMode
	batchApprovalScroll  int
	gitStatusScroll      int
	undoHistoryScroll    int
	showingBatchApproval bool
	selectedEditIndex    int

	// Configuration
	enableTestFirst  bool
	maxContextTokens int
}

// BatchApprovalMsg represents a batch approval request
type BatchApprovalMsg struct {
	Execution *task.ActionPlanExecution
	Preview   string
}

// ActionPlanCreatedMsg indicates a new action plan was created
type ActionPlanCreatedMsg struct {
	Plan      *task.ActionPlan
	Execution *task.ActionPlanExecution
}

// GitStatusMsg contains Git status information
type GitStatusMsg struct {
	Status *git.RepositoryStatus
}

// UndoMsg represents an undo operation result
type UndoMsg struct {
	Action *undo.UndoAction
	Error  error
}

// Enhanced styles for the new UI components
var (
	actionPlanStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1).
			Margin(1)

	stagedEditStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("214")).
			Padding(0, 1).
			Margin(0, 0, 1, 0)

	gitStatusStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("28")).
			Padding(1).
			Margin(1)

	undoActionStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("196")).
			Padding(0, 1).
			Margin(0, 0, 1, 0)
)

// NewEnhancedModel creates a new enhanced TUI model
func NewEnhancedModel(baseModel *model, workspacePath string, maxContextTokens int) (*EnhancedModel, error) {
	// Initialize enhanced components
	contextManager := context.NewContextManager(baseModel.index, maxContextTokens)

	stagedExecutor, err := task.NewStagedExecutor(baseModel.taskExecutor, contextManager, workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create staged executor: %w", err)
	}

	gitRepo, err := git.NewRepository(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize git repository: %w", err)
	}

	undoManager, err := undo.NewUndoManager(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create undo manager: %w", err)
	}

	secretDetector := security.NewSecretDetector()

	return &EnhancedModel{
		model:             baseModel,
		contextManager:    contextManager,
		stagedExecutor:    stagedExecutor,
		gitRepo:           gitRepo,
		undoManager:       undoManager,
		secretDetector:    secretDetector,
		enhancedView:      viewEnhancedChat,
		maxContextTokens:  maxContextTokens,
		enableTestFirst:   false, // Can be configured
		selectedEditIndex: 0,
	}, nil
}

// Update handles enhanced TUI updates
func (em *EnhancedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle enhanced messages first
	switch msg := msg.(type) {
	case ActionPlanCreatedMsg:
		em.currentActionPlan = msg.Plan
		em.planExecution = msg.Execution

		// Switch to batch approval view if needed
		if msg.Execution.ApprovalNeeded {
			return em.showBatchApproval()
		}
		return em, nil

	case BatchApprovalMsg:
		em.planExecution = msg.Execution
		em.showingBatchApproval = true
		em.enhancedView = viewEnhancedBatchApproval
		return em, nil

	case GitStatusMsg:
		// Update git status display
		return em, nil

	case UndoMsg:
		if msg.Error == nil {
			// Add success message to chat
			em.messages = append(em.messages, fmt.Sprintf("✅ Undone: %s", msg.Action.Description))
		} else {
			em.messages = append(em.messages, fmt.Sprintf("❌ Undo failed: %v", msg.Error))
		}
		return em, nil

	case tea.KeyMsg:
		return em.handleEnhancedKeys(msg)
	}

	// Delegate to base model for standard handling
	baseModel, cmd := em.model.Update(msg)
	em.model = baseModel.(*model)
	return em, cmd
}

// handleEnhancedKeys handles enhanced keyboard input
func (em *EnhancedModel) handleEnhancedKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle batch approval view
	if em.showingBatchApproval {
		return em.handleBatchApprovalKeys(msg)
	}

	// Handle enhanced view switching
	switch msg.String() {
	case "ctrl+p": // Action Plan view
		em.enhancedView = viewEnhancedActionPlan
		return em, nil
	case "ctrl+g": // Git status view
		em.enhancedView = viewEnhancedGitStatus
		return em, em.updateGitStatus()
	case "ctrl+u": // Undo view
		em.enhancedView = viewEnhancedUndo
		return em, nil
	case "ctrl+z": // Quick undo
		return em, em.performUndo()
	case "/commit":
		if em.enhancedView == viewEnhancedChat {
			return em, em.handleCommitCommand()
		}
	case "/undo":
		if em.enhancedView == viewEnhancedChat {
			return em, em.performUndo()
		}
	case "/git":
		if em.enhancedView == viewEnhancedChat {
			return em, em.showGitStatus()
		}
	}

	// Handle scrolling in enhanced views
	switch em.enhancedView {
	case viewEnhancedBatchApproval:
		return em.handleBatchApprovalScroll(msg)
	case viewEnhancedGitStatus:
		return em.handleGitStatusScroll(msg)
	case viewEnhancedUndo:
		return em.handleUndoScroll(msg)
	}

	// Delegate to base model
	baseModel, cmd := em.model.Update(msg)
	em.model = baseModel.(*model)
	return em, cmd
}

// handleBatchApprovalKeys handles keys in batch approval mode
func (em *EnhancedModel) handleBatchApprovalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "a", "A": // Approve all
		return em, em.approveActionPlan()
	case "r", "R": // Reject all
		return em, em.rejectActionPlan()
	case "enter": // Approve selected edit
		return em, em.approveSelectedEdit()
	case "x", "X": // Reject selected edit
		return em, em.rejectSelectedEdit()
	case "up", "k":
		if em.selectedEditIndex > 0 {
			em.selectedEditIndex--
		}
		return em, nil
	case "down", "j":
		if em.planExecution != nil && em.selectedEditIndex < len(em.planExecution.StagedEdits)-1 {
			em.selectedEditIndex++
		}
		return em, nil
	case "escape", "q":
		em.showingBatchApproval = false
		em.enhancedView = viewEnhancedChat
		return em, nil
	case "ctrl+c":
		return em, tea.Quit
	}
	return em, nil
}

// handleBatchApprovalScroll handles scrolling in batch approval view
func (em *EnhancedModel) handleBatchApprovalScroll(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if em.batchApprovalScroll > 0 {
			em.batchApprovalScroll--
		}
	case "down":
		em.batchApprovalScroll++
	case "pgup":
		em.batchApprovalScroll -= 10
		if em.batchApprovalScroll < 0 {
			em.batchApprovalScroll = 0
		}
	case "pgdn":
		em.batchApprovalScroll += 10
	}
	return em, nil
}

// handleGitStatusScroll handles scrolling in git status view
func (em *EnhancedModel) handleGitStatusScroll(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if em.gitStatusScroll > 0 {
			em.gitStatusScroll--
		}
	case "down":
		em.gitStatusScroll++
	}
	return em, nil
}

// handleUndoScroll handles scrolling in undo view
func (em *EnhancedModel) handleUndoScroll(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if em.undoHistoryScroll > 0 {
			em.undoHistoryScroll--
		}
	case "down":
		em.undoHistoryScroll++
	}
	return em, nil
}

// View renders the enhanced TUI
func (em *EnhancedModel) View() string {
	if em.showingBatchApproval {
		return em.renderBatchApproval()
	}

	switch em.enhancedView {
	case viewEnhancedActionPlan:
		return em.renderActionPlanView()
	case viewEnhancedGitStatus:
		return em.renderGitStatusView()
	case viewEnhancedUndo:
		return em.renderUndoView()
	default:
		// Render base view with enhanced status bar
		baseView := em.model.View()
		return em.addEnhancedStatusBar(baseView)
	}
}

// renderBatchApproval renders the batch approval interface
func (em *EnhancedModel) renderBatchApproval() string {
	if em.planExecution == nil {
		return "No action plan to approve"
	}

	var content strings.Builder

	// Header
	content.WriteString("📋 Action Plan Approval\n")
	content.WriteString(strings.Repeat("=", 60) + "\n\n")

	// Plan summary
	content.WriteString(actionPlanStyle.Render(em.planExecution.Plan.Summary()))
	content.WriteString("\n")

	// Staged edits
	if len(em.planExecution.StagedEdits) > 0 {
		content.WriteString("📝 Staged Changes:\n\n")

		for i, edit := range em.planExecution.StagedEdits {
			style := stagedEditStyle
			if i == em.selectedEditIndex {
				style = style.BorderForeground(lipgloss.Color("226")).Bold(true)
			}

			// Show only file name and status, not the actual diff preview
			editContent := fmt.Sprintf("File: %s\nChanges prepared for approval", edit.FilePath)
			content.WriteString(style.Render(editContent))
			content.WriteString("\n")
		}
	}

	// Shell commands
	shellCommands := 0
	for _, planTask := range em.planExecution.Plan.Tasks {
		if planTask.Type == task.TaskTypeRunShell {
			shellCommands++
			content.WriteString(fmt.Sprintf("🔧 Shell Command %d: %s\n", shellCommands, planTask.Command))
		}
	}

	// Controls
	content.WriteString("\n" + strings.Repeat("-", 60) + "\n")
	content.WriteString("Controls:\n")
	content.WriteString("• A - Approve all changes\n")
	content.WriteString("• R - Reject all changes\n")
	content.WriteString("• ↑/↓ - Navigate edits\n")
	content.WriteString("• Enter - Approve selected edit\n")
	content.WriteString("• X - Reject selected edit\n")
	content.WriteString("• Esc/Q - Cancel and return to chat\n")

	return content.String()
}

// renderActionPlanView renders the action plan view
func (em *EnhancedModel) renderActionPlanView() string {
	var content strings.Builder

	content.WriteString("🎯 Action Plan View\n")
	content.WriteString(strings.Repeat("=", 50) + "\n\n")

	if em.currentActionPlan == nil {
		content.WriteString("No active action plan.\n")
		content.WriteString("Action plans are created when the AI suggests multiple coordinated tasks.\n")
	} else {
		content.WriteString(actionPlanStyle.Render(em.currentActionPlan.Summary()))

		if em.planExecution != nil {
			content.WriteString("\n📊 Execution Status:\n")
			content.WriteString(fmt.Sprintf("Status: %s\n", em.planExecution.Status))
			content.WriteString(fmt.Sprintf("Staged Edits: %d\n", len(em.planExecution.StagedEdits)))

			if !em.planExecution.StartTime.IsZero() {
				if em.planExecution.EndTime.IsZero() {
					duration := time.Since(em.planExecution.StartTime)
					content.WriteString(fmt.Sprintf("Duration: %v (ongoing)\n", duration.Round(time.Second)))
				} else {
					duration := em.planExecution.EndTime.Sub(em.planExecution.StartTime)
					content.WriteString(fmt.Sprintf("Duration: %v\n", duration.Round(time.Second)))
				}
			}
		}
	}

	content.WriteString("\nPress Tab to return to chat, Ctrl+G for Git status")
	return content.String()
}

// renderGitStatusView renders the Git status view
func (em *EnhancedModel) renderGitStatusView() string {
	var content strings.Builder

	content.WriteString("📋 Git Status\n")
	content.WriteString(strings.Repeat("=", 50) + "\n\n")

	// This would be populated by the git status command
	content.WriteString("Git status information will be displayed here.\n")
	content.WriteString("(Implementation requires async Git status loading)\n")

	content.WriteString("\nPress Tab to return to chat")
	return content.String()
}

// renderUndoView renders the undo history view
func (em *EnhancedModel) renderUndoView() string {
	var content strings.Builder

	content.WriteString("⏪ Undo History\n")
	content.WriteString(strings.Repeat("=", 50) + "\n\n")

	history := em.undoManager.GetUndoHistory()
	if len(history) == 0 {
		content.WriteString("No undo history available.\n")
	} else {
		// Show most recent actions first
		for i := len(history) - 1; i >= 0; i-- {
			action := history[i]

			style := undoActionStyle
			if action.Applied && !action.Undone {
				style = style.BorderForeground(lipgloss.Color("28")) // Green for active
			} else if action.Undone {
				style = style.BorderForeground(lipgloss.Color("8")) // Gray for undone
			}

			actionContent := fmt.Sprintf("%s: %s\n%s",
				action.Type, action.Description,
				action.Timestamp.Format("2006-01-02 15:04:05"))

			content.WriteString(style.Render(actionContent))
			content.WriteString("\n")
		}
	}

	content.WriteString("\nPress Ctrl+Z to undo last action, Tab to return to chat")
	return content.String()
}

// addEnhancedStatusBar adds an enhanced status bar to the base view
func (em *EnhancedModel) addEnhancedStatusBar(baseView string) string {
	var statusParts []string

	// Git status
	if em.gitRepo.IsGitRepository() {
		statusParts = append(statusParts, "Git: Available")
	}

	// Current action plan
	if em.currentActionPlan != nil {
		statusParts = append(statusParts, fmt.Sprintf("Plan: %s", em.currentActionPlan.Status))
	}

	// Context info
	statusParts = append(statusParts, fmt.Sprintf("Context: %d tokens max", em.maxContextTokens))

	// Secret detection
	statusParts = append(statusParts, fmt.Sprintf("Security: %d patterns", em.secretDetector.GetPatternCount()))

	if len(statusParts) > 0 {
		statusBar := "📊 " + strings.Join(statusParts, " | ")
		return baseView + "\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Render(statusBar)
	}

	return baseView
}

// Enhanced commands

// showBatchApproval shows the batch approval interface
func (em *EnhancedModel) showBatchApproval() (tea.Model, tea.Cmd) {
	em.showingBatchApproval = true
	em.enhancedView = viewEnhancedBatchApproval
	em.selectedEditIndex = 0
	return em, nil
}

// approveActionPlan approves the entire action plan
func (em *EnhancedModel) approveActionPlan() tea.Cmd {
	return func() tea.Msg {
		if em.planExecution == nil {
			return UndoMsg{Error: fmt.Errorf("no action plan to approve")}
		}

		// Apply the action plan
		if err := em.stagedExecutor.ApplyActionPlan(em.planExecution); err != nil {
			return UndoMsg{Error: fmt.Errorf("failed to apply action plan: %w", err)}
		}

		// Record undo actions for each file edit
		for _, edit := range em.planExecution.StagedEdits {
			if action, err := em.undoManager.RecordFileEdit(edit.FilePath,
				fmt.Sprintf("Action plan: %s", em.currentActionPlan.Title)); err == nil {
				em.undoManager.MarkApplied(action.ID)
			}
		}

		// Stage files in Git if available
		if em.gitRepo.IsGitRepository() {
			editedFiles := em.currentActionPlan.GetEditedFiles()
			if len(editedFiles) > 0 {
				em.gitRepo.StageFiles(editedFiles)
			}
		}

		return UndoMsg{Action: &undo.UndoAction{
			Description: fmt.Sprintf("Applied action plan: %s", em.currentActionPlan.Title),
		}}
	}
}

// rejectActionPlan rejects the action plan
func (em *EnhancedModel) rejectActionPlan() tea.Cmd {
	return func() tea.Msg {
		em.showingBatchApproval = false
		em.enhancedView = viewEnhancedChat
		em.currentActionPlan = nil
		em.planExecution = nil
		return UndoMsg{Action: &undo.UndoAction{
			Description: "Rejected action plan",
		}}
	}
}

// approveSelectedEdit approves only the selected edit
func (em *EnhancedModel) approveSelectedEdit() tea.Cmd {
	return func() tea.Msg {
		// Implementation would approve individual edit
		return UndoMsg{Action: &undo.UndoAction{
			Description: "Approved individual edit",
		}}
	}
}

// rejectSelectedEdit rejects the selected edit
func (em *EnhancedModel) rejectSelectedEdit() tea.Cmd {
	return func() tea.Msg {
		// Implementation would reject individual edit
		return UndoMsg{Action: &undo.UndoAction{
			Description: "Rejected individual edit",
		}}
	}
}

// updateGitStatus updates the Git status
func (em *EnhancedModel) updateGitStatus() tea.Cmd {
	return func() tea.Msg {
		status, err := em.gitRepo.GetStatus()
		if err != nil {
			return UndoMsg{Error: fmt.Errorf("failed to get git status: %w", err)}
		}
		return GitStatusMsg{Status: status}
	}
}

// performUndo performs an undo operation
func (em *EnhancedModel) performUndo() tea.Cmd {
	return func() tea.Msg {
		action, err := em.undoManager.UndoLast()
		return UndoMsg{Action: action, Error: err}
	}
}

// showGitStatus shows Git status in chat
func (em *EnhancedModel) showGitStatus() tea.Cmd {
	return func() tea.Msg {
		status, err := em.gitRepo.GetStatus()
		if err != nil {
			return UndoMsg{Error: fmt.Errorf("failed to get git status: %w", err)}
		}

		// Add git status to chat messages
		statusText := status.FormatStatus()
		return UndoMsg{Action: &undo.UndoAction{
			Description: fmt.Sprintf("Git Status: %s", statusText),
		}}
	}
}

// handleCommitCommand handles the /commit command
func (em *EnhancedModel) handleCommitCommand() tea.Cmd {
	return func() tea.Msg {
		// Implementation would show commit dialog
		return UndoMsg{Action: &undo.UndoAction{
			Description: "Commit dialog opened",
		}}
	}
}

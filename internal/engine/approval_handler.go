package engine

import (
	"fmt"
	"sync"

	"github.com/loom/loom/internal/tool"
)

// ApprovalHandler manages user approval and choice interactions for the Engine.
type ApprovalHandler struct {
	bridge           UIBridge
	approvals        map[string]chan bool
	choices          map[string]chan int
	approvalMu       sync.Mutex
	autoApproveShell bool
	autoApproveEdits bool
}

// NewApprovalHandler creates a new approval handler.
func NewApprovalHandler(bridge UIBridge) *ApprovalHandler {
	return &ApprovalHandler{
		bridge:    bridge,
		approvals: make(map[string]chan bool),
		choices:   make(map[string]chan int),
	}
}

// SetAutoApprove toggles auto-approval behaviors based on settings.
func (ah *ApprovalHandler) SetAutoApprove(shell bool, edits bool) {
	ah.approvalMu.Lock()
	defer ah.approvalMu.Unlock()
	ah.autoApproveShell = shell
	ah.autoApproveEdits = edits
}

// SetBridge updates the UI bridge for the approval handler.
func (ah *ApprovalHandler) SetBridge(bridge UIBridge) {
	ah.approvalMu.Lock()
	defer ah.approvalMu.Unlock()
	ah.bridge = bridge
}

// ResolveApproval resolves a pending approval request.
func (ah *ApprovalHandler) ResolveApproval(id string, approved bool) {
	ah.approvalMu.Lock()
	defer ah.approvalMu.Unlock()

	if ch, ok := ah.approvals[id]; ok {
		ch <- approved
		delete(ah.approvals, id)
	}
}

// ResolveChoice resolves a pending choice request.
func (ah *ApprovalHandler) ResolveChoice(id string, selectedIndex int) {
	ah.approvalMu.Lock()
	defer ah.approvalMu.Unlock()

	if ch, ok := ah.choices[id]; ok {
		ch <- selectedIndex
		delete(ah.choices, id)
	}
}

// UserApproved prompts for approval and waits for the response.
func (ah *ApprovalHandler) UserApproved(toolCall *tool.ToolCall, diff string) bool {
	// Auto-approval rules
	if toolCall != nil {
		if (toolCall.Name == "run_shell" || toolCall.Name == "apply_shell") && ah.autoApproveShell {
			return true
		}
		if (toolCall.Name == "edit_file" || toolCall.Name == "apply_edit") && ah.autoApproveEdits {
			return true
		}
	}

	summary := fmt.Sprintf("Tool: %s", toolCall.Name)

	// Create a channel for the response
	responseCh := make(chan bool)

	// Register the approval request
	ah.approvalMu.Lock()
	ah.approvals[toolCall.ID] = responseCh
	ah.approvalMu.Unlock()

	// Ask the bridge for approval
	ah.bridge.PromptApproval(toolCall.ID, summary, diff)

	// Wait for response
	approved := <-responseCh
	return approved
}

// UserChoice prompts for a choice and waits for the response.
func (ah *ApprovalHandler) UserChoice(toolCall *tool.ToolCall, question string, options []string) int {
	// Create a channel for the response
	responseCh := make(chan int)

	// Register the choice request
	ah.approvalMu.Lock()
	ah.choices[toolCall.ID] = responseCh
	ah.approvalMu.Unlock()

	// Ask the bridge for a choice (this will always return -1 for async handling)
	ah.bridge.PromptChoice(toolCall.ID, question, options)

	// Wait for response
	selected := <-responseCh
	return selected
}

// IsAutoApproveEnabled returns the current auto-approval settings.
func (ah *ApprovalHandler) IsAutoApproveEnabled() (shell, edits bool) {
	ah.approvalMu.Lock()
	defer ah.approvalMu.Unlock()
	return ah.autoApproveShell, ah.autoApproveEdits
}

package toolexec

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/loom/loom/internal/engine"
	"github.com/loom/loom/internal/memory"
	"github.com/loom/loom/internal/tool"
	"github.com/loom/loom/internal/workflow"
)

// ApprovalService handles user approvals for tools
type ApprovalService interface {
	UserApproved(toolCall *tool.ToolCall, diff string) bool
	UserChoice(toolCall *tool.ToolCall, question string, options []string) int
}

// UnifiedToolExecutor consolidates all tool execution logic
// This eliminates 300+ lines of duplication between orchestrator and handlers
type UnifiedToolExecutor struct {
	registry         *tool.Registry
	bridge           engine.UIBridge
	workflow         *workflow.Store
	approvals        ApprovalService
	workspaceDir     string
	autoApproveShell bool
	autoApproveEdits bool
}

// NewUnifiedToolExecutor creates a comprehensive tool executor
func NewUnifiedToolExecutor(
	registry *tool.Registry,
	bridge engine.UIBridge,
	wf *workflow.Store,
	approvals ApprovalService,
	workspaceDir string,
) *UnifiedToolExecutor {
	return &UnifiedToolExecutor{
		registry:     registry,
		bridge:       bridge,
		workflow:     wf,
		approvals:    approvals,
		workspaceDir: workspaceDir,
	}
}

// SetAutoApprove configures auto-approval settings
func (ute *UnifiedToolExecutor) SetAutoApprove(shell, edits bool) {
	ute.autoApproveShell = shell
	ute.autoApproveEdits = edits
}

// ExecuteResult represents the result of tool execution
type ExecuteResult struct {
	Content      string
	Safe         bool
	Diff         string
	ShouldReturn bool // Whether to return from processing loop
}

// ExecuteToolCall handles the complete tool execution flow
// This is the single implementation that replaces duplicated logic in orchestrator and handlers
func (ute *UnifiedToolExecutor) ExecuteToolCall(
	ctx context.Context,
	toolCall *tool.ToolCall,
	convo *memory.Conversation,
) (*ExecuteResult, error) {
	// Record tool_use event to workflow state
	if ute.workflow != nil {
		_ = ute.workflow.ApplyEvent(ctx, map[string]any{
			"ts":   time.Now().Unix(),
			"type": "tool_use",
			"tool": toolCall.Name,
			"args": string(toolCall.Args),
			"id":   toolCall.ID,
		})
	}

	// Record the assistant tool_use in conversation for Anthropic
	if convo != nil {
		convo.AddAssistantToolUse(toolCall.Name, toolCall.ID, string(toolCall.Args))
	}

	// Execute the tool
	execResult, err := ute.registry.InvokeToolCall(ctx, toolCall)
	if err != nil {
		errorMsg := fmt.Sprintf("Error executing tool %s: %v", toolCall.Name, err)
		// Attach as tool_result with the same tool_use_id for Anthropic
		if convo != nil {
			convo.AddToolResult(toolCall.Name, toolCall.ID, errorMsg)
		}
		if ute.bridge != nil {
			ute.bridge.SendChat("system", errorMsg)
		}
		return nil, err
	}

	// Record tool_result summary
	if ute.workflow != nil {
		_ = ute.workflow.ApplyEvent(ctx, map[string]any{
			"ts":      time.Now().Unix(),
			"type":    "tool_result",
			"tool":    toolCall.Name,
			"ok":      err == nil,
			"summary": strings.TrimSpace(execResult.Content),
			"id":      toolCall.ID,
		})
	}

	// Hint UI to open file if tool was file-related
	ute.hintUIFileOpen(toolCall)

	result := &ExecuteResult{
		Content: execResult.Content,
		Safe:    execResult.Safe,
		Diff:    execResult.Diff,
	}

	// Handle different tool execution flows
	if !execResult.Safe && toolCall.Name == "user_choice" {
		ute.handleUserChoice(toolCall, convo)
	} else if !execResult.Safe {
		// Regular approval path for other tools
		approved := ute.handleToolApproval(ctx, toolCall, execResult)
		payload := map[string]any{
			"tool":     toolCall.Name,
			"approved": approved,
			"diff":     execResult.Diff,
			"message":  execResult.Content,
		}
		b, _ := json.Marshal(payload)
		if convo != nil {
			convo.AddToolResult(toolCall.Name, toolCall.ID, string(b))
		}

		// Auto-apply edits if approved and auto-approve is enabled
		if approved && ute.autoApproveEdits && toolCall.Name == "edit_file" {
			ute.autoApplyEdit(ctx, toolCall)
		}
	} else {
		// Safe tool: add to conversation and show in UI
		if convo != nil {
			convo.AddToolResult(toolCall.Name, toolCall.ID, execResult.Content)
		}
		// Send tool result to UI for immediate display
		if strings.TrimSpace(execResult.Content) != "" && ute.bridge != nil {
			ute.bridge.SendChat("tool", execResult.Content)
		}
	}

	return result, nil
}

// hintUIFileOpen suggests opening a file in UI for file-related tools
func (ute *UnifiedToolExecutor) hintUIFileOpen(toolCall *tool.ToolCall) {
	if ute.bridge == nil {
		return
	}

	// Try to extract a path field from args JSON for known tools
	type pathArg struct {
		Path string `json:"path"`
	}
	var pa pathArg
	_ = json.Unmarshal(toolCall.Args, &pa)
	if pa.Path == "" {
		// Some tools may use "file" key
		var alt map[string]any
		if json.Unmarshal(toolCall.Args, &alt) == nil {
			if v, ok := alt["file"].(string); ok && strings.TrimSpace(v) != "" {
				pa.Path = v
			}
		}
	}
	if strings.TrimSpace(pa.Path) != "" &&
		(toolCall.Name == "read_file" || toolCall.Name == "edit_file" || toolCall.Name == "apply_edit") {
		ute.bridge.OpenFileInUI(pa.Path)
	}
}

// handleUserChoice processes user choice tool calls
func (ute *UnifiedToolExecutor) handleUserChoice(toolCall *tool.ToolCall, convo *memory.Conversation) {
	if convo == nil || ute.approvals == nil {
		return
	}

	// Parse the tool args to extract question and options
	type userChoiceArgs struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}
	var args userChoiceArgs
	if err := json.Unmarshal(toolCall.Args, &args); err == nil {
		selectedIndex := ute.approvals.UserChoice(toolCall, args.Question, args.Options)
		if selectedIndex >= 0 && selectedIndex < len(args.Options) {
			selectedOption := args.Options[selectedIndex]
			response := map[string]any{
				"selected_option": selectedOption,
				"selected_index":  selectedIndex,
			}
			responseJson, _ := json.Marshal(response)
			convo.AddToolResult(toolCall.Name, toolCall.ID, string(responseJson))
		} else {
			convo.AddToolResult(toolCall.Name, toolCall.ID, "Invalid choice selection")
		}
	} else {
		convo.AddToolResult(toolCall.Name, toolCall.ID, "Error parsing choice arguments")
	}
}

// handleToolApproval manages the approval process for unsafe tools
func (ute *UnifiedToolExecutor) handleToolApproval(
	ctx context.Context,
	toolCall *tool.ToolCall,
	execResult *tool.ExecutionResult,
) bool {
	// Auto-approval rules
	if (toolCall.Name == "run_shell" || toolCall.Name == "apply_shell") && ute.autoApproveShell {
		return true
	}
	if (toolCall.Name == "edit_file" || toolCall.Name == "apply_edit") && ute.autoApproveEdits {
		return true
	}

	approved := false
	if ute.approvals != nil {
		approved = ute.approvals.UserApproved(toolCall, execResult.Diff)
	}

	if ute.workflow != nil {
		_ = ute.workflow.ApplyEvent(ctx, map[string]any{
			"ts":     time.Now().Unix(),
			"type":   "approval",
			"tool":   toolCall.Name,
			"status": map[bool]string{true: "granted", false: "denied"}[approved],
			"id":     toolCall.ID,
		})
	}
	return approved
}

// autoApplyEdit automatically applies an approved edit
func (ute *UnifiedToolExecutor) autoApplyEdit(ctx context.Context, toolCall *tool.ToolCall) {
	applyCall := &tool.ToolCall{ID: toolCall.ID + ":apply", Name: "apply_edit", Args: toolCall.Args}
	applyResult, applyErr := ute.registry.InvokeToolCall(ctx, applyCall)
	if applyErr != nil {
		errorMsg := fmt.Sprintf("Error executing tool %s: %v", applyCall.Name, applyErr)
		if ute.bridge != nil {
			ute.bridge.SendChat("system", errorMsg)
		}
	} else {
		// Hint UI to open the file if path present
		ute.hintUIFileOpen(applyCall)
		// Inform via system chat
		if strings.TrimSpace(applyResult.Content) != "" && ute.bridge != nil {
			ute.bridge.SendChat("system", applyResult.Content)
		}
	}
}

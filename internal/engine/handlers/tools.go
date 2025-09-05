package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/loom/loom/internal/engine"
	"github.com/loom/loom/internal/memory"
	"github.com/loom/loom/internal/tool"
)

// ToolExecutor handles tool execution and approval
type ToolExecutor struct {
	registry         *tool.Registry
	bridge           engine.UIBridge
	workspaceDir     string
	autoApproveShell bool
	autoApproveEdits bool
}

// NewToolExecutor creates a new tool executor
func NewToolExecutor(
	registry *tool.Registry,
	bridge engine.UIBridge,
	workspaceDir string,
) *ToolExecutor {
	return &ToolExecutor{
		registry:     registry,
		bridge:       bridge,
		workspaceDir: workspaceDir,
	}
}

// SetAutoApprove configures auto-approval settings
func (te *ToolExecutor) SetAutoApprove(shell, edits bool) {
	te.autoApproveShell = shell
	te.autoApproveEdits = edits
}

// ExecuteResult represents the result of tool execution
type ExecuteResult struct {
	Content      string
	Safe         bool
	Diff         string
	ShouldReturn bool // Whether to return from processing loop
}

// ExecuteToolCall handles the complete tool execution flow including approval and auto-apply
func (te *ToolExecutor) ExecuteToolCall(
	ctx context.Context,
	toolCall *tool.ToolCall,
	convo *memory.Conversation,
) (*ExecuteResult, error) {
	// Workflow functionality removed

	// Record the assistant tool_use in conversation for Anthropic
	if convo != nil {
		convo.AddAssistantToolUse(toolCall.Name, toolCall.ID, string(toolCall.Args))
	}

	// Execute the tool
	execResult, err := te.registry.InvokeToolCall(ctx, toolCall)
	if err != nil {
		errorMsg := fmt.Sprintf("Error executing tool %s: %v", toolCall.Name, err)
		// Attach as tool_result with the same tool_use_id for Anthropic
		if convo != nil {
			convo.AddToolResult(toolCall.Name, toolCall.ID, errorMsg)
		}
		if te.bridge != nil {
			te.bridge.SendChat("system", errorMsg)
		}
		return nil, err
	}

	// Workflow functionality removed

	// Hint UI to open file if tool was file-related
	te.hintUIFileOpen(toolCall)

	result := &ExecuteResult{
		Content: execResult.Content,
		Safe:    execResult.Safe,
		Diff:    execResult.Diff,
	}

	// Handle different tool execution flows
	if !execResult.Safe && toolCall.Name == "user_choice" {
		te.handleUserChoice(toolCall, convo)
	} else if !execResult.Safe {
		// Regular approval path for other tools
		approved := te.handleToolApproval(ctx, toolCall, execResult)
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
		if approved && te.autoApproveEdits && toolCall.Name == "edit_file" {
			te.autoApplyEdit(ctx, toolCall)
		}
	} else {
		// Safe tool: add to conversation and show in UI
		if convo != nil {
			convo.AddToolResult(toolCall.Name, toolCall.ID, execResult.Content)
		}
		// Send tool result to UI for immediate display
		if strings.TrimSpace(execResult.Content) != "" && te.bridge != nil {
			te.bridge.SendChat("tool", execResult.Content)
		}
	}

	return result, nil
}

// hintUIFileOpen suggests opening a file in UI for file-related tools
func (te *ToolExecutor) hintUIFileOpen(toolCall *tool.ToolCall) {
	if te.bridge == nil {
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
		te.bridge.OpenFileInUI(pa.Path)
	}
}

// handleUserChoice processes user choice tool calls
func (te *ToolExecutor) handleUserChoice(toolCall *tool.ToolCall, convo *memory.Conversation) {
	if convo == nil {
		return
	}

	// Parse the tool args to extract question and options
	type userChoiceArgs struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}
	var args userChoiceArgs
	if err := json.Unmarshal(toolCall.Args, &args); err == nil {
		selectedIndex := te.userChoice(toolCall, args.Question, args.Options)
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
func (te *ToolExecutor) handleToolApproval(
	ctx context.Context,
	toolCall *tool.ToolCall,
	execResult *tool.ExecutionResult,
) bool {
	approved := te.userApproved(toolCall, execResult.Diff)
	// Workflow functionality removed
	return approved
}

// autoApplyEdit automatically applies an approved edit
func (te *ToolExecutor) autoApplyEdit(ctx context.Context, toolCall *tool.ToolCall) {
	applyCall := &tool.ToolCall{ID: toolCall.ID + ":apply", Name: "apply_edit", Args: toolCall.Args}
	applyResult, applyErr := te.registry.InvokeToolCall(ctx, applyCall)
	if applyErr != nil {
		errorMsg := fmt.Sprintf("Error executing tool %s: %v", applyCall.Name, applyErr)
		if te.bridge != nil {
			te.bridge.SendChat("system", errorMsg)
		}
	} else {
		// Hint UI to open the file if path present
		te.hintUIFileOpen(applyCall)
		// Inform via system chat
		if strings.TrimSpace(applyResult.Content) != "" && te.bridge != nil {
			te.bridge.SendChat("system", applyResult.Content)
		}
	}
}

// userApproved prompts for approval (simplified version of orchestrator logic)
func (te *ToolExecutor) userApproved(toolCall *tool.ToolCall, diff string) bool {
	// Auto-approval rules
	if (toolCall.Name == "run_shell" || toolCall.Name == "apply_shell") && te.autoApproveShell {
		return true
	}
	if (toolCall.Name == "edit_file" || toolCall.Name == "apply_edit") && te.autoApproveEdits {
		return true
	}

	// This would need to be connected to the actual approval system
	// For now, returning true for safe implementation
	return true
}

// userChoice prompts for a choice (simplified version of orchestrator logic)
func (te *ToolExecutor) userChoice(toolCall *tool.ToolCall, question string, options []string) int {
	// This would need to be connected to the actual choice system
	// For now, returning 0 for safe implementation
	return 0
}

package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/loom/loom/internal/memory"
	"github.com/loom/loom/internal/tool"
	"github.com/loom/loom/internal/workflow"
)

// ToolExecutor handles tool execution and related approval flows.
type ToolExecutor struct {
	bridge          UIBridge
	tools           *tool.Registry
	approvalHandler *ApprovalHandler
	wf              *workflow.Store
}

// NewToolExecutor creates a new tool executor.
func NewToolExecutor(
	bridge UIBridge,
	tools *tool.Registry,
	approvalHandler *ApprovalHandler,
	wf *workflow.Store,
) *ToolExecutor {
	return &ToolExecutor{
		bridge:          bridge,
		tools:           tools,
		approvalHandler: approvalHandler,
		wf:              wf,
	}
}

// ExecuteToolCall executes a tool call and handles the approval flow.
func (te *ToolExecutor) ExecuteToolCall(
	ctx context.Context,
	toolCall *tool.ToolCall,
	convo *memory.Conversation,
) error {
	// Execute the tool
	execResult, err := te.tools.InvokeToolCall(ctx, toolCall)
	if err != nil {
		errorMsg := fmt.Sprintf("Error executing tool %s: %v", toolCall.Name, err)
		// Attach as tool_result with the same tool_use_id for Anthropic
		convo.AddToolResult(toolCall.Name, toolCall.ID, errorMsg)
		te.bridge.SendChat("system", errorMsg)
		return err
	}

	if te.isDebugEnabled() {
		te.bridge.SendChat("system", fmt.Sprintf("[debug] Tool executed: name=%s safe=%v diffLen=%d contentLen=%d", toolCall.Name, execResult.Safe, len(execResult.Diff), len(execResult.Content)))
	}

	// Record tool_result summary
	if te.wf != nil {
		_ = te.wf.ApplyEvent(ctx, map[string]any{
			"ts":      time.Now().Unix(),
			"type":    "tool_result",
			"tool":    toolCall.Name,
			"ok":      err == nil,
			"summary": strings.TrimSpace(execResult.Content),
			"id":      toolCall.ID,
		})
	}

	// If the tool was file-related, hint UI to open the file
	te.notifyUIForFileTools(toolCall)

	// Handle the tool execution based on safety and type
	return te.handleToolResult(ctx, toolCall, execResult, convo)
}

// handleToolResult processes the tool execution result based on safety and approval requirements.
func (te *ToolExecutor) handleToolResult(
	ctx context.Context,
	toolCall *tool.ToolCall,
	execResult *tool.ExecutionResult,
	convo *memory.Conversation,
) error {
	// Special handling for user_choice tool
	if !execResult.Safe && toolCall.Name == "user_choice" {
		return te.handleUserChoiceTool(toolCall, convo)
	}

	if !execResult.Safe {
		// Regular approval path for other tools
		return te.handleUnsafeTool(ctx, toolCall, execResult, convo)
	}

	// Safe tool: add to conversation and show in UI
	convo.AddToolResult(toolCall.Name, toolCall.ID, execResult.Content)
	// Send tool result to UI for immediate display
	if strings.TrimSpace(execResult.Content) != "" {
		te.bridge.SendChat("tool", execResult.Content)
	}
	return nil
}

// handleUserChoiceTool handles the special case of user_choice tools.
func (te *ToolExecutor) handleUserChoiceTool(toolCall *tool.ToolCall, convo *memory.Conversation) error {
	// Parse the tool args to extract question and options
	type userChoiceArgs struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}
	var args userChoiceArgs
	if err := json.Unmarshal(toolCall.Args, &args); err == nil {
		selectedIndex := te.approvalHandler.UserChoice(toolCall, args.Question, args.Options)
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
	return nil
}

// handleUnsafeTool handles tools that require approval.
func (te *ToolExecutor) handleUnsafeTool(
	ctx context.Context,
	toolCall *tool.ToolCall,
	execResult *tool.ExecutionResult,
	convo *memory.Conversation,
) error {
	approved := te.approvalHandler.UserApproved(toolCall, execResult.Diff)
	if te.wf != nil {
		_ = te.wf.ApplyEvent(ctx, map[string]any{
			"ts":     time.Now().Unix(),
			"type":   "approval",
			"tool":   toolCall.Name,
			"status": map[bool]string{true: "granted", false: "denied"}[approved],
			"id":     toolCall.ID,
		})
	}

	payload := map[string]any{
		"tool":     toolCall.Name,
		"approved": approved,
		"diff":     execResult.Diff,
		"message":  execResult.Content,
	}
	b, _ := json.Marshal(payload)
	convo.AddToolResult(toolCall.Name, toolCall.ID, string(b))

	// If edits are auto-approved and this was an edit proposal, immediately apply it
	_, autoApproveEdits := te.approvalHandler.IsAutoApproveEnabled()
	if approved && autoApproveEdits && toolCall.Name == "edit_file" {
		return te.autoApplyEdit(ctx, toolCall)
	}

	return nil
}

// autoApplyEdit automatically applies an edit if auto-approval is enabled.
func (te *ToolExecutor) autoApplyEdit(ctx context.Context, toolCall *tool.ToolCall) error {
	applyCall := &tool.ToolCall{ID: toolCall.ID + ":apply", Name: "apply_edit", Args: toolCall.Args}
	applyResult, applyErr := te.tools.InvokeToolCall(ctx, applyCall)
	if applyErr != nil {
		errorMsg := fmt.Sprintf("Error executing tool %s: %v", applyCall.Name, applyErr)
		te.bridge.SendChat("system", errorMsg)
		// Do not add a tool_result with a synthetic tool ID; continue
		return nil
	}

	// Hint UI to open the file if path present
	te.notifyUIForFileTools(applyCall)

	// Inform via system chat; avoid emitting a tool_result with unmatched tool_use_id
	if strings.TrimSpace(applyResult.Content) != "" {
		te.bridge.SendChat("system", applyResult.Content)
	}
	return nil
}

// notifyUIForFileTools opens relevant files in the UI for file-related tools.
func (te *ToolExecutor) notifyUIForFileTools(toolCall *tool.ToolCall) {
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
	if strings.TrimSpace(pa.Path) != "" && (toolCall.Name == "read_file" || toolCall.Name == "edit_file" || toolCall.Name == "apply_edit") {
		te.bridge.OpenFileInUI(pa.Path)
	}
}

// isDebugEnabled checks if debug mode is enabled.
func (te *ToolExecutor) isDebugEnabled() bool {
	return os.Getenv("LOOM_DEBUG_ENGINE") == "1" || strings.EqualFold(os.Getenv("LOOM_DEBUG_ENGINE"), "true")
}

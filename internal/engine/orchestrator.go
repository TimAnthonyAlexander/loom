package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Message represents a single message in the chat.
type Message struct {
	Role    string `json:"role"`              // user, assistant, system, function, tool
	Content string `json:"content"`           // text content
	Name    string `json:"name,omitempty"`    // function/tool name when applicable
	ToolID  string `json:"tool_id,omitempty"` // ID for tool invocations
}

// TokenOrToolCall represents a token from LLM or a tool call request.
type TokenOrToolCall struct {
	Token    string
	ToolCall *ToolCall
}

// ToolCall represents an LLM's request to call a tool.
type ToolCall struct {
	ID     string
	Name   string
	Args   json.RawMessage
	IsSafe bool // true if doesn't require approval
}

// UIBridge interfaces with the user interface.
type UIBridge interface {
	SendChat(role, text string)
	PromptApproval(actionID string, summary string, diff string) (approved bool)
}

// ApprovalRequest tracks an outstanding approval request.
type ApprovalRequest struct {
	ID       string
	Summary  string
	Diff     string
	Response chan bool
}

// Engine is the core orchestrator for the Loom system.
type Engine struct {
	bridge     UIBridge
	messages   []Message
	llm        LLM
	mu         sync.RWMutex
	approvals  map[string]chan bool
	approvalMu sync.Mutex
}

// LLM is an interface to abstract different language model providers.
type LLM interface {
	Chat(ctx context.Context,
		messages []Message,
		tools []ToolSchema,
		stream bool,
	) (<-chan TokenOrToolCall, error)
}

// ToolSchema represents the schema for a tool.
type ToolSchema struct {
	Name        string
	Description string
	Schema      map[string]interface{}
}

// New creates a new Engine instance.
func New(llm LLM, bridge UIBridge) *Engine {
	return &Engine{
		llm:       llm,
		bridge:    bridge,
		messages:  []Message{},
		approvals: make(map[string]chan bool),
	}
}

// SetBridge sets the UI bridge for the engine.
func (e *Engine) SetBridge(bridge UIBridge) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bridge = bridge
}

// Enqueue adds a user message and starts the processing loop.
func (e *Engine) Enqueue(message string) {
	// Add user message to history
	e.mu.Lock()
	e.messages = append(e.messages, Message{
		Role:    "user",
		Content: message,
	})
	e.mu.Unlock()

	// Send to UI
	e.bridge.SendChat("user", message)

	// Start processing in a goroutine
	go e.processLoop(context.Background())
}

// ResolveApproval resolves a pending approval request.
func (e *Engine) ResolveApproval(id string, approved bool) {
	e.approvalMu.Lock()
	defer e.approvalMu.Unlock()

	if ch, ok := e.approvals[id]; ok {
		ch <- approved
		delete(e.approvals, id)
	}
}

// processLoop is the main processing loop for the engine.
func (e *Engine) processLoop(ctx context.Context) {
	// Get tools from registry
	tools := []ToolSchema{} // TODO: Get from tool registry

	// Create a new LLM chat stream
	e.mu.RLock()
	messages := append([]Message{}, e.messages...)
	e.mu.RUnlock()

	stream, err := e.llm.Chat(ctx, messages, tools, true)
	if err != nil {
		e.bridge.SendChat("system", "Error: "+err.Error())
		return
	}

	// Process the stream
	var currentMessage Message
	currentMessage.Role = "assistant"

	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-stream:
			if !ok {
				// Stream closed, add the last message if not empty
				if currentMessage.Content != "" {
					e.mu.Lock()
					e.messages = append(e.messages, currentMessage)
					e.mu.Unlock()
				}
				return
			}

			if item.ToolCall != nil {
				// Handle tool call
				_, _ = e.handleToolCall(ctx, item.ToolCall)

				// Continue stream with tool result
				// TODO: Implement tool result handling
			} else {
				// Handle token
				currentMessage.Content += item.Token
				// Send partial update to UI
				// TODO: Implement proper partial update logic
			}
		}
	}
}

// handleToolCall processes a tool call from the LLM.
func (e *Engine) handleToolCall(ctx context.Context, call *ToolCall) (json.RawMessage, error) {
	// For edit_file tools, we need special handling
	if call.Name == "edit_file" {
		return e.handleEditFileCall(ctx, call)
	}

	// Check if approval is needed for other tools
	if !call.IsSafe {
		// Create approval request
		approved := e.requestApproval(call)
		if !approved {
			// User rejected the tool call
			return json.RawMessage(`{"error": "User rejected this action"}`), nil
		}
	}

	// Execute the tool call
	// TODO: Implement tool invocation through registry
	return json.RawMessage(`{}`), nil
}

// handleEditFileCall specifically handles edit_file tool calls with proper diff generation and approval
func (e *Engine) handleEditFileCall(ctx context.Context, call *ToolCall) (json.RawMessage, error) {
	// Parse the edit arguments
	var editArgs struct {
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
		CreateNew bool   `json:"create_new,omitempty"`
	}

	if err := json.Unmarshal(call.Args, &editArgs); err != nil {
		return json.RawMessage(`{"error": "Failed to parse edit arguments"}`), nil
	}

	// Generate a summary for the approval
	summary := "Edit File: " + editArgs.Path
	if editArgs.CreateNew {
		summary = "Create File: " + editArgs.Path
	}

	// Request approval with the proper diff
	// TODO: Get workspace path properly
	workspacePath := "."

	// Create an edit plan to get a proper diff
	plan, err := e.createEditPlan(workspacePath, struct {
		Path      string
		OldString string
		NewString string
		CreateNew bool
	}{
		Path:      editArgs.Path,
		OldString: editArgs.OldString,
		NewString: editArgs.NewString,
		CreateNew: editArgs.CreateNew,
	})
	if err != nil {
		return json.RawMessage(fmt.Sprintf(`{"error": "%s"}`, err.Error())), nil
	}

	// Request approval with the detailed diff
	approved := e.requestApprovalWithDiff(call.ID, summary, plan.Diff)
	if !approved {
		return json.RawMessage(`{"error": "User rejected the edit"}`), nil
	}

	// If approved, execute the edit
	result, err := e.executeEdit(plan)
	if err != nil {
		return json.RawMessage(fmt.Sprintf(`{"error": "Failed to apply edit: %s"}`, err.Error())), nil
	}

	// Convert result to JSON
	resultJSON, _ := json.Marshal(result)
	return resultJSON, nil
}

// createEditPlan creates an edit plan from the edit arguments
func (e *Engine) createEditPlan(workspacePath string, args struct {
	Path      string
	OldString string
	NewString string
	CreateNew bool
}) (*struct {
	FilePath   string
	OldContent string
	NewContent string
	Diff       string
	IsCreation bool
	IsDeletion bool
}, error) {
	// Placeholder implementation - would normally call into the editor package
	// For now, just create a dummy plan
	return &struct {
		FilePath   string
		OldContent string
		NewContent string
		Diff       string
		IsCreation bool
		IsDeletion bool
	}{
		FilePath:   args.Path,
		OldContent: args.OldString,
		NewContent: args.NewString,
		Diff:       fmt.Sprintf("Diff for %s", args.Path),
		IsCreation: args.CreateNew,
		IsDeletion: args.NewString == "",
	}, nil
}

// executeEdit applies an edit plan to the filesystem
func (e *Engine) executeEdit(plan *struct {
	FilePath   string
	OldContent string
	NewContent string
	Diff       string
	IsCreation bool
	IsDeletion bool
}) (*struct {
	Success bool
	Message string
}, error) {
	// Placeholder implementation - would normally write to disk
	return &struct {
		Success bool
		Message string
	}{
		Success: true,
		Message: "Edit applied successfully",
	}, nil
}

// requestApprovalWithDiff requests approval with a detailed diff
func (e *Engine) requestApprovalWithDiff(id string, summary string, diff string) bool {
	// Create a channel for the response
	responseCh := make(chan bool)

	// Register the approval request
	e.approvalMu.Lock()
	e.approvals[id] = responseCh
	e.approvalMu.Unlock()

	// Ask the bridge for approval with the detailed diff
	go e.bridge.PromptApproval(id, summary, diff)

	// Wait for response
	return <-responseCh
}

// requestApproval asks the user for approval of a tool call.
func (e *Engine) requestApproval(call *ToolCall) bool {
	// Create a channel for the response
	responseCh := make(chan bool)

	// Register the approval request
	e.approvalMu.Lock()
	e.approvals[call.ID] = responseCh
	e.approvalMu.Unlock()

	// Ask the bridge for approval
	// TODO: Generate proper summary and diff
	go e.bridge.PromptApproval(call.ID, "Tool: "+call.Name, string(call.Args))

	// Wait for response
	approved := <-responseCh
	return approved
}

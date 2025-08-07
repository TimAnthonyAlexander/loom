package engine

import (
	"context"
	"encoding/json"
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
	// Check if approval is needed
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

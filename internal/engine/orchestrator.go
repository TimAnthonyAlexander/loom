package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/loom/loom/internal/memory"
	"github.com/loom/loom/internal/tool"
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
	EmitAssistant(text string)
	PromptApproval(actionID string, summary string, diff string) (approved bool)
	SetBusy(isBusy bool)
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
	bridge       UIBridge
	messages     []Message
	llm          LLM
	mu           sync.RWMutex
	approvals    map[string]chan bool
	approvalMu   sync.Mutex
	tools        *tool.Registry
	memory       *memory.Project
	workspaceDir string
	llmMu        sync.Mutex
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

// WithRegistry sets the tool registry for the engine.
func (e *Engine) WithRegistry(registry *tool.Registry) *Engine {
	e.tools = registry
	return e
}

// WithMemory sets the project memory for the engine.
func (e *Engine) WithMemory(project *memory.Project) *Engine {
	e.memory = project
	return e
}

// WithWorkspace sets the workspace directory path for the engine.
func (e *Engine) WithWorkspace(path string) *Engine {
	e.workspaceDir = path
	return e
}

// SetBridge sets the UI bridge for the engine.
func (e *Engine) SetBridge(bridge UIBridge) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bridge = bridge
}

// SetLLM updates the LLM used by the engine.
func (e *Engine) SetLLM(llm LLM) {
	e.llmMu.Lock()
	defer e.llmMu.Unlock()
	e.llm = llm
}

// Enqueue adds a user message and starts the processing loop.
func (e *Engine) Enqueue(message string) {
	// Send to UI
	e.bridge.SendChat("user", message)

	// Start processing in a goroutine
	go e.processLoop(context.Background(), message)
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

// UserApproved prompts for approval and waits for the response.
func (e *Engine) UserApproved(toolCall *tool.ToolCall, diff string) bool {
	summary := fmt.Sprintf("Tool: %s", toolCall.Name)

	// Create a channel for the response
	responseCh := make(chan bool)

	// Register the approval request
	e.approvalMu.Lock()
	e.approvals[toolCall.ID] = responseCh
	e.approvalMu.Unlock()

	// Ask the bridge for approval
	e.bridge.PromptApproval(toolCall.ID, summary, diff)

	// Wait for response
	approved := <-responseCh
	return approved
}

// processLoop is the main processing loop for the engine.
func (e *Engine) processLoop(ctx context.Context, userMsg string) error {
	// Indicate busy state to UI during the request lifecycle
	if e.bridge != nil {
		e.bridge.SetBusy(true)
		defer e.bridge.SetBusy(false)
	}
	// Initialize memory if needed
	if e.memory == nil {
		e.bridge.SendChat("system", "Error: Memory not initialized")
		return errors.New("memory not initialized")
	}

	// Initialize tool registry if needed
	if e.tools == nil {
		e.bridge.SendChat("system", "Error: Tool registry not initialized")
		return errors.New("tool registry not initialized")
	}

	// Fetch tool schemas for prompt generation and tool calling
	toolSchemas := e.tools.Schemas()

	// Start or load conversation
	convo := e.memory.StartConversation() // load history & summaries

	// Inject a system prompt once at the beginning of the conversation
	hasSystem := false
	for _, msg := range convo.History() {
		if msg.Role == "system" && msg.Content != "" {
			hasSystem = true
			break
		}
	}
	if !hasSystem {
		convo.AddSystem(GenerateSystemPrompt(toolSchemas))
	}

	// Add latest user message
	convo.AddUser(userMsg)

	// Prepare tool schemas for the adapter
	tools := toolSchemas // get all tool specs

	// Set up the adapter (LLM)
	adapter := e.llm

	// Set a maximum depth to prevent infinite loops
	for depth := 0; depth < 8; depth++ {
		// Convert memory messages to engine messages
		memoryMessages := convo.History()
		engineMessages := make([]Message, 0, len(memoryMessages))

		for _, msg := range memoryMessages {
			engineMsg := Message{
				Role:    msg.Role,
				Content: msg.Content,
				Name:    msg.Name,
				ToolID:  msg.ToolID,
			}
			engineMessages = append(engineMessages, engineMsg)
		}

		// Call the LLM with the conversation history
		stream, err := adapter.Chat(ctx, engineMessages, convertSchemas(tools), true)
		if err != nil {
			e.bridge.SendChat("system", "Error: "+err.Error())
			return err
		}

		// Process the LLM response
		var currentContent string
		var toolCallReceived *tool.ToolCall
		streamEnded := false

		// Process the stream; if slow, emit a one-time notice but do not break
		slowTicker := time.NewTicker(20 * time.Second)
		defer slowTicker.Stop()
		slowNotified := false
	StreamLoop:
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-slowTicker.C:
				if !slowNotified {
					e.bridge.SendChat("system", "Still working...")
					slowNotified = true
				}
			case item, ok := <-stream:
				if !ok {
					// Stream ended
					streamEnded = true
					break StreamLoop
				}
				// Any activity observed; no action needed

				if item.ToolCall != nil {
					// Got a tool call
					toolCallReceived = &tool.ToolCall{
						ID:   item.ToolCall.ID,
						Name: item.ToolCall.Name,
						Args: item.ToolCall.Args,
					}
					// Record the assistant tool_use in conversation for Anthropic
					if convo != nil {
						// Args is json.RawMessage
						convo.AddAssistantToolUse(toolCallReceived.Name, toolCallReceived.ID, string(toolCallReceived.Args))
					}
					break StreamLoop
				}

				// Got a token
				currentContent += item.Token
				e.bridge.EmitAssistant(currentContent)
			}
		}

		// If we got a tool call, execute it
		if toolCallReceived != nil {
			// Execute the tool
			execResult, err := e.tools.InvokeToolCall(ctx, toolCallReceived)
			if err != nil {
				errorMsg := fmt.Sprintf("Error executing tool %s: %v", toolCallReceived.Name, err)
				// Attach as tool_result with the same tool_use_id for Anthropic
				convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, errorMsg)
				e.bridge.SendChat("system", errorMsg)
				return err
			}

			// Check if approval is needed
			if !execResult.Safe {
				if !e.UserApproved(toolCallReceived, execResult.Diff) {
					// User denied the operation
					execResult.Content = "User denied operation"
				}
			}

			// Add the result to the conversation, referencing tool_use_id
			convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, execResult.Content)

			// Continue the loop to get the next assistant message
			continue
		}

		// If we reach here with content, it's the final assistant message
		if currentContent != "" {
			convo.AddAssistant(currentContent)
			return nil
		}

		// If stream ended with no content and no tool call, retry once without streaming
		if streamEnded && currentContent == "" && toolCallReceived == nil {
			// Retry non-streaming once
			e.bridge.SendChat("system", "Retrying without streaming...")
			fallbackStream, err := adapter.Chat(ctx, engineMessages, convertSchemas(tools), false)
			if err != nil {
				e.bridge.SendChat("system", "Error: "+err.Error())
				return err
			}
			// Collect the single-shot response
			for item := range fallbackStream {
				if item.ToolCall != nil {
					toolCallReceived = &tool.ToolCall{ID: item.ToolCall.ID, Name: item.ToolCall.Name, Args: item.ToolCall.Args}
					if convo != nil {
						convo.AddAssistantToolUse(toolCallReceived.Name, toolCallReceived.ID, string(toolCallReceived.Args))
					}
					break
				}
				if item.Token != "" {
					currentContent += item.Token
				}
			}
			if toolCallReceived != nil {
				// Execute the tool and continue the depth loop
				execResult, err := e.tools.InvokeToolCall(ctx, toolCallReceived)
				if err != nil {
					errorMsg := fmt.Sprintf("Error executing tool %s: %v", toolCallReceived.Name, err)
					convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, errorMsg)
					e.bridge.SendChat("system", errorMsg)
					return err
				}
				if !execResult.Safe {
					if !e.UserApproved(toolCallReceived, execResult.Diff) {
						execResult.Content = "User denied operation"
					}
				}
				convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, execResult.Content)
				continue
			}
			if currentContent != "" {
				convo.AddAssistant(currentContent)
				e.bridge.EmitAssistant(currentContent)
				return nil
			}
			// Still nothing
			e.bridge.SendChat("system", "No response from model.")
			return nil
		}
	}

	return errors.New("tool loop exceeded maximum depth")
}

// convertSchemas converts tool.Schema to ToolSchema
func convertSchemas(schemas []tool.Schema) []ToolSchema {
	result := make([]ToolSchema, len(schemas))
	for i, schema := range schemas {
		result[i] = ToolSchema{
			Name:        schema.Name,
			Description: schema.Description,
			Schema:      schema.Parameters,
		}
	}
	return result
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
	// Use the workspace directory from engine configuration
	workspacePath := e.workspaceDir

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

	// Generate a summary for the tool call
	summary := "Tool: " + call.Name

	// Ask the bridge for approval with the summary and args as diff
	diff := string(call.Args)
	go e.bridge.PromptApproval(call.ID, summary, diff)

	// Wait for response
	approved := <-responseCh
	return approved
}

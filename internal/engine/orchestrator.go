package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/loom/loom/internal/config"
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
	EmitReasoning(text string, done bool)
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
	// Settings-backed flags
	autoApproveShell bool
	autoApproveEdits bool
	// model label like "openai:gpt-4o" for titling
	currentModelLabel string
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
	// Provide the UI bridge to the tools registry for activity notifications
	if e.bridge != nil {
		registry.WithUI(e.bridge)
	}
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

// Workspace returns the configured workspace directory.
func (e *Engine) Workspace() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.workspaceDir
}

// SetBridge sets the UI bridge for the engine.
func (e *Engine) SetBridge(bridge UIBridge) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bridge = bridge
	// Propagate the bridge to the tools registry so it can emit activity messages
	if e.tools != nil && bridge != nil {
		e.tools.WithUI(bridge)
	}
}

// SetAutoApprove toggles auto-approval behaviors based on settings
func (e *Engine) SetAutoApprove(shell bool, edits bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.autoApproveShell = shell
	e.autoApproveEdits = edits
}

// SetLLM updates the LLM used by the engine.
func (e *Engine) SetLLM(llm LLM) {
	e.llmMu.Lock()
	defer e.llmMu.Unlock()
	e.llm = llm
}

// SetModelLabel sets a human-readable model label (e.g., "openai:gpt-4o").
func (e *Engine) SetModelLabel(label string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.currentModelLabel = label
}

// GetModelLabel returns the current model label if known.
func (e *Engine) GetModelLabel() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentModelLabel
}

// ListConversations returns summaries for available conversations.
func (e *Engine) ListConversations() ([]memory.ConversationSummary, error) {
	if e.memory == nil {
		return nil, errors.New("memory not initialized")
	}
	// Immediately remove any non-current empty conversations
	e.memory.CleanupEmptyConversations(e.memory.CurrentConversationID())
	return e.memory.ListConversationSummaries()
}

// CurrentConversationID returns the active conversation id.
func (e *Engine) CurrentConversationID() string {
	if e.memory == nil {
		return ""
	}
	return e.memory.CurrentConversationID()
}

// SetCurrentConversationID switches the active conversation id.
func (e *Engine) SetCurrentConversationID(id string) error {
	if e.memory == nil {
		return errors.New("memory not initialized")
	}
	return e.memory.SetCurrentConversationID(id)
}

// GetConversation returns the messages for the given conversation id.
func (e *Engine) GetConversation(id string) ([]Message, error) {
	if e.memory == nil {
		return nil, errors.New("memory not initialized")
	}
	var memMsgs []memory.Message
	if err := e.memory.Get("conversations/"+id, &memMsgs); err != nil {
		return nil, err
	}
	msgs := make([]Message, 0, len(memMsgs))
	for _, m := range memMsgs {
		msgs = append(msgs, Message{Role: m.Role, Content: m.Content, Name: m.Name, ToolID: m.ToolID})
	}
	return msgs, nil
}

// NewConversation creates and switches to a new conversation.
func (e *Engine) NewConversation() string {
	if e.memory == nil {
		return ""
	}
	id := e.memory.CreateNewConversation()
	// Immediately remove any non-current empty conversations
	e.memory.CleanupEmptyConversations(id)
	return id
}

// ClearConversation clears the current conversation history in memory and notifies the UI.
func (e *Engine) ClearConversation() {
	if e.memory != nil {
		newID := e.memory.CreateNewConversation()
		// Remove any non-current conversations with no user messages immediately
		e.memory.CleanupEmptyConversations(newID)
	}
	if e.bridge != nil {
		e.bridge.SendChat("system", "Conversation cleared.")
	}
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
	// Auto-approval rules
	if toolCall != nil {
		if (toolCall.Name == "run_shell" || toolCall.Name == "apply_shell") && e.autoApproveShell {
			return true
		}
		if (toolCall.Name == "edit_file" || toolCall.Name == "apply_edit") && e.autoApproveEdits {
			return true
		}
	}

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
		// Load dynamic rules and inject into system prompt
		userRules, projectRules, _ := config.LoadRules(e.workspaceDir)
		convo.AddSystem(GenerateSystemPromptWithRules(toolSchemas, userRules, projectRules))
	}

	// Add latest user message
	convo.AddUser(userMsg)
	// After the first user message in a conversation, if no title yet, set a title using the selected model
	if e.memory != nil {
		currentID := e.memory.CurrentConversationID()
		if currentID != "" && e.memory.GetConversationTitle(currentID) == "" {
			// Title: first (~50 chars) of the user's first message + current model label
			title := userMsg
			if len(title) > 50 {
				title = title[:50] + "…"
			}
			if label := e.GetModelLabel(); label != "" {
				title = title + " — " + label
			}
			_ = e.memory.SetConversationTitle(currentID, title)
		}
	}

	// Prepare tool schemas for the adapter
	tools := toolSchemas // get all tool specs

	// Set up the adapter (LLM)
	adapter := e.llm
	if adapter == nil {
		if e.bridge != nil {
			e.bridge.SendChat("system", "No model is configured. Open Settings to enter your API key and select a model.")
		}
		return errors.New("llm not configured")
	}

	// Track whether any tool has been used since the latest user message
	toolsUsed := false

	// Set a configurable maximum depth to prevent infinite loops but allow long tool chains
	maxDepth := 64
	if v := os.Getenv("LOOM_MAX_STEPS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxDepth = n
		}
	}
	for depth := 0; depth < maxDepth; depth++ {
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
		reasoningAccumulated := false

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
					// Guard against empty tool names from partial/ambiguous streams
					if item.ToolCall.Name == "" {
						// Ignore and continue reading tokens; likely a partial stream
						continue
					}
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
				tok := item.Token
				if strings.HasPrefix(tok, "[REASONING] ") {
					e.bridge.EmitReasoning(strings.TrimPrefix(tok, "[REASONING] "), false)
					reasoningAccumulated = true
					continue
				}
				if strings.HasPrefix(tok, "[REASONING_RAW] ") {
					e.bridge.EmitReasoning(strings.TrimPrefix(tok, "[REASONING_RAW] "), false)
					reasoningAccumulated = true
					continue
				}
				if strings.HasPrefix(tok, "[REASONING_DONE] ") {
					text := strings.TrimPrefix(tok, "[REASONING_DONE] ")
					if reasoningAccumulated {
						e.bridge.EmitReasoning("", true)
					} else if strings.TrimSpace(text) != "" {
						e.bridge.EmitReasoning(text, true)
					} else {
						e.bridge.EmitReasoning("", true)
					}
					// Do not add to assistant content
					continue
				}
				if strings.HasPrefix(tok, "[REASONING_RAW_DONE] ") {
					text := strings.TrimPrefix(tok, "[REASONING_RAW_DONE] ")
					if reasoningAccumulated {
						e.bridge.EmitReasoning("", true)
					} else if strings.TrimSpace(text) != "" {
						e.bridge.EmitReasoning(text, true)
					} else {
						e.bridge.EmitReasoning("", true)
					}
					continue
				}
				currentContent += tok
				e.bridge.EmitAssistant(currentContent)
			}
		}

		// If we got a tool call, execute it
		if toolCallReceived != nil {
			// Mark that at least one tool was used in this turn
			toolsUsed = true
			// Execute the tool
			execResult, err := e.tools.InvokeToolCall(ctx, toolCallReceived)
			if err != nil {
				errorMsg := fmt.Sprintf("Error executing tool %s: %v", toolCallReceived.Name, err)
				// Attach as tool_result with the same tool_use_id for Anthropic
				convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, errorMsg)
				e.bridge.SendChat("system", errorMsg)
				return err
			}

			// Approval path returns structured result to the model
			if !execResult.Safe {
				approved := e.UserApproved(toolCallReceived, execResult.Diff)
				payload := map[string]any{
					"tool":     toolCallReceived.Name,
					"approved": approved,
					"diff":     execResult.Diff,
					"message":  execResult.Content,
				}
				b, _ := json.Marshal(payload)
				convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, string(b))
			} else {
				// Safe tool: just return content
				convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, execResult.Content)
			}

			// If this was a finalize call, end now
			if toolCallReceived.Name == "finalize" {
				if execResult.Content != "" {
					convo.AddAssistant(execResult.Content)
					e.bridge.EmitAssistant(execResult.Content)
				}
				return nil
			}

			// Continue the loop to get the next assistant message
			continue
		}

		// If we reach here with content but no tool call, record it
		if currentContent != "" {
			convo.AddAssistant(currentContent)
			// If any tools were used earlier in this turn, nudge the model to finalize; otherwise end here
			if toolsUsed {
				convo.AddSystem("Reminder: Tools were used. If the objective is complete, call the finalize tool with a concise summary. If more steps are needed, call exactly one next tool.")
				// Continue depth loop to allow further tool calls or finalize
				continue
			}
			// Pure conversational response with no tools used — end the turn
			return nil
		}

		// If stream ended with no content and no tool call, retry once without streaming
		if streamEnded && currentContent == "" {
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
					approved := e.UserApproved(toolCallReceived, execResult.Diff)
					payload := map[string]any{
						"tool":     toolCallReceived.Name,
						"approved": approved,
						"diff":     execResult.Diff,
						"message":  execResult.Content,
					}
					b, _ := json.Marshal(payload)
					convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, string(b))
				} else {
					convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, execResult.Content)
				}
				if toolCallReceived.Name == "finalize" {
					if execResult.Content != "" {
						convo.AddAssistant(execResult.Content)
						e.bridge.EmitAssistant(execResult.Content)
					}
					return nil
				}
				continue
			}
			if currentContent != "" {
				convo.AddAssistant(currentContent)
				e.bridge.EmitAssistant(currentContent)
				// If tools were used in this turn, nudge to finalize; otherwise stop
				if toolsUsed {
					convo.AddSystem("Reminder: Tools were used. If the objective is complete, call the finalize tool with a concise summary. If more steps are needed, call exactly one next tool.")
					// Continue depth loop; do not finalize automatically on plain content
					continue
				}
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

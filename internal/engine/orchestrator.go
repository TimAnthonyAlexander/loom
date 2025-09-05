package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/loom/loom/internal/config"
	"github.com/loom/loom/internal/memory"
	"github.com/loom/loom/internal/tool"
	"github.com/loom/loom/internal/workflow"
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
	// EmitBilling notifies the UI of per-request usage and costs in USD
	EmitBilling(provider string, model string, inTokens int64, outTokens int64, inUSD float64, outUSD float64, totalUSD float64)
	PromptApproval(actionID string, summary string, diff string) (approved bool)
	PromptChoice(actionID string, question string, options []string) (selectedIndex int)
	SetBusy(isBusy bool)
	// Request the UI to open a file path (relative to workspace) in the file viewer
	OpenFileInUI(path string)
}

// ApprovalRequest tracks an outstanding approval request.
type ApprovalRequest struct {
	ID       string
	Summary  string
	Diff     string
	Response chan bool
}

// ChoiceRequest tracks an outstanding choice request.
type ChoiceRequest struct {
	ID       string
	Question string
	Options  []string
	Response chan int
}

// Engine is the core orchestrator for the Loom system.
type Engine struct {
	bridge       UIBridge
	messages     []Message
	llm          LLM
	mu           sync.RWMutex
	approvals    map[string]chan bool
	choices      map[string]chan int
	approvalMu   sync.Mutex
	tools        *tool.Registry
	memory       *memory.Project
	workspaceDir string
	llmMu        sync.Mutex
	// Settings-backed flags
	autoApproveShell bool
	autoApproveEdits bool
	// AI personality setting
	personality string
	// model label like "openai:gpt-4o" for titling
	currentModelLabel string
	// latest editor context as reported by the UI (workspace-relative path)
	editorCtx struct {
		Path   string
		Line   int
		Column int
	}
	// list of workspace-relative file paths attached by the user for extra context
	attachedFiles []string

	// workflow state store (feature-flagged)
	wf *workflow.Store

	// cancellation support for stopping LLM operations
	currentCtx    context.Context
	cancelCurrent context.CancelFunc
	ctxMu         sync.Mutex
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
		choices:   make(map[string]chan int),
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

// GetUsage exposes persisted usage totals for the current project.
func (e *Engine) GetUsage() memory.UsageTotals {
	if e.memory == nil {
		return memory.UsageTotals{}
	}
	return e.memory.GetUsage()
}

// ResetUsage clears persisted usage for the current project.
func (e *Engine) ResetUsage() error {
	if e.memory == nil {
		return nil
	}
	return e.memory.ResetUsage()
}

// WithWorkspace sets the workspace directory path for the engine.
func (e *Engine) WithWorkspace(path string) *Engine {
	e.workspaceDir = path
	if strings.TrimSpace(path) != "" {
		e.wf = workflow.NewStore(path)
	}
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

// SetPersonality sets the AI personality for system prompt injection
func (e *Engine) SetPersonality(personality string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.personality = personality
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

// SetEditorContext records the user's currently viewed file and cursor position.
// The path should be workspace-relative using forward slashes.
func (e *Engine) SetEditorContext(path string, line, column int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.editorCtx.Path = strings.TrimSpace(path)
	if line < 1 {
		line = 1
	}
	if column < 1 {
		column = 1
	}
	e.editorCtx.Line = line
	e.editorCtx.Column = column
}

// formatEditorContext returns a single-line hint about the user's current editor state.
func (e *Engine) formatEditorContext() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if strings.TrimSpace(e.editorCtx.Path) == "" {
		return ""
	}
	// If line/column are not set, still provide the file context
	if e.editorCtx.Line <= 0 || e.editorCtx.Column <= 0 {
		return fmt.Sprintf("The user is currently viewing the file %s.", e.editorCtx.Path)
	}
	return fmt.Sprintf("The user is currently viewing the file %s at line %d, column %d. Use this information if useful to the user request.", e.editorCtx.Path, e.editorCtx.Line, e.editorCtx.Column)
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
	// Clear any attached files for the new conversation
	e.mu.Lock()
	e.attachedFiles = nil
	e.mu.Unlock()
	return id
}

// ClearConversation clears the current conversation history in memory and notifies the UI.
func (e *Engine) ClearConversation() {
	if e.memory != nil {
		newID := e.memory.CreateNewConversation()
		// Remove any non-current conversations with no user messages immediately
		e.memory.CleanupEmptyConversations(newID)
	}
	// Clearing the conversation should also clear composer attachments
	e.mu.Lock()
	e.attachedFiles = nil
	e.mu.Unlock()
	if e.bridge != nil {
		e.bridge.SendChat("system", "Conversation cleared.")
	}
}

// Enqueue adds a user message and starts the processing loop.
func (e *Engine) Enqueue(message string) {
	// Send to UI
	e.bridge.SendChat("user", message)

	// Create cancellable context for this operation
	e.ctxMu.Lock()
	// Cancel any existing operation before starting a new one
	if e.cancelCurrent != nil {
		e.cancelCurrent()
	}
	e.currentCtx, e.cancelCurrent = context.WithCancel(context.Background())
	ctx := e.currentCtx
	e.ctxMu.Unlock()

	// Start processing in a goroutine
	go func() {
		_ = e.processLoop(ctx, message)
	}()
}

// Stop cancels any running LLM operation.
func (e *Engine) Stop() {
	e.ctxMu.Lock()
	defer e.ctxMu.Unlock()

	if e.cancelCurrent != nil {
		e.cancelCurrent()
		e.cancelCurrent = nil
		e.currentCtx = nil
	}
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

// ResolveChoice resolves a pending choice request.
func (e *Engine) ResolveChoice(id string, selectedIndex int) {
	e.approvalMu.Lock()
	defer e.approvalMu.Unlock()

	if ch, ok := e.choices[id]; ok {
		ch <- selectedIndex
		delete(e.choices, id)
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

// UserChoice prompts for a choice and waits for the response.
func (e *Engine) UserChoice(toolCall *tool.ToolCall, question string, options []string) int {
	// Create a channel for the response
	responseCh := make(chan int)

	// Register the choice request
	e.approvalMu.Lock()
	e.choices[toolCall.ID] = responseCh
	e.approvalMu.Unlock()

	// Ask the bridge for a choice (this will always return -1 for async handling)
	e.bridge.PromptChoice(toolCall.ID, question, options)

	// Wait for response
	selected := <-responseCh
	return selected
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
		// Load dynamic rules and inject into system prompt with project context
		userRules, projectRules, _ := config.LoadRules(e.workspaceDir)
		mems := loadUserMemoriesForPrompt()
		e.mu.RLock()
		currentPersonality := e.personality
		e.mu.RUnlock()
		base := GenerateSystemPromptUnified(SystemPromptOptions{
			Tools:                 toolSchemas,
			UserRules:             userRules,
			ProjectRules:          projectRules,
			Memories:              mems,
			Personality:           currentPersonality,
			WorkspaceRoot:         e.workspaceDir,
			IncludeProjectContext: true,
		})
		if ui := strings.TrimSpace(e.formatEditorContext()); ui != "" {
			base = strings.TrimSpace(base) + "\n\nUI Context:\n- " + ui
		}
		convo.AddSystem(base)
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

	// Track consecutive empty responses after tool usage to prevent pathological cases
	consecutiveEmptyAfterTools := 0
	maxConsecutiveEmpty := 3 // Allow up to 3 consecutive empty responses after tools before giving up

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

		// Append up-to-date UI editor context as a transient system hint for this turn
		if ui := strings.TrimSpace(e.formatEditorContext()); ui != "" {
			engineMessages = append(engineMessages, Message{Role: "system", Content: "UI Context: " + ui})
		}
		// Inject compact workflow state and recent events if enabled
		if e.wf != nil {
			if st, err := e.wf.Load(); err == nil {
				recent := workflow.RecentEventsForPrompt(e.workspaceDir, st.Budget.MaxEventsInPrompt)
				block := workflow.BuildPromptFromStore(ctx, e.workspaceDir, st, recent)
				if strings.TrimSpace(block) != "" {
					engineMessages = append(engineMessages, Message{Role: "system", Content: block})
				}
			}
		}
		// No longer inject attachments as system context; they are appended to the user message on send

		// Call the LLM with the conversation history (+ transient UI hint)
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
				// Send cancellation message to UI
				if e.bridge != nil {
					e.bridge.SendChat("system", "Operation stopped by user.")
				}
				return ctx.Err()
			case <-slowTicker.C:
				if !slowNotified {
					// e.bridge.SendChat("system", "Still working...")
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
					// Got a tool call; record it but continue reading the stream until completion so we can capture usage
					// Guard against empty tool names from partial/ambiguous streams
					if item.ToolCall.Name == "" {
						if os.Getenv("LOOM_DEBUG_ENGINE") == "1" || strings.EqualFold(os.Getenv("LOOM_DEBUG_ENGINE"), "true") {
							e.bridge.SendChat("system", "[debug] Received partial tool call with empty name; continuing to read stream")
						}
						continue
					}
					if toolCallReceived == nil {
						if os.Getenv("LOOM_DEBUG_ENGINE") == "1" || strings.EqualFold(os.Getenv("LOOM_DEBUG_ENGINE"), "true") {
							e.bridge.SendChat("system", fmt.Sprintf("[debug] Tool call received: id=%s name=%s argsLen=%d", item.ToolCall.ID, item.ToolCall.Name, len(item.ToolCall.Args)))
						}
						toolCallReceived = &tool.ToolCall{
							ID:   item.ToolCall.ID,
							Name: item.ToolCall.Name,
							Args: item.ToolCall.Args,
						}
						// Record tool_use event to workflow state
						if e.wf != nil {
							_ = e.wf.ApplyEvent(ctx, map[string]any{
								"ts":   time.Now().Unix(),
								"type": "tool_use",
								"tool": item.ToolCall.Name,
								"args": string(item.ToolCall.Args),
								"id":   item.ToolCall.ID,
							})
						}
						// Record the assistant tool_use in conversation for Anthropic
						if convo != nil {
							convo.AddAssistantToolUse(toolCallReceived.Name, toolCallReceived.ID, string(toolCallReceived.Args))
						}
					}
					continue
				}

				// Got a token
				tok := item.Token
				if strings.HasPrefix(tok, "[USAGE] ") {
					// Parse provider/model/in/out from token and emit billing event
					// Format: [USAGE] provider=xxx model=yyy in=N out=M
					usage := strings.TrimPrefix(tok, "[USAGE] ")
					var provider, model string
					var inTok, outTok int64
					fields := strings.Fields(usage)
					for _, f := range fields {
						if strings.HasPrefix(f, "provider=") {
							provider = strings.TrimPrefix(f, "provider=")
						} else if strings.HasPrefix(f, "model=") {
							model = strings.TrimPrefix(f, "model=")
						} else if strings.HasPrefix(f, "in=") {
							if v, err := strconv.ParseInt(strings.TrimPrefix(f, "in="), 10, 64); err == nil {
								inTok = v
							}
						} else if strings.HasPrefix(f, "out=") {
							if v, err := strconv.ParseInt(strings.TrimPrefix(f, "out="), 10, 64); err == nil {
								outTok = v
							}
						}
					}
					// Compute costs via config table
					inUSD, outUSD, totalUSD := config.CostUSDParts(model, inTok, outTok)
					if e.bridge != nil {
						e.bridge.EmitBilling(provider, model, inTok, outTok, inUSD, outUSD, totalUSD)
					}
					// Persist usage to project memory per workspace and to global store
					if e.memory != nil {
						_ = e.memory.AddUsage(provider, model, inTok, outTok, inUSD, outUSD)
					}
					_ = config.AddGlobalUsage(provider, model, inTok, outTok, inUSD, outUSD)
					// Do not append to assistant text
					continue
				}
				if strings.HasPrefix(tok, "[REASONING] ") {
					text := strings.TrimPrefix(tok, "[REASONING] ")
					// Show incremental reasoning to the UI but do not persist until the block ends
					e.bridge.EmitReasoning(text, false)
					reasoningAccumulated = true
					continue
				}
				if strings.HasPrefix(tok, "[REASONING_SIGNATURE] ") {
					// Signature is captured in the final JSON event; ignore incremental signature token
					continue
				}
				if strings.HasPrefix(tok, "[REASONING_JSON] ") {
					raw := strings.TrimPrefix(tok, "[REASONING_JSON] ")
					// Persist the full JSON so adapter can replay signature
					if convo != nil {
						// Try to parse and store; if parse fails, fall back to plain
						var tmp map[string]string
						if json.Unmarshal([]byte(raw), &tmp) == nil {
							convo.AddAssistantThinkingSigned(tmp["thinking"], tmp["signature"])
						}
					}
					continue
				}
				if strings.HasPrefix(tok, "[REASONING_RAW] ") {
					text := strings.TrimPrefix(tok, "[REASONING_RAW] ")
					// Show incremental reasoning to the UI but do not persist until the block ends
					e.bridge.EmitReasoning(text, false)
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
					// No need to persist a separate DONE marker; thinking content is already stored.
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
			// Reset empty response counter since we got a tool call
			consecutiveEmptyAfterTools = 0
			// Execute the tool
			execResult, err := e.tools.InvokeToolCall(ctx, toolCallReceived)
			if err != nil {
				errorMsg := fmt.Sprintf("Error executing tool %s: %v", toolCallReceived.Name, err)
				// Attach as tool_result with the same tool_use_id for Anthropic
				convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, errorMsg)
				e.bridge.SendChat("system", errorMsg)
				return err
			}
			if os.Getenv("LOOM_DEBUG_ENGINE") == "1" || strings.EqualFold(os.Getenv("LOOM_DEBUG_ENGINE"), "true") {
				e.bridge.SendChat("system", fmt.Sprintf("[debug] Tool executed: name=%s safe=%v diffLen=%d contentLen=%d", toolCallReceived.Name, execResult.Safe, len(execResult.Diff), len(execResult.Content)))
			}

			// Record tool_result summary
			if e.wf != nil {
				_ = e.wf.ApplyEvent(ctx, map[string]any{
					"ts":      time.Now().Unix(),
					"type":    "tool_result",
					"tool":    toolCallReceived.Name,
					"ok":      err == nil,
					"summary": strings.TrimSpace(execResult.Content),
					"id":      toolCallReceived.ID,
				})
			}

			// If the tool was file-related, hint UI to open the file
			if e.bridge != nil {
				// Try to extract a path field from args JSON for known tools
				type pathArg struct {
					Path string `json:"path"`
				}
				var pa pathArg
				_ = json.Unmarshal(toolCallReceived.Args, &pa)
				if pa.Path == "" {
					// Some tools may use "file" key
					var alt map[string]any
					if json.Unmarshal(toolCallReceived.Args, &alt) == nil {
						if v, ok := alt["file"].(string); ok && strings.TrimSpace(v) != "" {
							pa.Path = v
						}
					}
				}
				if strings.TrimSpace(pa.Path) != "" && (toolCallReceived.Name == "read_file" || toolCallReceived.Name == "edit_file" || toolCallReceived.Name == "apply_edit") {
					e.bridge.OpenFileInUI(pa.Path)
				}
			}

			// Special handling for user_choice tool
			if !execResult.Safe && toolCallReceived.Name == "user_choice" {
				// Parse the tool args to extract question and options
				type userChoiceArgs struct {
					Question string   `json:"question"`
					Options  []string `json:"options"`
				}
				var args userChoiceArgs
				if err := json.Unmarshal(toolCallReceived.Args, &args); err == nil {
					selectedIndex := e.UserChoice(toolCallReceived, args.Question, args.Options)
					if selectedIndex >= 0 && selectedIndex < len(args.Options) {
						selectedOption := args.Options[selectedIndex]
						response := map[string]any{
							"selected_option": selectedOption,
							"selected_index":  selectedIndex,
						}
						responseJson, _ := json.Marshal(response)
						convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, string(responseJson))
					} else {
						convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, "Invalid choice selection")
					}
				} else {
					convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, "Error parsing choice arguments")
				}
			} else if !execResult.Safe {
				// Regular approval path for other tools
				approved := e.UserApproved(toolCallReceived, execResult.Diff)
				if e.wf != nil {
					_ = e.wf.ApplyEvent(ctx, map[string]any{
						"ts":     time.Now().Unix(),
						"type":   "approval",
						"tool":   toolCallReceived.Name,
						"status": map[bool]string{true: "granted", false: "denied"}[approved],
						"id":     toolCallReceived.ID,
					})
				}
				payload := map[string]any{
					"tool":     toolCallReceived.Name,
					"approved": approved,
					"diff":     execResult.Diff,
					"message":  execResult.Content,
				}
				b, _ := json.Marshal(payload)
				convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, string(b))

				// If edits are auto-approved and this was an edit proposal, immediately apply it
				if approved && e.autoApproveEdits && toolCallReceived.Name == "edit_file" {
					applyCall := &tool.ToolCall{ID: toolCallReceived.ID + ":apply", Name: "apply_edit", Args: toolCallReceived.Args}
					applyResult, applyErr := e.tools.InvokeToolCall(ctx, applyCall)
					if applyErr != nil {
						errorMsg := fmt.Sprintf("Error executing tool %s: %v", applyCall.Name, applyErr)
						e.bridge.SendChat("system", errorMsg)
						// Do not add a tool_result with a synthetic tool ID; continue
					} else {
						// Hint UI to open the file if path present
						if e.bridge != nil {
							type pathArg struct {
								Path string `json:"path"`
							}
							var pa pathArg
							_ = json.Unmarshal(applyCall.Args, &pa)
							if strings.TrimSpace(pa.Path) != "" {
								e.bridge.OpenFileInUI(pa.Path)
							}
						}
						// Inform via system chat; avoid emitting a tool_result with unmatched tool_use_id
						if strings.TrimSpace(applyResult.Content) != "" {
							e.bridge.SendChat("system", applyResult.Content)
						}
					}
				}
			} else {
				// Safe tool: add to conversation and show in UI
				convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, execResult.Content)
				// Send tool result to UI for immediate display
				if strings.TrimSpace(execResult.Content) != "" {
					e.bridge.SendChat("tool", execResult.Content)
				}
			}

			// Continue the loop to get the next assistant message
			continue
		}

		// If we reach here with content but no tool call, record it
		if currentContent != "" {
			convo.AddAssistant(currentContent)
			// Reset empty response counter since we got content
			consecutiveEmptyAfterTools = 0
			// If any tools were used earlier in this turn, continue the loop to allow for more interactions
			if toolsUsed {
				continue
			}
			// Pure conversational response with no tools used — end the turn
			return nil
		}

		// If stream ended with no content and no tool call, retry once without streaming
		if streamEnded && currentContent == "" {
			// Only show retry message if no tools were used (to avoid user confusion after successful tool calls)
			if !toolsUsed {
				e.bridge.SendChat("system", "Retrying without streaming...")
			}
			fallbackStream, err := adapter.Chat(ctx, engineMessages, convertSchemas(tools), false)
			if err != nil {
				e.bridge.SendChat("system", "Error: "+err.Error())
				return err
			}
			// Collect the single-shot response
			for item := range fallbackStream {
				if item.ToolCall != nil {
					toolCallReceived = &tool.ToolCall{ID: item.ToolCall.ID, Name: item.ToolCall.Name, Args: item.ToolCall.Args}
					if os.Getenv("LOOM_DEBUG_ENGINE") == "1" || strings.EqualFold(os.Getenv("LOOM_DEBUG_ENGINE"), "true") {
						e.bridge.SendChat("system", fmt.Sprintf("[debug] Non-stream tool call received: id=%s name=%s argsLen=%d", item.ToolCall.ID, item.ToolCall.Name, len(item.ToolCall.Args)))
					}
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
				if os.Getenv("LOOM_DEBUG_ENGINE") == "1" || strings.EqualFold(os.Getenv("LOOM_DEBUG_ENGINE"), "true") {
					e.bridge.SendChat("system", fmt.Sprintf("[debug] Non-stream tool executed: name=%s safe=%v diffLen=%d contentLen=%d", toolCallReceived.Name, execResult.Safe, len(execResult.Diff), len(execResult.Content)))
				}
				if e.bridge != nil {
					type pathArg struct {
						Path string `json:"path"`
					}
					var pa pathArg
					_ = json.Unmarshal(toolCallReceived.Args, &pa)
					if pa.Path == "" {
						var alt map[string]any
						if json.Unmarshal(toolCallReceived.Args, &alt) == nil {
							if v, ok := alt["file"].(string); ok && strings.TrimSpace(v) != "" {
								pa.Path = v
							}
						}
					}
					if strings.TrimSpace(pa.Path) != "" && (toolCallReceived.Name == "read_file" || toolCallReceived.Name == "edit_file" || toolCallReceived.Name == "apply_edit") {
						e.bridge.OpenFileInUI(pa.Path)
					}
				}

				if !execResult.Safe && toolCallReceived.Name == "user_choice" {
					// Parse the tool args to extract question and options
					type userChoiceArgs struct {
						Question string   `json:"question"`
						Options  []string `json:"options"`
					}
					var args userChoiceArgs
					if err := json.Unmarshal(toolCallReceived.Args, &args); err == nil {
						selectedIndex := e.UserChoice(toolCallReceived, args.Question, args.Options)
						if selectedIndex >= 0 && selectedIndex < len(args.Options) {
							selectedOption := args.Options[selectedIndex]
							response := map[string]any{
								"selected_option": selectedOption,
								"selected_index":  selectedIndex,
							}
							responseJson, _ := json.Marshal(response)
							convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, string(responseJson))
						} else {
							convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, "Invalid choice selection")
						}
					} else {
						convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, "Error parsing choice arguments")
					}
				} else if !execResult.Safe {
					approved := e.UserApproved(toolCallReceived, execResult.Diff)
					payload := map[string]any{
						"tool":     toolCallReceived.Name,
						"approved": approved,
						"diff":     execResult.Diff,
						"message":  execResult.Content,
					}
					b, _ := json.Marshal(payload)
					convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, string(b))

					// Auto-apply on approval in non-stream path as well
					if approved && e.autoApproveEdits && toolCallReceived.Name == "edit_file" {
						applyCall := &tool.ToolCall{ID: toolCallReceived.ID + ":apply", Name: "apply_edit", Args: toolCallReceived.Args}
						applyResult, applyErr := e.tools.InvokeToolCall(ctx, applyCall)
						if applyErr != nil {
							errorMsg := fmt.Sprintf("Error executing tool %s: %v", applyCall.Name, applyErr)
							e.bridge.SendChat("system", errorMsg)
							// Avoid emitting a tool_result with a synthetic tool ID; continue
						} else {
							if e.bridge != nil {
								type pathArg struct {
									Path string `json:"path"`
								}
								var pa pathArg
								_ = json.Unmarshal(applyCall.Args, &pa)
								if strings.TrimSpace(pa.Path) != "" {
									e.bridge.OpenFileInUI(pa.Path)
								}
							}
							if strings.TrimSpace(applyResult.Content) != "" {
								e.bridge.SendChat("system", applyResult.Content)
							}
						}
					}
				} else {
					// Safe tool: add to conversation and show in UI
					convo.AddToolResult(toolCallReceived.Name, toolCallReceived.ID, execResult.Content)
					// Send tool result to UI for immediate display
					if strings.TrimSpace(execResult.Content) != "" {
						e.bridge.SendChat("tool", execResult.Content)
					}
				}
				continue
			}
			if currentContent != "" {
				convo.AddAssistant(currentContent)
				e.bridge.EmitAssistant(currentContent)
				// Reset empty response counter since we got content
				consecutiveEmptyAfterTools = 0
				// If tools were used in this turn, continue the loop to allow for more interactions
				if toolsUsed {
					continue
				}
				return nil
			}
			// Still nothing
			if os.Getenv("LOOM_DEBUG_ENGINE") == "1" || strings.EqualFold(os.Getenv("LOOM_DEBUG_ENGINE"), "true") {
				e.bridge.SendChat("system", "[debug] Fallback non-stream returned no content and no tool calls")
			}
			// If tools were used but we got empty response, continue to reprompt the model
			if toolsUsed {
				consecutiveEmptyAfterTools++
				if consecutiveEmptyAfterTools >= maxConsecutiveEmpty {
					e.bridge.SendChat("system", fmt.Sprintf("Model failed to respond after %d attempts following tool execution. Ending conversation.", maxConsecutiveEmpty))
					return errors.New("model failed to respond after tool execution")
				}
				if os.Getenv("LOOM_DEBUG_ENGINE") == "1" || strings.EqualFold(os.Getenv("LOOM_DEBUG_ENGINE"), "true") {
					e.bridge.SendChat("system", fmt.Sprintf("[debug] Reprompting model after tool execution with empty response (attempt %d/%d)", consecutiveEmptyAfterTools, maxConsecutiveEmpty))
				}
				continue
			}
			// If no tools were used and we have an empty response, that's an error
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

// SetAttachedFiles stores the list of workspace-relative files attached by the user.
// Paths are normalized to forward slashes and trimmed. Empty entries are ignored.
func (e *Engine) SetAttachedFiles(paths []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	normalized := make([]string, 0, len(paths))
	for _, p := range paths {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		s = strings.ReplaceAll(s, "\\", "/")
		// Remove any leading ./
		for strings.HasPrefix(s, "./") {
			s = strings.TrimPrefix(s, "./")
		}
		normalized = append(normalized, s)
	}
	e.attachedFiles = normalized
}

// loadUserMemoriesForPrompt reads ~/.loom/memories.json and returns entries for prompt injection.
func loadUserMemoriesForPrompt() []MemoryEntry {
	type mem struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	path := filepath.Join(home, ".loom", "memories.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var list []mem
	if json.Unmarshal(data, &list) == nil {
		out := make([]MemoryEntry, 0, len(list))
		for _, it := range list {
			out = append(out, MemoryEntry{ID: strings.TrimSpace(it.ID), Text: strings.TrimSpace(it.Text)})
		}
		return out
	}
	var wrapper struct {
		Memories []mem `json:"memories"`
	}
	if json.Unmarshal(data, &wrapper) == nil && wrapper.Memories != nil {
		out := make([]MemoryEntry, 0, len(wrapper.Memories))
		for _, it := range wrapper.Memories {
			out = append(out, MemoryEntry{ID: strings.TrimSpace(it.ID), Text: strings.TrimSpace(it.Text)})
		}
		return out
	}
	return nil
}

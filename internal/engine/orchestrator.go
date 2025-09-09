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
	tools        *tool.Registry
	memory       *memory.Project
	workspaceDir string
	llmMu        sync.Mutex
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

	// cancellation support for stopping LLM operations
	currentCtx    context.Context
	cancelCurrent context.CancelFunc
	ctxMu         sync.Mutex

	// extracted modules
	conversationMgr *ConversationManager
	approvalHandler *ApprovalHandler
	streamProcessor *StreamProcessor
	toolExecutor    *ToolExecutor
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
	e := &Engine{
		llm:      llm,
		bridge:   bridge,
		messages: []Message{},
	}
	// Initialize modules
	e.approvalHandler = NewApprovalHandler(bridge)
	return e
}

// WithRegistry sets the tool registry for the engine.
func (e *Engine) WithRegistry(registry *tool.Registry) *Engine {
	e.tools = registry
	// Provide the UI bridge to the tools registry for activity notifications
	if e.bridge != nil {
		registry.WithUI(e.bridge)
	}
	// Initialize tool executor with registry
	if e.approvalHandler != nil {
		e.toolExecutor = NewToolExecutor(e.bridge, registry, e.approvalHandler)
	}
	return e
}

// WithMemory sets the project memory for the engine.
func (e *Engine) WithMemory(project *memory.Project) *Engine {
	e.memory = project
	// Initialize conversation manager with memory
	e.conversationMgr = NewConversationManager(project)
	// Update stream processor with memory
	if e.streamProcessor != nil {
		e.streamProcessor = NewStreamProcessor(e.bridge, project)
	}
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
	// Initialize stream processor
	e.streamProcessor = NewStreamProcessor(e.bridge, e.memory)
	// Initialize tool executor
	if e.tools != nil {
		e.toolExecutor = NewToolExecutor(e.bridge, e.tools, e.approvalHandler)
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
	// Update modules with new bridge
	if e.approvalHandler != nil {
		e.approvalHandler.SetBridge(bridge)
	}
	e.streamProcessor = NewStreamProcessor(bridge, e.memory)
	if e.tools != nil {
		e.toolExecutor = NewToolExecutor(bridge, e.tools, e.approvalHandler)
	}
}

// SetAutoApprove toggles auto-approval behaviors based on settings
func (e *Engine) SetAutoApprove(shell bool, edits bool) {
	if e.approvalHandler != nil {
		e.approvalHandler.SetAutoApprove(shell, edits)
	}
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
	if e.conversationMgr == nil {
		return nil, errors.New("conversation manager not initialized")
	}
	return e.conversationMgr.ListConversations()
}

// CurrentConversationID returns the active conversation id.
func (e *Engine) CurrentConversationID() string {
	if e.conversationMgr == nil {
		return ""
	}
	return e.conversationMgr.CurrentConversationID()
}

// SetCurrentConversationID switches the active conversation id.
func (e *Engine) SetCurrentConversationID(id string) error {
	if e.conversationMgr == nil {
		return errors.New("conversation manager not initialized")
	}
	return e.conversationMgr.SetCurrentConversationID(id)
}

// GetConversation returns the messages for the given conversation id.
func (e *Engine) GetConversation(id string) ([]Message, error) {
	if e.conversationMgr == nil {
		return nil, errors.New("conversation manager not initialized")
	}
	return e.conversationMgr.GetConversation(id)
}

// NewConversation creates and switches to a new conversation.
func (e *Engine) NewConversation() string {
	if e.conversationMgr == nil {
		return ""
	}
	id := e.conversationMgr.NewConversation()
	// Clear any attached files for the new conversation
	e.mu.Lock()
	e.attachedFiles = nil
	e.mu.Unlock()
	return id
}

// ClearConversation clears the current conversation history in memory and notifies the UI.
func (e *Engine) ClearConversation() {
	if e.conversationMgr != nil {
		e.conversationMgr.ClearConversation()
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
	if e.approvalHandler != nil {
		e.approvalHandler.ResolveApproval(id, approved)
	}
}

// ResolveChoice resolves a pending choice request.
func (e *Engine) ResolveChoice(id string, selectedIndex int) {
	if e.approvalHandler != nil {
		e.approvalHandler.ResolveChoice(id, selectedIndex)
	}
}

// UserApproved prompts for approval and waits for the response.
func (e *Engine) UserApproved(toolCall *tool.ToolCall, diff string) bool {
	if e.approvalHandler != nil {
		return e.approvalHandler.UserApproved(toolCall, diff)
	}
	return false
}

// UserChoice prompts for a choice and waits for the response.
func (e *Engine) UserChoice(toolCall *tool.ToolCall, question string, options []string) int {
	if e.approvalHandler != nil {
		return e.approvalHandler.UserChoice(toolCall, question, options)
	}
	return -1
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

	// Always update the system prompt to reflect current personality and context
	// This allows personality changes to take effect mid-conversation
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
		ModelName:             e.GetModelLabel(),
	})
	if ui := strings.TrimSpace(e.formatEditorContext()); ui != "" {
		base = strings.TrimSpace(base) + "\n\nUI Context:\n- " + ui
	}
	convo.UpdateSystemMessage(base)

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
		// No longer inject attachments as system context; they are appended to the user message on send

		// Call the LLM with the conversation history (+ transient UI hint)
		stream, err := adapter.Chat(ctx, engineMessages, convertSchemas(tools), true)
		if err != nil {
			e.bridge.SendChat("system", "Error: "+err.Error())
			return err
		}

		// Process the LLM response using stream processor
		result := e.streamProcessor.ProcessStream(ctx, stream, convo)
		if ctx.Err() != nil {
			// Send cancellation message to UI
			if e.bridge != nil {
				e.bridge.SendChat("system", "Operation stopped by user.")
			}
			return ctx.Err()
		}

		currentContent := result.Content
		toolCallReceived := result.ToolCall
		streamEnded := result.StreamEnded

		// If we got a tool call, execute it
		if toolCallReceived != nil {
			// Mark that at least one tool was used in this turn
			toolsUsed = true
			// Reset empty response counter since we got a tool call
			consecutiveEmptyAfterTools = 0
			// Execute the tool using the tool executor
			if err := e.toolExecutor.ExecuteToolCall(ctx, toolCallReceived, convo); err != nil {
				return err
			}
			// Continue the loop to get the next assistant message
			continue
		}

		// If we reach here with content but no tool call, record it
		if currentContent != "" {
			convo.AddAssistant(currentContent)
			// Content received means conversation is complete, regardless of whether tools were used
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
				// Execute the tool using the tool executor
				if err := e.toolExecutor.ExecuteToolCall(ctx, toolCallReceived, convo); err != nil {
					return err
				}
				continue
			}
			if currentContent != "" {
				convo.AddAssistant(currentContent)
				e.bridge.EmitAssistant(currentContent)
				// Content received means conversation is complete, regardless of whether tools were used
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

// SetAttachedFiles stores the list of workspace-relative files attached by the user.
// Paths are normalized to forward slashes and trimmed. Empty entries are ignored.
func (e *Engine) SetAttachedFiles(paths []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.attachedFiles = normalizeAttachedFiles(paths)
}

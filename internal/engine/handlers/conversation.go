package handlers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/loom/loom/internal/config"
	"github.com/loom/loom/internal/engine"
	"github.com/loom/loom/internal/memory"
	"github.com/loom/loom/internal/tool"
)

// ConversationHandler manages conversation state and memory operations
type ConversationHandler struct {
	memory       *memory.Project
	workspaceDir string
}

// NewConversationHandler creates a new conversation handler
func NewConversationHandler(mem *memory.Project, workspaceDir string) *ConversationHandler {
	return &ConversationHandler{
		memory:       mem,
		workspaceDir: workspaceDir,
	}
}

// PrepareConversation initializes a conversation with system prompt and user message
func (ch *ConversationHandler) PrepareConversation(
	userMsg string,
	toolSchemas []tool.Schema,
	editorContext string,
	modelLabel string,
	personality string,
) (*memory.Conversation, error) {
	// Start or load conversation
	convo := ch.memory.StartConversation()

	// Always update the system prompt to reflect current personality and context
	// This allows personality changes to take effect mid-conversation
	userRules, projectRules, _ := config.LoadRules(ch.workspaceDir)
	mems := ch.loadUserMemoriesForPrompt()
	base := engine.GenerateSystemPromptUnified(engine.SystemPromptOptions{
		Tools:                 toolSchemas,
		UserRules:             userRules,
		ProjectRules:          projectRules,
		Memories:              mems,
		Personality:           personality,
		WorkspaceRoot:         ch.workspaceDir,
		IncludeProjectContext: true,
	})

	if ui := strings.TrimSpace(editorContext); ui != "" {
		base = strings.TrimSpace(base) + "\n\nUI Context:\n- " + ui
	}
	convo.UpdateSystemMessage(base)

	// Add latest user message
	convo.AddUser(userMsg)

	// Set title for new conversations
	if ch.memory != nil {
		currentID := ch.memory.CurrentConversationID()
		if currentID != "" && ch.memory.GetConversationTitle(currentID) == "" {
			// Title: first (~50 chars) of the user's first message
			title := userMsg
			if len(title) > 50 {
				title = title[:50] + "…"
			}
			_ = ch.memory.SetConversationTitle(currentID, title)
		}
	}

	return convo, nil
}

// ConvertToEngineMessages converts memory messages to engine messages
func (ch *ConversationHandler) ConvertToEngineMessages(memoryMessages []memory.Message) []engine.Message {
	engineMessages := make([]engine.Message, 0, len(memoryMessages))

	for _, msg := range memoryMessages {
		engineMsg := engine.Message{
			Role:    msg.Role,
			Content: msg.Content,
			Name:    msg.Name,
			ToolID:  msg.ToolID,
		}
		engineMessages = append(engineMessages, engineMsg)
	}

	return engineMessages
}

// AddTransientContext adds temporary context messages that won't be persisted
func (ch *ConversationHandler) AddTransientContext(
	messages []engine.Message,
	editorContext, workflowContext string,
) []engine.Message {
	// Append up-to-date UI editor context as a transient system hint for this turn
	if ui := strings.TrimSpace(editorContext); ui != "" {
		messages = append(messages, engine.Message{
			Role:    "system",
			Content: "UI Context: " + ui,
		})
	}

	return messages
}

// loadUserMemoriesForPrompt reads ~/.loom/memories.json and returns entries for prompt injection.
func (ch *ConversationHandler) loadUserMemoriesForPrompt() []engine.MemoryEntry {
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
		out := make([]engine.MemoryEntry, 0, len(list))
		for _, it := range list {
			out = append(out, engine.MemoryEntry{ID: strings.TrimSpace(it.ID), Text: strings.TrimSpace(it.Text)})
		}
		return out
	}
	var wrapper struct {
		Memories []mem `json:"memories"`
	}
	if json.Unmarshal(data, &wrapper) == nil && wrapper.Memories != nil {
		out := make([]engine.MemoryEntry, 0, len(wrapper.Memories))
		for _, it := range wrapper.Memories {
			out = append(out, engine.MemoryEntry{ID: strings.TrimSpace(it.ID), Text: strings.TrimSpace(it.Text)})
		}
		return out
	}
	return nil
}

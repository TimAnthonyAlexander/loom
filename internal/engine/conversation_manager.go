package engine

import (
	"errors"

	"github.com/loom/loom/internal/memory"
)

// ConversationManager handles conversation lifecycle operations for the Engine.
type ConversationManager struct {
	memory *memory.Project
}

// NewConversationManager creates a new conversation manager.
func NewConversationManager(mem *memory.Project) *ConversationManager {
	return &ConversationManager{
		memory: mem,
	}
}

// ListConversations returns summaries for available conversations.
func (cm *ConversationManager) ListConversations() ([]memory.ConversationSummary, error) {
	if cm.memory == nil {
		return nil, errors.New("memory not initialized")
	}
	// Immediately remove any non-current empty conversations
	cm.memory.CleanupEmptyConversations(cm.memory.CurrentConversationID())
	return cm.memory.ListConversationSummaries()
}

// CurrentConversationID returns the active conversation id.
func (cm *ConversationManager) CurrentConversationID() string {
	if cm.memory == nil {
		return ""
	}
	return cm.memory.CurrentConversationID()
}

// SetCurrentConversationID switches the active conversation id.
func (cm *ConversationManager) SetCurrentConversationID(id string) error {
	if cm.memory == nil {
		return errors.New("memory not initialized")
	}
	return cm.memory.SetCurrentConversationID(id)
}

// GetConversation returns the messages for the given conversation id.
func (cm *ConversationManager) GetConversation(id string) ([]Message, error) {
	if cm.memory == nil {
		return nil, errors.New("memory not initialized")
	}
	var memMsgs []memory.Message
	if err := cm.memory.Get("conversations/"+id, &memMsgs); err != nil {
		return nil, err
	}
	msgs := make([]Message, 0, len(memMsgs))
	for _, m := range memMsgs {
		msgs = append(msgs, Message{Role: m.Role, Content: m.Content, Name: m.Name, ToolID: m.ToolID})
	}
	return msgs, nil
}

// NewConversation creates and switches to a new conversation.
func (cm *ConversationManager) NewConversation() string {
	if cm.memory == nil {
		return ""
	}
	id := cm.memory.CreateNewConversation()
	// Immediately remove any non-current empty conversations
	cm.memory.CleanupEmptyConversations(id)
	return id
}

// ClearConversation clears the current conversation history in memory.
func (cm *ConversationManager) ClearConversation() string {
	if cm.memory != nil {
		newID := cm.memory.CreateNewConversation()
		// Remove any non-current conversations with no user messages immediately
		cm.memory.CleanupEmptyConversations(newID)
		return newID
	}
	return ""
}

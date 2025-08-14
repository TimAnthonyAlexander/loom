package memory

import (
	"encoding/json"
	"time"
)

// Message represents a single message in the conversation history.
type Message struct {
	Role      string      `json:"role"`               // user, assistant, system, function, tool
	Content   string      `json:"content"`            // text content of the message
	Name      string      `json:"name,omitempty"`     // function/tool name when applicable
	ToolID    string      `json:"tool_id,omitempty"`  // ID for tool invocations
	Metadata  interface{} `json:"metadata,omitempty"` // Optional metadata
	Timestamp time.Time   `json:"timestamp"`          // When the message was created
}

// Conversation manages a single conversation thread with the LLM.
type Conversation struct {
	project  *Project
	id       string
	messages []Message
}

// NewConversation creates a new conversation.
func NewConversation(project *Project, id string) *Conversation {
	conv := &Conversation{
		project:  project,
		id:       id,
		messages: []Message{},
	}

	// Try to load existing conversation
	var savedMessages []Message
	if project.Has("conversations/" + id) {
		err := project.Get("conversations/"+id, &savedMessages)
		if err == nil && len(savedMessages) > 0 {
			conv.messages = savedMessages
		}
	}

	return conv
}

// AddSystem adds a system message to the conversation.
func (c *Conversation) AddSystem(content string) {
	c.messages = append(c.messages, Message{
		Role:      "system",
		Content:   content,
		Timestamp: time.Now(),
	})
	c.save()
}

// AddUser adds a user message to the conversation.
func (c *Conversation) AddUser(content string) {
	c.messages = append(c.messages, Message{
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	})
	c.save()
}

// AddAssistant adds an assistant message to the conversation.
func (c *Conversation) AddAssistant(content string) {
	c.messages = append(c.messages, Message{
		Role:      "assistant",
		Content:   content,
		Timestamp: time.Now(),
	})
	c.save()
}

// AddAssistantThinking records an assistant thinking block so it can be
// preserved across tool calls for providers that require it (e.g., Anthropic).
func (c *Conversation) AddAssistantThinking(content string) {
	c.messages = append(c.messages, Message{
		Role:      "assistant",
		Name:      "thinking",
		Content:   content,
		Timestamp: time.Now(),
	})
	c.save()
}

// AddAssistantThinkingSigned records a complete thinking block along with its
// signature in the message content as JSON, so adapters can faithfully replay
// the thinking block with its signature.
// Content format: {"thinking":"...","signature":"..."}
func (c *Conversation) AddAssistantThinkingSigned(thinking string, signature string) {
	// Store as JSON payload in Content to preserve through engine -> adapter
	payload := map[string]string{
		"thinking":  thinking,
		"signature": signature,
	}
	b, _ := json.Marshal(payload)
	c.messages = append(c.messages, Message{
		Role:      "assistant",
		Name:      "thinking",
		Content:   string(b),
		Timestamp: time.Now(),
	})
	c.save()
}

// AddTool adds a tool message to the conversation.
func (c *Conversation) AddTool(name string, content string) {
	c.messages = append(c.messages, Message{
		Role:      "tool",
		Name:      name,
		Content:   content,
		Timestamp: time.Now(),
	})
	c.save()
}

// AddToolResult adds a tool result message with a reference to the tool use ID
func (c *Conversation) AddToolResult(name string, toolUseID string, content string) {
	c.messages = append(c.messages, Message{
		Role:      "tool",
		Name:      name,
		ToolID:    toolUseID,
		Content:   content,
		Timestamp: time.Now(),
	})
	c.save()
}

// AddAssistantToolUse adds an assistant tool_use message with the given ID and JSON input (as string)
func (c *Conversation) AddAssistantToolUse(name string, toolUseID string, inputJSON string) {
	c.messages = append(c.messages, Message{
		Role:      "assistant",
		Name:      name,
		ToolID:    toolUseID,
		Content:   inputJSON,
		Timestamp: time.Now(),
	})
	c.save()
}

// History returns the conversation history.
func (c *Conversation) History() []Message {
	return c.messages
}

// Clear removes all messages from the conversation.
func (c *Conversation) Clear() {
	c.messages = []Message{}
	c.save()
}

// save stores the conversation to persistent storage.
func (c *Conversation) save() {
	if c.project != nil {
		_ = c.project.Set("conversations/"+c.id, c.messages)
	}
}

// StartConversation creates a new conversation or continues an existing one.
func (p *Project) StartConversation() *Conversation {
	id := p.CurrentConversationID()
	if id == "" {
		id = "current"
		_ = p.SetCurrentConversationID(id)
	}
	return NewConversation(p, id)
}

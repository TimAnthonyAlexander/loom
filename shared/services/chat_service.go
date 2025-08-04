package services

import (
	"context"
	"loom/chat"
	"loom/llm"
	"loom/shared/events"
	"loom/shared/models"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ChatService handles chat operations and LLM interactions
type ChatService struct {
	session      *chat.Session
	llmAdapter   llm.LLMAdapter
	eventBus     *events.EventBus
	streamChan   chan llm.StreamChunk
	streamCancel context.CancelFunc
	isStreaming  bool
	mutex        sync.RWMutex
}

// NewChatService creates a new chat service
func NewChatService(workspacePath string, llmAdapter llm.LLMAdapter, eventBus *events.EventBus) *ChatService {
	session := chat.NewSession(workspacePath, 50) // Max 50 messages

	return &ChatService{
		session:    session,
		llmAdapter: llmAdapter,
		eventBus:   eventBus,
	}
}

// GetChatState returns the current chat state
func (cs *ChatService) GetChatState() models.ChatState {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	// Convert chat session messages to models.Message
	var messages []models.Message
	for _, msg := range cs.session.GetMessages() {
		messages = append(messages, models.Message{
			ID:        uuid.New().String(),
			Content:   msg.Content,
			IsUser:    msg.Role == "user",
			Timestamp: time.Now(), // Note: chat.Message doesn't have timestamp, using current time
			Type:      msg.Role,
		})
	}

	return models.ChatState{
		Messages:      messages,
		IsStreaming:   cs.isStreaming,
		SessionID:     "current-session", // Note: Session struct doesn't expose ID
		WorkspacePath: cs.session.workspacePath,
	}
}

// SendMessage sends a user message and starts LLM processing
func (cs *ChatService) SendMessage(content string) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// Add user message to session
	userMessage := llm.Message{
		Role:    "user",
		Content: content,
	}
	cs.session.AddMessage(userMessage)

	// Emit user message event
	cs.eventBus.EmitChatMessage(models.Message{
		ID:        uuid.New().String(),
		Content:   content,
		IsUser:    true,
		Timestamp: time.Now(),
		Type:      "user",
	})

	// Start LLM streaming
	go cs.streamLLMResponse()

	return nil
}

// streamLLMResponse handles streaming LLM responses
func (cs *ChatService) streamLLMResponse() {
	cs.mutex.Lock()
	cs.isStreaming = true
	cs.streamChan = make(chan llm.StreamChunk, 10)

	ctx, cancel := context.WithCancel(context.Background())
	cs.streamCancel = cancel
	cs.mutex.Unlock()

	// Emit stream started event
	cs.eventBus.Emit(events.ChatStreamStarted, nil)

	// Get messages for LLM
	messages := cs.session.GetMessages()

	// Start streaming
	go func() {
		defer close(cs.streamChan)

		// Note: LLMAdapter interface may need to be extended for streaming
		// For now, use regular completion and simulate streaming
		response, err := cs.llmAdapter.GenerateCompletion(ctx, messages)
		if err == nil {
			// Simulate streaming by sending the full response
			cs.streamChan <- llm.StreamChunk{Content: response, Done: true}
		}
		if err != nil {
			cs.eventBus.Emit(events.ChatError, map[string]string{
				"error": err.Error(),
			})
			return
		}
	}()

	// Process streaming chunks
	var fullResponse string
	for chunk := range cs.streamChan {
		if chunk.Error != nil {
			cs.eventBus.Emit(events.ChatError, map[string]string{
				"error": chunk.Error.Error(),
			})
			break
		}

		fullResponse += chunk.Content

		// Emit stream chunk event
		cs.eventBus.Emit(events.ChatStreamChunk, map[string]string{
			"content": chunk.Content,
			"full":    fullResponse,
		})

		if chunk.Done {
			break
		}
	}

	// Add assistant response to session
	if fullResponse != "" {
		assistantMessage := llm.Message{
			Role:    "assistant",
			Content: fullResponse,
		}
		cs.session.AddMessage(assistantMessage)

		// Emit final message event
		cs.eventBus.EmitChatMessage(models.Message{
			ID:        uuid.New().String(),
			Content:   fullResponse,
			IsUser:    false,
			Timestamp: time.Now(),
			Type:      "assistant",
		})
	}

	cs.mutex.Lock()
	cs.isStreaming = false
	cs.streamCancel = nil
	cs.mutex.Unlock()

	// Emit stream ended event
	cs.eventBus.Emit(events.ChatStreamEnded, nil)
}

// StopStreaming cancels the current streaming operation
func (cs *ChatService) StopStreaming() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	if cs.streamCancel != nil {
		cs.streamCancel()
		cs.streamCancel = nil
	}
	cs.isStreaming = false
}

// AddSystemMessage adds a system message to the chat
func (cs *ChatService) AddSystemMessage(content string) {
	systemMessage := llm.Message{
		Role:    "system",
		Content: content,
	}
	cs.session.AddMessage(systemMessage)

	cs.eventBus.EmitChatMessage(models.Message{
		ID:        uuid.New().String(),
		Content:   content,
		IsUser:    false,
		Timestamp: time.Now(),
		Type:      "system",
	})
}

// ClearChat clears all messages from the chat session
func (cs *ChatService) ClearChat() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// Create new session to clear messages
	cs.session = chat.NewSession(cs.session.workspacePath, 50)
}

// GetMessages returns all chat messages
func (cs *ChatService) GetMessages() []models.Message {
	state := cs.GetChatState()
	return state.Messages
}

package services

import (
	"context"
	"fmt"
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
	session       *chat.Session
	llmAdapter    llm.LLMAdapter
	eventBus      *events.EventBus
	streamChan    chan llm.StreamChunk
	streamCancel  context.CancelFunc
	isStreaming   bool
	workspacePath string // Store workspace path since session doesn't expose it
	mutex         sync.RWMutex
}

// NewChatService creates a new chat service
func NewChatService(workspacePath string, llmAdapter llm.LLMAdapter, eventBus *events.EventBus) *ChatService {
	session := chat.NewSession(workspacePath, 50) // Max 50 messages

	return &ChatService{
		session:       session,
		llmAdapter:    llmAdapter,
		eventBus:      eventBus,
		workspacePath: workspacePath,
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
		WorkspacePath: cs.getWorkspacePath(),
	}
}

// SendMessage sends a user message and starts LLM processing
func (cs *ChatService) SendMessage(content string) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// Don't start new streaming if already streaming
	if cs.isStreaming {
		return fmt.Errorf("already streaming a response")
	}

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

	// Channel to signal when streaming goroutine is done
	streamDone := make(chan bool, 1)

	// Start streaming
	go func() {
		defer func() {
			// Just signal that streaming is done - the LLM adapter closes the channel
			streamDone <- true
		}()

		// Use the Stream method from LLMAdapter
		// Note: The LLM adapter will close the channel when done
		err := cs.llmAdapter.Stream(ctx, messages, cs.streamChan)
		if err != nil {
			cs.eventBus.Emit(events.ChatError, map[string]string{
				"error": err.Error(),
			})
		}
	}()

	// Process streaming chunks
	var fullResponse string
	streamFinished := false

	// Use select to handle both channel reads and cancellation
	for !streamFinished {
		select {
		case chunk, ok := <-cs.streamChan:
			if !ok {
				// Channel is closed, streaming is done
				streamFinished = true
				continue
			}

			if chunk.Error != nil {
				cs.eventBus.Emit(events.ChatError, map[string]string{
					"error": chunk.Error.Error(),
				})
				streamFinished = true
				continue
			}

			fullResponse += chunk.Content

			// Emit stream chunk event
			cs.eventBus.Emit(events.ChatStreamChunk, map[string]string{
				"content": chunk.Content,
				"full":    fullResponse,
			})

			if chunk.Done {
				streamFinished = true
				continue
			}
		case <-ctx.Done():
			// Context was cancelled (stop streaming called)
			// The LLM adapter will close the channel when it detects cancellation
			streamFinished = true
		}
	}

	// Wait for the streaming goroutine to finish cleanup
	<-streamDone

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
	cs.streamChan = nil // LLM adapter closed the channel, we just clear our reference
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
	}
	// Note: Don't set streamCancel to nil here - let streamLLMResponse clean it up
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
	cs.session = chat.NewSession(cs.getWorkspacePath(), 50)
}

// GetMessages returns all chat messages
func (cs *ChatService) GetMessages() []models.Message {
	state := cs.GetChatState()
	return state.Messages
}

// getWorkspacePath returns the workspace path
func (cs *ChatService) getWorkspacePath() string {
	return cs.workspacePath
}

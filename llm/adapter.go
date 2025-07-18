package llm

import (
	"context"
	"time"
)

// Message represents a chat message
type Message struct {
	Role      string    `json:"role"`      // "system", "user", "assistant"
	Content   string    `json:"content"`   // The message content
	Timestamp time.Time `json:"timestamp"` // When the message was created
}

// StreamChunk represents a chunk of streaming response
type StreamChunk struct {
	Content string
	Error   error
	Done    bool
}

// LLMAdapter defines the interface for LLM providers
type LLMAdapter interface {
	// Send sends messages and returns the complete response
	Send(ctx context.Context, messages []Message) (*Message, error)

	// Stream sends messages and streams the response via the provided channel
	Stream(ctx context.Context, messages []Message, chunks chan<- StreamChunk) error

	// GetModelName returns the current model name
	GetModelName() string

	// IsAvailable checks if the adapter is properly configured and available
	IsAvailable() bool
}

// AdapterConfig contains common configuration for LLM adapters
type AdapterConfig struct {
	Model   string
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

// DefaultTimeout for LLM requests
const DefaultTimeout = 30 * time.Second

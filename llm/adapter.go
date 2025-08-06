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
	Visible   bool      `json:"visible"`   // Whether the message should be displayed in the UI
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
	Model          string
	APIKey         string
	BaseURL        string
	Timeout        time.Duration // General timeout for non-streaming requests
	StreamTimeout  time.Duration // Timeout for streaming requests (usually longer)
	MaxRetries     int           // Maximum number of retries for failed requests
	RetryDelayBase time.Duration // Base delay for exponential backoff (e.g., 1s)
}

// Default timeout values
const (
	DefaultTimeout        = 120 * time.Second // Increased from 30s to 120s
	DefaultStreamTimeout  = 300 * time.Second // 5 minutes for streaming (can be long)
	DefaultMaxRetries     = 3                 // Retry up to 3 times
	DefaultRetryDelayBase = 1 * time.Second   // Start with 1s delay, then 2s, 4s, etc.
)

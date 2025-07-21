package llm

import (
	"context"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/sashabaranov/go-openai"
)

// OpenAIAdapter implements LLMAdapter for OpenAI API
type OpenAIAdapter struct {
	client *openai.Client
	config AdapterConfig
}

// NewOpenAIAdapter creates a new OpenAI adapter
func NewOpenAIAdapter(config AdapterConfig) *OpenAIAdapter {
	client := openai.NewClient(config.APIKey)

	// Set custom base URL if provided
	if config.BaseURL != "" {
		clientConfig := openai.DefaultConfig(config.APIKey)
		clientConfig.BaseURL = config.BaseURL
		client = openai.NewClientWithConfig(clientConfig)
	}

	// Set default timeouts if not provided
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}
	if config.StreamTimeout == 0 {
		config.StreamTimeout = DefaultStreamTimeout
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = DefaultMaxRetries
	}
	if config.RetryDelayBase == 0 {
		config.RetryDelayBase = DefaultRetryDelayBase
	}

	return &OpenAIAdapter{
		client: client,
		config: config,
	}
}

// isRetryableError checks if an error is worth retrying
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := err.Error()

	// Retry on context deadline exceeded (timeout)
	if strings.Contains(errorStr, "context deadline exceeded") {
		return true
	}

	// Retry on network errors
	if strings.Contains(errorStr, "connection refused") ||
		strings.Contains(errorStr, "connection reset") ||
		strings.Contains(errorStr, "connection timeout") ||
		strings.Contains(errorStr, "no such host") ||
		strings.Contains(errorStr, "network is unreachable") {
		return true
	}

	// Check for specific network error types
	if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
		return true
	}

	// Check for syscall errors (like ECONNRESET)
	if syscallErr, ok := err.(syscall.Errno); ok {
		switch syscallErr {
		case syscall.ECONNRESET, syscall.ECONNREFUSED, syscall.ETIMEDOUT:
			return true
		}
	}

	// Retry on OpenAI rate limiting (HTTP 429) and server errors (5xx)
	if strings.Contains(errorStr, "rate limit") ||
		strings.Contains(errorStr, "429") ||
		strings.Contains(errorStr, "500") ||
		strings.Contains(errorStr, "502") ||
		strings.Contains(errorStr, "503") ||
		strings.Contains(errorStr, "504") {
		return true
	}

	return false
}

// calculateBackoffDelay calculates the delay for exponential backoff
func (o *OpenAIAdapter) calculateBackoffDelay(attempt int) time.Duration {
	// Exponential backoff: base * 2^attempt with jitter
	multiplier := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(o.config.RetryDelayBase) * multiplier)

	// Cap the delay at 30 seconds
	maxDelay := 30 * time.Second
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// Send implements LLMAdapter.Send
func (o *OpenAIAdapter) Send(ctx context.Context, messages []Message) (*Message, error) {
	var lastErr error

	for attempt := 0; attempt <= o.config.MaxRetries; attempt++ {
		// Create timeout context for this attempt
		timeoutCtx, cancel := context.WithTimeout(ctx, o.config.Timeout)

		// Convert our messages to OpenAI format
		openaiMessages := make([]openai.ChatCompletionMessage, len(messages))
		for i, msg := range messages {
			openaiMessages[i] = openai.ChatCompletionMessage{
				Role:    msg.Role,
				Content: msg.Content,
			}
		}

		resp, err := o.client.CreateChatCompletion(timeoutCtx, openai.ChatCompletionRequest{
			Model:    o.config.Model,
			Messages: openaiMessages,
		})

		cancel() // Always cancel the timeout context

		if err == nil {
			// Success!
			if len(resp.Choices) == 0 {
				return nil, fmt.Errorf("no response from OpenAI")
			}

			return &Message{
				Role:      resp.Choices[0].Message.Role,
				Content:   resp.Choices[0].Message.Content,
				Timestamp: time.Now(),
			}, nil
		}

		lastErr = err

		// Don't retry if the error is not retryable
		if !isRetryableError(err) {
			break
		}

		// Don't retry on the last attempt
		if attempt == o.config.MaxRetries {
			break
		}

		// Wait before retrying
		delay := o.calculateBackoffDelay(attempt)
		fmt.Printf("OpenAI request failed (attempt %d/%d), retrying in %v: %v\n", 
			attempt+1, o.config.MaxRetries+1, delay, err)

		select {
		case <-time.After(delay):
			// Continue with retry
		case <-ctx.Done():
			// Context was cancelled, don't retry
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("OpenAI API error after %d attempts: %w", o.config.MaxRetries+1, lastErr)
}

// Stream implements LLMAdapter.Stream
func (o *OpenAIAdapter) Stream(ctx context.Context, messages []Message, chunks chan<- StreamChunk) error {
	defer close(chunks)

	var lastErr error

	for attempt := 0; attempt <= o.config.MaxRetries; attempt++ {
		// Create timeout context for this attempt (use longer timeout for streaming)
		timeoutCtx, cancel := context.WithTimeout(ctx, o.config.StreamTimeout)

		// Convert our messages to OpenAI format
		openaiMessages := make([]openai.ChatCompletionMessage, len(messages))
		for i, msg := range messages {
			openaiMessages[i] = openai.ChatCompletionMessage{
				Role:    msg.Role,
				Content: msg.Content,
			}
		}

		stream, err := o.client.CreateChatCompletionStream(timeoutCtx, openai.ChatCompletionRequest{
			Model:    o.config.Model,
			Messages: openaiMessages,
			Stream:   true,
		})

		if err != nil {
			cancel()
			lastErr = err

			// Don't retry if the error is not retryable
			if !isRetryableError(err) {
				break
			}

			// Don't retry on the last attempt
			if attempt == o.config.MaxRetries {
				break
			}

			// Wait before retrying
			delay := o.calculateBackoffDelay(attempt)
			fmt.Printf("OpenAI stream creation failed (attempt %d/%d), retrying in %v: %v\n", 
				attempt+1, o.config.MaxRetries+1, delay, err)

			select {
			case <-time.After(delay):
				// Continue with retry
				continue
			case <-ctx.Done():
				// Context was cancelled, don't retry
				chunks <- StreamChunk{Error: ctx.Err()}
				return ctx.Err()
			}
		}

		// Stream created successfully, now read from it
		streamErr := o.readFromStream(timeoutCtx, stream, chunks)
		cancel() // Always cancel the timeout context

		if streamErr == nil {
			// Success!
			return nil
		}

		lastErr = streamErr

		// Don't retry if the error is not retryable
		if !isRetryableError(streamErr) {
			break
		}

		// Don't retry on the last attempt
		if attempt == o.config.MaxRetries {
			break
		}

		// Wait before retrying
		delay := o.calculateBackoffDelay(attempt)
		fmt.Printf("OpenAI stream read failed (attempt %d/%d), retrying in %v: %v\n", 
			attempt+1, o.config.MaxRetries+1, delay, streamErr)

		select {
		case <-time.After(delay):
			// Continue with retry
		case <-ctx.Done():
			// Context was cancelled, don't retry
			chunks <- StreamChunk{Error: ctx.Err()}
			return ctx.Err()
		}
	}

	finalErr := fmt.Errorf("OpenAI stream error after %d attempts: %w", o.config.MaxRetries+1, lastErr)
	chunks <- StreamChunk{Error: finalErr}
	return finalErr
}

// readFromStream reads data from the OpenAI stream
func (o *OpenAIAdapter) readFromStream(ctx context.Context, stream *openai.ChatCompletionStream, chunks chan<- StreamChunk) error {
	defer stream.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			response, err := stream.Recv()
			if err == io.EOF {
				chunks <- StreamChunk{Done: true}
				return nil
			}

			if err != nil {
				return fmt.Errorf("OpenAI stream recv error: %w", err)
			}

			if len(response.Choices) > 0 {
				content := response.Choices[0].Delta.Content
				if content != "" {
					chunks <- StreamChunk{Content: content}
				}
			}
		}
	}
}

// GetModelName implements LLMAdapter.GetModelName
func (o *OpenAIAdapter) GetModelName() string {
	return o.config.Model
}

// IsAvailable implements LLMAdapter.IsAvailable
func (o *OpenAIAdapter) IsAvailable() bool {
	return o.config.APIKey != "" && o.config.Model != ""
}

// parseOpenAIModel extracts the model name from a model string like "openai:gpt-4o"
func parseOpenAIModel(modelStr string) string {
	parts := strings.SplitN(modelStr, ":", 2)
	if len(parts) == 2 && parts[0] == "openai" {
		return parts[1]
	}
	// Fallback - assume it's just the model name
	return modelStr
}

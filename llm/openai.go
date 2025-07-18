package llm

import (
	"context"
	"fmt"
	"io"
	"strings"
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

	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}

	return &OpenAIAdapter{
		client: client,
		config: config,
	}
}

// Send implements LLMAdapter.Send
func (o *OpenAIAdapter) Send(ctx context.Context, messages []Message) (*Message, error) {
	ctx, cancel := context.WithTimeout(ctx, o.config.Timeout)
	defer cancel()

	// Convert our messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	resp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    o.config.Model,
		Messages: openaiMessages,
	})

	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	return &Message{
		Role:      resp.Choices[0].Message.Role,
		Content:   resp.Choices[0].Message.Content,
		Timestamp: time.Now(),
	}, nil
}

// Stream implements LLMAdapter.Stream
func (o *OpenAIAdapter) Stream(ctx context.Context, messages []Message, chunks chan<- StreamChunk) error {
	defer close(chunks)

	ctx, cancel := context.WithTimeout(ctx, o.config.Timeout)
	defer cancel()

	// Convert our messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	stream, err := o.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    o.config.Model,
		Messages: openaiMessages,
		Stream:   true,
	})

	if err != nil {
		chunks <- StreamChunk{Error: fmt.Errorf("OpenAI stream error: %w", err)}
		return err
	}
	defer stream.Close()

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			chunks <- StreamChunk{Done: true}
			return nil
		}

		if err != nil {
			chunks <- StreamChunk{Error: fmt.Errorf("OpenAI stream recv error: %w", err)}
			return err
		}

		if len(response.Choices) > 0 {
			content := response.Choices[0].Delta.Content
			if content != "" {
				chunks <- StreamChunk{Content: content}
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

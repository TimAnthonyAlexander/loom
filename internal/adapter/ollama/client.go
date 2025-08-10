package ollama

import (
	"context"

	"github.com/loom/loom/internal/adapter/openai"
	"github.com/loom/loom/internal/engine"
)

// Client is a wrapper around OpenAI client that connects to Ollama.
type Client struct {
	openaiClient *openai.Client
}

// New creates a new Ollama client using the OpenAI-compatible API.
func New(endpoint string, model string) *Client {
	// Ollama uses the same API schema as OpenAI but with a custom endpoint
	client := openai.New("ollama", model)

	// Set the endpoint to the Ollama API
	if endpoint == "" {
		endpoint = "http://localhost:11434/v1/chat/completions"
	}
	client.WithEndpoint(endpoint)

	return &Client{
		openaiClient: client,
	}
}

// Chat implements the engine.LLM interface.
func (c *Client) Chat(
	ctx context.Context,
	messages []engine.Message,
	tools []engine.ToolSchema,
	stream bool,
) (<-chan engine.TokenOrToolCall, error) {
	// Delegate to the wrapped OpenAI client
	return c.openaiClient.Chat(ctx, messages, tools, stream)
}

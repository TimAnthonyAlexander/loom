package openai

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/loom/loom/internal/engine"
)

// Client handles interaction with OpenAI APIs.
type Client struct {
	apiKey string
	model  string
}

// New creates a new OpenAI client.
func New(apiKey string, model string) *Client {
	if model == "" {
		model = "o4-mini" // Default model
	}

	return &Client{
		apiKey: apiKey,
		model:  model,
	}
}

// Chat implements the engine.LLM interface for OpenAI.
func (c *Client) Chat(
	ctx context.Context,
	messages []engine.Message,
	tools []engine.ToolSchema,
	stream bool,
) (<-chan engine.TokenOrToolCall, error) {
	if c.apiKey == "" {
		return nil, errors.New("OpenAI API key not set")
	}

	// Create output channel for tokens/tool calls
	resultCh := make(chan engine.TokenOrToolCall)

	// Convert messages and tools to OpenAI format
	openaiMessages := convertMessages(messages)
	openaiTools := convertTools(tools)

	// Prepare the request body
	requestBody := map[string]interface{}{
		"model":       c.model,
		"messages":    openaiMessages,
		"temperature": 0.2,
		"stream":      stream,
	}

	// Add tools if provided
	if len(tools) > 0 {
		requestBody["tools"] = openaiTools
		requestBody["tool_choice"] = "auto"
	}

	// Start a goroutine to handle the streaming response
	go func() {
		defer close(resultCh)

		// TODO: Implement the actual API call to OpenAI
		// This is just a placeholder - real implementation would use the official OpenAI Go SDK
		// or make HTTP requests directly to the OpenAI API

		// Mock response for now
		mockResponse(ctx, resultCh)
	}()

	return resultCh, nil
}

// convertMessages transforms engine messages to OpenAI format.
func convertMessages(messages []engine.Message) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))

	for _, msg := range messages {
		openaiMsg := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}

		// Add function name for function messages
		if msg.Role == "function" && msg.Name != "" {
			openaiMsg["name"] = msg.Name
		}

		result = append(result, openaiMsg)
	}

	return result
}

// convertTools transforms engine tool schemas to OpenAI format.
func convertTools(tools []engine.ToolSchema) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(tools))

	for _, tool := range tools {
		openaiTool := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Schema,
			},
		}

		result = append(result, openaiTool)
	}

	return result
}

// mockResponse is a placeholder for testing that simulates OpenAI responses.
func mockResponse(ctx context.Context, ch chan<- engine.TokenOrToolCall) {
	// Simulate some text tokens
	tokens := []string{"Hello", " there", "!", " I'm", " ready", " to", " help", " you", " with", " coding", "."}

	for _, token := range tokens {
		select {
		case <-ctx.Done():
			return
		case ch <- engine.TokenOrToolCall{Token: token}:
			// Successfully sent token
		}
	}

	// Simulate a tool call
	// Sample JSON structure for a tool call (for reference only)
	_ = `{
		"name": "read_file",
		"arguments": {
			"path": "README.md"
		}
	}`

	var args json.RawMessage
	if err := json.Unmarshal([]byte(`{"path": "README.md"}`), &args); err != nil {
		// Just skip the tool call if we can't parse it
		return
	}

	select {
	case <-ctx.Done():
		return
	case ch <- engine.TokenOrToolCall{
		ToolCall: &engine.ToolCall{
			ID:     "mock-tool-call-1",
			Name:   "read_file",
			Args:   args,
			IsSafe: true,
		},
	}:
		// Successfully sent tool call
	}
}

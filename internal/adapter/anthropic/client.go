package anthropic

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/loom/loom/internal/engine"
)

// Client handles interaction with Anthropic Claude APIs.
type Client struct {
	apiKey string
	model  string
}

// ToolUse represents a tool use request from Claude.
type ToolUse struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Input  json.RawMessage `json:"input"`
	Type   string          `json:"type"`
	IsSafe bool            // Not part of API, for internal use
}

// New creates a new Anthropic Claude client.
func New(apiKey string, model string) *Client {
	if model == "" {
		model = "claude-3-opus-20240229" // Default model
	}

	return &Client{
		apiKey: apiKey,
		model:  model,
	}
}

// Chat implements the engine.LLM interface for Anthropic Claude.
func (c *Client) Chat(
	ctx context.Context,
	messages []engine.Message,
	tools []engine.ToolSchema,
	stream bool,
) (<-chan engine.TokenOrToolCall, error) {
	if c.apiKey == "" {
		return nil, errors.New("Anthropic API key not set")
	}

	// Create output channel for tokens/tool calls
	resultCh := make(chan engine.TokenOrToolCall)

	// Convert messages and tools to Claude format
	claudeMessages := convertMessages(messages)
	claudeTools := convertTools(tools)

	// Prepare the request body
	requestBody := map[string]interface{}{
		"model":       c.model,
		"messages":    claudeMessages,
		"temperature": 0.2,
		"stream":      stream,
	}

	// Add tools if provided
	if len(tools) > 0 {
		requestBody["tools"] = claudeTools
	}

	// Start a goroutine to handle the streaming response
	go func() {
		defer close(resultCh)

		// TODO: Implement the actual API call to Anthropic
		// This is just a placeholder - real implementation would use the Anthropic API

		// Mock response for now
		mockResponse(ctx, resultCh)
	}()

	return resultCh, nil
}

// convertMessages transforms engine messages to Anthropic Claude format.
func convertMessages(messages []engine.Message) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))

	for _, msg := range messages {
		claudeMsg := map[string]interface{}{
			"role": convertRole(msg.Role),
		}

		// Handle content based on role
		switch msg.Role {
		case "function", "tool":
			claudeMsg["content"] = []map[string]interface{}{
				{
					"type":        "tool_result",
					"tool_use_id": msg.ToolID,
					"content":     msg.Content,
				},
			}
		default:
			claudeMsg["content"] = msg.Content
		}

		result = append(result, claudeMsg)
	}

	return result
}

// convertRole maps standard roles to Claude roles.
func convertRole(role string) string {
	switch role {
	case "assistant":
		return "assistant"
	case "function", "tool":
		return "assistant"
	case "system":
		return "system"
	default:
		return "user"
	}
}

// convertTools transforms engine tool schemas to Anthropic Claude format.
func convertTools(tools []engine.ToolSchema) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(tools))

	for _, tool := range tools {
		claudeTool := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"input_schema": map[string]interface{}{
				"type":       "object",
				"properties": tool.Schema["properties"],
				"required":   tool.Schema["required"],
			},
		}

		result = append(result, claudeTool)
	}

	return result
}

// mockResponse is a placeholder for testing that simulates Claude responses.
func mockResponse(ctx context.Context, ch chan<- engine.TokenOrToolCall) {
	// Simulate some text tokens
	tokens := []string{"Hello", " there", "!", " I'm", " Claude", " and", " I'm", " ready", " to", " help", " you", " with", " coding", "."}

	for _, token := range tokens {
		select {
		case <-ctx.Done():
			return
		case ch <- engine.TokenOrToolCall{Token: token}:
			// Successfully sent token
		}
	}

	// Simulate a tool call
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

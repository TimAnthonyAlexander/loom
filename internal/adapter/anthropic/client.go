package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/loom/loom/internal/engine"
)

// Client handles interaction with Anthropic Claude APIs.
type Client struct {
	apiKey     string
	model      string
	endpoint   string
	apiVersion string
	httpClient *http.Client
	maxTokens  int // Maximum tokens in response
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
		model = "claude-sonnet-4-20250514" // Default model
	}

	return &Client{
		apiKey:     apiKey,
		model:      model,
		endpoint:   "https://api.anthropic.com/v1/messages",
		apiVersion: "2023-06-01", // Required API version for Claude
		maxTokens:  4000,         // Default max tokens for responses
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// WithEndpoint sets a custom endpoint for the Anthropic API.
func (c *Client) WithEndpoint(endpoint string) *Client {
	c.endpoint = endpoint
	return c
}

// WithAPIVersion sets a custom API version for the Anthropic API.
func (c *Client) WithAPIVersion(version string) *Client {
	c.apiVersion = version
	return c
}

// WithMaxTokens sets the maximum number of tokens in the response.
func (c *Client) WithMaxTokens(maxTokens int) *Client {
	c.maxTokens = maxTokens
	return c
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

	// Extract system prompt and convert messages (Anthropic expects system at top-level)
	var systemPrompt string
	for _, m := range messages {
		if strings.ToLower(m.Role) == "system" && m.Content != "" {
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += m.Content
		}
	}

	// Convert messages and tools to Claude format (excluding system messages)
	claudeMessages := convertMessages(messages)
	claudeTools := convertTools(tools)

	// Remove provider prefix if present (e.g., "claude:" prefix)
	modelID := strings.TrimPrefix(c.model, "claude:")

	// Prepare the request body
	// Anthropic expects model IDs like "claude-opus-4-20250514" without provider prefix
	requestBody := map[string]interface{}{
		"model":       modelID,
		"messages":    claudeMessages,
		"max_tokens":  c.maxTokens, // Required parameter for Anthropic API
		"temperature": 0.2,
		// Temporarily disable streaming for reliability until SSE parser is hardened
		"stream": false,
	}
	if systemPrompt != "" {
		requestBody["system"] = systemPrompt
	}

	fmt.Printf("DEBUG: Using Anthropic model: %s (max_tokens: %d)\n", modelID, c.maxTokens)

	// Add tools if provided
	if len(tools) > 0 {
		requestBody["tools"] = claudeTools
	}

	// Start a goroutine to handle the streaming response
	go func() {
		defer close(resultCh)

		// Prepare request body
		reqBody, err := json.Marshal(requestBody)
		if err != nil {
			// Handle marshal error - can't do much but close the channel
			return
		}

		// Create request
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(reqBody))
		if err != nil {
			// Handle request creation error
			return
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		// Streaming responses are delivered via Server-Sent Events
		if stream {
			req.Header.Set("Accept", "text/event-stream")
			req.Header.Set("Cache-Control", "no-cache")
			req.Header.Set("Connection", "keep-alive")
		}
		fmt.Printf("DEBUG: Using Anthropic API key: %s...\n", c.apiKey[:min(10, len(c.apiKey))])
		// Anthropic requires 'x-api-key' header, not 'Authorization'
		req.Header.Set("x-api-key", c.apiKey)

		// API version is required for Anthropic
		req.Header.Set("anthropic-version", c.apiVersion)
		fmt.Printf("DEBUG: Using anthropic-version: %s\n", c.apiVersion)

		// Log request basics
		fmt.Printf("Anthropic: POST %s | model=%s | stream=false | messages=%d | tools=%d\n", c.endpoint, modelID, len(claudeMessages), len(claudeTools))
		// Make the request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Handle request error
			fmt.Printf("Anthropic HTTP error: %v\n", err)
			return
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			// Log or handle non-200 status with improved error message
			errorResponse, _ := io.ReadAll(resp.Body)
			fmt.Printf("Anthropic API error (status %d): %s\n", resp.StatusCode, errorResponse)
			fmt.Printf("Debug: Request sent to: %s with model: %s, max_tokens: %d\n",
				c.endpoint, c.model, c.maxTokens)
			return
		}

		fmt.Printf("Anthropic: status=%d content-type=%s\n", resp.StatusCode, resp.Header.Get("Content-Type"))
		// Handle response (non-streaming for now)
		c.handleNonStreamingResponse(ctx, resp.Body, resultCh)
	}()

	return resultCh, nil
}

// handleStreamingResponse processes a streaming response from the Anthropic API.
func (c *Client) handleStreamingResponse(ctx context.Context, body io.Reader, ch chan<- engine.TokenOrToolCall) {
	scanner := bufio.NewScanner(body)
	// Increase buffer for large SSE lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	// Keep track of the current assistant response
	var currentContent string
	var toolCallID string
	var toolCallName string
	var toolCallInput json.RawMessage

	// Process each line in the stream
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Lines from the stream start with "data: "
		if !strings.HasPrefix(line, "event: ") && !strings.HasPrefix(line, "data: ") {
			continue
		}

		// Check for event type
		if strings.HasPrefix(line, "event: ") {
			// Event might indicate content_block_start, content_block_delta, etc.
			continue
		}

		// Extract the data part
		data := strings.TrimPrefix(line, "data: ")

		// Parse the JSON
		var streamResp struct {
			Type       string `json:"type"`
			StopReason string `json:"stop_reason"`
			Delta      struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				ToolUse *struct {
					ID    string          `json:"id"`
					Name  string          `json:"name"`
					Type  string          `json:"type"`
					Input json.RawMessage `json:"input"`
				} `json:"tool_use"`
			} `json:"delta"`
			Message struct {
				Content []struct {
					Type    string `json:"type"`
					Text    string `json:"text"`
					ToolUse *struct {
						ID    string          `json:"id"`
						Name  string          `json:"name"`
						Type  string          `json:"type"`
						Input json.RawMessage `json:"input"`
					} `json:"tool_use"`
				} `json:"content"`
			} `json:"message"`
		}

		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			// Skip malformed JSON
			fmt.Printf("Anthropic stream unmarshal error: %v, raw: %s\n", err, data)
			continue
		}

		// Handle tool use
		if streamResp.StopReason == "tool_use" {
			// Find the tool use block in the message
			for _, content := range streamResp.Message.Content {
				if content.ToolUse != nil {
					toolCall := &engine.ToolCall{
						ID:   content.ToolUse.ID,
						Name: content.ToolUse.Name,
						Args: content.ToolUse.Input,
					}

					select {
					case <-ctx.Done():
						return
					case ch <- engine.TokenOrToolCall{ToolCall: toolCall}:
						// Successfully sent tool call
						return
					}
				}
			}
		}

		// Handle content deltas
		if streamResp.Type == "content_block_delta" && streamResp.Delta.Type == "text" && streamResp.Delta.Text != "" {
			select {
			case <-ctx.Done():
				return
			case ch <- engine.TokenOrToolCall{Token: streamResp.Delta.Text}:
				// Successfully sent token
				currentContent += streamResp.Delta.Text
			}
		}

		// Handle tool use deltas
		if streamResp.Delta.ToolUse != nil {
			if streamResp.Delta.ToolUse.ID != "" {
				toolCallID = streamResp.Delta.ToolUse.ID
			}
			if streamResp.Delta.ToolUse.Name != "" {
				toolCallName = streamResp.Delta.ToolUse.Name
			}
			if streamResp.Delta.ToolUse.Input != nil {
				toolCallInput = streamResp.Delta.ToolUse.Input
			}

			// If we have all the components of a tool call, send it
			if toolCallID != "" && toolCallName != "" && toolCallInput != nil {
				toolCall := &engine.ToolCall{
					ID:   toolCallID,
					Name: toolCallName,
					Args: toolCallInput,
				}

				select {
				case <-ctx.Done():
					return
				case ch <- engine.TokenOrToolCall{ToolCall: toolCall}:
					// Successfully sent tool call
					return
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("Anthropic stream scanner error: %v\n", err)
	}
}

// handleNonStreamingResponse processes a non-streaming response from the Anthropic API.
func (c *Client) handleNonStreamingResponse(ctx context.Context, body io.Reader, ch chan<- engine.TokenOrToolCall) {
	// Read the entire response
	respBody, err := io.ReadAll(body)
	if err != nil {
		return
	}

	// Parse the response
	var resp struct {
		Type       string `json:"type"`
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			ToolUse *struct {
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Type  string          `json:"type"`
				Input json.RawMessage `json:"input"`
			} `json:"tool_use"`
		} `json:"content"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return
	}

	// Check for tool use
	if resp.StopReason == "tool_use" {
		for _, content := range resp.Content {
			if content.ToolUse != nil {
				toolCall := &engine.ToolCall{
					ID:   content.ToolUse.ID,
					Name: content.ToolUse.Name,
					Args: content.ToolUse.Input,
				}

				select {
				case <-ctx.Done():
					return
				case ch <- engine.TokenOrToolCall{ToolCall: toolCall}:
					// Successfully sent tool call
					return
				}
			}
		}
	}

	// If no tool use, send the text content
	for _, content := range resp.Content {
		if content.Type == "text" && content.Text != "" {
			// Send the content character by character for consistency
			for _, char := range content.Text {
				select {
				case <-ctx.Done():
					return
				case ch <- engine.TokenOrToolCall{Token: string(char)}:
					// Successfully sent token
				}
			}
		}
	}
}

// convertMessages transforms engine messages to Anthropic Claude format.
func convertMessages(messages []engine.Message) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))

	for _, msg := range messages {
		// Skip system messages here; included via top-level system field
		if strings.ToLower(msg.Role) == "system" {
			continue
		}

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
			claudeMsg["content"] = []map[string]interface{}{
				{
					"type": "text",
					"text": msg.Content,
				},
			}
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

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

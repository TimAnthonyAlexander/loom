package openai

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

// Client handles interaction with OpenAI APIs.
type Client struct {
	apiKey     string
	model      string
	endpoint   string
	httpClient *http.Client
}

// New creates a new OpenAI client.
func New(apiKey string, model string) *Client {
	if model == "" {
		model = "o4-mini" // Default model
	}

	return &Client{
		apiKey:   apiKey,
		model:    model,
		endpoint: "https://api.openai.com/v1/chat/completions",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// WithEndpoint sets a custom endpoint for the OpenAI API.
func (c *Client) WithEndpoint(endpoint string) *Client {
	c.endpoint = endpoint
	return c
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
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

		// Make the request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Handle request error
			return
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			// Log or handle non-200 status
			errorResponse, _ := io.ReadAll(resp.Body)
			fmt.Printf("OpenAI API error: %s", errorResponse)
			return
		}

		// Handle streaming response
		if stream {
			c.handleStreamingResponse(ctx, resp.Body, resultCh)
		} else {
			// Handle non-streaming response
			c.handleNonStreamingResponse(ctx, resp.Body, resultCh)
		}
	}()

	return resultCh, nil
}

// handleStreamingResponse processes a streaming response from the OpenAI API.
func (c *Client) handleStreamingResponse(ctx context.Context, body io.Reader, ch chan<- engine.TokenOrToolCall) {
	scanner := bufio.NewScanner(body)

	// Keep track of the current assistant response
	var currentContent string
	var toolCall *engine.ToolCall

	// Process each line in the stream
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Lines from the stream start with "data: "
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		// Extract the data part
		data := line[6:] // Skip "data: "

		// The stream can contain a "[DONE]" message to indicate the end
		if data == "[DONE]" {
			break
		}

		// Parse the JSON delta
		var streamResp struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"` // "function"
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			// Skip malformed JSON
			continue
		}

		// Process the deltas
		if len(streamResp.Choices) > 0 {
			delta := streamResp.Choices[0].Delta

			// Handle text content
			if delta.Content != "" {
				currentContent += delta.Content

				select {
				case <-ctx.Done():
					return
				case ch <- engine.TokenOrToolCall{Token: delta.Content}:
					// Successfully sent token
				}
			}

			// Handle tool calls
			if len(delta.ToolCalls) > 0 {
				tc := delta.ToolCalls[0]

				// If we have a tool call ID, this is the start of a new tool call
				if tc.ID != "" {
					toolCall = &engine.ToolCall{
						ID:   tc.ID,
						Name: tc.Function.Name,
					}
				}

				// If we have arguments and a tool call, append them
				if tc.Function.Arguments != "" && toolCall != nil {
					// Parse the arguments
					var argsMap map[string]interface{}
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &argsMap); err == nil {
						// Convert to raw JSON
						if args, err := json.Marshal(argsMap); err == nil {
							toolCall.Args = args

							// Send the tool call
							select {
							case <-ctx.Done():
								return
							case ch <- engine.TokenOrToolCall{ToolCall: toolCall}:
								// Successfully sent tool call
								// Reset toolCall to nil as we've handled this one
								toolCall = nil
								return // Exit after sending a tool call
							}
						}
					}
				}
			}
		}
	}
}

// handleNonStreamingResponse processes a non-streaming response from the OpenAI API.
func (c *Client) handleNonStreamingResponse(ctx context.Context, body io.Reader, ch chan<- engine.TokenOrToolCall) {
	// Read the entire response
	respBody, err := io.ReadAll(body)
	if err != nil {
		return
	}

	// Parse the response
	var resp struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"` // "function"
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return
	}

	// Process the choices
	if len(resp.Choices) > 0 {
		message := resp.Choices[0].Message

		// Check for tool calls first
		if len(message.ToolCalls) > 0 {
			tc := message.ToolCalls[0]

			// Create a tool call
			toolCall := &engine.ToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
			}

			// Parse the arguments
			var argsMap map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &argsMap); err == nil {
				// Convert to raw JSON
				if args, err := json.Marshal(argsMap); err == nil {
					toolCall.Args = args

					// Send the tool call
					select {
					case <-ctx.Done():
						return
					case ch <- engine.TokenOrToolCall{ToolCall: toolCall}:
						// Successfully sent tool call
						return
					}
				}
			}
		} else if message.Content != "" {
			// If no tool calls, send the content token by token for consistency
			for _, char := range message.Content {
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

// convertMessages transforms engine messages to OpenAI format.
func convertMessages(messages []engine.Message) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))

	for _, msg := range messages {
		// OpenAI API expects specific format for tool responses
		if msg.Role == "tool" {
			openaiMsg := map[string]interface{}{
				"role":         "tool",
				"content":      msg.Content,
				"tool_call_id": msg.ToolID,
				"name":         msg.Name,
			}
			result = append(result, openaiMsg)
		} else if msg.Role == "function" {
			// Legacy function messages - convert to tool format for OpenAI
			openaiMsg := map[string]interface{}{
				"role":         "tool",
				"content":      msg.Content,
				"tool_call_id": msg.ToolID,
				"name":         msg.Name,
			}
			result = append(result, openaiMsg)
		} else {
			// Standard message types (user, assistant, system)
			openaiMsg := map[string]interface{}{
				"role":    msg.Role,
				"content": msg.Content,
			}
			result = append(result, openaiMsg)
		}
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

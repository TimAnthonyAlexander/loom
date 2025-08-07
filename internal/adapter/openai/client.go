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

// isReasoningModel checks if the model is an o-series reasoning model (o3, o3-mini, o4-mini)
func isReasoningModel(model string) bool {
	reasoningModels := []string{"o3", "o3-mini", "o4-mini", "gpt-5"}
	for _, m := range reasoningModels {
		if model == m || strings.HasPrefix(model, m+"-") {
			return true
		}
	}
	return false
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

	// Add debug logging of the raw message history
	fmt.Println("=== DEBUG: Message History ===")
	for i, msg := range messages {
		fmt.Printf("[%d] Role: %s, Name: %s, ToolID: %s, Content: %s\n",
			i, msg.Role, msg.Name, msg.ToolID, truncateString(msg.Content, 50))
	}
	fmt.Println("=== End Message History ===")

	// Create output channel for tokens/tool calls
	resultCh := make(chan engine.TokenOrToolCall)

	// Convert messages and tools to OpenAI format
	openaiMessages := convertMessages(messages)
	openaiTools := convertTools(tools)

	// Add debug logging of the converted OpenAI messages
	fmt.Println("=== DEBUG: OpenAI Messages ===")
	for i, msg := range openaiMessages {
		debugJSON, _ := json.MarshalIndent(msg, "", "  ")
		fmt.Printf("[%d] %s\n", i, string(debugJSON))
	}
	fmt.Println("=== End OpenAI Messages ===")

	// Prepare the request body
	requestBody := map[string]interface{}{
		"model":    c.model,
		"messages": openaiMessages,
		"stream":   stream,
	}

	// Add temperature only for non-reasoning models (not o-series)
	if !isReasoningModel(c.model) {
		requestBody["temperature"] = 0.2
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
			// Surface the error to the engine via a token so the UI shows something
			select {
			case <-ctx.Done():
				return
			case resultCh <- engine.TokenOrToolCall{Token: fmt.Sprintf("OpenAI marshal error: %v", err)}:
			}
			return
		}

		// Create request
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(reqBody))
		if err != nil {
			// Surface the error to the engine via a token so the UI shows something
			select {
			case <-ctx.Done():
				return
			case resultCh <- engine.TokenOrToolCall{Token: fmt.Sprintf("OpenAI request error: %v", err)}:
			}
			return
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

		// Make the request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Surface the error to the engine via a token so the UI shows something
			select {
			case <-ctx.Done():
				return
			case resultCh <- engine.TokenOrToolCall{Token: fmt.Sprintf("OpenAI HTTP error: %v", err)}:
			}
			return
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			// Read and surface non-200 status to the engine so the UI can display it
			errorResponse, _ := io.ReadAll(resp.Body)
			msg := fmt.Sprintf("OpenAI API error (%d): %s", resp.StatusCode, string(errorResponse))
			select {
			case <-ctx.Done():
				return
			case resultCh <- engine.TokenOrToolCall{Token: msg}:
			}
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

// preprocessMessagesForOpenAI rebuilds the conversation to ensure it has the correct structure for OpenAI API
// specifically, it makes sure each tool message is preceded by an assistant message with tool_calls
// Removed preprocessMessagesForOpenAI; we will rely on explicit assistant tool_use messages recorded by the engine

// truncateString shortens a string for logging purposes
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}

// handleStreamingResponse processes a streaming response from the OpenAI API.
func (c *Client) handleStreamingResponse(ctx context.Context, body io.Reader, ch chan<- engine.TokenOrToolCall) {
	scanner := bufio.NewScanner(body)
	// Increase the scanner buffer to safely handle larger SSE chunks
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	// Keep track of the current assistant response
	var currentContent string

	// Accumulate tool call deltas until finish_reason == "tool_calls"
	type partialCall struct {
		id    string
		name  string
		args  string
		index int
		order int
	}
	partials := make(map[string]*partialCall)
	var orderCounter int
	sawToolCalls := false

	// Process each line in the stream
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := line[6:]
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
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue
		}

		if len(streamResp.Choices) == 0 {
			continue
		}
		delta := streamResp.Choices[0].Delta
		finish := streamResp.Choices[0].FinishReason

		// Stream text tokens as they arrive
		if delta.Content != "" {
			currentContent += delta.Content
			select {
			case <-ctx.Done():
				return
			case ch <- engine.TokenOrToolCall{Token: delta.Content}:
			}
		}

		// Accumulate any tool_calls deltas
		if len(delta.ToolCalls) > 0 {
			sawToolCalls = true
			for _, tc := range delta.ToolCalls {
				// Prefer ID for accumulation; fall back to synthetic key using index
				id := tc.ID
				if id == "" {
					id = fmt.Sprintf("idx_%d", tc.Index)
				}
				p, ok := partials[id]
				if !ok {
					p = &partialCall{id: id, name: tc.Function.Name, index: tc.Index, order: orderCounter}
					orderCounter++
					partials[id] = p
				}
				if tc.Index >= 0 && p.index < 0 {
					p.index = tc.Index
				}
				if tc.Function.Name != "" {
					p.name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					p.args += tc.Function.Arguments
				}
			}
		}

		// Only emit tool call(s) after model signals completion of tool_calls
		if finish == "tool_calls" && sawToolCalls {
			// Choose the first tool call deterministically (by lowest index, then order)
			var chosen *partialCall
			for _, pc := range partials {
				if chosen == nil {
					chosen = pc
					continue
				}
				// Prefer lower index when available; otherwise fallback to insertion order
				if pc.index >= 0 && chosen.index >= 0 {
					if pc.index < chosen.index {
						chosen = pc
					}
				} else if pc.index >= 0 && chosen.index < 0 {
					chosen = pc
				} else if pc.index < 0 && chosen.index < 0 && pc.order < chosen.order {
					chosen = pc
				}
			}

			if chosen != nil {
				// Try to parse accumulated args once at finish
				var argsMap map[string]interface{}
				if err := json.Unmarshal([]byte(chosen.args), &argsMap); err == nil {
					if args, err := json.Marshal(argsMap); err == nil {
						call := &engine.ToolCall{ID: chosen.id, Name: chosen.name, Args: args}
						select {
						case <-ctx.Done():
							return
						case ch <- engine.TokenOrToolCall{ToolCall: call}:
							return
						}
					}
				}
				// If parsing failed, emit the raw args string as-is to surface the issue
				call := &engine.ToolCall{ID: chosen.id, Name: chosen.name, Args: json.RawMessage([]byte(chosen.args))}
				select {
				case <-ctx.Done():
					return
				case ch <- engine.TokenOrToolCall{ToolCall: call}:
					return
				}
			}
		}
	}

	// If the scanner ends without explicit finish_reason but we have a parsable tool call, try to emit it
	if err := scanner.Err(); err == nil && sawToolCalls && len(partials) > 0 {
		var chosen *partialCall
		for _, pc := range partials {
			if chosen == nil || pc.order < chosen.order {
				chosen = pc
			}
		}
		if chosen != nil {
			var argsMap map[string]interface{}
			if err := json.Unmarshal([]byte(chosen.args), &argsMap); err == nil {
				if args, err := json.Marshal(argsMap); err == nil {
					call := &engine.ToolCall{ID: chosen.id, Name: chosen.name, Args: args}
					select {
					case <-ctx.Done():
						return
					case ch <- engine.TokenOrToolCall{ToolCall: call}:
						return
					}
				}
			}
			call := &engine.ToolCall{ID: chosen.id, Name: chosen.name, Args: json.RawMessage([]byte(chosen.args))}
			select {
			case <-ctx.Done():
				return
			case ch <- engine.TokenOrToolCall{ToolCall: call}:
				return
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
		switch msg.Role {
		case "system", "user":
			result = append(result, map[string]interface{}{
				"role":    msg.Role,
				"content": msg.Content,
			})
		case "assistant":
			if msg.Name != "" && msg.ToolID != "" {
				arguments := msg.Content
				if arguments == "" {
					arguments = "{}"
				}
				result = append(result, map[string]interface{}{
					"role": "assistant",
					"tool_calls": []map[string]interface{}{
						{
							"id":   msg.ToolID,
							"type": "function",
							"function": map[string]interface{}{
								"name":      msg.Name,
								"arguments": arguments,
							},
						},
					},
				})
			} else {
				result = append(result, map[string]interface{}{
					"role":    "assistant",
					"content": msg.Content,
				})
			}
		case "tool", "function":
			openaiMsg := map[string]interface{}{
				"role":         "tool",
				"content":      msg.Content,
				"tool_call_id": msg.ToolID,
			}
			if msg.Name != "" {
				openaiMsg["name"] = msg.Name
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

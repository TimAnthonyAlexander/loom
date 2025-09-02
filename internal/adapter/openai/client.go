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
		// Default to a Chat Completions-capable model to ensure streaming works
		model = "o4-mini"
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

// isReasoningModel returns true for OpenAI reasoning models that don't support temperature.
// Treat all o3*, o4*, and gpt-5* variants as reasoning models.
func isReasoningModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "o3" || strings.HasPrefix(m, "o3-") {
		return true
	}
	if m == "o4" || strings.HasPrefix(m, "o4-") {
		return true
	}
	if m == "gpt-5" || strings.HasPrefix(m, "gpt-5-") {
		return true
	}
	return false
}

// validateToolArgs applies minimal, tool-specific validation for required fields
// to avoid prematurely emitting incomplete tool calls during streaming.
func validateToolArgs(toolName string, args map[string]interface{}) bool {
	switch toolName {
	case "read_file":
		// require path
		if v, ok := args["path"]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return true
			}
		}
		return false
	case "search_code":
		if v, ok := args["query"]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return true
			}
		}
		return false
	case "edit_file", "apply_edit":
		if v, ok := args["path"]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return true
			}
		}
		return false
	default:
		// For other tools, accept any JSON (including empty) by default
		return true
	}
}

// isEmptyResponse checks if a response is effectively empty (only whitespace).
func isEmptyResponse(content string) bool {
	return strings.TrimSpace(content) == ""
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

	// Removed verbose debug logging of raw message history

	// Create output channel for tokens/tool calls
	resultCh := make(chan engine.TokenOrToolCall)

	// Start a goroutine to handle the response with retry logic
	go func() {
		defer close(resultCh)
		c.chatWithRetry(ctx, messages, tools, stream, resultCh)
	}()

	return resultCh, nil
}

// chatWithRetry implements retry logic for empty responses
func (c *Client) chatWithRetry(ctx context.Context, messages []engine.Message, tools []engine.ToolSchema, stream bool, resultCh chan<- engine.TokenOrToolCall) {
	// Try first attempt and track if we receive any content
	contentReceived, toolCallReceived := c.attemptChat(ctx, messages, tools, stream, resultCh)

	// If we got empty response, retry with opposite streaming mode
	if !contentReceived && !toolCallReceived {
		select {
		case <-ctx.Done():
			return
		case resultCh <- engine.TokenOrToolCall{Token: "Retrying due to empty response..."}:
		}
		c.attemptChat(ctx, messages, tools, !stream, resultCh)
	}
}

// attemptChat performs a single chat attempt and returns whether content/toolcalls were received
func (c *Client) attemptChat(ctx context.Context, messages []engine.Message, tools []engine.ToolSchema, stream bool, resultCh chan<- engine.TokenOrToolCall) (contentReceived, toolCallReceived bool) {
	// Convert messages and tools to OpenAI format
	openaiMessages := convertMessages(messages)
	openaiTools := convertTools(tools)

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
		// Only include parallel_tool_calls for non-reasoning models.
		if !isReasoningModel(c.model) {
			requestBody["parallel_tool_calls"] = false
		}
	}

	// Prepare request body
	reqBody, err := json.Marshal(requestBody)
	if err != nil {
		// Surface the error to the engine via a token so the UI shows something
		select {
		case <-ctx.Done():
			return false, false
		case resultCh <- engine.TokenOrToolCall{Token: fmt.Sprintf("OpenAI marshal error: %v", err)}:
		}
		return false, false
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		// Surface the error to the engine via a token so the UI shows something
		select {
		case <-ctx.Done():
			return false, false
		case resultCh <- engine.TokenOrToolCall{Token: fmt.Sprintf("OpenAI request error: %v", err)}:
		}
		return false, false
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
			return false, false
		case resultCh <- engine.TokenOrToolCall{Token: fmt.Sprintf("OpenAI HTTP error: %v", err)}:
		}
		return false, false
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Read and surface non-200 status to the engine so the UI can display it
		errorResponse, _ := io.ReadAll(resp.Body)
		msg := fmt.Sprintf("OpenAI API error (%d): %s", resp.StatusCode, string(errorResponse))
		select {
		case <-ctx.Done():
			return false, false
		case resultCh <- engine.TokenOrToolCall{Token: msg}:
		}
		return false, false
	}

	// Handle streaming or non-streaming response with tracking
	if stream {
		// Create a cancellable context that we can cancel after emitting a tool call
		sseCtx, sseCancel := context.WithCancel(ctx)
		defer sseCancel()
		return c.handleStreamingResponseWithTracking(sseCtx, resp.Body, resultCh)
	} else {
		// Handle non-streaming response
		return c.handleNonStreamingResponseWithTracking(ctx, resp.Body, resultCh)
	}
}

// preprocessMessagesForOpenAI rebuilds the conversation to ensure it has the correct structure for OpenAI API
// specifically, it makes sure each tool message is preceded by an assistant message with tool_calls
// Removed preprocessMessagesForOpenAI; we will rely on explicit assistant tool_use messages recorded by the engine

// handleStreamingResponseWithTracking processes a streaming response and tracks content/tool calls
func (c *Client) handleStreamingResponseWithTracking(ctx context.Context, body io.Reader, ch chan<- engine.TokenOrToolCall) (contentReceived, toolCallReceived bool) {
	scanner := bufio.NewScanner(body)
	// Increase the scanner buffer to safely handle larger SSE chunks
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	// Keep track of the current assistant response
	var currentContent string

	// Accumulate tool call deltas until finish_reason == "tool_calls"
	type partialCall struct {
		id   string
		name string
		args string
	}
	// Key by index to avoid splitting state when an ID appears mid-stream
	partials := make(map[int]*partialCall)
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
		if delta.Content != "" && !isEmptyResponse(delta.Content) {
			currentContent += delta.Content
			contentReceived = true
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
				idx := tc.Index
				p, ok := partials[idx]
				if !ok {
					p = &partialCall{}
					partials[idx] = p
				}
				if tc.ID != "" {
					p.id = tc.ID
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
			// Choose the lowest index tool call (parallel tool calls disabled)
			var chosen *partialCall
			var chosenIdx int
			for idx, pc := range partials {
				if chosen == nil || idx < chosenIdx {
					chosen = pc
					chosenIdx = idx
				}
			}

			if chosen != nil {
				// If we never received a function name, do not emit a tool call.
				// Let the engine fall back to non-streaming parsing which typically includes full tool metadata.
				if strings.TrimSpace(chosen.name) == "" {
					continue
				}
				// Gather and validate arguments. If missing or invalid, do not emit; let engine retry non-streaming.
				argsStr := strings.TrimSpace(chosen.args)
				if argsStr == "" {
					return
				}

				var argsMap map[string]interface{}
				if err := json.Unmarshal([]byte(argsStr), &argsMap); err != nil {
					return
				}
				// Validate required args for known tools
				if !validateToolArgs(chosen.name, argsMap) {
					return
				}
				if args, err := json.Marshal(argsMap); err == nil {
					toolCallReceived = true
					id := chosen.id
					if id == "" {
						id = fmt.Sprintf("idx_%d", chosenIdx)
					}
					call := &engine.ToolCall{ID: id, Name: chosen.name, Args: args}
					select {
					case <-ctx.Done():
						return
					case ch <- engine.TokenOrToolCall{ToolCall: call}:
						return
					}
				}
			}
		}
	}

	// If the scanner ends without explicit finish_reason but we have a parsable tool call, try to emit it
	if err := scanner.Err(); err == nil && sawToolCalls && len(partials) > 0 {
		var chosen *partialCall
		var chosenIdx int
		for idx, pc := range partials {
			if chosen == nil || idx < chosenIdx {
				chosen = pc
				chosenIdx = idx
			}
		}
		if chosen != nil {
			// If function name is still missing at end of stream, do not emit a tool call.
			// This will signal the engine to perform a non-streaming retry.
			if strings.TrimSpace(chosen.name) == "" {
				return
			}
			argsStr := strings.TrimSpace(chosen.args)
			if argsStr == "" {
				return
			}
			var argsMap map[string]interface{}
			if err := json.Unmarshal([]byte(argsStr), &argsMap); err != nil {
				return
			}
			if !validateToolArgs(chosen.name, argsMap) {
				return
			}
			if args, err := json.Marshal(argsMap); err == nil {
				toolCallReceived = true
				id := chosen.id
				if id == "" {
					id = fmt.Sprintf("idx_%d", chosenIdx)
				}
				call := &engine.ToolCall{ID: id, Name: chosen.name, Args: args}
				select {
				case <-ctx.Done():
					return
				case ch <- engine.TokenOrToolCall{ToolCall: call}:
					return
				}
			}
		}
	}

	return
}

// handleNonStreamingResponseWithTracking processes a non-streaming response and tracks content/tool calls
func (c *Client) handleNonStreamingResponseWithTracking(ctx context.Context, body io.Reader, ch chan<- engine.TokenOrToolCall) (contentReceived, toolCallReceived bool) {
	// Read the entire response
	respBody, err := io.ReadAll(body)
	if err != nil {
		return false, false
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
		return false, false
	}

	// Process the choices
	if len(resp.Choices) > 0 {
		message := resp.Choices[0].Message

		// Check for tool calls first
		if len(message.ToolCalls) > 0 {
			tc := message.ToolCalls[0]

			// If the function name is missing, do not emit a tool call.
			// Prefer emitting content if present instead.
			if strings.TrimSpace(tc.Function.Name) == "" {
				if message.Content != "" && !isEmptyResponse(message.Content) {
					contentReceived = true
					for _, char := range message.Content {
						select {
						case <-ctx.Done():
							return
						case ch <- engine.TokenOrToolCall{Token: string(char)}:
						}
					}
				}
				return
			}

			// Create a tool call
			toolCall := &engine.ToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
			}

			// Parse the arguments (default empty to {})
			argsStr := strings.TrimSpace(tc.Function.Arguments)
			if argsStr == "" {
				argsStr = "{}"
			}
			var argsMap map[string]interface{}
			if err := json.Unmarshal([]byte(argsStr), &argsMap); err == nil {
				// Convert to raw JSON
				if args, err := json.Marshal(argsMap); err == nil {
					toolCall.Args = args
					toolCallReceived = true

					// Send the tool call
					select {
					case <-ctx.Done():
						return
					case ch <- engine.TokenOrToolCall{ToolCall: toolCall}:
						return
					}
				}
			}
			// If JSON still invalid, pass through raw string
			toolCall.Args = json.RawMessage([]byte(argsStr))
			toolCallReceived = true
			select {
			case <-ctx.Done():
				return
			case ch <- engine.TokenOrToolCall{ToolCall: toolCall}:
				return
			}
		} else if message.Content != "" && !isEmptyResponse(message.Content) {
			contentReceived = true
			// If no tool calls, send the content token by token for consistency
			for _, char := range message.Content {
				select {
				case <-ctx.Done():
					return
				case ch <- engine.TokenOrToolCall{Token: string(char)}:
				}
			}
		}
	}

	return
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

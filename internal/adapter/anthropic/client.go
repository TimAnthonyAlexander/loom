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

	// Add debug logging of the raw message history (parity with OpenAI adapter)
	fmt.Println("=== DEBUG: Message History ===")
	for i, msg := range messages {
		fmt.Printf("[%d] Role: %s, Name: %s, ToolID: %s, Content: %s\n",
			i, msg.Role, msg.Name, msg.ToolID, truncateString(msg.Content, 50))
	}
	fmt.Println("=== End Message History ===")

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
	// Anthropic requires the last message to be from the user. If not, append
	// a minimal nudge from user to prompt a continuation/tool call.
	if len(claudeMessages) == 0 {
		claudeMessages = append(claudeMessages, map[string]interface{}{
			"role":    "user",
			"content": []map[string]interface{}{{"type": "text", "text": "Either continue with your task at hand or use finalize tool."}},
		})
	} else {
		if role, _ := claudeMessages[len(claudeMessages)-1]["role"].(string); role != "user" {
			claudeMessages = append(claudeMessages, map[string]interface{}{
				"role":    "user",
				"content": []map[string]interface{}{{"type": "text", "text": "Either continue with your task at hand or use finalize tool."}},
			})
		}
	}
	claudeTools := convertTools(tools)

	// Add debug logging of the converted Anthropic messages (parity with OpenAI adapter)
	fmt.Println("=== DEBUG: Anthropic Messages ===")
	for i, msg := range claudeMessages {
		debugJSON, _ := json.MarshalIndent(msg, "", "  ")
		fmt.Printf("[%d] %s\n", i, string(debugJSON))
	}
	fmt.Println("=== End Anthropic Messages ===")

	// Remove provider prefix if present (e.g., "claude:" prefix)
	modelID := strings.TrimPrefix(c.model, "claude:")

	// Prepare the request body
	// Anthropic expects model IDs like "claude-opus-4-20250514" without provider prefix
	// Ensure max_tokens is compatible with thinking budget when streaming
	maxTokens := c.maxTokens
	if stream && maxTokens < 2 {
		maxTokens = 2
	}
	requestBody := map[string]interface{}{
		"model":      modelID,
		"messages":   claudeMessages,
		"max_tokens": maxTokens, // Required parameter for Anthropic API
		// Honor stream flag so we can deliver incremental messages & thinking
		"stream": stream,
	}
	// Enable extended thinking when streaming so UI can show reasoning.
	// Budget kept conservative; configurable later if needed.
	if stream {
		// Constrain thinking budget to always be < max_tokens and within a reasonable cap
		budget := maxTokens - 1
		if budget > 1024 {
			budget = 1024
		}
		if budget < 1 {
			budget = 1
		}
		requestBody["thinking"] = map[string]interface{}{
			"type":          "enabled",
			"budget_tokens": budget,
		}
		// Anthropic requires temperature to be 1 when thinking is enabled
		requestBody["temperature"] = 1
	} else {
		requestBody["temperature"] = 0.2
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
		fmt.Printf("Anthropic: POST %s | model=%s | stream=%v | messages=%d | tools=%d\n", c.endpoint, modelID, stream, len(claudeMessages), len(claudeTools))
		// Make the request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Surface request error via token
			fmt.Printf("Anthropic HTTP error: %v\n", err)
			select {
			case <-ctx.Done():
				return
			case resultCh <- engine.TokenOrToolCall{Token: fmt.Sprintf("Anthropic HTTP error: %v", err)}:
			}
			return
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			// Log and surface non-200 status
			errorResponse, _ := io.ReadAll(resp.Body)
			msg := fmt.Sprintf("Anthropic API error (%d): %s", resp.StatusCode, string(errorResponse))
			fmt.Println(msg)
			fmt.Printf("Debug: Request sent to: %s with model: %s, max_tokens: %d\n",
				c.endpoint, c.model, c.maxTokens)
			select {
			case <-ctx.Done():
				return
			case resultCh <- engine.TokenOrToolCall{Token: msg}:
			}
			return
		}

		fmt.Printf("Anthropic: status=%d content-type=%s\n", resp.StatusCode, resp.Header.Get("Content-Type"))
		// Handle response
		if stream {
			c.handleStreamingResponse(ctx, resp.Body, resultCh)
		} else {
			c.handleNonStreamingResponse(ctx, resp.Body, resultCh)
		}
	}()

	return resultCh, nil
}

// truncateString shortens a string for logging purposes
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}

// handleStreamingResponse processes a streaming response from the Anthropic API.
func (c *Client) handleStreamingResponse(ctx context.Context, body io.Reader, ch chan<- engine.TokenOrToolCall) {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	type blockState struct {
		BlockType   string
		ToolID      string
		ToolName    string
		InputJSON   string
		ThinkingBuf string
	}
	blocks := make(map[int]*blockState)
	currentEvent := ""

	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimSpace(line[7:])
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimSpace(line[6:])
		if payload == "" || payload == "[DONE]" {
			continue
		}

		// Define a loose struct to capture common fields
		var ev struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			// content_block_start
			ContentBlock *struct {
				Type  string `json:"type"`
				ID    string `json:"id,omitempty"`
				Name  string `json:"name,omitempty"`
				Input any    `json:"input,omitempty"`
			} `json:"content_block,omitempty"`
			// content_block_delta
			Delta *struct {
				Type        string `json:"type"`
				Text        string `json:"text,omitempty"`
				PartialJSON string `json:"partial_json,omitempty"`
				Thinking    string `json:"thinking,omitempty"`
				Signature   string `json:"signature,omitempty"`
			} `json:"delta,omitempty"`
			// message_delta
			MessageDelta *struct {
				StopReason string `json:"stop_reason,omitempty"`
			} `json:"message_delta,omitempty"`
			// error
			Error *struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}

		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			fmt.Printf("Anthropic SSE unmarshal error: %v, raw: %s\n", err, payload)
			continue
		}

		switch currentEvent {
		case "message_start":
			// No action
		case "content_block_start":
			bs := &blockState{}
			if ev.ContentBlock != nil {
				bs.BlockType = ev.ContentBlock.Type
				if ev.ContentBlock.Type == "tool_use" {
					bs.ToolID = ev.ContentBlock.ID
					bs.ToolName = ev.ContentBlock.Name
				}
			}
			blocks[ev.Index] = bs
		case "content_block_delta":
			bs := blocks[ev.Index]
			if bs == nil || ev.Delta == nil {
				continue
			}
			switch ev.Delta.Type {
			case "text_delta":
				if ev.Delta.Text != "" {
					select {
					case <-ctx.Done():
						return
					case ch <- engine.TokenOrToolCall{Token: ev.Delta.Text}:
					}
				}
			case "input_json_delta":
				bs.InputJSON += ev.Delta.PartialJSON
			case "thinking_delta":
				if ev.Delta.Thinking != "" {
					bs.ThinkingBuf += ev.Delta.Thinking
					select {
					case <-ctx.Done():
						return
					case ch <- engine.TokenOrToolCall{Token: "[REASONING] " + ev.Delta.Thinking}:
					}
				}
			case "signature_delta":
				// Ignore for now
			}
		case "content_block_stop":
			bs := blocks[ev.Index]
			if bs == nil {
				continue
			}
			// If this was a tool use, emit a tool call now
			if bs.BlockType == "tool_use" {
				argsStr := strings.TrimSpace(bs.InputJSON)
				if argsStr == "" {
					argsStr = "{}"
				}
				var raw json.RawMessage
				// Ensure valid JSON; if invalid, wrap as empty object
				if json.Valid([]byte(argsStr)) {
					raw = json.RawMessage(argsStr)
				} else {
					raw = json.RawMessage("{}")
				}
				tc := &engine.ToolCall{ID: bs.ToolID, Name: bs.ToolName, Args: raw}
				select {
				case <-ctx.Done():
					return
				case ch <- engine.TokenOrToolCall{ToolCall: tc}:
					return
				}
			}
			if bs.BlockType == "thinking" {
				// Signal reasoning done so UI can collapse; no extra text needed
				select {
				case <-ctx.Done():
					return
				case ch <- engine.TokenOrToolCall{Token: "[REASONING_DONE] "}:
				}
			}
			delete(blocks, ev.Index)
		case "message_delta":
			// We could look at stop_reason here, but content_block_stop handles tool_use
			_ = ev.MessageDelta
		case "message_stop":
			// End of assistant turn
			return
		case "error":
			if ev.Error != nil {
				select {
				case <-ctx.Done():
					return
				case ch <- engine.TokenOrToolCall{Token: fmt.Sprintf("Anthropic error: %s", ev.Error.Message)}:
					return
				}
			}
		default:
			// Unknown/ignored events: ping, etc.
		}
	}
	if err := sc.Err(); err != nil {
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
	// Debug: dump raw Anthropic response
	fmt.Printf("=== DEBUG: Anthropic Raw Response ===\n%s\n=== End Anthropic Raw Response ===\n", string(respBody))

	// Parse the response per Anthropic schema
	var resp struct {
		Type       string `json:"type"`
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return
	}

	// Prefer tool_use if present, regardless of stop_reason
	for _, content := range resp.Content {
		if strings.EqualFold(content.Type, "tool_use") {
			toolCall := &engine.ToolCall{
				ID:   content.ID,
				Name: content.Name,
				Args: content.Input,
			}
			select {
			case <-ctx.Done():
				return
			case ch <- engine.TokenOrToolCall{ToolCall: toolCall}:
				return
			}
		}
	}

	// If no tool use, send the text content
	for _, content := range resp.Content {
		if content.Type == "text" && content.Text != "" {
			select {
			case <-ctx.Done():
				return
			case ch <- engine.TokenOrToolCall{Token: content.Text}:
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

		claudeMsg := map[string]interface{}{}

		// Special handling: assistant tool_use messages recorded by the engine
		if msg.Role == "assistant" && msg.Name != "" && msg.ToolID != "" {
			claudeMsg["role"] = "assistant"
			// Parse input JSON if possible; default to empty object
			var input any
			if strings.TrimSpace(msg.Content) == "" {
				input = map[string]any{}
			} else {
				if err := json.Unmarshal([]byte(msg.Content), &input); err != nil {
					input = map[string]any{}
				}
			}
			claudeMsg["content"] = []map[string]interface{}{
				{
					"type":  "tool_use",
					"id":    msg.ToolID,
					"name":  msg.Name,
					"input": input,
				},
			}
			result = append(result, claudeMsg)
			continue
		}

		// Default role mapping after handling tool_use above
		claudeMsg["role"] = convertRole(msg.Role)

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
		// Anthropic expects tool results to be sent from the user
		return "user"
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
		var properties interface{} = tool.Schema["properties"]
		var required interface{} = tool.Schema["required"]
		claudeTool := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"input_schema": map[string]interface{}{
				"type":                 "object",
				"properties":           properties,
				"required":             required,
				"additionalProperties": false,
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

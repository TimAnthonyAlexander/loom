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

// lastAssistantTurnHasToolUseWithoutThinking inspects the message history to
// determine if the most recent assistant turn includes tool_use but lacks any
// preserved thinking block content in that turn. This helps decide whether to
// disable thinking to satisfy Anthropic's requirement.
func lastAssistantTurnHasToolUseWithoutThinking(messages []engine.Message) bool {
	// Scan from the end to find the last assistant turn boundary.
	foundAssistant := false
	hasToolUse := false
	hasThinkingWithSignature := false
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if strings.ToLower(m.Role) == "user" || strings.ToLower(m.Role) == "system" || strings.ToLower(m.Role) == "tool" || strings.ToLower(m.Role) == "function" {
			if foundAssistant {
				break
			}
			continue
		}
		if strings.ToLower(m.Role) == "assistant" {
			foundAssistant = true
			// thinking blocks are recorded as assistant with Name=="thinking"
			if m.Name == "thinking" && strings.TrimSpace(m.Content) != "" {
				var payload struct {
					Thinking  string `json:"thinking"`
					Signature string `json:"signature"`
				}
				if json.Unmarshal([]byte(m.Content), &payload) == nil && payload.Thinking != "" && payload.Signature != "" {
					hasThinkingWithSignature = true
				}
			}
			// tool_use messages are recorded as assistant with Name set and ToolID present
			if m.Name != "" && m.ToolID != "" {
				hasToolUse = true
			}
		}
	}
	return foundAssistant && hasToolUse && !hasThinkingWithSignature
}

// lastAssistantTurnHasThinking returns true if the most recent assistant turn
// contains a preserved thinking block (with a valid signature).
func lastAssistantTurnHasThinking(messages []engine.Message) bool {
	foundAssistant := false
	hasThinkingWithSignature := false
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		role := strings.ToLower(m.Role)
		// These roles delimit turns; once we've seen assistant and hit another role, stop.
		if role == "user" || role == "system" || role == "tool" || role == "function" {
			if foundAssistant {
				break
			}
			continue
		}
		if role == "assistant" {
			foundAssistant = true
			if m.Name == "thinking" && strings.TrimSpace(m.Content) != "" {
				var payload struct {
					Thinking  string `json:"thinking"`
					Signature string `json:"signature"`
				}
				if json.Unmarshal([]byte(m.Content), &payload) == nil && payload.Thinking != "" && payload.Signature != "" {
					hasThinkingWithSignature = true
				}
			}
		}
	}
	return hasThinkingWithSignature
}

// Chat implements the engine.LLM interface for Anthropic Claude.
func (c *Client) Chat(
	ctx context.Context,
	messages []engine.Message,
	tools []engine.ToolSchema,
	stream bool,
) (<-chan engine.TokenOrToolCall, error) {
	if c.apiKey == "" {
		return nil, errors.New("anthropic API key not set")
	}

	// Removed verbose debug logging of raw message history

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

	// Decide thinking mode BEFORE converting messages so we can include/exclude
	// thinking blocks consistently with the request.
	modelID := strings.TrimPrefix(c.model, "claude:")
	modelSupportsThinking := supportsThinkingForModel(modelID)
	mustReplayThinking := lastAssistantTurnHasThinking(messages)
	enableThinking := modelSupportsThinking && (stream || mustReplayThinking)

	// If the last assistant turn in history includes tool_use without a preceding
	// preserved thinking block, Anthropic requires either: (a) disable thinking or
	// (b) include the previous thinking unmodified. We choose (a) automatically
	// to avoid 400s; the engine will still show tokens normally.
	if enableThinking && lastAssistantTurnHasToolUseWithoutThinking(messages) {
		enableThinking = false
	}

	// Convert messages and tools to Claude format (excluding system messages).
	// When thinking is disabled, we must NOT include any thinking/redacted_thinking
	// blocks in the transcript to avoid API 400 errors.
	claudeMessages := convertMessages(messages, enableThinking)
	// Anthropic requires the last message to be from the user. If not, append
	// a minimal nudge from user to prompt a continuation/tool call.
	if len(claudeMessages) == 0 {
		claudeMessages = append(claudeMessages, map[string]interface{}{
			"role":    "user",
			"content": []map[string]interface{}{{"type": "text", "text": "Either continue with your task at hand or write finalizing message."}},
		})
	} else {
		if role, _ := claudeMessages[len(claudeMessages)-1]["role"].(string); role != "user" {
			claudeMessages = append(claudeMessages, map[string]interface{}{
				"role":    "user",
				"content": []map[string]interface{}{{"type": "text", "text": "Either continue with your task at hand or write finalizing message."}},
			})
		}
	}
	claudeTools := convertTools(tools)

	// Removed verbose debug logging of converted Anthropic messages

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
	// enableThinking already decided above

	if enableThinking {
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

	// Removed debug log for model selection

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
			// Opt-in to Anthropic interleaved thinking beta (no-op on unsupported models)
			if enableThinking {
				req.Header.Set("interleaved-thinking-2025-05-14", "true")
			}
		}
		// Removed debug log for API key
		// Anthropic requires 'x-api-key' header, not 'Authorization'
		req.Header.Set("x-api-key", c.apiKey)

		// API version is required for Anthropic
		req.Header.Set("anthropic-version", c.apiVersion)

		// Removed verbose request logging
		// Make the request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Surface request error via token
			select {
			case <-ctx.Done():
				return
			case resultCh <- engine.TokenOrToolCall{Token: fmt.Sprintf("Anthropic HTTP error: %v", err)}:
			}
			return
		}
		defer func() { _ = resp.Body.Close() }()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			// Surface non-200 status
			errorResponse, _ := io.ReadAll(resp.Body)
			msg := fmt.Sprintf("Anthropic API error (%d): %s", resp.StatusCode, string(errorResponse))
			select {
			case <-ctx.Done():
				return
			case resultCh <- engine.TokenOrToolCall{Token: msg}:
			}
			return
		}

		// Removed verbose response status logging
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
// Removed unused truncateString helper (debug only)

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
		Signature   string
	}
	blocks := make(map[int]*blockState)
	currentEvent := ""
	// Track usage for billing: input once at message_start, output cumulative on message_delta
	var inputTokens int64
	var outputTokens int64
	// Normalize model id (strip provider prefix if present)
	normalizedModel := strings.TrimPrefix(c.model, "claude:")

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
			// Skip malformed SSE chunk silently
			continue
		}

		switch currentEvent {
		case "message_start":
			// Capture input tokens from usage if present
			var v struct {
				Type    string `json:"type"`
				Message struct {
					Usage *struct {
						InputTokens int64 `json:"input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal([]byte(payload), &v); err == nil && v.Message.Usage != nil {
				inputTokens = v.Message.Usage.InputTokens
			}
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
				if ev.Delta.Signature != "" {
					bs.Signature = ev.Delta.Signature
					select {
					case <-ctx.Done():
						return
					case ch <- engine.TokenOrToolCall{Token: "[REASONING_SIGNATURE] " + ev.Delta.Signature}:
					}
				}
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
					// Do not return immediately; allow following blocks like message_stop
					// to be consumed, but we will break out by returning at message_stop.
				}
			}
			if bs.BlockType == "thinking" {
				// Emit final JSON payload with thinking + signature so the engine can persist
				finalPayload := map[string]string{
					"thinking":  bs.ThinkingBuf,
					"signature": bs.Signature,
				}
				b, _ := json.Marshal(finalPayload)
				select {
				case <-ctx.Done():
					return
				case ch <- engine.TokenOrToolCall{Token: "[REASONING_JSON] " + string(b)}:
				}
				// Signal reasoning done so UI can collapse; no extra text needed
				select {
				case <-ctx.Done():
					return
				case ch <- engine.TokenOrToolCall{Token: "[REASONING_DONE] "}:
				}
			}
			delete(blocks, ev.Index)
		case "message_delta":
			// Capture cumulative output tokens if present
			var d struct {
				Type  string `json:"type"`
				Usage *struct {
					OutputTokens int64 `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(payload), &d); err == nil && d.Usage != nil {
				outputTokens = d.Usage.OutputTokens
			}
		case "message_stop":
			// End of assistant turn — emit usage marker for engine to compute costs/UI
			usage := fmt.Sprintf("[USAGE] provider=anthropic model=%s in=%d out=%d", normalizedModel, inputTokens, outputTokens)
			select {
			case <-ctx.Done():
				return
			case ch <- engine.TokenOrToolCall{Token: usage}:
			}
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
	_ = sc.Err()
}

// handleNonStreamingResponse processes a non-streaming response from the Anthropic API.
func (c *Client) handleNonStreamingResponse(ctx context.Context, body io.Reader, ch chan<- engine.TokenOrToolCall) {
	// Read the entire response
	respBody, err := io.ReadAll(body)
	if err != nil {
		return
	}
	// Removed verbose dump of raw Anthropic response

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
func convertMessages(messages []engine.Message, includeThinking bool) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))

	// We coalesce consecutive assistant messages into a single assistant message
	// with an ordered list of content blocks. If both thinking and tool_use
	// happened in the same turn, we ensure that the first content block is a
	// thinking or redacted_thinking block to satisfy Anthropic's rule.
	flushAssistant := func(pending *[]map[string]interface{}) {
		if len(*pending) == 0 {
			return
		}
		result = append(result, map[string]interface{}{
			"role":    "assistant",
			"content": *pending,
		})
		*pending = nil
	}

	var pendingAssistant []map[string]interface{}

	for _, msg := range messages {
		// Skip system messages here; included via top-level system field
		if strings.ToLower(msg.Role) == "system" {
			continue
		}

		switch msg.Role {
		case "assistant":
			// Build appropriate content item
			if msg.Name == "thinking" {
				// Only include thinking blocks that have a signature as required by Anthropic.
				var payload struct {
					Thinking  string `json:"thinking"`
					Signature string `json:"signature"`
				}
				if includeThinking && json.Unmarshal([]byte(msg.Content), &payload) == nil && payload.Thinking != "" && payload.Signature != "" {
					item := map[string]interface{}{
						"type":      "thinking",
						"thinking":  payload.Thinking,
						"signature": payload.Signature,
					}
					pendingAssistant = append(pendingAssistant, item)
				}
				// If no valid signature, omit the thinking block entirely.
				continue
			}
			if msg.Name != "" && msg.ToolID != "" {
				// tool_use item
				var input any
				if strings.TrimSpace(msg.Content) == "" {
					input = map[string]any{}
				} else if err := json.Unmarshal([]byte(msg.Content), &input); err != nil {
					input = map[string]any{}
				}
				pendingAssistant = append(pendingAssistant, map[string]interface{}{
					"type":  "tool_use",
					"id":    msg.ToolID,
					"name":  msg.Name,
					"input": input,
				})
				continue
			}
			// Plain assistant text
			pendingAssistant = append(pendingAssistant, map[string]interface{}{
				"type": "text",
				"text": msg.Content,
			})
		case "tool", "function":
			// Flush any pending assistant content before switching roles
			flushAssistant(&pendingAssistant)
			// Tool results are sent from the user with a tool_result block
			result = append(result, map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type":        "tool_result",
						"tool_use_id": msg.ToolID,
						"content":     msg.Content,
						"is_error":    false,
					},
				},
			})
		default:
			// Flush any pending assistant content before switching roles
			flushAssistant(&pendingAssistant)
			// user message or others
			result = append(result, map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": msg.Content,
					},
				},
			})
		}
	}

	// Flush trailing assistant content
	if len(pendingAssistant) > 0 {
		// Ensure thinking appears first if present per Anthropic requirement
		// When both thinking and tool_use/text exist, reorder so thinking leads
		hasThinking := false
		for _, it := range pendingAssistant {
			if it["type"] == "thinking" || it["type"] == "redacted_thinking" {
				hasThinking = true
				break
			}
		}
		if includeThinking && hasThinking {
			reordered := make([]map[string]interface{}, 0, len(pendingAssistant))
			// First, all thinking-like blocks
			for _, it := range pendingAssistant {
				if it["type"] == "thinking" || it["type"] == "redacted_thinking" {
					reordered = append(reordered, it)
				}
			}
			// Then, everything else in original order
			for _, it := range pendingAssistant {
				if it["type"] != "thinking" && it["type"] != "redacted_thinking" {
					reordered = append(reordered, it)
				}
			}
			pendingAssistant = reordered
		}
		result = append(result, map[string]interface{}{
			"role":    "assistant",
			"content": pendingAssistant,
		})
	}

	return result
}

// convertTools transforms engine tool schemas to Anthropic Claude format.
func convertTools(tools []engine.ToolSchema) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(tools))

	for _, tool := range tools {
		properties := tool.Schema["properties"]
		required := tool.Schema["required"]
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

// supportsThinkingForModel returns whether the Anthropic model supports the
// "thinking" parameter. We disable thinking for all `claude-3-*` models
// except the newer `claude-3-5-*` and `claude-3-7-*` series. All other models
// (e.g., 3.5, 3.7, 4.x) keep the existing behavior.
func supportsThinkingForModel(modelID string) bool {
	// Normalize common provider prefix already trimmed to modelID earlier.
	// Only block classic 3.x (opus/sonnet/haiku) which do not support thinking.
	if strings.HasPrefix(modelID, "claude-3-") {
		if strings.HasPrefix(modelID, "claude-3-5-") || strings.HasPrefix(modelID, "claude-3-7-") {
			return true
		}
		return false
	}
	return true
}

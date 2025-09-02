package responses

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/loom/loom/internal/engine"
)

// ResponseStatus represents the lifecycle state of a response
type ResponseStatus int

const (
	ResponsePending ResponseStatus = iota
	ResponseComplete
	ResponseAborted
	ResponseStale
)

// ResponseTracker tracks the lifecycle of responses for proper previous_response_id handling
type ResponseTracker struct {
	ID        string
	Status    ResponseStatus
	CreatedAt time.Time
}

// Client implements the OpenAI Responses API as an engine.LLM.
type Client struct {
	apiKey     string
	model      string
	endpoint   string
	httpClient *http.Client
	debug      bool

	// Response lifecycle tracking
	mu           sync.RWMutex
	lastResponse *ResponseTracker
	responseTTL  time.Duration // TTL for response IDs (default 10 minutes)
}

// New creates a new Responses client.
func New(apiKey string, model string) *Client {
	if strings.TrimSpace(model) == "" {
		model = "o4-mini"
	}
	// Enable debug logs via env flags
	debug := false
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("LOOM_DEBUG_OPENAI"))); v == "1" || v == "true" || v == "yes" || v == "debug" {
		debug = true
	} else if v := strings.ToLower(strings.TrimSpace(os.Getenv("LOOM_DEBUG"))); v == "1" || v == "true" || v == "yes" || v == "debug" {
		debug = true
	}
	return &Client{
		apiKey:   apiKey,
		model:    model,
		endpoint: "https://api.openai.com/v1/responses",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		debug:       debug,
		responseTTL: 10 * time.Minute, // Default 10 minute TTL for response IDs
	}
}

// WithEndpoint overrides the default API endpoint.
func (c *Client) WithEndpoint(endpoint string) *Client {
	if strings.TrimSpace(endpoint) != "" {
		c.endpoint = endpoint
	}
	return c
}

// WithDebug enables or disables debug logging to stdout.
func (c *Client) WithDebug(debug bool) *Client {
	c.debug = debug
	return c
}

func (c *Client) debugf(format string, args ...interface{}) {
	if c != nil && c.debug {
		fmt.Printf("[openai:responses] "+format+"\n", args...)
	}
}

// startResponse marks the beginning of a new response
func (c *Client) startResponse(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastResponse = &ResponseTracker{
		ID:        id,
		Status:    ResponsePending,
		CreatedAt: time.Now(),
	}
	if c.debug {
		c.debugf("Started tracking response: %s", id)
	}
}

// completeResponse marks a response as successfully completed
func (c *Client) completeResponse(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastResponse != nil && c.lastResponse.ID == id {
		c.lastResponse.Status = ResponseComplete
		if c.debug {
			c.debugf("Marked response complete: %s", id)
		}
	}
}

// abortResponse marks a response as aborted (interrupted/cancelled)
func (c *Client) abortResponse() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastResponse != nil && c.lastResponse.Status == ResponsePending {
		c.lastResponse.Status = ResponseAborted
		if c.debug {
			c.debugf("Marked response aborted: %s", c.lastResponse.ID)
		}
	}
}

// markStale marks the current response as stale (API rejected previous_response_id)
func (c *Client) markStale() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastResponse != nil {
		c.lastResponse.Status = ResponseStale
		if c.debug {
			c.debugf("Marked response stale: %s", c.lastResponse.ID)
		}
	}
}

// getSafePreviousResponseID returns the previous response ID only if it's safe to use
func (c *Client) getSafePreviousResponseID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.lastResponse == nil {
		return ""
	}

	// Only use complete responses
	if c.lastResponse.Status != ResponseComplete {
		if c.debug {
			c.debugf("Skipping previous_response_id due to status: %d", c.lastResponse.Status)
		}
		return ""
	}

	// Check TTL
	if time.Since(c.lastResponse.CreatedAt) > c.responseTTL {
		if c.debug {
			c.debugf("Skipping previous_response_id due to TTL: %s", c.lastResponse.ID)
		}
		return ""
	}

	if c.debug {
		c.debugf("Using previous_response_id: %s", c.lastResponse.ID)
	}
	return c.lastResponse.ID
}

// isPreviousResponseNotFoundError checks if an error indicates a stale previous_response_id
func isPreviousResponseNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "previous response") &&
		strings.Contains(errStr, "not found") &&
		(strings.Contains(errStr, "previous_response_id") ||
			strings.Contains(errStr, "invalid_request_error"))
}

// makeRequest executes an HTTP request with proper error handling and retry logic for stale response IDs
func (c *Client) makeRequest(ctx context.Context, req responsesRequest) (*http.Response, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %v", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("request creation error: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	c.debugf("POST %s", c.endpoint)
	if req.PreviousID != "" {
		c.debugf("Using previous_response_id: %s", req.PreviousID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP error: %v", err)
	}

	if resp.StatusCode == http.StatusOK {
		return resp, nil
	}

	// Read error response for detailed error handling
	data, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("API error (%d): failed to read response body", resp.StatusCode)
	}

	apiError := fmt.Errorf("API error (%d): %s", resp.StatusCode, string(data))

	// Check if this is a stale previous_response_id error and retry is possible
	if resp.StatusCode == http.StatusBadRequest && isPreviousResponseNotFoundError(apiError) && req.PreviousID != "" {
		c.debugf("Detected stale previous_response_id, retrying without it")
		c.markStale() // Mark current response as stale

		// Retry without previous_response_id
		retryReq := req
		retryReq.PreviousID = ""

		retryBodyBytes, err := json.Marshal(retryReq)
		if err != nil {
			return nil, fmt.Errorf("retry marshal error: %v", err)
		}

		retryHttpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(retryBodyBytes))
		if err != nil {
			return nil, fmt.Errorf("retry request creation error: %v", err)
		}
		retryHttpReq.Header.Set("Content-Type", "application/json")
		retryHttpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

		c.debugf("Retrying POST %s without previous_response_id", c.endpoint)
		return c.httpClient.Do(retryHttpReq)
	}

	return nil, apiError
}

// isReasoningModel mirrors the heuristic from the Chat Completions client.
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

// Minimal tool-args validator to avoid emitting incomplete tool calls.
func validateToolArgs(toolName string, args map[string]interface{}) bool {
	switch toolName {
	case "read_file":
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
		return true
	}
}

// responsesRequest is the payload for the Responses API.
type responsesRequest struct {
	Model        string                   `json:"model"`
	Instructions string                   `json:"instructions,omitempty"`
	Input        []map[string]interface{} `json:"input,omitempty"`
	Tools        []map[string]interface{} `json:"tools,omitempty"`
	ToolChoice   interface{}              `json:"tool_choice,omitempty"`
	Stream       bool                     `json:"stream,omitempty"`
	Temperature  *float64                 `json:"temperature,omitempty"`
	MaxTokens    *int                     `json:"max_completion_tokens,omitempty"`
	PreviousID   string                   `json:"previous_response_id,omitempty"`
	Reasoning    *struct {
		Effort  string `json:"effort,omitempty"`
		Summary string `json:"summary,omitempty"`
	} `json:"reasoning,omitempty"`
}

func toResponsesInput(msgs []engine.Message) (instructions string, items []map[string]interface{}) {
	var systems []string
	for _, m := range msgs {
		switch m.Role {
		case "system":
			systems = append(systems, m.Content)
		case "user":
			items = append(items, map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "input_text", "text": m.Content},
				},
			})
		case "assistant":
			if m.Name == "" && m.ToolID == "" && strings.TrimSpace(m.Content) != "" {
				items = append(items, map[string]interface{}{
					"role": "assistant",
					"content": []map[string]interface{}{
						{"type": "output_text", "text": m.Content},
					},
				})
			}
		case "tool", "function":
			if m.ToolID != "" {
				items = append(items, map[string]interface{}{
					"type":    "function_call_output",
					"call_id": m.ToolID,
					"output":  m.Content,
				})
			}
		}
	}
	instructions = strings.Join(systems, "\n")
	return
}

// isEmptyResponse checks if a response is effectively empty (only whitespace).
func isEmptyResponse(content string) bool {
	return strings.TrimSpace(content) == ""
}

// Chat implements engine.LLM using the Responses API.
func (c *Client) Chat(
	ctx context.Context,
	messages []engine.Message,
	tools []engine.ToolSchema,
	stream bool,
) (<-chan engine.TokenOrToolCall, error) {
	if c.apiKey == "" {
		return nil, errors.New("OpenAI API key not set")
	}

	out := make(chan engine.TokenOrToolCall)

	// Start a goroutine to handle the response with retry logic
	go func() {
		defer close(out)
		c.chatWithRetry(ctx, messages, tools, stream, out)
	}()

	return out, nil
}

// chatWithRetry implements retry logic for empty responses
func (c *Client) chatWithRetry(ctx context.Context, messages []engine.Message, tools []engine.ToolSchema, stream bool, out chan<- engine.TokenOrToolCall) {
	// Try first attempt and track if we receive any content
	contentReceived, toolCallReceived := c.attemptChat(ctx, messages, tools, stream, out)

	// If we got empty response, retry with opposite streaming mode
	if !contentReceived && !toolCallReceived {
		select {
		case <-ctx.Done():
			return
		case out <- engine.TokenOrToolCall{Token: "Retrying due to empty response..."}:
		}
		c.attemptChat(ctx, messages, tools, !stream, out)
	}
}

// attemptChat performs a single chat attempt and returns whether content/toolcalls were received
func (c *Client) attemptChat(ctx context.Context, messages []engine.Message, tools []engine.ToolSchema, stream bool, out chan<- engine.TokenOrToolCall) (contentReceived, toolCallReceived bool) {
	instructions, input := toResponsesInput(messages)
	req := responsesRequest{
		Model:        c.model,
		Instructions: instructions,
		Input:        input,
		Stream:       stream,
	}
	// Enable reasoning summaries on supported models
	if isReasoningModel(c.model) {
		req.Reasoning = &struct {
			Effort  string `json:"effort,omitempty"`
			Summary string `json:"summary,omitempty"`
		}{Effort: "medium", Summary: "auto"}
	}
	if len(tools) > 0 {
		req.Tools = convertTools(tools)
		req.ToolChoice = "auto"
	}
	if !isReasoningModel(c.model) {
		t := 0.2
		req.Temperature = &t
	}
	// If we are replying with function_call_output items, providing the previous_response_id is required
	// but only use it if the previous response was completed successfully
	safePrevID := c.getSafePreviousResponseID()
	if safePrevID != "" {
		for _, it := range input {
			if typ, ok := it["type"].(string); ok && typ == "function_call_output" {
				req.PreviousID = safePrevID
				break
			}
		}
	}

	defer func() {
		// Mark response as aborted if context was cancelled
		if ctx.Err() != nil {
			c.abortResponse()
		}
	}()

	c.debugf("Chat request: model=%s stream=%v messages=%d tools=%d", c.model, stream, len(messages), len(tools))

	// Execute request with retry logic for stale response IDs
	resp, err := c.makeRequest(ctx, req)
	if err != nil {
		select {
		case <-ctx.Done():
			c.abortResponse()
			return false, false
		case out <- engine.TokenOrToolCall{Token: fmt.Sprintf("OpenAI error: %v", err)}:
		}
		return false, false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		msg := fmt.Sprintf("OpenAI API error (%d): %s", resp.StatusCode, string(data))
		c.debugf(msg)
		select {
		case <-ctx.Done():
			c.abortResponse()
			return false, false
		case out <- engine.TokenOrToolCall{Token: msg}:
		}
		return false, false
	}

	if stream {
		c.debugf("Begin streaming response...")
		return c.handleResponsesStreamWithTracking(ctx, resp.Body, out)
	}

	nonStreamBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, false
	}
	if c.debug {
		// Truncate to avoid excessive logs
		dump := string(nonStreamBody)
		if len(dump) > 4000 {
			dump = dump[:4000] + "…(truncated)"
		}
		c.debugf("Non-streaming body: %s", dump)
	}
	return c.handleResponsesNonStreamingWithTracking(ctx, nonStreamBody, out)
}

// Tool/Function call streaming events
type sseEvent struct {
	Type string `json:"type"`
	Item *struct {
		ID string `json:"id"`
	} `json:"item,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
	Error    *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Delta string `json:"delta,omitempty"`

	OutputIndex *int   `json:"output_index,omitempty"`
	CallID      string `json:"call_id,omitempty"`
	Name        string `json:"name,omitempty"`
	Arguments   string `json:"arguments,omitempty"`

	// Reasoning summary fields
	SummaryIndex *int   `json:"summary_index,omitempty"`
	Text         string `json:"text,omitempty"`
}

type partialCall struct {
	Name        string
	Args        string
	ItemID      string
	CallID      string
	OutputIndex int
	Done        bool
}

type responsesNonStream struct {
	ID     string `json:"id"`
	Output []struct {
		Type    string `json:"type"`
		Message *struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message,omitempty"`
		FunctionCall *struct {
			CallID    string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function_call,omitempty"`
		Summary []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"summary,omitempty"`
	} `json:"output"`
}

// handleResponsesStreamWithTracking processes a streaming response and tracks content/tool calls
func (c *Client) handleResponsesStreamWithTracking(ctx context.Context, r io.Reader, out chan<- engine.TokenOrToolCall) (contentReceived, toolCallReceived bool) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var currentEvent string
	parts := make(map[string]*partialCall)
	var lastItemID string
	var responseID string // Track the response ID for lifecycle management

	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimSpace(line[7:])
			if c.debug {
				c.debugf("SSE event: %s", currentEvent)
			}
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimSpace(line[6:])
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var ev sseEvent
		raw := []byte(payload)
		if err := json.Unmarshal(raw, &ev); err != nil {
			c.debugf("Failed to unmarshal SSE payload for event %s: %v", currentEvent, err)
			continue
		}

		// Track latest item id as a fallback for function_call events lacking call_id
		if ev.Item != nil && ev.Item.ID != "" {
			lastItemID = ev.Item.ID
		}

		switch currentEvent {
		case "response.created":
			// Capture the response id and start tracking it
			if len(ev.Response) > 0 {
				var meta struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal(ev.Response, &meta); err == nil && strings.TrimSpace(meta.ID) != "" {
					responseID = meta.ID
					c.startResponse(responseID)
					if c.debug {
						c.debugf("Started tracking response (stream): %s", responseID)
					}
				}
			}
		case "response.output_text.delta":
			if ev.Delta != "" && !isEmptyResponse(ev.Delta) {
				contentReceived = true
				select {
				case <-ctx.Done():
					return
				case out <- engine.TokenOrToolCall{Token: ev.Delta}:
				}
			}

		case "response.output_item.added":
			// Parse full payload to capture function_call name/args/output_index
			var e struct {
				OutputIndex int `json:"output_index"`
				Item        struct {
					ID        string `json:"id"`
					Type      string `json:"type"`
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
					CallID    string `json:"call_id"`
				} `json:"item"`
			}
			if err := json.Unmarshal(raw, &e); err == nil && e.Item.Type == "function_call" {
				id := e.Item.ID
				if id == "" {
					id = lastItemID
				}
				if id != "" {
					pc := parts[id]
					if pc == nil {
						pc = &partialCall{ItemID: id}
						parts[id] = pc
					}
					if pc.Name == "" {
						pc.Name = e.Item.Name
					}
					if pc.Args == "" && e.Item.Arguments != "" {
						pc.Args = e.Item.Arguments
					}
					if pc.CallID == "" && e.Item.CallID != "" {
						pc.CallID = e.Item.CallID
					}
					pc.OutputIndex = e.OutputIndex
					if c.debug {
						c.debugf("output_item.added captured function_call: item_id=%s call_id=%s name=%s argsLen=%d idx=%d", id, pc.CallID, pc.Name, len(pc.Args), pc.OutputIndex)
					}
				}
			}

			// Try to extract function_call metadata directly from the response payload
			if len(ev.Response) > 0 {
				var env struct {
					Item struct {
						ID string `json:"id"`
					} `json:"item"`
					Output []struct {
						Type         string `json:"type"`
						FunctionCall *struct {
							CallID    string `json:"call_id"`
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function_call"`
					} `json:"output"`
				}
				if err := json.Unmarshal(ev.Response, &env); err == nil {
					for _, out := range env.Output {
						if out.FunctionCall != nil {
							id := out.FunctionCall.CallID
							if id == "" {
								id = env.Item.ID
							}
							if id == "" {
								id = lastItemID
							}
							if id == "" {
								continue
							}
							if _, ok := parts[id]; !ok {
								parts[id] = &partialCall{CallID: id}
							}
							if out.FunctionCall.Name != "" {
								parts[id].Name = out.FunctionCall.Name
							}
							if out.FunctionCall.Arguments != "" {
								parts[id].Args += out.FunctionCall.Arguments
							}
							if c.debug {
								c.debugf("output_item.added captured function_call: id=%s name=%s argsLen=%d", id, out.FunctionCall.Name, len(out.FunctionCall.Arguments))
							}
						}
					}
				}
			}
		case "response.reasoning_summary.delta", "response.reasoning_summary_text.delta":
			if ev.Delta != "" {
				select {
				case <-ctx.Done():
					return
				case out <- engine.TokenOrToolCall{Token: "[REASONING] " + ev.Delta}:
				}
			}
		case "response.reasoning_summary.done", "response.reasoning_summary_text.done":
			if ev.Text != "" {
				select {
				case <-ctx.Done():
					return
				case out <- engine.TokenOrToolCall{Token: "[REASONING_DONE] " + ev.Text}:
				}
			}
		case "response.reasoning.delta":
			if ev.Delta != "" {
				select {
				case <-ctx.Done():
					return
				case out <- engine.TokenOrToolCall{Token: "[REASONING_RAW] " + ev.Delta}:
				}
			}
		case "response.reasoning.done":
			if ev.Text != "" {
				select {
				case <-ctx.Done():
					return
				case out <- engine.TokenOrToolCall{Token: "[REASONING_RAW_DONE] " + ev.Text}:
				}
			}
		case "response.function_call.arguments.delta", "response.function_call_arguments.delta", "response.function_call.delta":
			// Parse standardized delta payload
			var d struct {
				ItemID      string `json:"item_id"`
				OutputIndex int    `json:"output_index"`
				CallID      string `json:"call_id"`
				Delta       string `json:"delta"`
			}
			_ = json.Unmarshal(raw, &d)
			id := d.ItemID
			if id == "" && ev.Item != nil {
				id = ev.Item.ID
			}
			if id == "" {
				id = lastItemID
			}
			if id == "" {
				break
			}
			pc := parts[id]
			if pc == nil {
				pc = &partialCall{ItemID: id}
				parts[id] = pc
			}
			if pc.CallID == "" && d.CallID != "" {
				pc.CallID = d.CallID
			}
			if d.Delta != "" {
				pc.Args += d.Delta
			}
			if d.OutputIndex != 0 {
				pc.OutputIndex = d.OutputIndex
			}
			if c.debug {
				c.debugf("function_call.arguments.delta: item_id=%s call_id=%s name=%q args+=%dB idx=%d", id, pc.CallID, pc.Name, len(d.Delta), pc.OutputIndex)
			}
		case "response.function_call.arguments.done", "response.function_call_arguments.done", "response.function_call.done":
			// Attempt to assemble and emit a tool call as soon as the function_call is done
			var d struct {
				ItemID    string `json:"item_id"`
				CallID    string `json:"call_id"`
				Arguments string `json:"arguments"`
			}
			_ = json.Unmarshal(raw, &d)
			if c.debug {
				c.debugf("function_call.arguments.done: attempting to assemble tool call from %d parts (item_id=%s)", len(parts), d.ItemID)
			}
			// Prefer to emit for the specific item id when available
			if d.ItemID != "" {
				if pc, ok := parts[d.ItemID]; ok {
					if pc.CallID == "" && d.CallID != "" {
						pc.CallID = d.CallID
					}
					if pc.Args == "" && d.Arguments != "" {
						pc.Args = d.Arguments
					}
					// Try recover missing name from args keys if needed
					if strings.TrimSpace(pc.Name) == "" && strings.TrimSpace(pc.Args) != "" {
						var tmp map[string]interface{}
						if json.Unmarshal([]byte(pc.Args), &tmp) == nil {
							if v, ok := tmp["tool"]; ok {
								if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
									pc.Name = s
								}
							} else if v, ok := tmp["name"]; ok {
								if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
									pc.Name = s
								}
							}
						}
					}
					if strings.TrimSpace(pc.Name) != "" && strings.TrimSpace(pc.Args) != "" && strings.TrimSpace(pc.CallID) != "" {
						var argsMap map[string]interface{}
						if json.Unmarshal([]byte(pc.Args), &argsMap) == nil && validateToolArgs(pc.Name, argsMap) {
							rawArgs, _ := json.Marshal(argsMap)
							tc := &engine.ToolCall{ID: pc.CallID, Name: pc.Name, Args: rawArgs}
							toolCallReceived = true
							if c.debug {
								c.debugf("EMIT tool_call (on .done) call_id=%s item_id=%s name=%s args=%q", pc.CallID, pc.ItemID, pc.Name, truncate(string(rawArgs), 300))
							}
							select {
							case <-ctx.Done():
								return
							case out <- engine.TokenOrToolCall{ToolCall: tc}:
								// Do not return; continue reading until response.completed to capture usage
							}
						}
					}
				}
			}
		case "response.function_call.name.delta", "response.function_call_name.delta":
			// Capture tool name fragments
			id := ev.CallID
			if id == "" && ev.Item != nil {
				id = ev.Item.ID
			}
			if id == "" {
				id = lastItemID
			}
			if id == "" {
				break
			}
			if _, ok := parts[id]; !ok {
				parts[id] = &partialCall{CallID: id}
			}
			nameFrag := ev.Name
			if nameFrag == "" && ev.Delta != "" {
				nameFrag = ev.Delta
			}
			if nameFrag != "" {
				parts[id].Name += nameFrag
			}
			if c.debug {
				c.debugf("function_call.name.delta: id=%s name+=%q", id, truncate(nameFrag, 100))
			}
		case "response.completed":
			// Mark response as complete
			if responseID != "" {
				c.completeResponse(responseID)
				if c.debug {
					c.debugf("Marked response complete (stream): %s", responseID)
				}
			}
			if c.debug {
				c.debugf("response.completed: assembling %d potential calls", len(parts))
			}
			// First, inspect the embedded response to recover any missing name/arguments
			if len(ev.Response) > 0 {
				var r struct {
					Output []struct {
						Type        string `json:"type"`
						ID          string `json:"id"`
						Name        string `json:"name"`
						Arguments   string `json:"arguments"`
						CallID      string `json:"call_id"`
						OutputIndex int    `json:"output_index"`
					} `json:"output"`
				}
				if json.Unmarshal(ev.Response, &r) == nil {
					for _, it := range r.Output {
						if it.Type != "function_call" {
							continue
						}
						id := it.ID
						if id == "" {
							id = lastItemID
						}
						if id == "" {
							continue
						}
						pc := parts[id]
						if pc == nil {
							pc = &partialCall{ItemID: id}
							parts[id] = pc
						}
						if pc.Name == "" {
							pc.Name = it.Name
						}
						if pc.Args == "" && it.Arguments != "" {
							pc.Args = it.Arguments
						}
						if pc.CallID == "" && it.CallID != "" {
							pc.CallID = it.CallID
						}
						if it.OutputIndex != 0 {
							pc.OutputIndex = it.OutputIndex
						}
					}
				}
			}
			for _, pc := range parts {
				if strings.TrimSpace(pc.Name) == "" || strings.TrimSpace(pc.Args) == "" {
					// Try to recover missing name from args, same as in .done
					if strings.TrimSpace(pc.Name) == "" && strings.TrimSpace(pc.Args) != "" {
						var tmp map[string]interface{}
						if json.Unmarshal([]byte(pc.Args), &tmp) == nil {
							if v, ok := tmp["tool"]; ok {
								if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
									pc.Name = s
								}
							} else if v, ok := tmp["name"]; ok {
								if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
									pc.Name = s
								}
							}
						}
					}
					if strings.TrimSpace(pc.Name) == "" || strings.TrimSpace(pc.Args) == "" {
						if c.debug {
							c.debugf("skip call id=%s due to empty fields: name=%q argsLen=%d", pc.CallID, pc.Name, len(pc.Args))
						}
						continue
					}
				}
				var argsMap map[string]interface{}
				if err := json.Unmarshal([]byte(pc.Args), &argsMap); err != nil {
					if c.debug {
						c.debugf("invalid JSON args for id=%s name=%s: %v; raw=%q", pc.CallID, pc.Name, err, truncate(pc.Args, 300))
					}
					continue
				}
				if !validateToolArgs(pc.Name, argsMap) {
					if c.debug {
						c.debugf("tool args failed validation for id=%s name=%s; args=%q", pc.CallID, pc.Name, truncate(pc.Args, 300))
					}
					continue
				}
				raw, err := json.Marshal(argsMap)
				if err != nil {
					if c.debug {
						c.debugf("failed to marshal validated args for id=%s name=%s: %v", pc.CallID, pc.Name, err)
					}
					continue
				}
				tc := &engine.ToolCall{ID: pc.CallID, Name: pc.Name, Args: raw}
				toolCallReceived = true
				if c.debug {
					c.debugf("EMIT tool_call id=%s name=%s args=%q", pc.CallID, pc.Name, truncate(string(raw), 300))
				}
				select {
				case <-ctx.Done():
					return
				case out <- engine.TokenOrToolCall{ToolCall: tc}:
					// Continue to also emit usage below
				}
			}
			// Try to parse usage from the embedded response and emit a usage marker
			if len(ev.Response) > 0 {
				var r struct {
					Usage *struct {
						InputTokens  int64 `json:"input_tokens"`
						OutputTokens int64 `json:"output_tokens"`
					} `json:"usage"`
				}
				if json.Unmarshal(ev.Response, &r) == nil && r.Usage != nil {
					usage := fmt.Sprintf("[USAGE] provider=openai model=%s in=%d out=%d", c.model, r.Usage.InputTokens, r.Usage.OutputTokens)
					select {
					case <-ctx.Done():
						return
					case out <- engine.TokenOrToolCall{Token: usage}:
					}
				}
			}
			return
		case "response.error":
			if ev.Error != nil {
				if c.debug {
					c.debugf("response.error: %s", ev.Error.Message)
				}
				select {
				case <-ctx.Done():
					return
				case out <- engine.TokenOrToolCall{Token: "OpenAI error: " + ev.Error.Message}:
					return
				}
			}
		}
	}
	return
}

// handleResponsesNonStreamingWithTracking processes a non-streaming response and tracks content/tool calls
func (c *Client) handleResponsesNonStreamingWithTracking(ctx context.Context, body []byte, out chan<- engine.TokenOrToolCall) (contentReceived, toolCallReceived bool) {
	var resp responsesNonStream
	if err := json.Unmarshal(body, &resp); err != nil {
		c.debugf("Non-streaming unmarshal error: %v", err)
		return false, false
	}

	// Start tracking the response and immediately mark as complete for non-streaming
	if resp.ID != "" {
		c.startResponse(resp.ID)
		c.completeResponse(resp.ID)
		if c.debug {
			c.debugf("Non-streaming response completed: %s", resp.ID)
		}
	}
	for _, it := range resp.Output {
		// Non-streaming reasoning summary
		if it.Type == "reasoning" && len(it.Summary) > 0 {
			for _, s := range it.Summary {
				if s.Text != "" && !isEmptyResponse(s.Text) {
					contentReceived = true
					for _, ch := range s.Text {
						select {
						case <-ctx.Done():
							return
						case out <- engine.TokenOrToolCall{Token: string(ch)}:
						}
					}
				}
			}
		}
		if it.FunctionCall != nil {
			if c.debug {
				c.debugf("Non-streaming function_call: name=%s argsLen=%d call_id=%s", it.FunctionCall.Name, len(it.FunctionCall.Arguments), it.FunctionCall.CallID)
			}
			// Validate arguments if possible
			argsStr := strings.TrimSpace(it.FunctionCall.Arguments)
			if argsStr == "" {
				argsStr = "{}"
			}
			var argsMap map[string]interface{}
			if err := json.Unmarshal([]byte(argsStr), &argsMap); err == nil {
				if !validateToolArgs(it.FunctionCall.Name, argsMap) {
					// Skip emitting invalid tool calls
					if c.debug {
						c.debugf("Non-streaming function_call failed validation: name=%s args=%q", it.FunctionCall.Name, truncate(argsStr, 300))
					}
				} else if raw, err := json.Marshal(argsMap); err == nil {
					tc := &engine.ToolCall{ID: it.FunctionCall.CallID, Name: it.FunctionCall.Name, Args: raw}
					toolCallReceived = true
					if c.debug {
						c.debugf("EMIT non-stream tool_call id=%s name=%s args=%q", it.FunctionCall.CallID, it.FunctionCall.Name, truncate(string(raw), 300))
					}
					select {
					case <-ctx.Done():
						return
					case out <- engine.TokenOrToolCall{ToolCall: tc}:
						return
					}
				}
			} else {
				// Fallback: pass raw string
				tc := &engine.ToolCall{ID: it.FunctionCall.CallID, Name: it.FunctionCall.Name, Args: json.RawMessage(argsStr)}
				toolCallReceived = true
				if c.debug {
					c.debugf("EMIT non-stream tool_call (raw-args) id=%s name=%s args=%q", it.FunctionCall.CallID, it.FunctionCall.Name, truncate(argsStr, 300))
				}
				select {
				case <-ctx.Done():
					return
				case out <- engine.TokenOrToolCall{ToolCall: tc}:
					return
				}
			}
		}
		if it.Message != nil {
			for _, cpart := range it.Message.Content {
				if cpart.Type == "output_text" && cpart.Text != "" && !isEmptyResponse(cpart.Text) {
					contentReceived = true
					for _, ch := range cpart.Text {
						select {
						case <-ctx.Done():
							return
						case out <- engine.TokenOrToolCall{Token: string(ch)}:
						}
					}
				}
			}
		}
	}
	return
}

// convertTools converts engine tool schemas to Responses-compatible format.
func convertTools(tools []engine.ToolSchema) []map[string]interface{} {
	// Responses API expects function tools with top-level name/parameters, not nested under "function"
	result := make([]map[string]interface{}, 0, len(tools))
	for _, t := range tools {
		result = append(result, map[string]interface{}{
			"type":        "function",
			"name":        t.Name,
			"description": t.Description,
			"parameters":  t.Schema,
		})
	}
	return result
}

// truncate returns a shortened version of s with an ellipsis if it exceeds n runes.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return s[:n-1] + "…"
}

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
	"strings"
	"time"

	"github.com/loom/loom/internal/engine"
)

// Client implements the OpenAI Responses API as an engine.LLM.
type Client struct {
	apiKey     string
	model      string
	endpoint   string
	httpClient *http.Client
	// Optionally track last response id in future for previous_response_id
}

// New creates a new Responses client.
func New(apiKey string, model string) *Client {
	if strings.TrimSpace(model) == "" {
		model = "o4-mini"
	}
	return &Client{
		apiKey:   apiKey,
		model:    model,
		endpoint: "https://api.openai.com/v1/responses",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// WithEndpoint overrides the default API endpoint.
func (c *Client) WithEndpoint(endpoint string) *Client {
	if strings.TrimSpace(endpoint) != "" {
		c.endpoint = endpoint
	}
	return c
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
	case "finalize":
		if v, ok := args["summary"]; ok {
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

	// Build request
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

	go func() {
		defer close(out)

		bodyBytes, err := json.Marshal(req)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case out <- engine.TokenOrToolCall{Token: fmt.Sprintf("OpenAI marshal error: %v", err)}:
			}
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(bodyBytes))
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case out <- engine.TokenOrToolCall{Token: fmt.Sprintf("OpenAI request error: %v", err)}:
			}
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case out <- engine.TokenOrToolCall{Token: fmt.Sprintf("OpenAI HTTP error: %v", err)}:
			}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(resp.Body)
			msg := fmt.Sprintf("OpenAI API error (%d): %s", resp.StatusCode, string(data))
			select {
			case <-ctx.Done():
				return
			case out <- engine.TokenOrToolCall{Token: msg}:
			}
			return
		}

		if stream {
			c.handleResponsesStream(ctx, resp.Body, out)
			return
		}

		nonStreamBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return
		}
		c.handleResponsesNonStreaming(ctx, nonStreamBody, out)
	}()

	return out, nil
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

type partialCall struct{ Name, Args, CallID string }

func (c *Client) handleResponsesStream(ctx context.Context, r io.Reader, out chan<- engine.TokenOrToolCall) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var currentEvent string
	parts := make(map[string]*partialCall)

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
		// Removed verbose SSE event logging
		var ev sseEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue
		}

		switch currentEvent {
		case "response.output_text.delta":
			if ev.Delta != "" {
				select {
				case <-ctx.Done():
					return
				case out <- engine.TokenOrToolCall{Token: ev.Delta}:
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
		case "response.function_call.arguments.delta":
			id := ev.CallID
			if id == "" && ev.Item != nil {
				id = ev.Item.ID
			}
			if _, ok := parts[id]; !ok {
				parts[id] = &partialCall{CallID: id}
			}
			if ev.Name != "" {
				parts[id].Name += ev.Name
			}
			if ev.Arguments != "" {
				parts[id].Args += ev.Arguments
			}
		case "response.completed":
			for _, pc := range parts {
				if strings.TrimSpace(pc.Name) == "" || strings.TrimSpace(pc.Args) == "" {
					continue
				}
				var argsMap map[string]interface{}
				if err := json.Unmarshal([]byte(pc.Args), &argsMap); err != nil {
					continue
				}
				if !validateToolArgs(pc.Name, argsMap) {
					continue
				}
				raw, err := json.Marshal(argsMap)
				if err != nil {
					continue
				}
				tc := &engine.ToolCall{ID: pc.CallID, Name: pc.Name, Args: raw}
				select {
				case <-ctx.Done():
					return
				case out <- engine.TokenOrToolCall{ToolCall: tc}:
					return
				}
			}
			return
		case "response.error":
			if ev.Error != nil {
				select {
				case <-ctx.Done():
					return
				case out <- engine.TokenOrToolCall{Token: "OpenAI error: " + ev.Error.Message}:
					return
				}
			}
		}
	}
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

func (c *Client) handleResponsesNonStreaming(ctx context.Context, body []byte, out chan<- engine.TokenOrToolCall) {
	var resp responsesNonStream
	if err := json.Unmarshal(body, &resp); err != nil {
		return
	}
	for _, it := range resp.Output {
		// Non-streaming reasoning summary
		if it.Type == "reasoning" && len(it.Summary) > 0 {
			for _, s := range it.Summary {
				if s.Text != "" {
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
			// Validate arguments if possible
			argsStr := strings.TrimSpace(it.FunctionCall.Arguments)
			if argsStr == "" {
				argsStr = "{}"
			}
			var argsMap map[string]interface{}
			if err := json.Unmarshal([]byte(argsStr), &argsMap); err == nil {
				if !validateToolArgs(it.FunctionCall.Name, argsMap) {
					// Skip emitting invalid tool calls
				} else if raw, err := json.Marshal(argsMap); err == nil {
					tc := &engine.ToolCall{ID: it.FunctionCall.CallID, Name: it.FunctionCall.Name, Args: raw}
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
				if cpart.Type == "output_text" && cpart.Text != "" {
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

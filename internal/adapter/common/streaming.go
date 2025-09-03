package common

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/loom/loom/internal/engine"
)

// StreamEvent represents a parsed streaming event
type StreamEvent struct {
	Type      string                 `json:"type,omitempty"`
	Delta     *Delta                 `json:"delta,omitempty"`
	Response  json.RawMessage        `json:"response,omitempty"`
	Usage     *UsageInfo             `json:"usage,omitempty"`
	Error     *ErrorInfo             `json:"error,omitempty"`
	ToolCall  *StreamingToolCall     `json:"tool_call,omitempty"`
	Reasoning string                 `json:"reasoning,omitempty"`
	Raw       map[string]interface{} `json:"-"` // For provider-specific data
}

// Delta represents content or tool call deltas
type Delta struct {
	Content   string `json:"content,omitempty"`
	Text      string `json:"text,omitempty"`
	Reasoning string `json:"reasoning,omitempty"`
	ToolCalls []struct {
		Index    int    `json:"index"`
		ID       string `json:"id,omitempty"`
		Type     string `json:"type,omitempty"`
		Function struct {
			Name      string `json:"name,omitempty"`
			Arguments string `json:"arguments,omitempty"`
		} `json:"function,omitempty"`
	} `json:"tool_calls,omitempty"`
}

// UsageInfo represents token usage information
type UsageInfo struct {
	InputTokens      int64 `json:"input_tokens,omitempty"`
	OutputTokens     int64 `json:"output_tokens,omitempty"`
	PromptTokens     int64 `json:"prompt_tokens,omitempty"`
	CompletionTokens int64 `json:"completion_tokens,omitempty"`
	TotalTokens      int64 `json:"total_tokens,omitempty"`
}

// ErrorInfo represents error information
type ErrorInfo struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message"`
}

// StreamingToolCall represents a tool call being assembled during streaming
type StreamingToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Index     int    `json:"index,omitempty"`
	Done      bool   `json:"done,omitempty"`
}

// StreamHandler defines the interface for provider-specific streaming logic
type StreamHandler interface {
	// ParseLine parses a single SSE line and returns a StreamEvent
	ParseLine(currentEvent, data string) (*StreamEvent, error)

	// HandleSpecialTokens processes provider-specific special tokens
	// Returns (handled, shouldContinue)
	HandleSpecialTokens(token string, ch chan<- engine.TokenOrToolCall) (bool, bool)

	// FormatUsage formats usage information as a token
	FormatUsage(usage *UsageInfo) string

	// ShouldEmitToolCall determines if a tool call should be emitted now
	ShouldEmitToolCall(finishReason string, partials map[int]*PartialCall) bool
}

// PartialCall tracks a tool call being assembled during streaming
type PartialCall struct {
	ID          string
	Name        string
	Args        string
	Index       int
	Done        bool
	OutputIndex int // For Responses API
}

// StreamProcessor handles unified streaming logic across all providers
type StreamProcessor struct {
	handler   StreamHandler
	debug     bool
	modelName string
}

// NewStreamProcessor creates a new unified stream processor
func NewStreamProcessor(handler StreamHandler, modelName string, debug bool) *StreamProcessor {
	return &StreamProcessor{
		handler:   handler,
		debug:     debug,
		modelName: modelName,
	}
}

// ProcessStream processes a streaming response using the provider-specific handler
func (sp *StreamProcessor) ProcessStream(
	ctx context.Context,
	body io.Reader,
	ch chan<- engine.TokenOrToolCall,
) (contentReceived, toolCallReceived bool) {
	scanner := sp.createScanner(body)

	var currentEvent string
	partials := make(map[int]*PartialCall)
	var lastItemID string // For providers that need fallback IDs

	// Track slow responses
	slowTicker := time.NewTicker(20 * time.Second)
	defer slowTicker.Stop()
	slowNotified := false

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return contentReceived, toolCallReceived
		case <-slowTicker.C:
			if !slowNotified {
				// Optionally notify about slow response
				slowNotified = true
			}
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse SSE line
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimSpace(line[7:])
			if sp.debug {
				sp.debugf("SSE event: %s", currentEvent)
			}
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimSpace(line[6:])
		if data == "" || data == "[DONE]" {
			continue
		}

		// Let provider-specific handler parse the event
		event, err := sp.handler.ParseLine(currentEvent, data)
		if err != nil {
			if sp.debug {
				sp.debugf("Failed to parse event %s: %v", currentEvent, err)
			}
			continue
		}

		if event == nil {
			continue
		}

		// Handle the parsed event
		if sp.handleStreamEvent(ctx, event, partials, &lastItemID, ch, &contentReceived, &toolCallReceived) {
			// Stream ended
			break
		}
	}

	return contentReceived, toolCallReceived
}

// handleStreamEvent processes a single parsed stream event
func (sp *StreamProcessor) handleStreamEvent(
	ctx context.Context,
	event *StreamEvent,
	partials map[int]*PartialCall,
	lastItemID *string,
	ch chan<- engine.TokenOrToolCall,
	contentReceived, toolCallReceived *bool,
) bool {
	// Handle text content
	if event.Delta != nil {
		if content := sp.getContent(event.Delta); content != "" && !IsEmptyResponse(content) {
			*contentReceived = true
			select {
			case <-ctx.Done():
				return true
			case ch <- engine.TokenOrToolCall{Token: content}:
			}
		}

		// Handle reasoning content
		if event.Delta.Reasoning != "" {
			select {
			case <-ctx.Done():
				return true
			case ch <- engine.TokenOrToolCall{Token: "[REASONING] " + event.Delta.Reasoning}:
			}
		}

		// Handle tool call deltas
		if len(event.Delta.ToolCalls) > 0 {
			sp.handleToolCallDeltas(event.Delta.ToolCalls, partials, lastItemID)
		}
	}

	// Handle special tokens
	if event.Type != "" {
		token := fmt.Sprintf("[%s]", strings.ToUpper(event.Type))
		if handled, shouldContinue := sp.handler.HandleSpecialTokens(token, ch); handled {
			if !shouldContinue {
				return true
			}
		}
	}

	// Handle usage information
	if event.Usage != nil {
		usage := sp.handler.FormatUsage(event.Usage)
		if usage != "" {
			select {
			case <-ctx.Done():
				return true
			case ch <- engine.TokenOrToolCall{Token: usage}:
			}
		}
	}

	// Handle errors
	if event.Error != nil {
		select {
		case <-ctx.Done():
			return true
		case ch <- engine.TokenOrToolCall{Token: fmt.Sprintf("Error: %s", event.Error.Message)}:
			return true
		}
	}

	// Check if we should emit tool calls
	finishReason := sp.extractFinishReason(event)
	if sp.handler.ShouldEmitToolCall(finishReason, partials) {
		sp.emitToolCalls(ctx, partials, ch, toolCallReceived)
	}

	return false
}

// Helper methods

func (sp *StreamProcessor) createScanner(body io.Reader) *bufio.Scanner {
	base := &BaseAdapter{}
	return base.CreateScanner(body)
}

func (sp *StreamProcessor) debugf(format string, args ...interface{}) {
	if sp.debug {
		fmt.Printf("[stream] "+format+"\n", args...)
	}
}

func (sp *StreamProcessor) getContent(delta *Delta) string {
	if delta.Content != "" {
		return delta.Content
	}
	return delta.Text
}

func (sp *StreamProcessor) handleToolCallDeltas(toolCalls []struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function,omitempty"`
}, partials map[int]*PartialCall, lastItemID *string) {
	for _, tc := range toolCalls {
		idx := tc.Index
		p, ok := partials[idx]
		if !ok {
			p = &PartialCall{Index: idx}
			partials[idx] = p
		}

		if tc.ID != "" {
			p.ID = tc.ID
			*lastItemID = tc.ID
		}
		if tc.Function.Name != "" {
			p.Name = tc.Function.Name
		}
		if tc.Function.Arguments != "" {
			p.Args += tc.Function.Arguments
		}
	}
}

func (sp *StreamProcessor) extractFinishReason(event *StreamEvent) string {
	if event.Raw != nil {
		if choices, ok := event.Raw["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if reason, ok := choice["finish_reason"].(string); ok {
					return reason
				}
			}
		}
	}
	return ""
}

func (sp *StreamProcessor) emitToolCalls(
	ctx context.Context,
	partials map[int]*PartialCall,
	ch chan<- engine.TokenOrToolCall,
	toolCallReceived *bool,
) {
	// Find the tool call with the lowest index (for consistency)
	var chosen *PartialCall
	var chosenIdx int
	for idx, pc := range partials {
		if chosen == nil || idx < chosenIdx {
			chosen = pc
			chosenIdx = idx
		}
	}

	if chosen != nil && sp.isValidToolCall(chosen) {
		// Validate and emit the tool call
		var argsMap map[string]interface{}
		argsStr := strings.TrimSpace(chosen.Args)
		if argsStr == "" {
			argsStr = "{}"
		}

		if err := json.Unmarshal([]byte(argsStr), &argsMap); err == nil {
			if ValidateToolArgs(chosen.Name, argsMap) {
				if args, err := json.Marshal(argsMap); err == nil {
					*toolCallReceived = true
					id := chosen.ID
					if id == "" {
						id = fmt.Sprintf("idx_%d", chosenIdx)
					}

					call := &engine.ToolCall{ID: id, Name: chosen.Name, Args: args}
					select {
					case <-ctx.Done():
						return
					case ch <- engine.TokenOrToolCall{ToolCall: call}:
					}
				}
			}
		}
	}
}

func (sp *StreamProcessor) isValidToolCall(pc *PartialCall) bool {
	return strings.TrimSpace(pc.Name) != "" && strings.TrimSpace(pc.Args) != ""
}

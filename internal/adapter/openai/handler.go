package openai

import (
	"encoding/json"
	"fmt"

	"github.com/loom/loom/internal/adapter/common"
	"github.com/loom/loom/internal/engine"
)

// Handler implements the StreamHandler interface for OpenAI-specific streaming logic
type Handler struct {
	model string
	debug bool
}

// NewHandler creates a new OpenAI stream handler
func NewHandler(model string, debug bool) *Handler {
	return &Handler{
		model: model,
		debug: debug,
	}
}

// ParseLine parses a single SSE line and returns a StreamEvent
func (h *Handler) ParseLine(currentEvent, data string) (*common.StreamEvent, error) {
	if data == "" {
		return nil, nil
	}

	// Parse the JSON delta for OpenAI format
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
		Usage *struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage,omitempty"`
	}

	if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
		return nil, err
	}

	if len(streamResp.Choices) == 0 {
		return nil, nil
	}

	choice := streamResp.Choices[0]
	event := &common.StreamEvent{
		Raw: map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"finish_reason": choice.FinishReason,
				},
			},
		},
	}

	// Handle content delta
	if choice.Delta.Content != "" {
		event.Delta = &common.Delta{
			Content: choice.Delta.Content,
		}
	}

	// Handle tool call deltas
	if len(choice.Delta.ToolCalls) > 0 {
		event.Delta = &common.Delta{
			ToolCalls: make([]struct {
				Index    int    `json:"index"`
				ID       string `json:"id,omitempty"`
				Type     string `json:"type,omitempty"`
				Function struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				} `json:"function,omitempty"`
			}, len(choice.Delta.ToolCalls)),
		}

		for i, tc := range choice.Delta.ToolCalls {
			event.Delta.ToolCalls[i] = struct {
				Index    int    `json:"index"`
				ID       string `json:"id,omitempty"`
				Type     string `json:"type,omitempty"`
				Function struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				} `json:"function,omitempty"`
			}{
				Index: tc.Index,
				ID:    tc.ID,
				Type:  tc.Type,
				Function: struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				}{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	// Handle usage information
	if streamResp.Usage != nil {
		event.Usage = &common.UsageInfo{
			PromptTokens:     streamResp.Usage.PromptTokens,
			CompletionTokens: streamResp.Usage.CompletionTokens,
			TotalTokens:      streamResp.Usage.TotalTokens,
		}
	}

	return event, nil
}

// HandleSpecialTokens processes provider-specific special tokens
func (h *Handler) HandleSpecialTokens(token string, ch chan<- engine.TokenOrToolCall) (bool, bool) {
	// OpenAI doesn't have special tokens beyond standard ones
	return false, true
}

// FormatUsage formats usage information as a token
func (h *Handler) FormatUsage(usage *common.UsageInfo) string {
	if usage == nil {
		return ""
	}

	inTokens := usage.PromptTokens
	outTokens := usage.CompletionTokens
	if inTokens == 0 {
		inTokens = usage.InputTokens
	}
	if outTokens == 0 {
		outTokens = usage.OutputTokens
	}

	return fmt.Sprintf("[USAGE] provider=openai model=%s in=%d out=%d",
		h.model, inTokens, outTokens)
}

// ShouldEmitToolCall determines if a tool call should be emitted now
func (h *Handler) ShouldEmitToolCall(finishReason string, partials map[int]*common.PartialCall) bool {
	return finishReason == "tool_calls" && len(partials) > 0
}

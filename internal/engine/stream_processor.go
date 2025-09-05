package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/loom/loom/internal/config"
	"github.com/loom/loom/internal/memory"
	"github.com/loom/loom/internal/tool"
)

// StreamResult represents the result of processing a stream.
type StreamResult struct {
	Content            string
	ToolCall           *tool.ToolCall
	StreamEnded        bool
	ReasoningProcessed bool
}

// StreamProcessor handles the complex stream processing logic.
type StreamProcessor struct {
	bridge UIBridge
	memory *memory.Project
}

// NewStreamProcessor creates a new stream processor.
func NewStreamProcessor(bridge UIBridge, memory *memory.Project) *StreamProcessor {
	return &StreamProcessor{
		bridge: bridge,
		memory: memory,
	}
}

// ProcessStream processes a stream of tokens and tool calls from the LLM.
func (sp *StreamProcessor) ProcessStream(
	ctx context.Context,
	stream <-chan TokenOrToolCall,
	convo *memory.Conversation,
) *StreamResult {
	var currentContent string
	var toolCallReceived *tool.ToolCall
	streamEnded := false
	reasoningAccumulated := false

	// Process the stream; if slow, emit a one-time notice but do not break
	slowTicker := time.NewTicker(20 * time.Second)
	defer slowTicker.Stop()
	slowNotified := false

StreamLoop:
	for {
		select {
		case <-ctx.Done():
			return &StreamResult{
				Content:            currentContent,
				ToolCall:           toolCallReceived,
				StreamEnded:        true,
				ReasoningProcessed: reasoningAccumulated,
			}
		case <-slowTicker.C:
			if !slowNotified {
				// sp.bridge.SendChat("system", "Still working...")
				slowNotified = true
			}
		case item, ok := <-stream:
			if !ok {
				// Stream ended
				streamEnded = true
				break StreamLoop
			}

			if item.ToolCall != nil {
				toolCallReceived = sp.processToolCall(ctx, item.ToolCall, convo)
				continue
			}

			// Got a token
			tok := item.Token
			if processed := sp.processSpecialToken(tok, convo, &reasoningAccumulated); processed {
				continue
			}

			currentContent += tok
			sp.bridge.EmitAssistant(currentContent)
		}
	}

	return &StreamResult{
		Content:            currentContent,
		ToolCall:           toolCallReceived,
		StreamEnded:        streamEnded,
		ReasoningProcessed: reasoningAccumulated,
	}
}

// processToolCall handles a tool call from the stream.
func (sp *StreamProcessor) processToolCall(ctx context.Context, toolCall *ToolCall, convo *memory.Conversation) *tool.ToolCall {
	// Guard against empty tool names from partial/ambiguous streams
	if toolCall.Name == "" {
		if sp.isDebugEnabled() {
			sp.bridge.SendChat("system", "[debug] Received partial tool call with empty name; continuing to read stream")
		}
		return nil
	}

	if sp.isDebugEnabled() {
		sp.bridge.SendChat("system", fmt.Sprintf("[debug] Tool call received: id=%s name=%s argsLen=%d", toolCall.ID, toolCall.Name, len(toolCall.Args)))
	}

	result := &tool.ToolCall{
		ID:   toolCall.ID,
		Name: toolCall.Name,
		Args: toolCall.Args,
	}

	// Workflow functionality removed

	// Record the assistant tool_use in conversation for Anthropic
	if convo != nil {
		convo.AddAssistantToolUse(result.Name, result.ID, string(result.Args))
	}

	return result
}

// processSpecialToken handles special tokens like usage, reasoning, etc.
// Returns true if the token was processed and should not be added to content.
func (sp *StreamProcessor) processSpecialToken(tok string, convo *memory.Conversation, reasoningAccumulated *bool) bool {
	if strings.HasPrefix(tok, "[USAGE] ") {
		sp.processUsageToken(tok)
		return true
	}

	if strings.HasPrefix(tok, "[REASONING] ") {
		text := strings.TrimPrefix(tok, "[REASONING] ")
		sp.bridge.EmitReasoning(text, false)
		*reasoningAccumulated = true
		return true
	}

	if strings.HasPrefix(tok, "[REASONING_SIGNATURE] ") {
		// Signature is captured in the final JSON event; ignore incremental signature token
		return true
	}

	if strings.HasPrefix(tok, "[REASONING_JSON] ") {
		sp.processReasoningJSON(tok, convo)
		return true
	}

	if strings.HasPrefix(tok, "[REASONING_RAW] ") {
		text := strings.TrimPrefix(tok, "[REASONING_RAW] ")
		sp.bridge.EmitReasoning(text, false)
		*reasoningAccumulated = true
		return true
	}

	if strings.HasPrefix(tok, "[REASONING_DONE] ") {
		text := strings.TrimPrefix(tok, "[REASONING_DONE] ")
		sp.finishReasoning(text, *reasoningAccumulated)
		return true
	}

	if strings.HasPrefix(tok, "[REASONING_RAW_DONE] ") {
		text := strings.TrimPrefix(tok, "[REASONING_RAW_DONE] ")
		sp.finishReasoning(text, *reasoningAccumulated)
		return true
	}

	return false
}

// processUsageToken handles usage tokens and emits billing events.
func (sp *StreamProcessor) processUsageToken(tok string) {
	// Parse provider/model/in/out from token and emit billing event
	// Format: [USAGE] provider=xxx model=yyy in=N out=M
	usage := strings.TrimPrefix(tok, "[USAGE] ")
	var provider, model string
	var inTok, outTok int64
	fields := strings.Fields(usage)
	for _, f := range fields {
		if strings.HasPrefix(f, "provider=") {
			provider = strings.TrimPrefix(f, "provider=")
		} else if strings.HasPrefix(f, "model=") {
			model = strings.TrimPrefix(f, "model=")
		} else if strings.HasPrefix(f, "in=") {
			if v, err := strconv.ParseInt(strings.TrimPrefix(f, "in="), 10, 64); err == nil {
				inTok = v
			}
		} else if strings.HasPrefix(f, "out=") {
			if v, err := strconv.ParseInt(strings.TrimPrefix(f, "out="), 10, 64); err == nil {
				outTok = v
			}
		}
	}

	// Compute costs via config table
	inUSD, outUSD, totalUSD := config.CostUSDParts(model, inTok, outTok)
	if sp.bridge != nil {
		sp.bridge.EmitBilling(provider, model, inTok, outTok, inUSD, outUSD, totalUSD)
	}

	// Persist usage to project memory per workspace and to global store
	if sp.memory != nil {
		_ = sp.memory.AddUsage(provider, model, inTok, outTok, inUSD, outUSD)
	}
	_ = config.AddGlobalUsage(provider, model, inTok, outTok, inUSD, outUSD)
}

// processReasoningJSON handles reasoning JSON tokens.
func (sp *StreamProcessor) processReasoningJSON(tok string, convo *memory.Conversation) {
	raw := strings.TrimPrefix(tok, "[REASONING_JSON] ")
	// Persist the full JSON so adapter can replay signature
	if convo != nil {
		// Try to parse and store; if parse fails, fall back to plain
		var tmp map[string]string
		if json.Unmarshal([]byte(raw), &tmp) == nil {
			convo.AddAssistantThinkingSigned(tmp["thinking"], tmp["signature"])
		}
	}
}

// finishReasoning completes the reasoning process.
func (sp *StreamProcessor) finishReasoning(text string, reasoningAccumulated bool) {
	if reasoningAccumulated {
		sp.bridge.EmitReasoning("", true)
	} else if strings.TrimSpace(text) != "" {
		sp.bridge.EmitReasoning(text, true)
	} else {
		sp.bridge.EmitReasoning("", true)
	}
}

// isDebugEnabled checks if debug mode is enabled.
func (sp *StreamProcessor) isDebugEnabled() bool {
	return os.Getenv("LOOM_DEBUG_ENGINE") == "1" || strings.EqualFold(os.Getenv("LOOM_DEBUG_ENGINE"), "true")
}

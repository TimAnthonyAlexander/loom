package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/loom/loom/internal/engine"
	"github.com/loom/loom/internal/memory"
	"github.com/loom/loom/internal/tool"
)

// StreamProcessor handles LLM streaming interactions
type StreamProcessor struct {
	llm          engine.LLM
	uiEmitter    *UIEmitter
	toolExecutor *ToolExecutor
	debug        bool
}

// NewStreamProcessor creates a new stream processor
func NewStreamProcessor(
	llm engine.LLM,
	uiEmitter *UIEmitter,
	toolExecutor *ToolExecutor,
	debug bool,
) *StreamProcessor {
	return &StreamProcessor{
		llm:          llm,
		uiEmitter:    uiEmitter,
		toolExecutor: toolExecutor,
		debug:        debug,
	}
}

// ProcessResult represents the result of stream processing
type ProcessResult struct {
	ContentReceived   bool
	ToolCallReceived  bool
	ShouldReturnEarly bool
}

// ProcessLLMStream handles the complete LLM streaming interaction
func (sp *StreamProcessor) ProcessLLMStream(
	ctx context.Context,
	messages []engine.Message,
	tools []engine.ToolSchema,
	stream bool,
	convo *memory.Conversation,
) (*ProcessResult, error) {
	// Call the LLM with the conversation history
	llmStream, err := sp.llm.Chat(ctx, messages, tools, stream)
	if err != nil {
		sp.uiEmitter.SendSystemMessage("Error: " + err.Error())
		return nil, err
	}

	// Process the LLM response
	var currentContent string
	var toolCallReceived *tool.ToolCall
	streamEnded := false

	// Process the stream with timeout handling
	slowTicker := time.NewTicker(20 * time.Second)
	defer slowTicker.Stop()
	slowNotified := false

StreamLoop:
	for {
		select {
		case <-ctx.Done():
			// Send cancellation message to UI
			sp.uiEmitter.SendSystemMessage("Operation stopped by user.")
			return nil, ctx.Err()
		case <-slowTicker.C:
			if !slowNotified {
				// Could notify about slow response if needed
				slowNotified = true
			}
		case item, ok := <-llmStream:
			if !ok {
				// Stream ended
				streamEnded = true
				break StreamLoop
			}

			if item.ToolCall != nil {
				// Got a tool call; record it but continue reading the stream until completion
				if item.ToolCall.Name == "" {
					if sp.debug {
						sp.uiEmitter.SendSystemMessage("[debug] Received partial tool call with empty name; continuing to read stream")
					}
					continue
				}
				if toolCallReceived == nil {
					if sp.debug {
						sp.uiEmitter.SendSystemMessage(fmt.Sprintf("[debug] Tool call received: id=%s name=%s argsLen=%d", item.ToolCall.ID, item.ToolCall.Name, len(item.ToolCall.Args)))
					}
					toolCallReceived = &tool.ToolCall{
						ID:   item.ToolCall.ID,
						Name: item.ToolCall.Name,
						Args: item.ToolCall.Args,
					}
				}
				continue
			}

			// Got a token - process it
			tok := item.Token
			handled, _ := sp.uiEmitter.ProcessToken(tok)
			if !handled {
				// Regular content token
				currentContent += tok
				sp.uiEmitter.EmitContent(currentContent)
			}
		}
	}

	result := &ProcessResult{
		ContentReceived:  currentContent != "",
		ToolCallReceived: toolCallReceived != nil,
	}

	// If we got a tool call, execute it
	if toolCallReceived != nil {
		result.ToolCallReceived = true
		_, err := sp.toolExecutor.ExecuteToolCall(ctx, toolCallReceived, convo)
		if err != nil {
			return result, err
		}
		// Continue the loop to get the next assistant message
		return result, nil
	}

	// If we reach here with content but no tool call, record it
	if currentContent != "" {
		if convo != nil {
			convo.AddAssistant(currentContent)
		}
		return result, nil
	}

	// If stream ended with no content and no tool call, try non-streaming retry
	if streamEnded {
		return sp.handleEmptyStreamRetry(ctx, messages, tools, convo)
	}

	return result, nil
}

// handleEmptyStreamRetry handles the fallback non-streaming retry
func (sp *StreamProcessor) handleEmptyStreamRetry(
	ctx context.Context,
	messages []engine.Message,
	tools []engine.ToolSchema,
	convo *memory.Conversation,
) (*ProcessResult, error) {
	sp.uiEmitter.SendSystemMessage("Retrying without streaming...")

	fallbackStream, err := sp.llm.Chat(ctx, messages, tools, false)
	if err != nil {
		sp.uiEmitter.SendSystemMessage("Error: " + err.Error())
		return nil, err
	}

	var currentContent string
	var toolCallReceived *tool.ToolCall

	// Collect the single-shot response
	for item := range fallbackStream {
		if item.ToolCall != nil {
			toolCallReceived = &tool.ToolCall{
				ID:   item.ToolCall.ID,
				Name: item.ToolCall.Name,
				Args: item.ToolCall.Args,
			}
			if sp.debug {
				sp.uiEmitter.SendSystemMessage(fmt.Sprintf("[debug] Non-stream tool call received: id=%s name=%s argsLen=%d", item.ToolCall.ID, item.ToolCall.Name, len(item.ToolCall.Args)))
			}
			if convo != nil {
				convo.AddAssistantToolUse(toolCallReceived.Name, toolCallReceived.ID, string(toolCallReceived.Args))
			}
			break
		}
		if item.Token != "" {
			currentContent += item.Token
		}
	}

	result := &ProcessResult{
		ContentReceived:  currentContent != "",
		ToolCallReceived: toolCallReceived != nil,
	}

	if toolCallReceived != nil {
		// Execute the tool and continue
		_, err := sp.toolExecutor.ExecuteToolCall(ctx, toolCallReceived, convo)
		if err != nil {
			return result, err
		}
		return result, nil
	}

	if currentContent != "" {
		if convo != nil {
			convo.AddAssistant(currentContent)
		}
		sp.uiEmitter.EmitContent(currentContent)
		return result, nil
	}

	// Still nothing
	if sp.debug {
		sp.uiEmitter.SendSystemMessage("[debug] Fallback non-stream returned no content and no tool calls")
	}
	sp.uiEmitter.SendSystemMessage("No response from model.")
	return result, nil
}

// Debugf logs debug messages if debug mode is enabled
func (sp *StreamProcessor) Debugf(format string, args ...interface{}) {
	if sp.debug {
		fmt.Printf("[stream] "+format+"\n", args...)
	}
}

package tokens

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/loom/loom/internal/config"
	"github.com/loom/loom/internal/engine"
	"github.com/loom/loom/internal/memory"
)

// TokenProcessor handles all special token processing in one place
// This eliminates 90+ lines of duplicated token processing logic
type TokenProcessor struct {
	bridge engine.UIBridge
	memory *memory.Project
	debug  bool
}

// NewTokenProcessor creates a new unified token processor
func NewTokenProcessor(bridge engine.UIBridge, memory *memory.Project, debug bool) *TokenProcessor {
	return &TokenProcessor{
		bridge: bridge,
		memory: memory,
		debug:  debug,
	}
}

// ProcessTokenResult represents the result of token processing
type ProcessTokenResult struct {
	Handled              bool   // Whether the token was handled
	ShouldContinue       bool   // Whether to continue processing
	ReasoningAccumulated *bool  // Pointer to reasoning state (can be nil)
	AssistantContent     string // Content to add to assistant message
}

// ProcessToken handles all special token types and returns processing result
func (tp *TokenProcessor) ProcessToken(token string, convo *memory.Conversation) *ProcessTokenResult {
	result := &ProcessTokenResult{
		Handled:        false,
		ShouldContinue: true,
	}

	// Handle usage tokens
	if strings.HasPrefix(token, "[USAGE] ") {
		tp.handleUsageToken(token)
		result.Handled = true
		return result
	}

	// Handle reasoning tokens
	if strings.HasPrefix(token, "[REASONING] ") {
		text := strings.TrimPrefix(token, "[REASONING] ")
		if tp.bridge != nil {
			tp.bridge.EmitReasoning(text, false)
		}
		reasoningState := true
		result.ReasoningAccumulated = &reasoningState
		result.Handled = true
		return result
	}

	// Handle reasoning signature (ignore incremental)
	if strings.HasPrefix(token, "[REASONING_SIGNATURE] ") {
		result.Handled = true
		return result
	}

	// Handle reasoning JSON
	if strings.HasPrefix(token, "[REASONING_JSON] ") {
		raw := strings.TrimPrefix(token, "[REASONING_JSON] ")
		tp.handleReasoningJSON(raw, convo)
		result.Handled = true
		return result
	}

	// Handle raw reasoning tokens
	if strings.HasPrefix(token, "[REASONING_RAW] ") {
		text := strings.TrimPrefix(token, "[REASONING_RAW] ")
		if tp.bridge != nil {
			tp.bridge.EmitReasoning(text, false)
		}
		reasoningState := true
		result.ReasoningAccumulated = &reasoningState
		result.Handled = true
		return result
	}

	// Handle reasoning done tokens
	if strings.HasPrefix(token, "[REASONING_DONE] ") {
		tp.handleReasoningDone(token)
		result.Handled = true
		return result
	}

	// Handle raw reasoning done tokens
	if strings.HasPrefix(token, "[REASONING_RAW_DONE] ") {
		tp.handleReasoningDone(token)
		result.Handled = true
		return result
	}

	// Regular content token - not handled here
	result.AssistantContent = token
	return result
}

// handleUsageToken processes usage tokens and emits billing information
func (tp *TokenProcessor) handleUsageToken(token string) {
	// Parse provider/model/in/out from token and emit billing event
	// Format: [USAGE] provider=xxx model=yyy in=N out=M
	usage := strings.TrimPrefix(token, "[USAGE] ")
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
	if tp.bridge != nil {
		tp.bridge.EmitBilling(provider, model, inTok, outTok, inUSD, outUSD, totalUSD)
	}

	// Persist usage to project memory per workspace and to global store
	if tp.memory != nil {
		_ = tp.memory.AddUsage(provider, model, inTok, outTok, inUSD, outUSD)
	}
	_ = config.AddGlobalUsage(provider, model, inTok, outTok, inUSD, outUSD)
}

// handleReasoningJSON processes reasoning JSON tokens
func (tp *TokenProcessor) handleReasoningJSON(raw string, convo *memory.Conversation) {
	// Persist the full JSON so adapter can replay signature
	if convo != nil {
		// Try to parse and store; if parse fails, fall back to plain
		var tmp map[string]string
		if json.Unmarshal([]byte(raw), &tmp) == nil {
			convo.AddAssistantThinkingSigned(tmp["thinking"], tmp["signature"])
		}
	}
}

// handleReasoningDone processes reasoning done tokens
func (tp *TokenProcessor) handleReasoningDone(token string) {
	var text string
	if strings.HasPrefix(token, "[REASONING_DONE] ") {
		text = strings.TrimPrefix(token, "[REASONING_DONE] ")
	} else if strings.HasPrefix(token, "[REASONING_RAW_DONE] ") {
		text = strings.TrimPrefix(token, "[REASONING_RAW_DONE] ")
	}

	if tp.bridge != nil {
		if strings.TrimSpace(text) != "" {
			tp.bridge.EmitReasoning(text, true)
		} else {
			tp.bridge.EmitReasoning("", true)
		}
	}
}

// SetReasoningAccumulated handles reasoning accumulation state
func (tp *TokenProcessor) SetReasoningAccumulated(accumulated bool) {
	// This can be used by callers to track reasoning state
	if tp.debug {
		// Debug logging if needed
	}
}

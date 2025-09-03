package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/loom/loom/internal/config"
	"github.com/loom/loom/internal/engine"
	"github.com/loom/loom/internal/memory"
)

// UIEmitter handles all UI interactions and token processing
type UIEmitter struct {
	bridge    engine.UIBridge
	memory    *memory.Project
	modelName string
	debug     bool
}

// NewUIEmitter creates a new UI emitter
func NewUIEmitter(bridge engine.UIBridge, memory *memory.Project, modelName string, debug bool) *UIEmitter {
	return &UIEmitter{
		bridge:    bridge,
		memory:    memory,
		modelName: modelName,
		debug:     debug,
	}
}

// ProcessToken handles different types of tokens from the LLM stream
func (ue *UIEmitter) ProcessToken(token string) (shouldContinue bool, reasoningAccumulated *bool) {
	if ue.bridge == nil {
		return true, reasoningAccumulated
	}

	// Handle usage tokens
	if strings.HasPrefix(token, "[USAGE] ") {
		ue.handleUsageToken(token)
		return true, reasoningAccumulated
	}

	// Handle reasoning tokens
	if strings.HasPrefix(token, "[REASONING] ") {
		text := strings.TrimPrefix(token, "[REASONING] ")
		ue.bridge.EmitReasoning(text, false)
		if reasoningAccumulated != nil {
			*reasoningAccumulated = true
		}
		return true, reasoningAccumulated
	}

	// Handle reasoning signature (ignore incremental)
	if strings.HasPrefix(token, "[REASONING_SIGNATURE] ") {
		return true, reasoningAccumulated
	}

	// Handle reasoning JSON
	if strings.HasPrefix(token, "[REASONING_JSON] ") {
		// This is handled by the conversation handler for persistence
		return true, reasoningAccumulated
	}

	// Handle raw reasoning tokens
	if strings.HasPrefix(token, "[REASONING_RAW] ") {
		text := strings.TrimPrefix(token, "[REASONING_RAW] ")
		ue.bridge.EmitReasoning(text, false)
		if reasoningAccumulated != nil {
			*reasoningAccumulated = true
		}
		return true, reasoningAccumulated
	}

	// Handle reasoning done tokens
	if strings.HasPrefix(token, "[REASONING_DONE] ") {
		text := strings.TrimPrefix(token, "[REASONING_DONE] ")
		if reasoningAccumulated != nil && *reasoningAccumulated {
			ue.bridge.EmitReasoning("", true)
		} else if strings.TrimSpace(text) != "" {
			ue.bridge.EmitReasoning(text, true)
		} else {
			ue.bridge.EmitReasoning("", true)
		}
		return true, reasoningAccumulated
	}

	// Handle raw reasoning done tokens
	if strings.HasPrefix(token, "[REASONING_RAW_DONE] ") {
		text := strings.TrimPrefix(token, "[REASONING_RAW_DONE] ")
		if reasoningAccumulated != nil && *reasoningAccumulated {
			ue.bridge.EmitReasoning("", true)
		} else if strings.TrimSpace(text) != "" {
			ue.bridge.EmitReasoning(text, true)
		} else {
			ue.bridge.EmitReasoning("", true)
		}
		return true, reasoningAccumulated
	}

	return false, reasoningAccumulated // Not handled, should be processed as regular content
}

// EmitContent emits regular content to the UI
func (ue *UIEmitter) EmitContent(content string) {
	if ue.bridge != nil {
		ue.bridge.EmitAssistant(content)
	}
}

// SendSystemMessage sends a system message to the UI
func (ue *UIEmitter) SendSystemMessage(message string) {
	if ue.bridge != nil {
		ue.bridge.SendChat("system", message)
	}
}

// SendToolResult sends a tool result to the UI
func (ue *UIEmitter) SendToolResult(content string) {
	if ue.bridge != nil && strings.TrimSpace(content) != "" {
		ue.bridge.SendChat("tool", content)
	}
}

// SetBusy updates the busy state in the UI
func (ue *UIEmitter) SetBusy(busy bool) {
	if ue.bridge != nil {
		ue.bridge.SetBusy(busy)
	}
}

// handleUsageToken processes usage tokens and emits billing information
func (ue *UIEmitter) handleUsageToken(token string) {
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
	if ue.bridge != nil {
		ue.bridge.EmitBilling(provider, model, inTok, outTok, inUSD, outUSD, totalUSD)
	}

	// Persist usage to project memory per workspace and to global store
	if ue.memory != nil {
		_ = ue.memory.AddUsage(provider, model, inTok, outTok, inUSD, outUSD)
	}
	_ = config.AddGlobalUsage(provider, model, inTok, outTok, inUSD, outUSD)
}

// Debugf logs debug messages if debug mode is enabled
func (ue *UIEmitter) Debugf(format string, args ...interface{}) {
	if ue.debug {
		fmt.Printf("[ui] "+format+"\n", args...)
	}
}

package bridge

import (
	"context"
	"log"
	"os"
	"runtime/debug"

	"github.com/loom/loom/internal/adapter"
	"github.com/loom/loom/internal/engine"
	"github.com/loom/loom/internal/tool"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main structure for the wails UI bridge.
type App struct {
	engine *engine.Engine
	tools  *tool.Registry
	config adapter.Config
	ctx    context.Context
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// WithEngine connects the engine to the UI bridge.
func (a *App) WithEngine(eng *engine.Engine) *App {
	a.engine = eng
	return a
}

// WithTools connects the tool registry to the UI bridge.
func (a *App) WithTools(tools *tool.Registry) *App {
	a.tools = tools
	return a
}

// WithConfig sets the configuration for the UI bridge.
func (a *App) WithConfig(config adapter.Config) *App {
	a.config = config
	return a
}

// WithContext sets the Wails context for the UI bridge.
func (a *App) WithContext(ctx context.Context) *App {
	a.ctx = ctx
	return a
}

// SendUserMessage sends a user message to the engine for processing.
func (a *App) SendUserMessage(message string) {
	if a.engine != nil {
		a.engine.Enqueue(message)
	} else {
		log.Println("Engine not initialized")
	}
}

// Approve resolves an approval request with the given decision.
func (a *App) Approve(id string, approved bool) {
	if a.engine != nil {
		a.engine.ResolveApproval(id, approved)
	} else {
		log.Println("Engine not initialized")
	}
}

// GetTools returns a list of available tools.
func (a *App) GetTools() []map[string]interface{} {
	if a.tools == nil {
		return []map[string]interface{}{}
	}

	schemas := a.tools.Schemas()
	result := make([]map[string]interface{}, len(schemas))

	for i, schema := range schemas {
		// Convert tool.Schema to a map
		toolInfo := map[string]interface{}{
			"name":        schema.Name,
			"description": schema.Description,
			"safe":        schema.Safe,
			"schema":      schema.Parameters,
		}
		result[i] = toolInfo
	}

	return result
}

// SetModel updates the model selection.
func (a *App) SetModel(model string) {
	// Parse the model string to get provider and model ID
	provider, modelID, err := adapter.GetProviderFromModel(model)
	if err != nil {
		log.Printf("Failed to parse model string: %v", err)
		return
	}

	// Read proper API key based on the provider
	var apiKey string
	switch provider {
	case adapter.ProviderOpenAI:
		apiKey = os.Getenv("OPENAI_API_KEY")
	case adapter.ProviderAnthropic:
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	default:
		apiKey = a.config.APIKey // Keep existing key for other providers like Ollama
	}

	// Update the configuration
	newConfig := adapter.Config{
		Provider: provider,
		Model:    modelID,
		APIKey:   apiKey,
		Endpoint: a.config.Endpoint,
	}

	// Log the model change for debugging
	log.Printf("Switching model to %s:%s with API key %s...", provider, modelID, apiKey[:min(10, len(apiKey))])

	// Create a new LLM adapter with the updated model
	llm, err := adapter.New(newConfig)
	if err != nil {
		log.Printf("Failed to create new LLM adapter: %v", err)
		return
	}

	// Update the engine with the new LLM
	if a.engine != nil {
		a.engine.SetLLM(llm)
		// Update stored config
		a.config = newConfig
	} else {
		log.Println("Engine not initialized")
	}
}

// SendChat emits a chat message to the UI.
func (a *App) SendChat(role, text string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in SendChat: %v\n%s", r, debug.Stack())
		}
	}()

	// Create a chat message
	message := map[string]interface{}{
		"role":    role,
		"content": text,
	}

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "chat:new", message)
	} else {
		log.Println("Warning: Wails context not initialized in SendChat")
	}
}

// EmitAssistant sends partial assistant tokens to the UI.
func (a *App) EmitAssistant(text string) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "assistant-msg", text)
	} else {
		log.Println("Warning: Wails context not initialized in EmitAssistant")
	}
}

// PromptApproval asks the user for approval of an action.
func (a *App) PromptApproval(actionID, summary, diff string) bool {
	// Create an approval request
	request := map[string]string{
		"id":      actionID,
		"summary": summary,
		"diff":    diff,
	}

	// Send the request to the UI
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "task:prompt", request)
	} else {
		log.Println("Warning: Wails context not initialized in PromptApproval")
	}

	// The actual approval will come back via the Approve method
	return false // Placeholder return, actual approval handled asynchronously
}

// min returns the smaller of a and b
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

package bridge

import (
	"context"
	"log"
	"os"
	"runtime/debug"

	"github.com/loom/loom/internal/adapter"
	"github.com/loom/loom/internal/config"
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
	busy   bool
	// persisted settings (API keys, endpoints)
	settings config.Settings
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

// WithSettings sets persisted settings for the UI bridge.
func (a *App) WithSettings(s config.Settings) *App {
	a.settings = s
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

	// Determine API key based on provider, preferring persisted settings, then env
	var apiKey string
	switch provider {
	case adapter.ProviderOpenAI:
		if a.settings.OpenAIAPIKey != "" {
			apiKey = a.settings.OpenAIAPIKey
		} else {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
	case adapter.ProviderAnthropic:
		if a.settings.AnthropicAPIKey != "" {
			apiKey = a.settings.AnthropicAPIKey
		} else {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
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

// ensureSettingsLoaded loads settings from disk into memory if not already loaded.
func (a *App) ensureSettingsLoaded() {
	if a.settings != (config.Settings{}) {
		return
	}
	if s, err := config.Load(); err == nil {
		a.settings = s
	}
}

// applyAndSaveSettings persists the provided settings and applies them to the current engine if applicable.
func (a *App) applyAndSaveSettings(s config.Settings) {
	// Persist to disk
	if err := config.Save(s); err != nil {
		log.Printf("Failed to save settings: %v", err)
		return
	}

	// Update in-memory settings
	a.settings = s

	// If current provider uses one of these keys, update config and LLM
	var updatedConfig = a.config
	switch a.config.Provider {
	case adapter.ProviderOpenAI:
		if s.OpenAIAPIKey != "" {
			updatedConfig.APIKey = s.OpenAIAPIKey
		}
	case adapter.ProviderAnthropic:
		if s.AnthropicAPIKey != "" {
			updatedConfig.APIKey = s.AnthropicAPIKey
		}
	case adapter.ProviderOllama:
		if s.OllamaEndpoint != "" {
			updatedConfig.Endpoint = s.OllamaEndpoint
		}
	}

	// Recreate LLM if config changed materially
	if updatedConfig != a.config {
		llm, err := adapter.New(updatedConfig)
		if err != nil {
			log.Printf("Failed to apply new settings to LLM: %v", err)
			return
		}
		if a.engine != nil {
			a.engine.SetLLM(llm)
			a.config = updatedConfig
		}
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

// GetSettings exposes persisted settings to the frontend.
func (a *App) GetSettings() map[string]string {
	a.ensureSettingsLoaded()
	s := a.settings
	return map[string]string{
		"openai_api_key":    s.OpenAIAPIKey,
		"anthropic_api_key": s.AnthropicAPIKey,
		"ollama_endpoint":   s.OllamaEndpoint,
	}
}

// SaveSettings saves settings provided by the frontend.
func (a *App) SaveSettings(settings map[string]string) {
	s := config.Settings{
		OpenAIAPIKey:    settings["openai_api_key"],
		AnthropicAPIKey: settings["anthropic_api_key"],
		OllamaEndpoint:  settings["ollama_endpoint"],
	}
	a.applyAndSaveSettings(s)
}

// GetRules exposes user and project rules to the frontend.
func (a *App) GetRules() map[string][]string {
	user, project, _ := config.LoadRules(a.engine.Workspace())
	return map[string][]string{
		"user":    user,
		"project": project,
	}
}

// SaveRules persists rules coming from the frontend. The payload is
// { user: string[], project: string[] }.
func (a *App) SaveRules(payload map[string][]string) {
	// Save user rules
	if userRules, ok := payload["user"]; ok {
		if err := config.SaveUserRules(userRules); err != nil {
			log.Printf("Failed to save user rules: %v", err)
		}
	}
	// Save project rules
	if projectRules, ok := payload["project"]; ok {
		wp := ""
		if a.engine != nil {
			wp = a.engine.Workspace()
		}
		if wp == "" {
			log.Printf("Cannot save project rules: workspace not set")
		} else if err := config.SaveProjectRules(wp, projectRules); err != nil {
			log.Printf("Failed to save project rules: %v", err)
		}
	}
}

// SetBusy updates the busy state and notifies the frontend to enable/disable inputs
func (a *App) SetBusy(isBusy bool) {
	a.busy = isBusy
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "system:busy", isBusy)
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

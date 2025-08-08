package bridge

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/loom/loom/internal/adapter"
	"github.com/loom/loom/internal/config"
	"github.com/loom/loom/internal/engine"
	"github.com/loom/loom/internal/indexer"
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
	// Apply auto-approve flags to engine if available
	if a.engine != nil {
		a.engine.SetAutoApprove(s.AutoApproveShell, s.AutoApproveEdits)
	}
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

// ClearConversation clears the current conversation in the engine/memory and emits a UI event to clear local state.
func (a *App) ClearConversation() {
	if a.engine != nil {
		a.engine.ClearConversation()
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "chat:clear")
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

	// Persist last selected model to settings
	a.ensureSettingsLoaded()
	// Compose back to provider-prefixed model string for persistence
	var providerPrefix string
	switch provider {
	case adapter.ProviderOpenAI:
		providerPrefix = "openai"
	case adapter.ProviderAnthropic:
		providerPrefix = "claude"
	case adapter.ProviderOllama:
		providerPrefix = "ollama"
	default:
		providerPrefix = string(provider)
	}
	a.settings.LastModel = providerPrefix + ":" + modelID
	if err := config.Save(a.settings); err != nil {
		log.Printf("Failed to persist last model: %v", err)
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

	// Apply engine flags for auto-approve regardless of LLM update
	if a.engine != nil {
		a.engine.SetAutoApprove(s.AutoApproveShell, s.AutoApproveEdits)
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
	// Fallback to engine workspace if settings don't have one yet
	lastWorkspace := s.LastWorkspace
	if lastWorkspace == "" && a.engine != nil {
		lastWorkspace = a.engine.Workspace()
	}
	// Fallback API keys to environment if not persisted
	openaiKey := s.OpenAIAPIKey
	if openaiKey == "" {
		openaiKey = os.Getenv("OPENAI_API_KEY")
	}
	anthropicKey := s.AnthropicAPIKey
	if anthropicKey == "" {
		anthropicKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	return map[string]string{
		"openai_api_key":     openaiKey,
		"anthropic_api_key":  anthropicKey,
		"ollama_endpoint":    s.OllamaEndpoint,
		"last_workspace":     lastWorkspace,
		"last_model":         s.LastModel,
		"auto_approve_shell": boolToStr(s.AutoApproveShell),
		"auto_approve_edits": boolToStr(s.AutoApproveEdits),
	}
}

// SaveSettings saves settings provided by the frontend.
func (a *App) SaveSettings(settings map[string]string) {
	// Merge with existing settings to avoid wiping fields (e.g. last_workspace) when omitted by the UI
	a.ensureSettingsLoaded()
	s := a.settings

	if v, ok := settings["openai_api_key"]; ok {
		s.OpenAIAPIKey = v
	}
	if v, ok := settings["anthropic_api_key"]; ok {
		s.AnthropicAPIKey = v
	}
	if v, ok := settings["ollama_endpoint"]; ok {
		s.OllamaEndpoint = v
	}
	if v, ok := settings["last_workspace"]; ok && strings.TrimSpace(v) != "" {
		s.LastWorkspace = normalizeWorkspacePath(v)
	}
	if v, ok := settings["last_model"]; ok && v != "" {
		s.LastModel = v
	}
	if v, ok := settings["auto_approve_shell"]; ok {
		s.AutoApproveShell = strToBool(v)
	}
	if v, ok := settings["auto_approve_edits"]; ok {
		s.AutoApproveEdits = strToBool(v)
	}

	a.applyAndSaveSettings(s)
}

// SetWorkspace updates the engine and tools for a new workspace and persists it as last workspace.
func (a *App) SetWorkspace(path string) {
	if path == "" {
		return
	}
	// Normalize provided path: expand ~ and make absolute/clean
	norm := normalizeWorkspacePath(path)
	log.Printf("SetWorkspace: switching to %s (normalized: %s)", path, norm)
	// Update engine workspace
	if a.engine != nil {
		a.engine.WithWorkspace(norm)
	}
	// Re-register tools with new workspace paths
	if a.tools != nil {
		// Create a new registry to avoid stale state
		newRegistry := tool.NewRegistry().WithUI(a)
		// Re-register tools using main.registerTools equivalent
		// We cannot import main.registerTools; re-register a minimal set here
		// In this context, we expect the Registry to already contain tools registered at startup.
		// For correctness, try to re-register using the same helpers.
		// Note: we rely on tool package Register* functions.
		if err := tool.RegisterReadFile(newRegistry, norm); err != nil {
			log.Printf("Failed to register read_file tool for new workspace: %v", err)
		}
		idx := indexer.NewRipgrepIndexer(norm)
		if err := tool.RegisterSearchCode(newRegistry, idx); err != nil {
			log.Printf("Failed to register search_code tool for new workspace: %v", err)
		}
		if err := tool.RegisterEditFile(newRegistry, norm); err != nil {
			log.Printf("Failed to register edit_file tool for new workspace: %v", err)
		}
		if err := tool.RegisterApplyEdit(newRegistry, norm); err != nil {
			log.Printf("Failed to register apply_edit tool for new workspace: %v", err)
		}
		if err := tool.RegisterListDir(newRegistry, norm); err != nil {
			log.Printf("Failed to register list_dir tool for new workspace: %v", err)
		}
		if err := tool.RegisterFinalize(newRegistry); err != nil {
			log.Printf("Failed to register finalize tool for new workspace: %v", err)
		}
		if err := tool.RegisterRunShell(newRegistry, norm); err != nil {
			log.Printf("Failed to register run_shell tool for new workspace: %v", err)
		}
		if err := tool.RegisterApplyShell(newRegistry, norm); err != nil {
			log.Printf("Failed to register apply_shell tool for new workspace: %v", err)
		}
		if err := tool.RegisterHTTPRequest(newRegistry); err != nil {
			log.Printf("Failed to register http_request tool for new workspace: %v", err)
		}
		a.tools = newRegistry
		if a.engine != nil {
			a.engine.WithRegistry(newRegistry)
		}
	}
	// Persist as last workspace
	a.ensureSettingsLoaded()
	a.settings.LastWorkspace = norm
	if err := config.Save(a.settings); err != nil {
		log.Printf("Failed to persist last workspace: %v", err)
	}
	// After switching, log current rules snapshot for debug
	user, project, _ := config.LoadRules(path)
	log.Printf("SetWorkspace: loaded rules for %s -> user=%d, project=%d", path, len(user), len(project))
}

// normalizeWorkspacePath expands ~ and returns a cleaned absolute path
func normalizeWorkspacePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				p = home
			} else {
				p = filepath.Join(home, p[2:])
			}
		}
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return filepath.Clean(p)
}

// GetRules exposes user and project rules to the frontend.
func (a *App) GetRules() map[string][]string {
	ws := ""
	if a.engine != nil {
		ws = a.engine.Workspace()
	}
	user, project, _ := config.LoadRules(ws)
	// Debug log what we loaded
	payload := map[string]any{
		"workspace":           ws,
		"user_rules_count":    len(user),
		"project_rules_count": len(project),
		"user_rules":          user,
		"project_rules":       project,
	}
	if b, err := json.Marshal(payload); err == nil {
		log.Printf("GetRules: %s", string(b))
	} else {
		log.Printf("GetRules: workspace=%s user=%d project=%d", ws, len(user), len(project))
	}
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
		log.Printf("SaveRules: saving %d user rules", len(userRules))
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
			log.Printf("SaveRules: cannot save project rules: workspace not set")
		} else {
			log.Printf("SaveRules: saving %d project rules to workspace=%s", len(projectRules), wp)
			if err := config.SaveProjectRules(wp, projectRules); err != nil {
				log.Printf("Failed to save project rules: %v", err)
			}
		}
	}
}

// ChooseWorkspace opens a native directory picker and returns the selected path.
func (a *App) ChooseWorkspace() string {
	if a.ctx == nil {
		log.Printf("ChooseWorkspace: context not initialized")
		return ""
	}
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Workspace",
	})
	if err != nil {
		log.Printf("ChooseWorkspace error: %v", err)
		return ""
	}
	if dir == "" {
		return ""
	}
	return dir
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

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func strToBool(s string) bool {
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// GetConversations returns recent conversations and current id for the active workspace.
func (a *App) GetConversations() map[string]interface{} {
	result := map[string]interface{}{
		"current_id":    "",
		"conversations": []map[string]string{},
	}
	if a.engine == nil {
		return result
	}
	id := a.engine.CurrentConversationID()
	result["current_id"] = id
	summaries, err := a.engine.ListConversations()
	if err != nil {
		return result
	}
	list := make([]map[string]string, 0, len(summaries))
	for _, s := range summaries {
		list = append(list, map[string]string{
			"id":         s.ID,
			"title":      s.Title,
			"updated_at": s.UpdatedAt.Format(time.RFC3339),
		})
	}
	result["conversations"] = list
	return result
}

// LoadConversation switches to the specified conversation and emits its messages to the UI.
func (a *App) LoadConversation(id string) {
	if a.engine == nil || id == "" {
		return
	}
	if err := a.engine.SetCurrentConversationID(id); err != nil {
		log.Printf("LoadConversation: %v", err)
		return
	}
	// Clear UI then replay messages
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "chat:clear")
	}
	msgs, err := a.engine.GetConversation(id)
	if err != nil {
		log.Printf("LoadConversation: failed to get conversation %s: %v", id, err)
		return
	}
	for _, m := range msgs {
		a.SendChat(m.Role, m.Content)
	}
}

// NewConversation creates a new conversation and clears the UI.
func (a *App) NewConversation() string {
	if a.engine == nil {
		return ""
	}
	id := a.engine.NewConversation()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "chat:clear")
	}
	return id
}

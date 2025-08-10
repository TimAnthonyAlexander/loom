package bridge

import (
	"context"
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

	// Determine API key based on provider using persisted settings only
	var apiKey string
	switch provider {
	case adapter.ProviderOpenAI:
		apiKey = a.settings.OpenAIAPIKey
	case adapter.ProviderAnthropic:
		apiKey = a.settings.AnthropicAPIKey
	default:
		apiKey = a.config.APIKey // Keep existing key for other providers like Ollama
	}

	// Persist last selected model to settings immediately (even if LLM init fails)
	a.ensureSettingsLoaded()
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

	// Update the configuration
	newConfig := adapter.Config{
		Provider: provider,
		Model:    modelID,
		APIKey:   apiKey,
		Endpoint: a.config.Endpoint,
	}

	// Create a new LLM adapter with the updated model
	llm, err := adapter.New(newConfig)
	if err != nil {
		log.Printf("Failed to create new LLM adapter: %v", err)
		return
	}

	// Update the engine with the new LLM
	if a.engine != nil {
		a.engine.SetLLM(llm)
		a.config = newConfig
		a.engine.SetModelLabel(string(provider) + ":" + modelID)
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
	// Removed verbose debug logging for assistant content
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "assistant-msg", text)
	} else {
		log.Println("Warning: Wails context not initialized in EmitAssistant")
	}
}

// EmitReasoning sends reasoning text to the UI with a done flag for summaries
func (a *App) EmitReasoning(text string, done bool) {
	if a.ctx != nil {
		payload := map[string]any{
			"text": text,
			"done": done,
		}
		runtime.EventsEmit(a.ctx, "assistant-reasoning", payload)
	} else {
		log.Println("Warning: Wails context not initialized in EmitReasoning")
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
	// Do not fallback API keys to environment; only return persisted values
	openaiKey := s.OpenAIAPIKey
	anthropicKey := s.AnthropicAPIKey
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
		// Hide system and tool messages from the chat view when loading history
		if m.Role == "system" || m.Role == "tool" {
			continue
		}
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

// UIFileEntry represents a single file or directory for the UI explorer
type UIFileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"` // path relative to the workspace root
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size,omitempty"`
	ModTime string `json:"mod_time"`
}

// UIListDirResult is the response for directory listings in the UI
type UIListDirResult struct {
	Path    string        `json:"path"` // normalized relative path from workspace root
	Entries []UIFileEntry `json:"entries"`
	IsDir   bool          `json:"is_dir"`
	Error   string        `json:"error,omitempty"`
}

// ListWorkspaceDir lists files/directories within the current workspace.
// If relPath is empty or ".", the workspace root is used.
func (a *App) ListWorkspaceDir(relPath string) UIListDirResult {
	res := UIListDirResult{Path: "", Entries: []UIFileEntry{}, IsDir: true}
	// Ensure engine and workspace are available
	if a.engine == nil {
		res.Error = "engine not initialized"
		return res
	}
	root := strings.TrimSpace(a.engine.Workspace())
	if root == "" {
		res.Error = "workspace not set"
		return res
	}

	// Resolve path
	rel := strings.TrimSpace(relPath)
	if rel == "" || rel == "." || rel == "/" {
		rel = ""
	}
	// Prevent attempts to escape workspace via ..
	joined := filepath.Clean(filepath.Join(root, rel))
	// Normalize and ensure within workspace
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	// Ensure absJoined is within absRoot
	if absJoined != absRoot && !strings.HasPrefix(absJoined+string(os.PathSeparator), absRoot+string(os.PathSeparator)) {
		res.Error = "path outside workspace"
		return res
	}

	// Determine relative display path
	relDisplay, err := filepath.Rel(absRoot, absJoined)
	if err != nil || relDisplay == "." {
		relDisplay = ""
	}
	res.Path = filepath.ToSlash(relDisplay)

	fi, err := os.Stat(absJoined)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if !fi.IsDir() {
		// If a file was targeted, set IsDir=false and return no entries
		res.IsDir = false
		return res
	}

	// Read directory entries
	entries, err := os.ReadDir(absJoined)
	if err != nil {
		res.Error = err.Error()
		return res
	}

	// Build listing: show directories first, then files, both sorted by name
	var dirs, files []UIFileEntry
	for _, e := range entries {
		name := e.Name()
		// Skip .loom internal directory by default to reduce noise
		if name == ".loom" {
			continue
		}
		p := filepath.Join(relDisplay, name)
		p = filepath.ToSlash(strings.TrimPrefix(p, "./"))
		info, err := e.Info()
		if err != nil {
			continue
		}
		item := UIFileEntry{
			Name:    name,
			Path:    p,
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
		}
		if e.IsDir() {
			dirs = append(dirs, item)
		} else {
			files = append(files, item)
		}
	}
	// Simple name sort without importing sort.Slice for minimal diff
	// We'll append in two passes with a naive insertion order by name
	// Since determinism is helpful, use a basic bubble-like pass
	for i := 0; i < len(dirs); i++ {
		for j := i + 1; j < len(dirs); j++ {
			if strings.ToLower(dirs[j].Name) < strings.ToLower(dirs[i].Name) {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
		}
	}
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if strings.ToLower(files[j].Name) < strings.ToLower(files[i].Name) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
	res.Entries = make([]UIFileEntry, 0, len(dirs)+len(files))
	res.Entries = append(res.Entries, dirs...)
	res.Entries = append(res.Entries, files...)
	return res
}

// UIReadFileResult is the response when reading a file for the UI viewer
type UIReadFileResult struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Lines    int    `json:"lines"`
	Language string `json:"language,omitempty"`
}

// ReadWorkspaceFile reads a file within the current workspace and returns its content.
func (a *App) ReadWorkspaceFile(relPath string) UIReadFileResult {
	out := UIReadFileResult{Path: "", Content: "", Lines: 0}
	if a.engine == nil {
		return out
	}
	root := strings.TrimSpace(a.engine.Workspace())
	if root == "" {
		return out
	}
	rel := strings.TrimSpace(relPath)
	if rel == "" || rel == "." || rel == "/" {
		return out
	}
	candidate := filepath.Clean(filepath.Join(root, rel))
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return out
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return out
	}
	if absCandidate != absRoot && !strings.HasPrefix(absCandidate+string(os.PathSeparator), absRoot+string(os.PathSeparator)) {
		return out
	}
	data, err := os.ReadFile(absCandidate)
	if err != nil {
		return out
	}
	content := string(data)
	// Count lines quickly
	lines := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			lines++
		}
	}
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lines++
	}
	out.Path = filepath.ToSlash(rel)
	out.Content = content
	out.Lines = lines
	// Simple language hint from extension
	out.Language = detectLanguageByExt(rel)
	return out
}

// detectLanguageByExt is a minimal helper for UI display purposes only
func detectLanguageByExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".js":
		return "javascript"
	case ".jsx":
		return "jsx"
	case ".json":
		return "json"
	case ".md":
		return "markdown"
	case ".css":
		return "css"
	case ".html":
		return "html"
	default:
		return ""
	}
}

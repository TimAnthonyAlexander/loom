package bridge

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	gitignore "github.com/go-git/go-git/v5/plumbing/format/gitignore"
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
	// cached gitignore matcher for current workspace
	gitMatcher gitignore.Matcher
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
	// Subscribe to frontend attachment updates so we don't rely on regenerated bindings
	runtime.EventsOn(a.ctx, "chat:set_attachments", func(optionalData ...interface{}) {
		if a.engine == nil {
			return
		}
		// optionalData may be [paths] or a varargs of strings; handle both
		var paths []string
		if len(optionalData) == 1 {
			if v, ok := optionalData[0].([]string); ok {
				paths = v
			} else if arr, ok := optionalData[0].([]interface{}); ok {
				for _, x := range arr {
					if s, ok := x.(string); ok {
						paths = append(paths, s)
					}
				}
			}
		} else if len(optionalData) > 1 {
			for _, x := range optionalData {
				if s, ok := x.(string); ok {
					paths = append(paths, s)
				}
			}
		}
		a.engine.SetAttachedFiles(paths)
	})
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

// SetAttachments receives a list of workspace-relative file paths from the UI
// and forwards them to the engine to be injected into the system prompt context.
func (a *App) SetAttachments(paths []string) {
	if a.engine == nil {
		return
	}
	a.engine.SetAttachedFiles(paths)
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

// EmitBilling sends a usage/cost event to the UI. Provider should be a short id like "openai" or "anthropic".
// Model is the provider's model id (e.g., "gpt-4.1", "claude-3-5-sonnet-20241022").
func (a *App) EmitBilling(provider string, model string, inTokens int64, outTokens int64, inUSD float64, outUSD float64, totalUSD float64) {
	if a.ctx != nil {
		payload := map[string]interface{}{
			"provider":   provider,
			"model":      model,
			"in_tokens":  inTokens,
			"out_tokens": outTokens,
			"in_usd":     inUSD,
			"out_usd":    outUSD,
			"total_usd":  totalUSD,
		}
		runtime.EventsEmit(a.ctx, "billing:usage", payload)
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

// GetUsage returns persisted usage aggregates for the current workspace.
func (a *App) GetUsage() map[string]interface{} {
	result := map[string]interface{}{
		"total_in_tokens":  0,
		"total_out_tokens": 0,
		"total_in_usd":     0.0,
		"total_out_usd":    0.0,
		"per_provider":     map[string]interface{}{},
		"per_model":        map[string]interface{}{},
	}
	if a.engine != nil {
		totals := a.engine.GetUsage()
		// Convert to plain maps for JS bridge
		perProv := map[string]interface{}{}
		for k, v := range totals.PerProvider {
			perProv[k] = map[string]interface{}{
				"inTokens":    v.InTokens,
				"outTokens":   v.OutTokens,
				"totalTokens": v.TotalTokens,
				"inUSD":       v.InUSD,
				"outUSD":      v.OutUSD,
				"totalUSD":    v.TotalUSD,
			}
		}
		perModel := map[string]interface{}{}
		for k, v := range totals.PerModel {
			perModel[k] = map[string]interface{}{
				"provider":    v.Provider,
				"inTokens":    v.InTokens,
				"outTokens":   v.OutTokens,
				"totalTokens": v.TotalTokens,
				"inUSD":       v.InUSD,
				"outUSD":      v.OutUSD,
				"totalUSD":    v.TotalUSD,
			}
		}
		result["total_in_tokens"] = totals.TotalInTokens
		result["total_out_tokens"] = totals.TotalOutTokens
		result["total_in_usd"] = totals.TotalInUSD
		result["total_out_usd"] = totals.TotalOutUSD
		result["per_provider"] = perProv
		result["per_model"] = perModel
	}
	return result
}

// ResetUsage clears persisted usage for the current workspace.
func (a *App) ResetUsage() {
	if a.engine == nil {
		return
	}
	_ = a.engine.ResetUsage()
}

// GetGlobalUsage returns the global usage aggregates stored under ~/.loom/usages/aggregates.json
func (a *App) GetGlobalUsage() map[string]interface{} {
	g := config.GetGlobalUsage()
	perProv := map[string]interface{}{}
	for k, v := range g.PerProvider {
		perProv[k] = map[string]interface{}{
			"inTokens":    v.InTokens,
			"outTokens":   v.OutTokens,
			"totalTokens": v.TotalTokens,
			"inUSD":       v.InUSD,
			"outUSD":      v.OutUSD,
			"totalUSD":    v.TotalUSD,
		}
	}
	perModel := map[string]interface{}{}
	for k, v := range g.PerModel {
		perModel[k] = map[string]interface{}{
			"provider":    v.Provider,
			"inTokens":    v.InTokens,
			"outTokens":   v.OutTokens,
			"totalTokens": v.TotalTokens,
			"inUSD":       v.InUSD,
			"outUSD":      v.OutUSD,
			"totalUSD":    v.TotalUSD,
		}
	}
	return map[string]interface{}{
		"total_in_tokens":  g.TotalInTokens,
		"total_out_tokens": g.TotalOutTokens,
		"total_in_usd":     g.TotalInUSD,
		"total_out_usd":    g.TotalOutUSD,
		"per_provider":     perProv,
		"per_model":        perModel,
	}
}

// ResetGlobalUsage clears global usage aggregates.
func (a *App) ResetGlobalUsage() {
	_ = config.ResetGlobalUsage()
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
		}
		idx := indexer.NewRipgrepIndexer(norm)
		if err := tool.RegisterSearchCode(newRegistry, idx); err != nil {
		}
		if err := tool.RegisterEditFile(newRegistry, norm); err != nil {
		}
		if err := tool.RegisterApplyEdit(newRegistry, norm); err != nil {
		}
		if err := tool.RegisterListDir(newRegistry, norm); err != nil {
		}
		if err := tool.RegisterFinalize(newRegistry); err != nil {
		}
		if err := tool.RegisterRunShell(newRegistry, norm); err != nil {
		}
		if err := tool.RegisterApplyShell(newRegistry, norm); err != nil {
		}
		if err := tool.RegisterHTTPRequest(newRegistry); err != nil {
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
	}
	// Load .gitignore matcher for this workspace
	a.gitMatcher = a.buildGitignoreMatcher(norm)
	// After switching, log current rules snapshot for debug
	config.LoadRules(path)
}

// buildGitignoreMatcher scans the workspace for .gitignore files and builds a matcher
func (a *App) buildGitignoreMatcher(root string) gitignore.Matcher {
	absRoot, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil || absRoot == "" {
		return nil
	}
	var patterns []gitignore.Pattern
	// Always include patterns from .git/info/exclude if present
	readGitignoreFile := func(baseDir, filePath string) {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return
		}
		lines := strings.Split(string(data), "\n")
		relBase, err := filepath.Rel(absRoot, baseDir)
		if err != nil {
			relBase = ""
		}
		relBase = filepath.ToSlash(relBase)
		for _, line := range lines {
			line = strings.TrimRight(line, "\r")
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			var baseSegs []string
			if relBase != "" {
				baseSegs = strings.Split(relBase, "/")
			}
			p := gitignore.ParsePattern(line, baseSegs)
			patterns = append(patterns, p)
		}
	}

	// Load top-level .gitignore
	top := filepath.Join(absRoot, ".gitignore")
	if _, err := os.Stat(top); err == nil {
		readGitignoreFile(absRoot, top)
	}
	// Load .git/info/exclude if present
	if infoExclude := filepath.Join(absRoot, ".git", "info", "exclude"); func() bool { _, err := os.Stat(infoExclude); return err == nil }() {
		readGitignoreFile(absRoot, infoExclude)
	}
	// Walk to find nested .gitignore files
	_ = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			// Skip .git directory entirely
			if name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if name == ".gitignore" {
			base := filepath.Dir(path)
			readGitignoreFile(base, path)
		}
		return nil
	})
	if len(patterns) == 0 {
		return nil
	}
	m := gitignore.NewMatcher(patterns)
	return m
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
		if err := config.SaveUserRules(userRules); err != nil {
		}
	}
	// Save project rules
	if projectRules, ok := payload["project"]; ok {
		wp := ""
		if a.engine != nil {
			wp = a.engine.Workspace()
		}
		if wp == "" {
		} else {
			if err := config.SaveProjectRules(wp, projectRules); err != nil {
			}
		}
	}
}

// OpenProjectDataDir opens the per-project data directory in the system file browser.
// Path format: $HOME/.loom/projects/<projectID>
func (a *App) OpenProjectDataDir() {
	if a.ctx == nil {
		return
	}
	// Resolve current workspace
	ws := ""
	if a.engine != nil {
		ws = strings.TrimSpace(a.engine.Workspace())
	}
	if ws == "" {
		return
	}
	// Normalize to absolute path
	absWS, err := filepath.Abs(ws)
	if err != nil {
		return
	}
	// Compute project ID (first 16 hex chars of sha256 of workspace path)
	sum := sha256.Sum256([]byte(absWS))
	projectID := hex.EncodeToString(sum[:])[:16]
	// Build ~/.loom/projects/<id>
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".loom", "projects", projectID)
	// Ensure directory exists so opening doesn't 404
	_ = os.MkdirAll(dir, 0o755)
	// Try to open the directory in the system file manager
	openers := [][]string{{"open", dir}, {"xdg-open", dir}, {"explorer", dir}}
	for _, cmd := range openers {
		c := exec.Command(cmd[0], cmd[1:]...)
		if err := c.Start(); err == nil {
			return
		}
	}
	// Fallback: open as file:// URL in the browser
	url := "file://" + filepath.ToSlash(dir)
	runtime.BrowserOpenURL(a.ctx, url)
}

// ChooseWorkspace opens a native directory picker and returns the selected path.
func (a *App) ChooseWorkspace() string {
	if a.ctx == nil {
		return ""
	}
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Workspace",
	})
	if err != nil {
		return ""
	}
	if dir == "" {
		return ""
	}
	return dir
}

// ChooseSaveFile opens a native save file dialog and returns the selected path relative to the current workspace.
// If the selected file is outside the workspace or on error, returns an empty string.
func (a *App) ChooseSaveFile(suggestedName string) string {
	if a.ctx == nil || a.engine == nil {
		return ""
	}
	root := strings.TrimSpace(a.engine.Workspace())
	if root == "" {
		return ""
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	// Use suggestedName if provided, otherwise blank
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                      "Save As",
		DefaultDirectory:           absRoot,
		DefaultFilename:            strings.TrimSpace(suggestedName),
		ShowHiddenFiles:            false,
		CanCreateDirectories:       true,
		TreatPackagesAsDirectories: false,
	})
	if err != nil || strings.TrimSpace(path) == "" {
		return ""
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	// Ensure within workspace
	if absPath != absRoot && !strings.HasPrefix(absPath+string(os.PathSeparator), absRoot+string(os.PathSeparator)) {
		// Disallow saving outside workspace for now
		return ""
	}
	// Manual overwrite confirmation for Wails v2.10
	if _, statErr := os.Stat(absPath); statErr == nil {
		resp, derr := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:          runtime.WarningDialog,
			Title:         "File exists",
			Message:       fmt.Sprintf("Overwrite '%s'?", filepath.Base(absPath)),
			Buttons:       []string{"Overwrite", "Cancel"},
			DefaultButton: "Overwrite",
			CancelButton:  "Cancel",
		})
		if derr != nil || resp != "Overwrite" {
			return ""
		}
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
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
		return
	}
	// Clear UI then replay messages
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "chat:clear")
	}
	msgs, err := a.engine.GetConversation(id)
	if err != nil {
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

// OpenFileInUI emits an event for the frontend to open the given file in the viewer
func (a *App) OpenFileInUI(path string) {
	if a.ctx != nil && strings.TrimSpace(path) != "" {
		runtime.EventsEmit(a.ctx, "workspace:open_file", map[string]string{"path": path})
	}
}

// UpdateEditorContext records the active editor file and cursor position from the UI.
// The path should be workspace-relative using forward slashes.
func (a *App) UpdateEditorContext(path string, line int, column int) {
	if a.engine == nil {
		return
	}
	p := filepath.ToSlash(strings.TrimSpace(path))
	a.engine.SetEditorContext(p, line, column)
}

// SearchCode searches for text within files in the current workspace optionally scoped by a file glob.
// Returns a list of matches with relative paths and line information. If engine/workspace not set, returns empty result.
func (a *App) SearchCode(query string, filePattern string, maxResults int) []map[string]interface{} {
	out := []map[string]interface{}{}
	if a.engine == nil {
		return out
	}
	root := strings.TrimSpace(a.engine.Workspace())
	if root == "" || strings.TrimSpace(query) == "" {
		return out
	}
	idx := indexer.NewRipgrepIndexer(root)
	res, err := idx.Search(query, filePattern, maxResults)
	if err != nil || res == nil {
		return out
	}
	for _, m := range res.Matches {
		out = append(out, map[string]interface{}{
			"path":        filepath.ToSlash(m.Path),
			"line_number": m.LineNum,
			"line_text":   m.LineText,
			"start_char":  m.StartChar,
			"end_char":    m.EndChar,
		})
	}
	return out
}

// FindFiles returns up to maxResults files under the optional subdir that match the provided pattern.
// The pattern uses filepath.Match semantics applied to both the base name and the workspace-relative path.
// Common noisy directories are skipped (node_modules, .git, vendor, dist, build, .loom).
func (a *App) FindFiles(filePattern string, subdir string, maxResults int) []string {
	var results []string
	if a.engine == nil {
		return results
	}
	root := strings.TrimSpace(a.engine.Workspace())
	if root == "" {
		return results
	}
	start := filepath.Clean(filepath.Join(root, strings.TrimSpace(subdir)))
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return results
	}
	absStart, err := filepath.Abs(start)
	if err != nil {
		return results
	}
	// Ensure start is within workspace
	if absStart != absRoot && !strings.HasPrefix(absStart+string(os.PathSeparator), absRoot+string(os.PathSeparator)) {
		absStart = absRoot
	}

	// Normalized ignore set
	ignoreDirs := map[string]struct{}{
		".git":         {},
		"node_modules": {},
		"vendor":       {},
		"dist":         {},
		"build":        {},
		".loom":        {},
	}

	add := func(rel string) bool {
		results = append(results, filepath.ToSlash(rel))
		if maxResults > 0 && len(results) >= maxResults {
			return true
		}
		return false
	}

	// Precompute normalized query variants for fuzzy matching
	trimmedPattern := strings.TrimSpace(filePattern)
	patternLower := strings.ToLower(trimmedPattern)
	patternFuzzy := normalizeForFuzzy(patternLower)

	// Helper to test pattern against base name and relative path with fuzzy rules
	matches := func(relPath string) bool {
		if trimmedPattern == "" {
			return true
		}
		base := filepath.Base(relPath)
		baseLower := strings.ToLower(base)
		relLower := strings.ToLower(relPath)

		// 1) glob-like exact matching against base and rel
		if ok, _ := filepath.Match(trimmedPattern, base); ok {
			return true
		}
		if ok, _ := filepath.Match(trimmedPattern, relPath); ok {
			return true
		}

		// 2) simple substring on lowercased base and path
		if patternLower != "" && (strings.Contains(baseLower, patternLower) || strings.Contains(relLower, patternLower)) {
			return true
		}

		// 3) fuzzy normalization: ignore spaces/punctuation and case
		if patternFuzzy == "" {
			return true
		}
		baseFuzzy := normalizeForFuzzy(baseLower)
		relFuzzy := normalizeForFuzzy(relLower)
		if strings.Contains(baseFuzzy, patternFuzzy) || strings.Contains(relFuzzy, patternFuzzy) {
			return true
		}

		// 4) subsequence match (characters in order) to tolerate small gaps/typos
		if isSubsequence(patternFuzzy, baseFuzzy) || isSubsequence(patternFuzzy, relFuzzy) {
			return true
		}
		return false
	}

	_ = filepath.WalkDir(absStart, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if _, skip := ignoreDirs[name]; skip {
				return filepath.SkipDir
			}
			return nil
		}
		// Only files
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if matches(rel) {
			if add(rel) {
				return filepath.SkipDir
			}
		}
		return nil
	})

	return results
}

// normalizeForFuzzy removes spaces and punctuation, keeping only letters and digits.
// It also collapses consecutive separators and leaves result in lower-case.
func normalizeForFuzzy(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			// keep alphanumerics
			b.WriteRune(unicode.ToLower(r))
		}
		// ignore everything else (spaces, punctuation, path separators, underscores, dashes, dots)
	}
	return b.String()
}

// isSubsequence returns true if small is a subsequence of big (chars in order, not necessarily contiguous)
func isSubsequence(small, big string) bool {
	if small == "" {
		return true
	}
	if big == "" {
		return false
	}
	i := 0
	for j := 0; j < len(big) && i < len(small); j++ {
		if small[i] == big[j] {
			i++
		}
	}
	return i == len(small)
}

// UIFileEntry represents a single file or directory for the UI explorer
type UIFileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"` // path relative to the workspace root
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size,omitempty"`
	ModTime string `json:"mod_time"`
	// Additional UI flags
	Ignored bool `json:"ignored,omitempty"`
	Hidden  bool `json:"hidden,omitempty"`
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
		p := filepath.Join(relDisplay, name)
		p = filepath.ToSlash(strings.TrimPrefix(p, "./"))
		info, err := e.Info()
		if err != nil {
			continue
		}
		// Determine flags
		isHidden := strings.HasPrefix(name, ".") && name != "." && name != ".."
		isIgnored := false
		if a.gitMatcher != nil {
			segments := strings.Split(filepath.ToSlash(p), "/")
			isIgnored = a.gitMatcher.Match(segments, e.IsDir())
		}
		item := UIFileEntry{
			Name:    name,
			Path:    p,
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
			Ignored: isIgnored,
			Hidden:  isHidden,
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
	Path      string `json:"path"`
	Content   string `json:"content"`
	Lines     int    `json:"lines"`
	Language  string `json:"language,omitempty"`
	ServerRev string `json:"serverRev,omitempty"`
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
	out.ServerRev = computeServerRev(data)
	// Update engine editor context to reflect the file currently opened in the UI
	if a.engine != nil {
		a.engine.SetEditorContext(out.Path, 1, 1)
	}
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

// computeServerRev returns a short content hash for optimistic concurrency and cache-busting
func computeServerRev(data []byte) string {
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}

// WriteWorkspaceFile writes content to a file within the current workspace.
// Payload: { path: string, content: string, serverRev?: string }
// Returns: { serverRev: string }
func (a *App) WriteWorkspaceFile(payload map[string]string) map[string]string {
	res := map[string]string{"serverRev": ""}
	if a.engine == nil {
		return res
	}
	root := strings.TrimSpace(a.engine.Workspace())
	if root == "" {
		return res
	}
	rel := strings.TrimSpace(payload["path"])
	if rel == "" || rel == "." || rel == "/" {
		return res
	}
	candidate := filepath.Clean(filepath.Join(root, rel))
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return res
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return res
	}
	if absCandidate != absRoot && !strings.HasPrefix(absCandidate+string(os.PathSeparator), absRoot+string(os.PathSeparator)) {
		return res
	}
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(absCandidate), 0o755); err != nil {
		return res
	}
	content := payload["content"]
	// Write file
	if err := os.WriteFile(absCandidate, []byte(content), 0o644); err != nil {
		return res
	}
	// Compute and return new serverRev
	res["serverRev"] = computeServerRev([]byte(content))
	return res
}

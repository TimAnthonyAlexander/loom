package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/loom/loom/internal/adapter"
	"github.com/loom/loom/internal/bridge"
	"github.com/loom/loom/internal/config"
	"github.com/loom/loom/internal/engine"
	"github.com/loom/loom/internal/indexer"
	"github.com/loom/loom/internal/memory"
	"github.com/loom/loom/internal/symbols"
	"github.com/loom/loom/internal/tool"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var appCtx context.Context

//go:embed frontend/dist
var assets embed.FS

func main() {
	// Set up logging to show all levels
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Get current working directory as default workspace path
	workspacePath, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Create LLM adapter using factory
	configAdapter := adapter.DefaultConfig()

	// Load persisted settings and prefer them over env for API keys
	settings, err := config.Load()
	if err != nil {
		log.Printf("Warning: Failed to load settings: %v", err)
	}
	// Prefer last workspace from settings if present (normalize to abs path and expand ~)
	if settings.LastWorkspace != "" {
		workspacePath = normalizeWorkspacePath(settings.LastWorkspace)
	} else {
		workspacePath = normalizeWorkspacePath(workspacePath)
	}
	if settings.OpenAIAPIKey != "" && configAdapter.Provider == adapter.ProviderOpenAI {
		configAdapter.APIKey = settings.OpenAIAPIKey
	}
	if settings.AnthropicAPIKey != "" && configAdapter.Provider == adapter.ProviderAnthropic {
		configAdapter.APIKey = settings.AnthropicAPIKey
	}
	if settings.OllamaEndpoint != "" && configAdapter.Provider == adapter.ProviderOllama {
		configAdapter.Endpoint = settings.OllamaEndpoint
	}

	// If a last selected model exists, prefer it at startup
	if settings.LastModel != "" {
		if prov, modelID, err := adapter.GetProviderFromModel(settings.LastModel); err == nil {
			configAdapter.Provider = prov
			configAdapter.Model = modelID
		}
	}

	llm, err := adapter.New(configAdapter)
	if err != nil {
		log.Printf("Warning: Failed to initialize LLM adapter: %v", err)
		llm = nil
	}

	// Initialize memory store
	store, err := memory.NewStore("")
	if err != nil {
		log.Printf("Warning: Failed to initialize memory store: %v", err)
	}

	// Create project memory
	var projectMemory *memory.Project
	if store != nil {
		projectMemory, err = memory.NewProject(store, workspacePath)
		if err != nil {
			log.Printf("Warning: Failed to initialize project memory: %v", err)
		}
	}

	// Create a new tool registry AFTER final workspace is resolved
	registry := tool.NewRegistry()
	// Register core tools for the resolved workspace
	registerTools(registry, workspacePath)

	// Initialize symbols service and start indexing
	symsvc, err := symbols.NewService(workspacePath)
	if err != nil {
		log.Printf("Warning: symbols service init failed: %v", err)
	} else {
		go func() {
			if err := symsvc.StartIndexing(context.Background()); err != nil {
				log.Printf("symbols indexing error: %v", err)
			}
		}()
		// Register symbol tools
		if err := tool.RegisterSymbols(registry, symsvc); err != nil {
			log.Printf("Failed to register symbols tools: %v", err)
		}
	}

	// Create the engine and configure it
	eng := engine.New(llm, nil)
	eng.WithRegistry(registry)
	// Seed initial model label from startup config
	if configAdapter.Provider != "" && configAdapter.Model != "" {
		eng.SetModelLabel(string(configAdapter.Provider) + ":" + configAdapter.Model)
	}

	// Add memory if available
	if projectMemory != nil {
		eng.WithMemory(projectMemory)
	}

	// Set workspace path
	eng.WithWorkspace(workspacePath)

	// Create the application
	app := bridge.NewApp()
	app.WithEngine(eng)
	app.WithTools(registry)
	app.WithConfig(configAdapter)
	app.WithSettings(settings)

	// Connect the engine to the bridge
	eng.SetBridge(app)

	// Centralize MCP setup via the bridge so we don't double-start servers later
	// This will rebuild the registry (core + MCP) and wire it into the engine.
	app.SetWorkspace(workspacePath)

	// Run the application
	// Build the application menu
	m := menu.NewMenu()
	// App menu (About/Hide/Quit on macOS)
	m.Append(menu.AppMenu())
	// File menu first
	fileMenu := m.AddSubmenu("File")
	fileMenu.AddText("New Conversation", keys.CmdOrCtrl("N"), func(_ *menu.CallbackData) {
		if appCtx == nil {
			return
		}
		runtime.EventsEmit(appCtx, "menu:file:new_conversation")
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Open Workspace…", keys.CmdOrCtrl("O"), func(_ *menu.CallbackData) {
		if appCtx == nil {
			return
		}
		// Let the frontend open its workspace dialog to handle full UI refresh logic
		runtime.EventsEmit(appCtx, "menu:file:open_workspace")
	})
	openFileAccel, _ := keys.Parse("CmdOrCtrl+Shift+O")
	fileMenu.AddText("Open File…", openFileAccel, func(_ *menu.CallbackData) {
		if appCtx == nil {
			return
		}
		// Native picker, then instruct UI to open the file
		path, err := runtime.OpenFileDialog(appCtx, runtime.OpenDialogOptions{Title: "Open File"})
		if err == nil && strings.TrimSpace(path) != "" {
			// Reuse existing UI handler for opening files
			runtime.EventsEmit(appCtx, "workspace:open_file", map[string]string{"path": path})
		}
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Save", keys.CmdOrCtrl("S"), func(_ *menu.CallbackData) {
		if appCtx == nil {
			return
		}
		runtime.EventsEmit(appCtx, "menu:file:save")
	})
	saveAsAccel, _ := keys.Parse("CmdOrCtrl+Shift+S")
	fileMenu.AddText("Save As…", saveAsAccel, func(_ *menu.CallbackData) {
		if appCtx == nil {
			return
		}
		runtime.EventsEmit(appCtx, "menu:file:save_as")
	})
	fileMenu.AddText("Close Tab", keys.CmdOrCtrl("W"), func(_ *menu.CallbackData) {
		if appCtx == nil {
			return
		}
		runtime.EventsEmit(appCtx, "menu:file:close_tab")
	})
	// Edit menu (Undo/Redo/Cut/Copy/Paste)
	m.Append(menu.EditMenu())

	if err := wails.Run(&options.App{
		Title:            "Loom - by Tim Anthony Alexander",
		Width:            1280,
		Height:           800,
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		OnStartup: func(ctx context.Context) {
			// Set the Wails context in the app
			app.WithContext(ctx)
			// Wire registry UI bridge now that context exists
			registry.WithUI(app)
			appCtx = ctx
		},
		Bind: []interface{}{
			app,
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Menu: m,
		// Platform-specific options
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
		},
	}); err != nil {
		log.Fatal(err)
	}
}

// registerTools registers all available tools with the registry.
func registerTools(registry *tool.Registry, workspacePath string) {

	// Create indexer
	idx := indexer.NewRipgrepIndexer(workspacePath)

	// Register tools
	if err := tool.RegisterReadFile(registry, workspacePath); err != nil {
		log.Printf("Failed to register read_file tool: %v", err)
	}

	if err := tool.RegisterSearchCode(registry, idx); err != nil {
		log.Printf("Failed to register search_code tool: %v", err)
	}

	if err := tool.RegisterEditFile(registry, workspacePath); err != nil {
		log.Printf("Failed to register edit_file tool: %v", err)
	}

	if err := tool.RegisterApplyEdit(registry, workspacePath); err != nil {
		log.Printf("Failed to register apply_edit tool: %v", err)
	}

	if err := tool.RegisterListDir(registry, workspacePath); err != nil {
		log.Printf("Failed to register list_dir tool: %v", err)
	}

	if err := tool.RegisterFinalize(registry); err != nil {
		log.Printf("Failed to register finalize tool: %v", err)
	}

	// Shell tools
	if err := tool.RegisterRunShell(registry, workspacePath); err != nil {
		log.Printf("Failed to register run_shell tool: %v", err)
	}
	if err := tool.RegisterApplyShell(registry, workspacePath); err != nil {
		log.Printf("Failed to register apply_shell tool: %v", err)
	}

	// HTTP request tool
	if err := tool.RegisterHTTPRequest(registry); err != nil {
		log.Printf("Failed to register http_request tool: %v", err)
	}

	// Memories tool (user-scoped)
	if err := tool.RegisterMemories(registry); err != nil {
		log.Printf("Failed to register memories tool: %v", err)
	}
}

// (removed) sanitizeToolName: use tool.SanitizeToolName directly where needed

// normalizeWorkspacePath expands a leading ~ and returns a cleaned absolute path
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

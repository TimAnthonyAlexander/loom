package main

import (
	"context"
	"embed"
	"log"
	"os"

	"github.com/loom/loom/internal/adapter"
	"github.com/loom/loom/internal/adapter/openai"
	"github.com/loom/loom/internal/bridge"
	"github.com/loom/loom/internal/engine"
	"github.com/loom/loom/internal/indexer"
	"github.com/loom/loom/internal/memory"
	"github.com/loom/loom/internal/tool"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

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

	// Create a new tool registry
	registry := tool.NewRegistry()

	// Register basic tools (would be expanded later)
	registerTools(registry, workspacePath)

	// Create LLM adapter using factory
	config := adapter.DefaultConfig()
	llm, err := adapter.New(config)
	if err != nil {
		log.Printf("Warning: Failed to initialize LLM adapter: %v", err)
		log.Printf("Using OpenAI as fallback")
		// Fallback to OpenAI if no other provider configured
		llm = openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4o")
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

	// Create the engine and configure it
	eng := engine.New(llm, nil)
	eng.WithRegistry(registry)

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
	app.WithConfig(config)

	// Connect the engine to the bridge
	eng.SetBridge(app)

	// Run the application
	if err := wails.Run(&options.App{
		Title:            "Loom v2",
		Width:            1280,
		Height:           800,
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		OnStartup: func(ctx context.Context) {
			// Set the Wails context in the app
			app.WithContext(ctx)
		},
		Bind: []interface{}{
			app,
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
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
}

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
	"github.com/loom/loom/internal/tool"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create a new tool registry
	registry := tool.NewRegistry()

	// Register basic tools (would be expanded later)
	registerTools(registry)

	// Create LLM adapter using factory
	config := adapter.DefaultConfig()
	llm, err := adapter.New(config)
	if err != nil {
		log.Printf("Warning: Failed to initialize LLM adapter: %v", err)
		log.Printf("Using OpenAI as fallback")
		// Fallback to OpenAI if no other provider configured
		llm = openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4o")
	}

	// Create the application
	app := bridge.NewApp(engine.New(llm, nil), registry)

	// Run the application
	if err := wails.Run(&options.App{
		Title:            "Loom v2",
		Width:            1280,
		Height:           800,
		Assets:           assets,
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		OnStartup: func(ctx context.Context) {
			app.SetContext(ctx)
		},
		Bind: []interface{}{
			app,
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
	}); err != nil {
		log.Fatal(err)
	}
}

// registerTools registers all available tools with the registry.
func registerTools(registry *tool.Registry) {
	// Get current working directory as default workspace path
	workspacePath, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

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
}

package main

import (
	"embed"
	"fmt"
	"loom/config"
	"loom/indexer"
	"loom/workspace"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Detect workspace
	workspacePath, err := workspace.DetectWorkspace()
	if err != nil {
		fmt.Printf("Error detecting workspace: %v\n", err)
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.LoadConfig(workspacePath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize or load index
	idx, err := indexer.LoadFromCache(workspacePath, cfg.MaxFileSize)
	if err != nil {
		// Create new index if cache doesn't exist or is invalid
		fmt.Println("Building workspace index...")
		idx = indexer.NewIndex(workspacePath, cfg.MaxFileSize)
		err = idx.BuildIndex()
		if err != nil {
			fmt.Printf("Error building index: %v\n", err)
			os.Exit(1)
		}

		// Save to cache
		err = idx.SaveToCache()
		if err != nil {
			fmt.Printf("Error saving index cache: %v\n", err)
			// Continue anyway
		}
	}

	// Start file watching
	err = idx.StartWatching()
	if err != nil {
		fmt.Printf("Warning: Could not start file watching: %v\n", err)
		// Continue anyway
	}
	defer idx.StopWatching()

	// Create an instance of the app structure
	app := NewApp(workspacePath, cfg, idx)

	// Create application with options
	err = wails.Run(&options.App{
		Title:     "Loom - AI Coding Assistant",
		Width:     1400,
		Height:    900,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 250, G: 250, B: 250, A: 1}, // Light background
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

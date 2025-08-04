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
	// For macOS app bundles, try to use a more reasonable default workspace
	// if detection fails, instead of panicking
	workspacePath, err := workspace.DetectWorkspace()
	if err != nil {
		// If workspace detection fails (common when launched via .app bundle),
		// default to user's home directory to avoid indexing the entire filesystem
		homeDir, homeErr := os.UserHomeDir()
		if homeErr != nil {
			fmt.Printf("Error detecting workspace: %v\nError getting home directory: %v\n", err, homeErr)
			os.Exit(1)
		}
		workspacePath = homeDir
		fmt.Printf("Warning: Could not detect workspace (%v), using home directory: %s\n", err, workspacePath)
	}

	// Safety check: Don't index from filesystem root or system directories
	// This prevents CPU issues when the app is launched incorrectly
	if workspacePath == "/" || workspacePath == "/System" || workspacePath == "/usr" {
		homeDir, homeErr := os.UserHomeDir()
		if homeErr != nil {
			fmt.Printf("Error getting home directory: %v\n", homeErr)
			os.Exit(1)
		}
		workspacePath = homeDir
		fmt.Printf("Warning: Prevented indexing from system directory, using home directory: %s\n", workspacePath)
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
		fmt.Printf("Building workspace index for: %s...\n", workspacePath)
		idx = indexer.NewIndex(workspacePath, cfg.MaxFileSize)

		// For safety, don't build index if it looks like we're in a problematic directory
		// This prevents excessive CPU usage on app launch issues
		if workspacePath == "/" || len(workspacePath) < 3 {
			fmt.Printf("Warning: Skipping index build for potentially problematic directory: %s\n", workspacePath)
		} else {
			err = idx.BuildIndex()
			if err != nil {
				fmt.Printf("Warning: Error building index: %v\n", err)
				// Don't exit - create empty index instead
				idx = indexer.NewIndex(workspacePath, cfg.MaxFileSize)
			} else {
				// Save to cache only if build was successful
				err = idx.SaveToCache()
				if err != nil {
					fmt.Printf("Warning: Error saving index cache: %v\n", err)
					// Continue anyway
				}
			}
		}
	}

	// Start file watching (but be more cautious)
	if workspacePath != "/" && len(workspacePath) > 3 {
		err = idx.StartWatching()
		if err != nil {
			fmt.Printf("Warning: Could not start file watching: %v\n", err)
			// Continue anyway
		}
		defer idx.StopWatching()
	} else {
		fmt.Printf("Warning: Skipping file watching for potentially problematic directory: %s\n", workspacePath)
	}

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

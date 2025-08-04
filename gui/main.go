package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Start with minimal initialization - no workspace setup yet
	// The GUI will prompt user to select workspace first

	// Create an instance of the app structure with deferred initialization
	app := NewAppWithDeferredInit()

	// Create application with options
	err := wails.Run(&options.App{
		Title:            "Loom - AI Coding Assistant",
		Width:            1400,
		Height:           900,
		MinWidth:         800,
		MinHeight:        600,
		MaxWidth:         0, // 0 means no limit
		MaxHeight:        0, // 0 means no limit
		WindowStartState: options.Normal,
		Frameless:        false, // Ensure we have window frame/decorations
		DisableResize:    false, // Allow resizing
		Fullscreen:       false, // Start in windowed mode
		AlwaysOnTop:      false, // Don't stay on top
		Debug: options.Debug{
			OpenInspectorOnStartup: true, // Enable DevTools on startup
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Mac: &mac.Options{
			DisableZoom: false,
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

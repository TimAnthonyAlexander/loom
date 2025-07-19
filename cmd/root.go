package cmd

import (
	"fmt"
	"loom/config"
	"loom/indexer"
	"loom/tui"
	"loom/workspace"
	"os"

	"github.com/spf13/cobra"
)

var (
	continueSession bool
	sessionID       string
)

var rootCmd = &cobra.Command{
	Use:   "loom",
	Short: "Loom is a terminal-based, AI-driven coding assistant",
	Long: `Loom is a terminal-based, AI-driven coding assistant written in Go.
It runs inside any project folder and gives developers a conversational 
interface to modify and extend their codebase.`,
	Run: func(cmd *cobra.Command, args []string) {
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

		// Ensure .loom directory exists
		err = workspace.EnsureLoomDir(workspacePath)
		if err != nil {
			fmt.Printf("Error creating .loom directory: %v\n", err)
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

		// Start TUI with session options
		sessionOptions := tui.SessionOptions{
			ContinueLatest: continueSession,
			SessionID:      sessionID,
		}
		if err := tui.StartTUI(workspacePath, cfg, idx, sessionOptions); err != nil {
			fmt.Printf("Error starting TUI: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Configure command line flags
	rootCmd.Flags().BoolVarP(&continueSession, "continue", "c", false, "Continue from the latest chat session")
	rootCmd.Flags().StringVarP(&sessionID, "session", "s", "", "Continue from a specific session ID")
	
	// Add subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(indexCmd)
}

package cmd

import (
	"fmt"
	"loom/config"
	"loom/tui"
	"loom/workspace"
	"os"

	"github.com/spf13/cobra"
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

		// Start TUI
		if err := tui.StartTUI(workspacePath, cfg); err != nil {
			fmt.Printf("Error starting TUI: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(configCmd)
}

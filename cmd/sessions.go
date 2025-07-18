package cmd

import (
	"fmt"
	"loom/chat"
	"loom/workspace"
	"os"

	"github.com/spf13/cobra"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List available chat sessions",
	Long:  `List all available chat sessions that can be continued with --session flag`,
	Run: func(cmd *cobra.Command, args []string) {
		// Detect workspace
		workspacePath, err := workspace.DetectWorkspace()
		if err != nil {
			fmt.Printf("Error detecting workspace: %v\n", err)
			os.Exit(1)
		}

		// List available sessions
		sessions, err := chat.ListAvailableSessions(workspacePath)
		if err != nil {
			fmt.Printf("Error listing sessions: %v\n", err)
			os.Exit(1)
		}

		if len(sessions) == 0 {
			fmt.Println("No chat sessions found.")
			fmt.Println("Start a new session with: loom")
			return
		}

		fmt.Printf("Available chat sessions in %s:\n\n", workspacePath)
		for i, sessionID := range sessions {
			status := ""
			if i == 0 {
				status = " (latest)"
			}
			fmt.Printf("  %s%s\n", sessionID, status)
		}

		fmt.Printf("\nUsage:\n")
		fmt.Printf("  loom --continue       # Continue from latest session (%s)\n", sessions[0])
		fmt.Printf("  loom --session ID     # Continue from specific session\n")
		fmt.Printf("  loom                  # Start new session (default)\n")
	},
}

func init() {
	rootCmd.AddCommand(sessionsCmd)
}
